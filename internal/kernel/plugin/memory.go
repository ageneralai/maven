package plugin

import (
	"context"
	"time"

	"github.com/ageneralai/maven/internal/kernel/config"
)

// MemoryKind classifies a memory entry. Unknown kinds are accepted and ignored by plugins that do not support them.
type MemoryKind string

const (
	MemoryKindFact       MemoryKind = "fact"
	MemoryKindEvent      MemoryKind = "event"
	MemoryKindPreference MemoryKind = "preference"
)

// MemoryEntry is one item returned by a memory plugin.
type MemoryEntry struct {
	Source    string
	Content   string
	Kind      MemoryKind
	Timestamp time.Time
}

// MemoryQuery filters what a memory plugin returns. Zero values mean no filter.
type MemoryQuery struct {
	Kinds  []MemoryKind
	MaxAge time.Duration
	Limit  int
}

// MemoryPlugin is the plugin axis for long-term agent memory.
// Exactly one registered plugin must return Primary() == true; the kernel writes only to that plugin.
// All plugins contribute Read results.
type MemoryPlugin interface {
	Plugin
	Primary() bool
	Read(ctx context.Context, cfg *config.Config, q MemoryQuery) ([]MemoryEntry, error)
	Write(ctx context.Context, cfg *config.Config, e MemoryEntry) error
}
