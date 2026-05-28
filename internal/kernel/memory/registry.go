package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"log/slog"

	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/plugin"
)

const (
	readTimeout      = 500 * time.Millisecond
	memoryMdMaxChars = 12288
)

// Registry fans out memory reads to all plugins and routes writes to the primary.
type Registry struct {
	plugins []plugin.MemoryPlugin
	log     *slog.Logger
}

// NewRegistry constructs a Registry. Returns error if zero or more than one plugin is primary.
func NewRegistry(lg *slog.Logger, plugins ...plugin.MemoryPlugin) (*Registry, error) {
	n := 0
	for _, p := range plugins {
		if p.Primary() {
			n++
		}
	}
	if n != 1 {
		return nil, fmt.Errorf("memory registry: exactly one primary plugin required, got %d", n)
	}
	cp := make([]plugin.MemoryPlugin, len(plugins))
	copy(cp, plugins)
	return &Registry{plugins: cp, log: lg}, nil
}

// Context reads long-term memory (MEMORY.md) from all plugins concurrently with a 500ms budget
// and returns a formatted string for system prompt injection. Daily journal files are excluded —
// the agent retrieves episodic memory on demand via memory_search / memory_get tools.
// Errors from individual plugins are logged and skipped — never fatal.
func (r *Registry) Context(ctx context.Context, cfg *config.Config, q plugin.MemoryQuery) string {
	if r == nil || len(r.plugins) == 0 {
		return ""
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	type result struct {
		entries []plugin.MemoryEntry
		err     error
	}
	ch := make(chan result, len(r.plugins))
	for _, p := range r.plugins {
		go func(mp plugin.MemoryPlugin) {
			entries, err := mp.Read(ctx, cfg, q)
			ch <- result{entries, err}
		}(p)
	}
	var all []plugin.MemoryEntry
	for range r.plugins {
		res := <-ch
		if res.err != nil {
			r.log.Warn("memory read error (skipped)", "err", res.err)
			continue
		}
		all = append(all, res.entries...)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].Timestamp.After(all[j].Timestamp)
	})
	return formatEntries(all)
}

// Write routes to the primary plugin.
func (r *Registry) Write(ctx context.Context, cfg *config.Config, e plugin.MemoryEntry) error {
	if r == nil {
		return nil
	}
	for _, p := range r.plugins {
		if p.Primary() {
			return p.Write(ctx, cfg, e)
		}
	}
	return fmt.Errorf("memory: no primary plugin")
}

func formatEntries(entries []plugin.MemoryEntry) string {
	if len(entries) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("# Long-term Memory\n")
	for _, e := range entries {
		content := truncateChars(e.Content, memoryMdMaxChars)
		sb.WriteString(content)
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

// truncateChars truncates s to at most maxChars Unicode code points, appending "…" if truncated.
func truncateChars(s string, maxChars int) string {
	if utf8.RuneCountInString(s) <= maxChars {
		return s
	}
	return string([]rune(s)[:maxChars]) + "…"
}
