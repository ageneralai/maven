package memory

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/plugin"
	"log/slog"
)

// Plugin is the filesystem memory plugin.
type Plugin struct {
	log *slog.Logger
}

func NewPlugin(lg *slog.Logger) *Plugin {
	return &Plugin{log: lg}
}

func (p *Plugin) Name() string                { return "memory-file" }
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
		Timestamp: time.Time{},
	}}, nil
}

func memoryDir(cfg *config.Config) string {
	return filepath.Join(cfg.Agent.Workspace, "memory")
}
