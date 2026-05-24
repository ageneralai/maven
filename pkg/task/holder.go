package task

import (
	"sync"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
)

// RuntimeHolder binds the Task tool to the SDK runtime after api.New.
type RuntimeHolder struct {
	mu sync.RWMutex
	rt *api.Runtime
}

func (h *RuntimeHolder) Set(rt *api.Runtime) {
	if h == nil {
		return
	}
	h.mu.Lock()
	h.rt = rt
	h.mu.Unlock()
}

func (h *RuntimeHolder) Get() *api.Runtime {
	if h == nil {
		return nil
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.rt
}
