package throttle

import "net"

// Rate is the Throttle throughput parameter - measured in data packets per
// second (dps).
type Rate uint64

// Throttle Rate parameters - data packets per second
const (
	RateZero Rate = 0 // halt communication over network.
	Rate1h        = 100
	Rate1k        = 1_000
	Rate10k       = 10_000
	Rate100K      = 100_000
)

// Throttle controls read/write operation over a connection in order to prevent
// congestion. There is a `throughput` parameter, which is a `Rate` (data
// packets per second). The Throttle owner can modify this parameter by either
// setting an exact value or scaling the current value by a particular factor
// 0 < `f` <= 1, using the {set,scale}Throughput methods respectively.
type Throttle interface {
	Throughput() Rate
	setThroughput(rate Rate)
	scaleThroughput(f uint8)

	ReadFrom(readBuff []byte) (int, net.Addr, error)
	WriteTo(data []byte, addr net.Addr) (int, error)
}
