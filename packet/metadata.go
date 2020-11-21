package packet

import "sync"

// KVMeta is a concurrency-safe Metadata implementation.
type KVMeta struct {
	mu   sync.RWMutex
	meta map[string]string
}

func NewKVMeta() *KVMeta {
	return &KVMeta{
		mu:   sync.RWMutex{},
		meta: make(map[string]string),
	}
}

func (m *KVMeta) Add(key, val string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.meta[key] = val
}

func (m *KVMeta) Get(key string) (val string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.meta[key]
}

func (m *KVMeta) setMeta(meta map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.meta = meta
}
