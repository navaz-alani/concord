package server

import "github.com/navaz-alani/voip/packet"

// A definition of the interface satisfied by the server. Every packet that the
// server receives invokes a "target" in the server. A "target" is an action
// that can be performed (through the execution of a callback). Upon creation,
// the server is configured with a set of targets (each with a callback). When a
// packet is received and decoded, its "target" is first checked and the server
// invokes the callback of the target. If a request has an invalid target, the
// packet is ignored.
//
// With respect to error management, the server does not handle any errors
// related to encoding/decoding packets - they are quietly ignored.
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
