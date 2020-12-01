package internal

import "github.com/navaz-alani/concord/packet"

// Context Status codes
const (
	// CodeContinue 0 means "continue callback queue execution".
	CodeContinue = 0
	// CodeStopError  means "stop callback queue execution, send error message (ctx.Msg)"
	CodeStopError = -1
	// CodeStopCloseSend means "stop callback queue execution, close response and send".
	CodeStopCloseSend = 1
	// CodeStopNoop means "stop callback queue execution, and do nothing".
	CodeStopNoop = 2
)

type PipelineCtx struct {
	Pkt  packet.Packet
	Stat int
	Msg  string
}

// TransformContext is information shared by all BufferTransform functions
// executed on a buffer.
type TransformContext struct {
	PipelineCtx
	PipelineName string
	From         string
}

// BufferTransform is a function which acts on a byte slice, updating its
// contents in some way. An example of a buffer transform is a cryptographic
// function which decodes/encodes the contents of the buffer.
type BufferTransform func(ctx *TransformContext, buff []byte) (result []byte)

// DataProcessor is used to build pipelines for operating on binary data, using
// BufferTransforms. Multiple BufferTransforms may be run, in succession, on
// one piece of data, forming a data pipeline. The pipeline may have to
// prematurely exit (for example if the data is malformed) and this can be done
// using the transform context by setting the Stat field to -1 and, possibly,
// setting an error message in the Msg field.
type DataProcessor interface {
	// Lock processor - prevent writes, only executing pipelines allowed
	Lock()
	// Unlock processort - enable writes
	Unlock()
	// AddTransform adds the given transform to the pipeline specfied. If the
	// processor is already locked, then nothing is done.
	AddTransform(pipelineName string, transform BufferTransform)
	// Process runs the pipeline specified on the given data and returns the
	// transformed data and any error that may have occurred.
	Process(ctx *TransformContext, data []byte) ([]byte, error)
}

// TargetCallback defines the signature of a callback for a target in the server.
type TargetCallback func(ctx *TargetCtx, pw packet.Writer)

// TargetCtx is the server's callback queue execution context. To end the
// callback queue execution for a particular packet, TargetCallbacks should set
// Stat to -1 and the server will terminate the execution. In such a case, the
// server will inform the sender of the error using the content of the Msg
// field. These two fields appear in the Metadata of a Packet at the keys
// "_stat" and "_msg" respectively. Other status codes will cause the server to
// respond differently, but a non-zero status code will surely stop execution of
// the callback queue.
// Each of these status codes have names, provided in the constants section of
// the `internal` package, which are more sensible and easy to remember.
type TargetCtx struct {
	PipelineCtx
	TargetName string
	From       string
}

// PacketProcessor is used to build callback queues for different targets.
// Packets are then processed according to their target. The TargetCtx allows
// TargetCallback functions to alter the execution of the callback queue.
type PacketProcessor interface {
	// AddCallback adds the given calback function to the callback queue for the
	// given target name.
	AddCallback(targetName string, cb TargetCallback)
	// Process executes the callback queue for the given packet's target
	Process(ctx *TargetCtx, pw packet.Writer) error
}

type Processor interface {
	DataProcessor() DataProcessor
	PacketProcessor() PacketProcessor
}

type Extension interface {
	// Extend extends the given processor to use the Crypto extension. `kind` is a
	// string: either "server" or "client". On a server, it installs the key
	// exchange endpoints.
	Extend(kind string, target Processor) error
}
