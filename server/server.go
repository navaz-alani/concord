package server

import "github.com/navaz-alani/voip/packet"

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
// same `*ServerCtx` and `packet.Writer`. The execution of the queue of callbacks
// should compose a response packet (through `pw`) to be returned to the sender.
// The user can use a callback in the callback queue for a particular target to,
// for example, verify that the request is authenticated. If authentication
// fails, then the rest of the queue need not be executed and the application
// may cancel the request by setting the `Cancelled` field in the given
// `*ServerCtx`.
//
// With respect to error management, the server does not handle any errors
// related to encoding/decoding packets. In situations where the sender can be
// notified, a response is sent.
type Server interface {
	// Begin server RW loop
	Serve() error
	// AddTargetCallback pushes the given callback onto the callback queue for the
	// specified target.
	AddTargetCallback(target string, cb TargetCallback)
}

// TargetCallback defines the signature of a callback for a target in the server.
type TargetCallback func(ctx *ServerCtx, pw packet.Writer)

// ServerCtx is the server's callback queue execution context. To end the
// callback queue execution for a particular packet, TargetCallbacks should set
// Stat to -1 and the server will terminate the execution. In such a case, the
// server will inform the sender of the error using the content of the Msg
// field. These two fields appear in the Metadata of a Packet at the keys
// "_stat" and "_msg" respectively.
type ServerCtx struct {
	Stat int
	Msg  string
	Pkt  packet.Packet
	From string
}
