package packet

import "io"

// Metadata keys
const (
	KeySvrStatus string = "_stat"
	KeySvrMsg           = "_msg"
	KeyRef              = "_ref"
	KeyTarget           = "_tgt"
)

// Metadata defines a key-value metadata store for Packets.
type Metadata interface {
	Add(key, val string)
	Get(key string) (val string)
	Clear()
}

// Packet defines the required behaviour of a type to be serialized/deserialized
// for over-the-wire transport. For users, the `Data` method returns the data
// sent in the Packet. Applications may choose to interpret this information in
// any way that they wish. Packets mainly work off of metadata and the target of
// a packet is specified using the metadata key "_tgt".
//
// In the case that processing fails and the Server returns a Packet, there are
// special metadata keys set by the server: "_stat" and "_msg". The former
// is an integer (as a string), representing a status code (servers use "-1" for
// a generic error) and the latter is a string explaining the error. There is
// also the "_ref" key which can be set on a packet by the client. The server
// then maintains this "_ref" key metadata on its response, thereby enabling the
// Client to determine which request the response packet corresponds to. Note
// that Client is responsible for setting this "_ref" metadata.
type Packet interface {
	// Dest returns the address to which this packet is destined.
	Dest() string
	// SetDest sets the destination of the packet.
	SetDest(dest string)
	// Meta is the metadata accessor.
	Meta() Metadata
	// Data is the packet data accessor.
	Data() []byte
	// Marshal encodes the Packet to binary.
	Marshal() (bin []byte, err error)
	// Unmarshal decodes the Packet from binary.
	Unmarshal(bin []byte) (err error)

	Writer() Writer
}

// PacketCreator abstracts creation of packets so that Server types may be
// completely agnostic to Packet types and applications may instantiate Servers
// with configurable Packet types. The `ref` string is included in the following
// PacketCreator method signatures to indicate the importance of the `ref`
// metadata on requests (of course, the `ref` can also be set by using a
// Packet.Meta().Add(packet.KeyRef, "<ref>") call, as with any other metadata).
// The `ref` is important because it is the only piece of information unique to
// a request sent by a particular address.
//
// The PacketCreator possesses an underlying pool of packets which could be
// useful in high-throughput cases to avoid the overhead of packet allocations.
// The Warmup method warms the pool up with a specific number of packets. The
// New*Pkt methods return packets from the underlying pool and the PutBack
// method returns packets to the pool.
type PacketCreator interface {
	// Warmup warms the underlying pool up with the given number of packets.
	Warmup(numPackets int)
	// PutBack returns the given packet to the PacketCreator pool.
	PutBack(Packet)
	// NewPkt returns a new packet with the given ref and dest.
	NewPkt(ref, dest string) Packet
	// NewErrPkt creates error packets for Server/Client user.
	NewErrPkt(ref, dest, msg string) Packet
}

// Writer describes the behaviour of a Packet writer, used to compose Packets.
type Writer interface {
	io.WriteCloser
	Meta() Metadata
	Clear()
}
