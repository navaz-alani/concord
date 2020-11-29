package server

import (
	"fmt"
	"net"

	"github.com/navaz-alani/concord/internal"
	throttle "github.com/navaz-alani/concord/internal/throttle"
	"github.com/navaz-alani/concord/packet"
)

type writePacket struct {
	data []byte
	addr *net.UDPAddr
}

// UDPServer is an implementation of the Server type. As suggested by the name,
// it uses UDP for underlying Packet transfer in the transport layer. It is also
// concurrent, by default - each incoming packet is processed in its own
// go-routine.
type UDPServer struct {
	addr      *net.UDPAddr
	conn      *net.UDPConn
	th        throttle.Throttle
	pipelines struct {
		data   *internal.DataPipeline
		packet *internal.PacketPipeline
	}
	pc        packet.PacketCreator
	send      chan packet.Packet
	dist      chan writePacket
	shutdown  chan string
	done      chan bool
	rBuffSize int
}

func NewUDPServer(addr *net.UDPAddr, rBuffSize int, pc packet.PacketCreator,
	throttleRate throttle.Rate) (*UDPServer, error) {
	// initialize connection
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	svr := &UDPServer{
		addr: addr,
		conn: conn,
		th:   throttle.NewUDPThrottle(throttleRate, conn, rBuffSize),
		pipelines: struct {
			data   *internal.DataPipeline
			packet *internal.PacketPipeline
		}{
			data:   internal.NewDataPipeline(),
			packet: internal.NewPacketPipeline(),
		},
		pc:        pc,
		send:      make(chan packet.Packet),
		dist:      make(chan writePacket),
		shutdown:  make(chan string),
		done:      make(chan bool),
		rBuffSize: rBuffSize,
	}
	svr.pipelines.packet.AddCallback(TargetRelay, svr.relayCallback)
	return svr, nil
}

func (svr *UDPServer) DataProcessor() internal.DataProcessor {
	return svr.pipelines.data
}

func (svr *UDPServer) PacketProcessor() internal.PacketProcessor {
	return svr.pipelines.packet
}

func (svr *UDPServer) Serve() error {
	svr.pipelines.data.Lock()
	// begin read/write routines
	go svr.sendPkts()
	go svr.writePkts()
	go svr.readPkts()
	msg := <-svr.shutdown // wait for shutdown signal
	svr.done <- true      // kill pktDist routine
	svr.done <- true      // kill write routine
	return fmt.Errorf(msg)
}

// relayCallback implements packet forwarding
func (svr *UDPServer) relayCallback(ctx *internal.TargetCtx, pw packet.Writer) {
	ref := ctx.Pkt.Meta().Get(packet.KeyRef)
	relayAddr := ctx.Pkt.Meta().Get(KeyRelayTo)
	// create a new packet to be forwarded and send it
	fwdPkt := svr.pc.NewPkt(ref, relayAddr)
	fwdPkt.Meta().Add(KeyRelayFrom, ctx.From)
	fwdPkt.Writer().Write(ctx.Pkt.Data())
	fwdPkt.Writer().Close()
	svr.send <- fwdPkt
	// can stop processing of packet here, no more actions needed
	ctx.Stat = internal.CodeStopNoop
	ctx.Msg = "packet forwarded"
}

func (svr *UDPServer) fmtMsg(msg string) string {
	return fmt.Sprintf("[UDPServer@%s] %s", svr.addr.String(), msg)
}

func (svr *UDPServer) processPkt(data []byte, senderAddr net.Addr) {
	// pre-processing data buffer
	var err error
	transformCtx := &internal.TransformContext{
		PipelineName: "_in_",
		From:         senderAddr.String(),
	}
	if data, err = svr.pipelines.data.Process(transformCtx, data); err != nil {
		svr.send <- svr.pc.NewErrPkt("", senderAddr.String(), "data pipeline error: "+err.Error())
		return
	} else if transformCtx.Stat == internal.CodeStopNoop {
		return
	}

	pkt := svr.pc.NewPkt("", "")
	if err := pkt.Unmarshal(data); err != nil { // decode packet
		svr.send <- svr.pc.NewErrPkt("", senderAddr.String(), "malformed packet")
		return
	}
	// execute packet target callback queue
	ref := pkt.Meta().Get(packet.KeyRef)
	resp := svr.pc.NewPkt(ref, senderAddr.String())
	ctx := &internal.TargetCtx{
		TargetName: pkt.Meta().Get(packet.KeyTarget),
		Pkt:        pkt,
		From:       senderAddr.String(),
	}
	// execute callback queue
	if err := svr.pipelines.packet.Process(ctx, resp.Writer()); err != nil {
		svr.send <- svr.pc.NewErrPkt(ref, senderAddr.String(), "packet pipeline error: "+err.Error())
	} else if ctx.Stat != internal.CodeStopNoop {
		resp.Writer().Close()
		svr.send <- resp
	}
}

// read is a routune which reads and decodes packets from the underlying
// connection and spawns a routine to process each packet read.
func (svr *UDPServer) readPkts() {
	for {
		if data, senderAddr, err := svr.th.ReadFrom(); err != nil {
			// send shutdown signal to end write routine
			svr.shutdown <- svr.fmtMsg("read fail - connection error")
			break
		} else {
			go svr.processPkt(data, senderAddr)
		}
	}
}

// write is a routine which distributes packets by writing them over the
// underlying UDP connection. Any encoding/write errors are ignored.
func (svr *UDPServer) writePkts() {
	var pkt writePacket
	for {
		select {
		case pkt = <-svr.dist:
			svr.th.WriteTo(pkt.data, pkt.addr)
		case <-svr.done: // server is done, break out
			break
		}
	}
}

func (svr *UDPServer) sendPkts() {
	var pkt packet.Packet
	for {
		select {
		case pkt = <-svr.send:
			if bin, err := pkt.Marshal(); err == nil {
				if addr, err := net.ResolveUDPAddr("udp", pkt.Dest()); err == nil {
					// pre-processing data buffer
					transformCtx := &internal.TransformContext{
						PipelineName: "_out_",
						Dest:         pkt.Dest(),
					}
					if bin, err := svr.pipelines.data.Process(transformCtx, bin); err != nil {
						svr.send <- svr.pc.NewErrPkt(pkt.Meta().Get(packet.KeyRef),
							pkt.Dest(), "pipeline error: "+err.Error())
					} else if transformCtx.Stat == internal.CodeStopNoop {
					} else {
						svr.dist <- writePacket{
							data: bin,
							addr: addr,
						}
					}
				}
			}
		case <-svr.done: // server is done, break out
			break
		}
	}
}
