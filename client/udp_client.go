package client

import (
	"fmt"
	"math/rand"
	"net"
	"sync"

	"github.com/navaz-alani/concord/internal"
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

type writePacket struct {
	data   []byte
	respCh chan packet.Packet
}

// UDPClient is a Client implementation over a UDP connection, to a UDPServer.
type UDPClient struct {
	mu           sync.RWMutex
	ReadBuffSize int
	pc           packet.PacketCreator
	addr         *net.UDPAddr
	conn         *net.UDPConn
	pipelines    struct {
		data   *internal.DataPipeline
		packet *internal.PacketPipeline
	}
	th             throttle.Throttle
	activeRoutines int
	writeCh        chan *writePacket
	sendCh         chan packet.Packet
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
		pipelines: struct {
			data   *internal.DataPipeline
			packet *internal.PacketPipeline
		}{
			data:   internal.NewDataPipeline(),
			packet: internal.NewPacketPipeline(),
		},
		th:       throttle.NewUDPThrottle(throttleRate, conn, readBuffSize),
		writeCh:  make(chan *writePacket),
		sendCh:   make(chan packet.Packet),
		doneCh:   make(chan bool),
		requests: make(map[string]requestCtx),
	}
	// initialize client routines
	go client.recv()
	go client.write()
	client.activeRoutines += 2
	return client, nil
}

func (c *UDPClient) PacketProcessor() internal.PacketProcessor {
	return c.pipelines.packet
}

func (c *UDPClient) DataProcessor() internal.DataProcessor {
	return c.pipelines.data
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
	var letters = []rune(`abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()_+{}[];':",./<>?\|`)
	s := make([]rune, n)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}

// Send attempts to send the given packet to its set destination address. If
// there are any errors with processing this packet, they will be returned.
// When a packet is received by the user in respCh, it should be checked that it
// is not an error packet informing the caller that the write operation failed.
// This can be done by checking the "_stat" (should be -1) and "_msg" metadata
// fields. Note that error packets sent by the clients will set the same error
// fields as the ones sent from the server, the only difference being that, the
// internally sent client error packets will not have a ref metadata value,
// whereas server sent error packets do.
func (c *UDPClient) Send(pkt packet.Packet, respCh chan packet.Packet) error {
	// create ref for packet
	ref := genRef(5)
	pkt.Meta().Add(packet.KeyRef, ref)
	if bin, err := pkt.Marshal(); err != nil {
		return fmt.Errorf("packet encode failure")
	} else {
		transformCtx := &internal.TransformContext{
			PipelineName: "_out_",
			Dest:         pkt.Dest(),
		}
		var err error
		if bin, err = c.pipelines.data.Process(transformCtx, bin); err != nil {
			return fmt.Errorf("data pipeline error: " + err.Error())
		} else if transformCtx.Stat == internal.CodeStopNoop {
			return fmt.Errorf("data pipeline enforced noop")
		}
		c.writeCh <- &writePacket{
			data:   bin,
			respCh: respCh,
		}
	}
	c.mu.Lock()
	c.requests[ref] = requestCtx{
		respCh: respCh,
	}
	c.mu.Unlock()
	return nil
}

func (c *UDPClient) write() {
	for {
		select {
		case <-c.doneCh:
			return
		case pkt := <-c.writeCh:
			if _, err := c.th.WriteTo(pkt.data, c.addr); err != nil {
				pkt.respCh <- c.pc.NewErrPkt("", "", "packet write error: "+err.Error())
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
			if data, _, err := c.th.ReadFrom(); err == nil {
				go c.processIncoming(data)
			}
		}
	}
}

func (c *UDPClient) processIncoming(data []byte) {
	transformCtx := &internal.TransformContext{
		PipelineName: "_in_",
		From:         c.addr.String(),
	}
	var err error
	if data, err = c.pipelines.data.Process(transformCtx, data); err != nil {
		return // ignoring packet if pipeline fails to process it
	}

	pkt := c.pc.NewPkt("", "")
	if err := pkt.Unmarshal(data); err == nil {
		// ignoring malformed response error
		ref := pkt.Meta().Get(packet.KeyRef)
		c.mu.RLock()
		ctx, refValid := c.requests[ref]
		c.mu.RUnlock()
		if refValid {
			ctx.respCh <- pkt
			delete(c.requests, ref)
		}
	}
}
