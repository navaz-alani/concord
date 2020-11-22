package throttle

import (
	"net"
	"sync"
	"time"
)

const sec uint64 = 1_000_000_000 // 10^9 - nanoseconds in a second

type readPkt struct {
	data   []byte
	sender *net.UDPAddr
	err    error
}

type writePkt struct {
	data   []byte
	to     net.Addr
	respCh chan *writeStatus
}

type writeStatus struct {
	written int
	err     error
}

type UDPThrottle struct {
	mu        sync.RWMutex // mu protects `rate` and `tpo` from concurrent operations
	rate      Rate         // throttle throughput
	tpo       uint64       // tpo is "time per operation" - computed from the `rate`
	rbuffSize int
	conn      *net.UDPConn
	// internal _buffered_ channels for packet processing
	recv chan *readPkt
	send chan *writePkt
}

func NewUDPThrottle(initialRate Rate, conn *net.UDPConn, readBuffSize int) *UDPThrottle {
	th := &UDPThrottle{
		mu:        sync.RWMutex{},
		rate:      initialRate,
		tpo:       sec / uint64(initialRate),
		rbuffSize: readBuffSize,
		conn:      conn,
		recv:      make(chan *readPkt, 100),
		send:      make(chan *writePkt, 100),
	}
	return th
}

func (th *UDPThrottle) Throughput() Rate {
	th.mu.RLock()
	defer th.mu.RUnlock()
	return th.rate
}

func (th *UDPThrottle) SetThroughput(rate Rate) {
	th.mu.Lock()
	defer th.mu.Unlock()
	th.rate = rate
	th.tpo = sec / uint64(rate)
}

func (th *UDPThrottle) ScaleThroughput(f uint8) {
	th.mu.Lock()
	defer th.mu.Unlock()
	th.rate = Rate(int64(th.rate) * int64(f))
	th.tpo = sec / uint64(th.rate)
}

func (th *UDPThrottle) ReadFrom() ([]byte, net.Addr, error) {
	pkt := <-th.recv
	return pkt.data, pkt.sender, pkt.err
}

func (th *UDPThrottle) WriteTo(data []byte, addr net.Addr) (int, error) {
	respCh := make(chan *writeStatus)
	th.send <- &writePkt{
		data:   data,
		to:     addr,
		respCh: respCh,
	}
	status := <-respCh
	return status.written, status.err
}

func (th *UDPThrottle) read() {
	var start time.Time
	rbuff := make([]byte, th.rbuffSize)
	for {
		start = time.Now()
		n, senderAddr, err := th.conn.ReadFromUDP(rbuff)
		data := make([]byte, n)
		copy(data, rbuff)
		th.recv <- &readPkt{
			data:   data,
			sender: senderAddr,
			err:    err,
		}
		th.throttleOperation(uint64(start.Sub(time.Now())))
	}
}

func (th *UDPThrottle) write() {
	var start time.Time
	for {
		start = time.Now()
		pkt := <-th.send
		n, err := th.conn.WriteTo(pkt.data, pkt.to)
		pkt.respCh <- &writeStatus{
			written: n,
			err:     err,
		}
		th.throttleOperation(uint64(start.Sub(time.Now())))
	}
}

// throttleOperation ensures that a read/write operation takes at least as long
// as what is spcefied by the `rate` parameter of the Throttle.
func (th *UDPThrottle) throttleOperation(dur uint64) {
	th.mu.RLock()
	tpo := th.tpo
	th.mu.RUnlock()
	if remainingTime := tpo - dur; remainingTime > 0 {
		time.Sleep(time.Duration(remainingTime))
	}
}
