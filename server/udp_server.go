package server

import (
	"net"

	"github.com/navaz-alani/voip/packet"
)

type UDPServer struct {
	addr         *net.UDPAddr
	targets      map[string]TargetCallback
	pc           packet.PacketCreator
	dist         chan packet.Packet
	shutdown     chan bool
	readBuffSize int
}

func NewUDPServer(pc packet.PacketCreator, addr *net.UDPAddr, readBuffSize int) *UDPServer {
	return &UDPServer{
		addr:         addr,
		targets:      make(map[string]TargetCallback),
		dist:         make(chan packet.Packet),
		shutdown:     make(chan bool),
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
	// wait for shutdown signal
	<-svr.shutdown
	return nil
}

// read is a routune which reads and decodes packets from the underlying
// connection and handles them accordingly.
func (svr *UDPServer) read(conn *net.UDPConn) {
	readBuff := make([]byte, svr.readBuffSize)
	req := svr.pc.NewPkt("")
	var resp packet.Packet
	for {
		if n, senderAddr, err := conn.ReadFromUDP(readBuff); err != nil {
			// todo: handle error
		} else {
			// decode packet
			if err := req.Unmarshal(readBuff[:n]); err != nil {
				// todo: handle error
			}

			// create new response packet and populate it by executing target
			resp = svr.pc.NewPkt(senderAddr.String())
			if cb, ok := svr.targets[resp.Target()]; !ok {
				// ignore non-existent target
				continue
			} else {
				// invoke callback and send response
				go func() {
					cb(&Request{
						Pkt:  req,
						From: senderAddr.String(),
					}, resp.Writer())
					svr.dist <- resp
				}()
			}
		}
	}
}

// write is a routine which distributes packets by writing them over the
// underlying UDP connection.
func (svr *UDPServer) write(conn *net.UDPConn) {
	var pkt packet.Packet
	for {
		pkt = <-svr.dist
		if bin, err := pkt.Marshal(); err == nil {
			if addr, err := net.ResolveUDPAddr("udp", pkt.Dest()); err == nil {
				_, _ = conn.WriteToUDP(bin, addr)
			}
		}
	}
}
