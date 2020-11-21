package packet

import "io"

// Metadata defines a key-value metadata store for Packets.
type Metadata interface {
	Add(key, val string)
	Get(key string) (val string)
}

// Packet defines the required behaviour of a type to be serialized/deserialized
// for over-the-wire transport. For users, the `Data` method returns the data
// sent in the Packet. Applications may choose to interpret this information in
// any way that they wish.
type Packet interface {
	// Dest returns the address to which this packet is destined.
	Dest() string
	// Target is the target that the packet invokes on the server.
	Target() string
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
// with configurable Packet types.
type PacketCreator interface {
	NewPkt(dest string) Packet
}

// Writer describes the behaviour of a Packet writer, used to compose Packets.
type Writer interface {
	io.WriteCloser
	Meta() Metadata
	SetTarget(t string)
}
