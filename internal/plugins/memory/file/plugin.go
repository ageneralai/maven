package memory

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	sdkapi "github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/hook"
	"github.com/ageneralai/maven/internal/kernel/plugin"
)

type Plugin struct {
	log       *slog.Logger
	newShadow shadowRuntimeFactory
	mu        sync.Mutex
	rt        *sdkapi.Runtime
}

var _ plugin.PostTurnPlugin = (*Plugin)(nil)

func NewPlugin(lg *slog.Logger) *Plugin {
	return &Plugin{log: lg, newShadow: defaultShadowRuntime}
}

func (p *Plugin) Name() string                { return "memory-file" }
func (p *Plugin) Start(context.Context) error { return nil }

func (p *Plugin) Stop() error {
	p.mu.Lock()
	oldRt := p.rt
	p.rt = nil
	p.mu.Unlock()
	if oldRt != nil {
		_ = oldRt.Close()
	}
	return nil
}

func (p *Plugin) PostTurnHandler(cfg *config.Config) hook.PostTurnHandler {
	newRt, err := p.newShadow(cfg, shadowSystemPrompt, shadowTools(p.Tools(cfg)))
	if err != nil {
		p.log.Warn("memory-file: shadow runtime init failed", "err", err)
		return nil
	}
	p.mu.Lock()
	oldRt := p.rt
	p.rt = newRt
	p.mu.Unlock()
	if oldRt != nil {
		go func() { _ = oldRt.Close() }()
	}
	log := p.log
	return func(ctx context.Context, ev hook.PostTurnEvent) {
		if ev.UserMsg == "" && ev.AssistantMsg == "" {
			return
		}
		runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shadowTurnTimeout)
		defer cancel()
		runShadowTurn(runCtx, newRt, log, ev)
	}
}

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
