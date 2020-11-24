package client

import (
	"math/rand"
	"net"
	"sync"

	"github.com/navaz-alani/concord/packet"
	"github.com/navaz-alani/concord/throttle"
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
	th             throttle.Throttle
	activeRoutines int
	sendCh         chan packet.Packet
	recvCh         chan packet.Packet
	doneCh         chan bool
	requests       map[string]requestCtx
}

func NewUDPClient(addr *net.UDPAddr, readBuffSize int,
	pc packet.PacketCreator, throttleRate throttle.Rate) (Client, error) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: []byte{0, 0, 0, 0}})
	if err != nil {
		return nil, err
	}
	client := &UDPClient{
		mu:           sync.RWMutex{},
		ReadBuffSize: readBuffSize,
		pc:           pc,
		addr:         addr,
		conn:         conn,
		th:           throttle.NewUDPThrottle(throttleRate, conn, readBuffSize),
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
	c.th.Shutdown() // purge throttle resources
	c.conn.Close()  // close underlying udp connection
	return nil
}

// Helper to generate a length-dependent ref for a packet.
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
	pkt.Meta().Add(packet.KeyRef, ref)
	c.mu.Lock()
	c.requests[ref] = requestCtx{
		respCh: respCh,
		status: requestStatusWaiting,
	}
	c.mu.Unlock()
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
				ref := pkt.Meta().Get(packet.KeyRef)
				var ctx requestCtx
				var refValid bool
				if bin, err := pkt.Marshal(); err != nil {
					c.mu.RLock()
					if ctx, refValid = c.requests[ref]; refValid {
						ctx.respCh <- c.pc.NewErrPkt("", "", "packet encode failure")
						delete(c.requests, ref)
					}
					c.mu.RUnlock()
				} else {
					if _, err := c.th.WriteTo(bin, c.addr); err != nil {
						ctx.respCh <- c.pc.NewErrPkt("", "", "packed write failure")
						delete(c.requests, ref)
					}
				}
			}
		}
	}
}

// read routine reads packets from underlying connection and sends them over the
// `recvCh`, one by one.
func (c *UDPClient) recv() {
	for {
		select {
		case <-c.doneCh:
			return
		default:
			{
				if data, _, err := c.th.ReadFrom(); err == nil {
					pkt := c.pc.NewPkt("", "")
					if err := pkt.Unmarshal(data); err == nil {
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
				ref = pkt.Meta().Get(packet.KeyRef)
				c.mu.RLock()
				if ctx, refValid := c.requests[ref]; refValid {
					ctx.respCh <- pkt
					delete(c.requests, ref)
				}
				c.mu.RUnlock()
			}
		}
	}
}
