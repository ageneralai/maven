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

	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/events"
	"github.com/ageneralai/maven/internal/kernel/plugin"
)

type Plugin struct {
	log         *slog.Logger
	newShadow   shadowRuntimeFactory
	mu          sync.Mutex
	rt          shadowRuntime
	eventBus    *events.Fanout
	unsubscribe func()
}

func NewPlugin(lg *slog.Logger) *Plugin {
	return &Plugin{log: lg, newShadow: defaultShadowRuntime}
}

func (p *Plugin) Name() string { return "memory-file" }

func (p *Plugin) SetEventBus(f *events.Fanout) { p.eventBus = f }

func (p *Plugin) Start(ctx context.Context) error {
	if p.eventBus != nil {
		p.unsubscribe = p.eventBus.Subscribe(events.EventTurnCompleted, p.onTurnCompleted)
	}
	return nil
}

func (p *Plugin) Stop() error {
	if p.unsubscribe != nil {
		p.unsubscribe()
		p.unsubscribe = nil
	}
	p.mu.Lock()
	oldRt := p.rt
	p.rt = nil
	p.mu.Unlock()
	if oldRt != nil {
		_ = oldRt.Close()
	}
	return nil
}

// ConfigureTurnJournal rebuilds the shadow runtime from cfg. Call on each gateway Apply.
func (p *Plugin) ConfigureTurnJournal(cfg *config.Config) {
	if !cfg.ShadowJournal.Enabled {
		p.mu.Lock()
		oldRt := p.rt
		p.rt = nil
		p.mu.Unlock()
		if oldRt != nil {
			go func() { _ = oldRt.Close() }()
		}
		return
	}
	newRt, err := p.newShadow(cfg, shadowSystemPrompt, shadowTools(p.Tools(cfg)))
	if err != nil {
		p.log.Warn("memory-file: shadow runtime init failed", "err", err)
		return
	}
	p.mu.Lock()
	oldRt := p.rt
	p.rt = newRt
	p.mu.Unlock()
	if oldRt != nil {
		go func() { _ = oldRt.Close() }()
	}
}

func (p *Plugin) onTurnCompleted(ctx context.Context, e events.Event) {
	ev, ok := e.Payload.(events.TurnCompleted)
	if !ok {
		return
	}
	if ev.UserMsg == "" && ev.AssistantMsg == "" {
		return
	}
	p.mu.Lock()
	rt := p.rt
	p.mu.Unlock()
	if rt == nil {
		return
	}
	runCtx, cancel := context.WithTimeout(ctx, shadowTurnTimeout)
	defer cancel()
	runShadowTurn(runCtx, rt, p.log, ev)
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
