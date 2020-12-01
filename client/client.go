package client

import (
	"github.com/navaz-alani/concord/internal"
	"github.com/navaz-alani/concord/packet"
)

// Client defines the interface through which clients send and receive packets
// to and from the Server. To send a packet, the process is as follows: create
// the packet and write some data and/or metadata to it. Then, create a channel
// over which the response to that packet will be sent, then call the Send
// method of the client, giving it the packet to send as well as the channel
// over which the response shall be sent. The first packet received from that
// channel will be either a response from the server or an packet informing of
// an error.
type Client interface {
	// Send sends the given packet `pkt` through the underlying connection. The
	// response from the server is sent on the `resp` channel.
	Send(pkt packet.Packet, resp chan packet.Packet) error
	// Cleanup purges the client's resources. The client should not be used after
	// this method has been called.
	Cleanup() error
	// Misc returns a channel over which packets without a ref/with an recognized
	// ref are sent. The client can then handle these packets as desired. For
	// example, the initial packet sent to a client by another client (forwarded
	// using the Server's relay target) does not bear a ref valid ref on the
	// destination's client, so this packet would be received in the Misc channel.
	Misc() <-chan packet.Packet

	// The following will most probably not be used by clients. They exist for the
	// purposes of extending the functionality of the client. An example is the
	// crypto.Crypto extension which adds transport layer encryption to a
	// client/server.

	// Access the internal data processor
	DataProcessor() internal.DataProcessor
	// Access the internal packet processor
	PacketProcessor() internal.PacketProcessor
}
