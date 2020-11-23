package throttle

import "net"

// Rate is the Throttle throughput parameter - measured in data packets per
// second (dpps).
type Rate uint64

// Throttle Rate parameters - data packets per second
const (
	Rate1h   = 100
	Rate1k   = 1_000
	Rate10k  = 10_000
	Rate100K = 100_000
)

// Throttle controls read/write operation over a connection in order to prevent
// congestion. There is a `throughput` parameter, which is a `Rate` (data
// packets per second). The Throttle owner can modify this parameter by either
// setting an exact value or scaling the current value by a particular factor
// 0 < `f` <= 1, using the {set,scale}Throughput methods respectively.
//
// Note: Throttle does not own the underlying connection that it is managing.
// The creator of that connection is responsible for closing the connection.
type Throttle interface {
	Throughput() Rate
	SetThroughput(rate Rate)
	ScaleThroughput(f uint8)

	// ReadFrom reads a packet from the underlying connection and returns the data
	// read, the sender address and any error encountered.
	ReadFrom() ([]byte, net.Addr, error)
	// WriteTo writes the given `data` over the underlying connection and returns
	// the number of bytes written and any error encountered.
	WriteTo(data []byte, addr net.Addr) (int, error)

	// Shutdown sends a kill signal and purges the Throttle's resources.
	Shutdown()
}
