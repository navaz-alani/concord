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
	var err error
	for _, transform := range pipelines {
		if data, err = transform(ctx, data); err != nil {
			return data, fmt.Errorf("pipeline error: " + err.Error())
		} else if ctx.Stat == -1 {
			return data, fmt.Errorf("pipeline terminated: " + ctx.Msg)
		}
	}
	return data, nil
}
