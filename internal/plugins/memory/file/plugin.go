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
	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/kernel/agent"
	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/plugin"
	"github.com/google/uuid"
)

const shadowSystemPrompt = `Given the conversation exchange below, journal any new facts, decisions, goals, tools, or context worth keeping. Use memory_search() first to check if the fact is already in today's journal, then call remember() only for net-new information. If nothing new was shared, do not call remember().`

// Plugin is the filesystem memory plugin.
type Plugin struct {
	log      *slog.Logger
	mu       sync.Mutex
	shadowRt *sdkapi.Runtime
}

func NewPlugin(lg *slog.Logger) *Plugin {
	return &Plugin{log: lg}
}

func (p *Plugin) Name() string                { return "memory-file" }
func (p *Plugin) Start(context.Context) error { return nil }

func (p *Plugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.shadowRt != nil {
		_ = p.shadowRt.Close()
		p.shadowRt = nil
	}
	return nil
}

// PostTurnHandler implements plugin.PostTurnPlugin. It builds a shadow runtime with only the
// memory tools and returns a handler that journals net-new facts after each conversation turn.
func (p *Plugin) PostTurnHandler(cfg *config.Config) func(ctx context.Context, userMsg, assistantMsg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.shadowRt != nil {
		_ = p.shadowRt.Close()
		p.shadowRt = nil
	}
	tools := shadowTools(p.Tools(cfg))
	if len(tools) == 0 {
		return nil
	}
	rt, err := sdkapi.New(context.Background(), sdkapi.Options{
		ProjectRoot:   cfg.Agent.Workspace,
		ModelFactory:  agent.NewProvider(cfg),
		SystemPrompt:  shadowSystemPrompt,
		MaxIterations: 2,
		MaxSessions:   1,
		CustomTools:   tools,
	})
	if err != nil {
		p.log.Warn("memory-file: shadow runtime init failed", "err", err)
		return nil
	}
	p.shadowRt = rt
	log := p.log
	return func(ctx context.Context, userMsg, assistantMsg string) {
		if userMsg == "" && assistantMsg == "" {
			return
		}
		prompt := fmt.Sprintf("User: %s\n\nAssistant: %s", userMsg, assistantMsg)
		sessionID := uuid.New().String()
		go func() {
			resp, err := rt.Run(context.WithoutCancel(ctx), sdkapi.Request{
				Prompt:    prompt,
				SessionID: sessionID,
			})
			if err != nil {
				log.Debug("memory-file: shadow turn failed", "err", err)
				return
			}
			if resp != nil && resp.Result != nil && len(resp.Result.ToolCalls) > 0 {
				log.Info("memory-file: remembered", "content", resp.Result.ToolCalls[0].Arguments["content"])
			} else {
				log.Debug("memory-file: nothing to journal")
			}
		}()
	}
}

func shadowTools(all []tool.Tool) []tool.Tool {
	var out []tool.Tool
	for _, t := range all {
		switch t.Name() {
		case "remember", "memory_search":
			out = append(out, t)
		}
	}
	return out
}

var _ plugin.PostTurnPlugin = (*Plugin)(nil)

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
