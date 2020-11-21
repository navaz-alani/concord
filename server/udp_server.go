package server

import (
	"fmt"
	"net"

	"github.com/navaz-alani/voip/packet"
)

// UDPServer is an implementation of the Server type. As suggested by the name,
// it uses UDP for underlying Packet transfer in the transport layer.
type UDPServer struct {
	addr         *net.UDPAddr
	targets      map[string][]TargetCallback
	pc           packet.PacketCreator
	dist         chan packet.Packet
	shutdown     chan string
	done         chan bool
	readBuffSize int
}

func NewUDPServer(pc packet.PacketCreator, addr *net.UDPAddr, readBuffSize int) *UDPServer {
	return &UDPServer{
		addr:         addr,
		targets:      make(map[string][]TargetCallback),
		pc:           pc,
		dist:         make(chan packet.Packet),
		shutdown:     make(chan string),
		done:         make(chan bool),
		readBuffSize: readBuffSize,
	}
}

func (svr *UDPServer) AddTarget(name string, cb TargetCallback) {
	svr.targets[name] = append(svr.targets[name], cb)
}

func (svr *UDPServer) Serve() error {
	// initialize connection
	conn, err := net.ListenUDP("udp", svr.addr)
	if err != nil {
		return err
	}
	// begin read/write routines
	go svr.read(conn)
	go svr.write(conn)
	msg := <-svr.shutdown // wait for shutdown signal
	svr.done <- true      // kill write routine
	return fmt.Errorf(msg)
}

func (svr *UDPServer) fmtMsg(msg string) string {
	return fmt.Sprintf("[UDPServer@%s] %s", svr.addr.String(), msg)
}

// read is a routune which reads and decodes packets from the underlying
// connection and executes their callback queues.
func (svr *UDPServer) read(conn *net.UDPConn) {
	readBuff := make([]byte, svr.readBuffSize)
	for {
		if n, senderAddr, err := conn.ReadFromUDP(readBuff); err != nil {
			// send shutdown signal to end write routine
			svr.shutdown <- svr.fmtMsg("read fail - connection error")
		} else {
			pkt := svr.pc.NewPkt("")
			if err := pkt.Unmarshal(readBuff[:n]); err != nil {
				fmt.Println("malformed packet")
				svr.dist <- svr.pc.NewErrPkt(senderAddr.String(), "malformed packet")
			} else if cbq, ok := svr.targets[pkt.Target()]; !ok {
				fmt.Println("non-existent target", pkt.Target())
				svr.dist <- svr.pc.NewErrPkt(senderAddr.String(), "non-existent target")
			} else {
				fmt.Println("got packet")
				go svr.execCallbackQueue(senderAddr.String(), cbq, pkt)
			}
		}
	}
}

func (svr *UDPServer) execCallbackQueue(senderAddr string, cbq []TargetCallback, pkt packet.Packet) {
	// prepare response and server callback execution context
	resp := svr.pc.NewPkt(senderAddr)
	ctx := &ServerCtx{
		Pkt:  pkt,
		From: senderAddr,
	}
	// execute callback queue
	for _, cb := range cbq {
		if ctx.Stat == -1 {
			break
		}
		cb(ctx, resp.Writer())
	}
	// if processing was successful on application side, send response
	if !(ctx.Stat == -1) {
		svr.dist <- resp // send response
	} else {
		svr.dist <- svr.pc.NewErrPkt(senderAddr, ctx.Msg) // notify error
	}
}

// write is a routine which distributes packets by writing them over the
// underlying UDP connection. Any encoding/write errors are ignored.
func (svr *UDPServer) write(conn *net.UDPConn) {
	var pkt packet.Packet
	for {
		select {
		case pkt = <-svr.dist:
			if bin, err := pkt.Marshal(); err == nil {
        if addr, err := net.ResolveUDPAddr("udp", pkt.Dest()); err == nil {
          _, _ = conn.WriteToUDP(bin, addr)
        }
      }
      case <-svr.done: // server is done, break out
      break
    }
  }
}
