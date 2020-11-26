package server

import "github.com/navaz-alani/concord/internal"

// A definition of the interface satisfied by the server. Every packet that the
// server receives invokes a "target" in the server. A "target" is a set of
// actions that can be performed (through the execution of callbacks) when a
// Packet has been received.
//
// Upon creation, the server is configured with a set of targets (each with a
// callback queue). When a packet is received and decoded, its "target" is first
// checked and the server executes the callback queue of the target. If a
// request has an invalid target, the packet is ignored. The callback queue is
// executed by calling the configured callbacks first to last, passing them the
// same `*TargetCtx` and `packet.Writer`. The execution of the queue of callbacks
// should compose a response packet (through `pw`) to be returned to the sender.
// The user can add a callback in the callback queue for a particular target to,
// for example, verify that the request is authenticated. If authentication
// fails, then the rest of the queue need not be executed and the application
// may cancel the request by setting the `Stat` field in the given `*TargetCtx`
// to -1 and providing an error message in the `Msg` field.
//
// With respect to error management, the server does not handle any errors
// related to encoding/decoding packets. In situations where the sender can be
// notified, a response is sent.
//
// The Server also offers the ability to extend its capabilities using the
// DataProcessor and the PacketProcessor. Briefly, the DataProcessor manages
// operations to be performed on binary data. The Crypto extension, for example,
// uses the DataProcessor to perform end to end encryption of packets by
// decoding the (encrypted) binary data when it is read from the connection and
// encrypting binary data before it is written to the connection (using the
// shared key with the recipient). The PacketProcessor manages operations
// performed on Packets i.e. targets & their callback queues. The Crypto
// extension also uses the PacketProcessor to setup targets for key-exchange
// with the server and with other clients.
type Server interface {
	// Begin server RW loop
	Serve() error
	// Access the DataProcessor to perform extensions on the Server.
	DataProcessor() internal.DataProcessor
	// Access the PacketProcessor to configure targets and callback queues.
	PacketProcessor() internal.PacketProcessor
}
