package server

import (
	"fmt"
	"net"

	"github.com/navaz-alani/voip/packet"
	"github.com/navaz-alani/voip/throttle"
)

// UDPServer is an implementation of the Server type. As suggested by the name,
// it uses UDP for underlying Packet transfer in the transport layer.
type UDPServer struct {
	addr         *net.UDPAddr
	conn         *net.UDPConn
	th           throttle.Throttle
	targets      map[string][]TargetCallback
	pc           packet.PacketCreator
	dist         chan packet.Packet
	shutdown     chan string
	done         chan bool
	readBuffSize int
	completed    int
}

func NewUDPServer(pc packet.PacketCreator, addr *net.UDPAddr, readBuffSize int) (*UDPServer, error) {
	// initialize connection
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	svr := &UDPServer{
		addr:         addr,
		conn:         conn,
		th:           throttle.NewUDPThrottle(throttle.Rate10k, conn, readBuffSize),
		targets:      make(map[string][]TargetCallback),
		pc:           pc,
		dist:         make(chan packet.Packet),
		shutdown:     make(chan string),
		done:         make(chan bool),
		readBuffSize: readBuffSize,
	}
	return svr, nil
}

func (svr *UDPServer) AddTarget(name string, cb TargetCallback) {
	svr.targets[name] = append(svr.targets[name], cb)
}

func (svr *UDPServer) Serve() error {
	// begin read/write routines
	go svr.read()
	go svr.write()
	msg := <-svr.shutdown // wait for shutdown signal
	svr.done <- true      // kill write routine
	return fmt.Errorf(msg)
}

func (svr *UDPServer) fmtMsg(msg string) string {
	return fmt.Sprintf("[UDPServer@%s] %s", svr.addr.String(), msg)
}

// read is a routune which reads and decodes packets from the underlying
// connection and executes their callback queues.
func (svr *UDPServer) read() {
	for {
		if data, senderAddr, err := svr.th.ReadFrom(); err != nil {
			// send shutdown signal to end write routine
			svr.shutdown <- svr.fmtMsg("read fail - connection error")
		} else {
			pkt := svr.pc.NewPkt("", "")
			if err := pkt.Unmarshal(data); err != nil {
				svr.dist <- svr.pc.NewErrPkt("", senderAddr.String(), "malformed packet")
			} else if cbq, ok := svr.targets[pkt.Meta().Get(packet.KeyTarget)]; !ok {
				svr.dist <- svr.pc.NewErrPkt(pkt.Meta().Get(packet.KeyRef), senderAddr.String(), "non-existent target")
			} else {
				go svr.execCallbackQueue(senderAddr.String(), cbq, pkt)
			}
		}
	}
}

func (svr *UDPServer) execCallbackQueue(senderAddr string, cbq []TargetCallback, pkt packet.Packet) {
	// prepare response and server context for callback queue exec
	ref := pkt.Meta().Get(packet.KeyRef)
	resp := svr.pc.NewPkt(ref, senderAddr)
	ctx := &ServerCtx{
		Pkt:  pkt,
		From: senderAddr,
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
		svr.dist <- resp // send response
	} else {
		svr.dist <- svr.pc.NewErrPkt(ref, senderAddr, ctx.Msg) // notify error
	}
}

// write is a routine which distributes packets by writing them over the
// underlying UDP connection. Any encoding/write errors are ignored.
func (svr *UDPServer) write() {
	var pkt packet.Packet
	for {
		select {
		case pkt = <-svr.dist:
			if bin, err := pkt.Marshal(); err == nil {
				if addr, err := net.ResolveUDPAddr("udp", pkt.Dest()); err == nil {
					if _, err = svr.th.WriteTo(bin, addr); err == nil {
						svr.completed++
					}
				}
			}
		case <-svr.done: // server is done, break out
			break
		}
	}
}
