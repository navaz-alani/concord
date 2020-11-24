package server

import "github.com/navaz-alani/concord/client"

// ServerExtensionInstaller installs additional capabilities on a Server through
// its DataProcessor or its target and callback-queue interface.
type ServerExtensionInstaller interface {
	// Install the extension on the server.
	InstallServer(s Server) error
	// Install the extension on the server.
	InstallClient(c client.Client) error
}

// TransformContext is information shared by all BufferTransform functions
// executed on a buffer.
type TransformContext struct {
	Stat int
	Msg  string
}

// BufferTransform is a function which acts on a byte slice, updating its
// contents in some way. An example of a buffer transform is a cryptographic
// function which decodes/encodes the contents of the buffer.
type BufferTransform func(ctx TransformContext, buff []byte) (result []byte, err error)

// DataProcessor is used to build pipelines for operating on data, using
// BufferTransforms.  Multiple BufferTransforms may be run, in succession, on
// one piece of data, forming a data pipeline. The pipeline may have to
// prematurely exit (for example if the data is malformed) and this can be done
// using the transform context by setting the Stat field to -1 and, possibly,
// setting an error message in the Msg field.
type DataProcessor interface {
	Lock()   // lock processor - prevent writes, only executing pipelines allowed
	Unlock() // unlock processort - enable writes
	// AddTransform adds the given transform to the pipeline specfied. If the
	// processor is already locked, then nothing is done.
	AddTransform(transform BufferTransform, pipelineName string)
	// Process runs the pipeline specified on the given data and returns the
	// transformed data and any error that may have occurred.
	Process(pipelineName string, data []byte) ([]byte, error)
}
