package memory

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/plugin"
	"log/slog"
)

// Plugin is the filesystem memory plugin. It is always the primary write target.
type Plugin struct {
	log       *slog.Logger
	refreshFn func(context.Context) error
}

func NewPlugin(lg *slog.Logger) *Plugin {
	return &Plugin{log: lg}
}

func (p *Plugin) SetRefreshFn(fn func(context.Context) error) {
	p.refreshFn = fn
}

func (p *Plugin) Name() string                { return "memory-file" }
func (p *Plugin) Primary() bool               { return true }
func (p *Plugin) Start(context.Context) error { return nil }
func (p *Plugin) Stop() error                 { return nil }

func (p *Plugin) Read(ctx context.Context, cfg *config.Config, q plugin.MemoryQuery) ([]plugin.MemoryEntry, error) {
	dir := memoryDir(cfg)
	data, err := os.ReadFile(filepath.Join(dir, "MEMORY.md"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("memory file read: %w", err)
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return nil, nil
	}
	return []plugin.MemoryEntry{{
		Source:    "file:MEMORY.md",
		Content:   content,
		Kind:      plugin.MemoryKindFact,
		Timestamp: time.Time{},
	}}, nil
}

func (p *Plugin) Write(ctx context.Context, cfg *config.Config, e plugin.MemoryEntry) error {
	dir := memoryDir(cfg)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("memory dir: %w", err)
	}
	switch e.Kind {
	case plugin.MemoryKindEvent:
		path := filepath.Join(dir, time.Now().Format("2006-01-02")+".md")
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		_, err = fmt.Fprintf(f, "%s\n", strings.TrimSpace(e.Content))
		return err
	case plugin.MemoryKindPreference:
		path := filepath.Join(dir, "MEMORY.md")
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		_, err = fmt.Fprintf(f, "\n## Preferences\n%s\n", strings.TrimSpace(e.Content))
		return err
	default:
		path := filepath.Join(dir, "MEMORY.md")
		return os.WriteFile(path, []byte(strings.TrimSpace(e.Content)+"\n"), 0o600)
	}
}

func memoryDir(cfg *config.Config) string {
	return filepath.Join(cfg.Agent.Workspace, "memory")
}

func readDailyFiles(dir string, q plugin.MemoryQuery) ([]plugin.MemoryEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var dateFiles []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".md") && name != "MEMORY.md" {
			dateFiles = append(dateFiles, name)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dateFiles)))
	cutoff := time.Time{}
	if q.MaxAge > 0 {
		cutoff = time.Now().Add(-q.MaxAge)
	}
	var out []plugin.MemoryEntry
	for _, name := range dateFiles {
		dateStr := strings.TrimSuffix(name, ".md")
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		if !cutoff.IsZero() && t.Before(cutoff) {
			break
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		out = append(out, plugin.MemoryEntry{
			Source:    "file:" + name,
			Content:   content,
			Kind:      plugin.MemoryKindEvent,
			Timestamp: t,
		})
		if q.Limit > 0 && len(out) >= q.Limit {
			break
		}
	}
	return out, nil
}
