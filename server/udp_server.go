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
	targets      map[string]TargetCallback
	pc           packet.PacketCreator
	dist         chan packet.Packet
	shutdown     chan string
	done         chan bool
	readBuffSize int
}

func NewUDPServer(pc packet.PacketCreator, addr *net.UDPAddr, readBuffSize int) *UDPServer {
	return &UDPServer{
		addr:         addr,
		targets:      make(map[string]TargetCallback),
		dist:         make(chan packet.Packet),
		shutdown:     make(chan string),
		done:         make(chan bool),
		readBuffSize: readBuffSize,
	}
}

func (svr *UDPServer) AddTarget(name string, cb TargetCallback) {
	svr.targets[name] = cb
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
	return fmt.Sprintf("[UDPServer@%s] ", svr.addr.String())
}

// read is a routune which reads and decodes packets from the underlying
// connection and handles them accordingly.
func (svr *UDPServer) read(conn *net.UDPConn) {
	readBuff := make([]byte, svr.readBuffSize)
	req := svr.pc.NewPkt("")
	var resp packet.Packet
	for {
		if n, senderAddr, err := conn.ReadFromUDP(readBuff); err != nil {
			// send shutdown signal to end write routine
			svr.shutdown <- svr.fmtMsg("read fail - connection error")
		} else {
			if err := req.Unmarshal(readBuff[:n]); err != nil { // ignore malformed packets
			} else if cb, ok := svr.targets[req.Target()]; !ok { // ignore non-existent targets
			} else {
				// execute callback corresponding to the request target.
				go func(senderAddr string, cb TargetCallback, req packet.Packet) {
					resp = svr.pc.NewPkt(senderAddr)
					cb(&Request{
						Pkt:  req,
						From: senderAddr,
					}, resp.Writer()) // invoke callback
					svr.dist <- resp // send response
				}(senderAddr.String(), cb, req)
			}
		}
	}
}

// write is a routine which distributes packets by writing them over the
// underlying UDP connection.
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
