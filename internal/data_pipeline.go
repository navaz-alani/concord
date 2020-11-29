package internal

import (
	"fmt"
	"sync"
)

// DataPipeline manages the transformations to be performed on binary data
// immediately after it has been read and just before it is written to the
// connection.
type DataPipeline struct {
	mu        sync.RWMutex
	locked    bool
	pipelines map[string][]BufferTransform
}

func NewDataPipeline() *DataPipeline {
	return &DataPipeline{
		pipelines: make(map[string][]BufferTransform),
	}
}

func (d *DataPipeline) Lock() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.locked = true
}

func (d *DataPipeline) Unlock() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.locked = false
}

func (d *DataPipeline) AddTransform(pipelineName string, transform BufferTransform) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	d.pipelines[pipelineName] = append(d.pipelines[pipelineName], transform)
}

func (d *DataPipeline) Process(ctx *TransformContext, data []byte) ([]byte, error) {
	d.mu.RLock()
	pipelines := d.pipelines[ctx.PipelineName]
	d.mu.RUnlock()
	for _, transform := range pipelines {
		data = transform(ctx, data)
		switch ctx.Stat {
		case CodeStopError:
			return data, fmt.Errorf("pipeline terminated: " + ctx.Msg)
		case CodeStopNoop, CodeStopCloseSend:
			return data, nil
		}
	}
	return data, nil
}
