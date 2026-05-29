package terminal

import (
	"io"
	"sync"
)

const (
	userLabel  = "you ▸ "
	MavenLabel = "maven ▸ "
)

// Transcript serializes REPL transcript writes.
type Transcript struct {
	Out io.Writer
	mu  sync.Mutex
}

func (t *Transcript) Write(p []byte) (int, error) {
	if t == nil || t.Out == nil {
		return len(p), nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.Out.Write(p)
}
