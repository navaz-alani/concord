package server

import (
	"fmt"
	"net"

	"github.com/navaz-alani/concord/packet"
	"github.com/navaz-alani/concord/throttle"
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
	pipeline  *DataPipeline
	pc        packet.PacketCreator
	targets   map[string][]TargetCallback
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
		addr:      addr,
		conn:      conn,
		th:        throttle.NewUDPThrottle(throttleRate, conn, rBuffSize),
		pipeline:  NewDataPipeline(),
		pc:        pc,
		targets:   make(map[string][]TargetCallback),
		send:      make(chan packet.Packet),
		dist:      make(chan writePacket),
		shutdown:  make(chan string),
		done:      make(chan bool),
		rBuffSize: rBuffSize,
	}
	return svr, nil
}

func (svr *UDPServer) AddTarget(name string, cb TargetCallback) {
	svr.targets[name] = append(svr.targets[name], cb)
}

func (svr *UDPServer) Serve() error {
	// begin read/write routines
	go svr.sendPkts()
	go svr.writePkts()
	go svr.readPkts()
	msg := <-svr.shutdown // wait for shutdown signal
	svr.done <- true      // kill pktDist routine
	svr.done <- true      // kill write routine
	return fmt.Errorf(msg)
}

func (svr *UDPServer) fmtMsg(msg string) string {
	return fmt.Sprintf("[UDPServer@%s] %s", svr.addr.String(), msg)
}

func (svr *UDPServer) processPkt(data []byte, senderAddr net.Addr) {
	// pre-processing data buffer
	var err error
	if data, err = svr.pipeline.Process("_in_", data); err != nil {
		svr.send <- svr.pc.NewErrPkt("", senderAddr.String(), "pipeline error: "+err.Error())
	}

	pkt := svr.pc.NewPkt("", "")
	if err := pkt.Unmarshal(data); err != nil { // decode packet
		svr.send <- svr.pc.NewErrPkt("", senderAddr.String(), "malformed packet")
		return
	} else if cbq, ok := svr.targets[pkt.Meta().Get(packet.KeyTarget)]; !ok { // lookup packet target
		svr.send <- svr.pc.NewErrPkt(pkt.Meta().Get(packet.KeyRef), senderAddr.String(), "non-existent target")
		return
	} else {
		// execute packet target callback queue
		ref := pkt.Meta().Get(packet.KeyRef)
		resp := svr.pc.NewPkt(ref, senderAddr.String())
		ctx := &ServerCtx{
			Pkt:  pkt,
			From: senderAddr.String(),
		}
		// execute callback queue
		for _, cb := range cbq {
			if ctx.Stat != 0 {
				break
			}
			cb(ctx, resp.Writer())
		}
		resp.Writer().Close()
		// if processing was successful on application side, send response
		if ctx.Stat == 0 {
			svr.send <- resp // send response
		} else {
			svr.send <- svr.pc.NewErrPkt(ref, senderAddr.String(), ctx.Msg) // notify error
		}
	}
}

// read is a routune which reads and decodes packets from the underlying
// connection and spawns a routine to process each packet read.
func (svr *UDPServer) readPkts() {
	for {
		if data, senderAddr, err := svr.th.ReadFrom(); err != nil {
			// send shutdown signal to end write routine
			svr.shutdown <- svr.fmtMsg("read fail - connection error")
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
					if bin, err := svr.pipeline.Process("_out_", bin); err != nil {
						svr.send <- svr.pc.NewErrPkt("", pkt.Dest(), "pipeline error: "+err.Error())
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
