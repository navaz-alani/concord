package server

import "sync"

// DataPipeline is an internal type used to manage the transformations to be
// performed on binary data immediately after it has been read and just before
// it is written to the connection.
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

func (d *DataPipeline) AddTransform(transform BufferTransform, pipelineName string) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	d.pipelines[pipelineName] = append(d.pipelines[pipelineName], transform)
}

func (d *DataPipeline) Process(pipelineName string, data []byte) ([]byte, error) {
	return nil, nil
}
