package plugin

import (
	"context"
	"time"

	"github.com/ageneralai/maven/internal/kernel/config"
)

// MemoryEntry is one item returned by a memory plugin.
type MemoryEntry struct {
	Source    string
	Content   string
	Timestamp time.Time
}

// MemoryQuery filters what a memory plugin returns. Zero values mean no filter.
type MemoryQuery struct {
	MaxAge time.Duration
	Limit  int
}

// MemoryPlugin is the plugin axis for long-term agent memory.
type MemoryPlugin interface {
	Plugin
	Read(ctx context.Context, cfg *config.Config, q MemoryQuery) ([]MemoryEntry, error)
}
