package core

import (
	"fmt"
	"sync"

	"github.com/navaz-alani/concord/packet"
)

type PacketPipeline struct {
	mu             sync.RWMutex
	callbackQueues map[string][]TargetCallback
}

func NewPacketPipeline() *PacketPipeline {
	return &PacketPipeline{
		mu:             sync.RWMutex{},
		callbackQueues: make(map[string][]TargetCallback),
	}
}

func (pp *PacketPipeline) AddCallback(targetName string, cb TargetCallback) {
	pp.mu.Lock()
	pp.callbackQueues[targetName] = append(pp.callbackQueues[targetName], cb)
	pp.mu.Unlock()
}

func (pp *PacketPipeline) Process(ctx *TargetCtx, pw packet.Writer) error {
	pp.mu.RLock()
	pipelines, ok := pp.callbackQueues[ctx.TargetName]
	pp.mu.RUnlock()
	if !ok {
		return fmt.Errorf("target not found")
	}
	for _, cb := range pipelines {
		cb(ctx, pw)
		switch ctx.Stat {
		case CodeStopError: // stop cbq exec and return error
			return fmt.Errorf(ctx.Msg)
		case CodeStopNoop, CodeStopCloseSend: // stop cbq exec, no error
			return nil
		}
	}
	return nil
}
