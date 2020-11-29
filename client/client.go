package client

import (
	"github.com/navaz-alani/concord/internal"
	"github.com/navaz-alani/concord/packet"
)

// Client defines the interface through which clients send and receive packets
// to and from the Server.
type Client interface {
	// Send sends the given packet `pkt` through the underlying connection. The
	// response from the server is sent on the `resp` channel.
	Send(pkt packet.Packet, resp chan packet.Packet) error
	// Cleanup purges the client's resources. The client should not be used after
	// this method has been called.
	Cleanup() error
	DataProcessor() internal.DataProcessor
	PacketProcessor() internal.PacketProcessor
}
