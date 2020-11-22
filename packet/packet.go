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
// any way that they wish. When a packet is sent to the Server by a client, the
// `Target` field is significant. When a packet is sent to the client by the
// Server, the Target field is not significant i.e. it holds no meaning for the
// client and is usually empty.
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
	NewPkt(ref, dest string) Packet
	// NewErrPkt creates error packets for Server user.
	NewErrPkt(ref, dest, msg string) Packet
}

// Writer describes the behaviour of a Packet writer, used to compose Packets.
type Writer interface {
	io.WriteCloser
	Meta() Metadata
	SetTarget(t string)
}
