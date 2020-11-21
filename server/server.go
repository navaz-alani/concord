package server

import "github.com/navaz-alani/voip/packet"

// A definition of the interface satisfied by the server. Every packet
// that the server receives invokes a "target" in the server. A "target" is an
// action that can be performed. Upon creation, the server is
// configured with a set of targets, each of which has a callback. When a packet
// has been decoded, its "target" is first checked.
// If the target is a "server-target" (a special target handled by the server),
// then the target action is performed and work on the packet is ended.
// Otherwise, the server invokes the callback of the target (if configured).
type Server interface {
	// Begin server RW loop
	Serve() error
	AddTarget(targetName string, cb TargetCallback)
}

// TargetCallback defines the signature of a callback for a target in the
// server.
type TargetCallback func(r *Request, pw packet.Writer)

type Request struct {
	Pkt  packet.Packet
	From string
}
