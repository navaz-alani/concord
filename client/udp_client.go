package client

import (
	"math/rand"
	"net"
	"sync"

	"github.com/navaz-alani/voip/packet"
)

// Internal request statuses.
const (
	requestStatusWaiting uint8 = iota
	requestStatusError
	requestStatusTimeout
)

// requestCtx stores the status of the request as well as the response channel
// over which the requestor can be delivered the response.
type requestCtx struct {
	respCh chan packet.Packet
	msg    string
	status uint8
}

// UDPClient is a Client implementation over a UDP connection, to a UDPServer.
type UDPClient struct {
	mu             sync.RWMutex
	ReadBuffSize   int
	pc             packet.PacketCreator
	addr           *net.UDPAddr
	conn           *net.UDPConn
	activeRoutines int
	sendCh         chan packet.Packet
	recvCh         chan packet.Packet
	doneCh         chan bool
	requests       map[string]requestCtx
}

func NewUDPClient(addr *net.UDPAddr, readBuffSize int) (Client, error) {
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	client := &UDPClient{
		mu:           sync.RWMutex{},
		ReadBuffSize: readBuffSize,
		addr:         addr,
		conn:         conn,
		sendCh:       make(chan packet.Packet),
		recvCh:       make(chan packet.Packet),
		doneCh:       make(chan bool),
		requests:     make(map[string]requestCtx),
	}
	// initialize client routines
	go client.send()
	go client.recv()
	go client.reply()
	client.activeRoutines += 3
	return client, nil
}

func (c *UDPClient) Cleanup() error {
	// kill all active routines
	for i := 0; i < c.activeRoutines; i++ {
		c.doneCh <- true
	}
	return nil
}

func genRef(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}

func (c *UDPClient) Send(pkt packet.Packet, respCh chan packet.Packet) error {
	// create ref for packet
	ref := genRef(3)
	pkt.Meta().Add("_ref", ref)
	c.requests[ref] = requestCtx{
		respCh: respCh,
		status: requestStatusWaiting,
	}
	c.sendCh <- pkt
	return nil
}

// send routine writes packets to the connection, one at a time.
func (c *UDPClient) send() {
	for {
		select {
		case <-c.doneCh:
			return
		case pkt := <-c.sendCh:
			{
				ref := pkt.Meta().Get("_ref")
				var ctx requestCtx
				var refValid bool
				if bin, err := pkt.Marshal(); err != nil {
					if ctx, refValid = c.requests[ref]; refValid {
						ctx.respCh <- c.pc.NewErrPkt("", "packet encode failure")
						delete(c.requests, ref)
					}
				} else {
					if _, err := c.conn.WriteTo(bin, c.addr); err != nil {
						ctx.respCh <- c.pc.NewErrPkt("", "packed write failure")
					}
				}
			}
		default:
		}
	}
}

// read routine reads packets from underlying connection and sends them over the
// `recvCh`, one by one.
func (c *UDPClient) recv() {
	readBuff := make([]byte, c.ReadBuffSize)
	for {
		select {
		case <-c.doneCh:
			return
		default:
			{
				if n, _, err := c.conn.ReadFromUDP(readBuff); err == nil {
					pkt := c.pc.NewPkt("")
					if err := pkt.Unmarshal(readBuff[:n]); err == nil {
						// ignoring malformed response error
						c.recvCh <- pkt
					}
				}
			}
		}
	}
}

// reply routine forwards received response packets to request senders.
func (c *UDPClient) reply() {
	var ref string
	for {
		select {
		case <-c.doneCh:
			return
		case pkt := <-c.recvCh:
			{
				// send received packet to caller
				ref = pkt.Meta().Get("_ref")
				if ctx, refValid := c.requests[ref]; refValid {
					ctx.respCh <- pkt
					delete(c.requests, ref)
				}
			}
		default:
		}
	}
}
