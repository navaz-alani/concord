package server

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/navaz-alani/concord/core"
	"github.com/navaz-alani/concord/packet"
)

type TCPServer struct {
	addr        *net.TCPAddr
	listener    *net.TCPListener
	mu          *sync.RWMutex
	pipelines   pipelines
	rbuffSize   int
	connections map[*connection]bool
	pc          packet.PacketCreator
}

func NewTCPServer(laddr *net.TCPAddr, rbuffSize int, pc packet.PacketCreator) (*TCPServer, error) {
	listener, err := net.ListenTCP("tcp", laddr)
	if err != nil {
		return nil, fmt.Errorf("tcp listen err: %s", err.Error())
	}
	svr := &TCPServer{
		addr:     laddr,
		listener: listener,
		pipelines: pipelines{
			data:   core.NewDataPipeline(),
			packet: core.NewPacketPipeline(),
		},
		rbuffSize: rbuffSize,
		pc:        pc,
	}
	return svr, nil
}

func (svr *TCPServer) DataProcessor() core.DataProcessor {
	return svr.pipelines.data
}

func (svr *TCPServer) PacketProcessor() core.PacketProcessor {
	return svr.pipelines.packet
}

func (svr *TCPServer) Serve() error {
	var tempDelay time.Duration // how long to sleep on accept failure
	for {
		if tcpConn, err := svr.listener.AcceptTCP(); err != nil {
			if err != nil {
				// tcp error accept err handling - taken from net.Server.Serve
				if ne, ok := err.(net.Error); ok && ne.Temporary() {
					if tempDelay == 0 {
						tempDelay = 5 * time.Millisecond
					} else {
						tempDelay *= 2
					}
					if max := 1 * time.Second; tempDelay > max {
						tempDelay = max
					}
					time.Sleep(tempDelay)
					continue
				}
				return err
			}
			// create new connection and start read/write routines
			conn := &connection{
				TCPServer:   svr,
				TCPConn:     tcpConn,
				done:        make(chan struct{}),
				sendStream:  make(chan packet.Packet),
				writeStream: make(chan *writePacket),
			}
			go conn.serve()
		}
	}
}

func (svr *TCPServer) registerConnection(c *connection) {
	svr.mu.Lock()
	defer svr.mu.Unlock()
	svr.connections[c] = true
}

func (svr *TCPServer) unregisterConnection(c *connection) {
	svr.mu.Lock()
	defer svr.mu.Unlock()
	delete(svr.connections, c)
}

type connection struct {
	*TCPServer
	*net.TCPConn
	mu          *sync.RWMutex
	done        chan struct{}
	sendStream  chan packet.Packet
	writeStream chan *writePacket
}

func (c *connection) send() chan<- packet.Packet {
	return c.sendStream
}

func (c *connection) write() chan<- *writePacket {
	return c.writeStream
}

func (c *connection) serve() {
	c.TCPServer.registerConnection(c)
	defer c.TCPServer.unregisterConnection(c)
	go c.writeConn()
	go c.readConn()
	// this routine can only receive from the done channel
	<-(<-chan struct{})(c.done)
}

func (c *connection) sendPkt() {
	// this routine can only receive from the done channel
	done := (<-chan struct{})(c.done)
	for {
		select {
		case <-done:
			return
		case pkt := <-c.sendStream:
			go c.processOutgoing(pkt)
		}
	}
}

func (c *connection) writeConn() {
	// this routine can only receive from the done channel
	done := (<-chan struct{})(c.done)
	for {
		select {
		case <-done:
			return
		case wpkt := <-c.writeStream:
			if _, err := c.TCPConn.Write(wpkt.data); err != nil {
				// TODO: handle error
			}
		}
	}
}

func (c *connection) readConn() {
	// this routine owns the done channel
	for {
		rbuff := make([]byte, c.rbuffSize)
		if _, err := c.TCPConn.Read(rbuff); err != nil {
			if err, ok := err.(net.Error); ok && err.Temporary() {
				// wait
				continue
			}
			close(c.done)
			return
		}
		go c.processIncoming(rbuff)
	}
}

func (c *connection) processOutgoing(pkt packet.Packet) {
	defer c.pc.PutBack(pkt)
	if bin, err := pkt.Marshal(); err == nil {
		// pre-processing data buffer
		transformCtx := &core.TransformContext{
			PipelineCtx: core.PipelineCtx{
				Pkt: pkt,
			},
			PipelineName: "_out_",
		}
		if bin, err := c.pipelines.data.Process(transformCtx, bin); err != nil {
			c.send() <- c.pc.NewErrPkt(pkt.Meta().Get(packet.KeyRef),
				pkt.Dest(), "response data pipeline error: "+err.Error())
		} else if transformCtx.Stat != core.CodeStopNoop {
			c.write() <- &writePacket{
				data: bin,
			}
		}
	}
}

func (c *connection) processIncoming(data []byte) {
	var err error
	transformCtx := &core.TransformContext{
		PipelineName: "_in_",
		From:         c.addr.String(),
	}
	if data, err = c.pipelines.data.Process(transformCtx, data); err != nil {
		c.send() <- c.pc.NewErrPkt("", c.addr.String(), "data pipeline error: "+err.Error())
		return
	} else if transformCtx.Stat == core.CodeStopNoop {
		return
	}
	// obtain intermediate packet to decode `data`
	pkt := c.pc.NewPkt("", "")
	defer c.pc.PutBack(pkt)
	if err := pkt.Unmarshal(data); err != nil { // decode packet
		c.send() <- c.pc.NewErrPkt("", c.addr.String(), "malformed packet")
		return
	}
	// execute packet target callback queue
	ref := pkt.Meta().Get(packet.KeyRef)
	resp := c.TCPServer.pc.NewPkt(ref, c.addr.String())
	ctx := &core.TargetCtx{
		PipelineCtx: core.PipelineCtx{
			Pkt: pkt,
		},
		TargetName: pkt.Meta().Get(packet.KeyTarget),
		From:       c.addr.String(),
	}
	// execute callback queue
	if err := c.pipelines.packet.Process(ctx, resp.Writer()); err != nil {
		c.TCPServer.pc.PutBack(resp)
		c.send() <- c.pc.NewErrPkt(ref, c.addr.String(), "packet pipeline error: "+err.Error())
	} else if ctx.Stat == core.CodeStopNoop {
		c.TCPServer.pc.PutBack(resp)
	} else {
		resp.Writer().Close()
		c.send() <- resp
	}
}
