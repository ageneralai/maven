package gateway

import (
	"context"
	"fmt"
	"log/slog"

	sdkapi "github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/kernel/agent"
	"github.com/ageneralai/maven/internal/kernel/config"
	fmemory "github.com/ageneralai/maven/internal/plugins/memory/file"
	"github.com/google/uuid"
)

const journalerSystemPrompt = `Given the conversation exchange below, journal any new facts, decisions, goals, tools, or context worth keeping. Use memory_search() first to check if the fact is already in today's journal, then call remember() only for net-new information. If nothing new was shared, do not call remember().`

// journaler runs a lightweight shadow turn after each main agent turn to persist net-new facts.
type journaler struct {
	rt  *sdkapi.Runtime
	log *slog.Logger
}

func newJournaler(cfg *config.Config, memPlug *fmemory.Plugin, lg *slog.Logger) (*journaler, error) {
	tools := journalerToolsFrom(memPlug, cfg)
	if len(tools) == 0 {
		return nil, fmt.Errorf("journaler: remember tool not found in memory plugin")
	}
	provider := agent.NewProvider(cfg)
	rt, err := sdkapi.New(context.Background(), sdkapi.Options{
		ProjectRoot:   cfg.Agent.Workspace,
		ModelFactory:  provider,
		SystemPrompt:  journalerSystemPrompt,
		MaxIterations: 2,
		MaxSessions:   1,
		CustomTools:   tools,
	})
	if err != nil {
		return nil, fmt.Errorf("journaler runtime: %w", err)
	}
	return &journaler{rt: rt, log: lg}, nil
}

// Journal fires off a shadow turn with the exchange. Errors are logged only.
func (j *journaler) Journal(ctx context.Context, userMsg, assistantMsg string) {
	if userMsg == "" && assistantMsg == "" {
		return
	}
	prompt := fmt.Sprintf("User: %s\n\nAssistant: %s", userMsg, assistantMsg)
	sessionID := uuid.New().String()
	go func() {
		// j.log.Info("journaler shadow turn start")
		resp, err := j.rt.Run(context.WithoutCancel(ctx), sdkapi.Request{
			Prompt:    prompt,
			SessionID: sessionID,
		})
		if err != nil {
			j.log.Debug("journaler shadow turn failed", "err", err)
			return
		}
		if resp != nil && resp.Result != nil && len(resp.Result.ToolCalls) > 0 {
			j.log.Info("journaler remembered", "content", resp.Result.ToolCalls[0].Arguments["content"])
		} else {
			j.log.Debug("journaler: nothing to journal")
		}
	}()
}

func (j *journaler) close() {
	_ = j.rt.Close()
}

func journalerToolsFrom(memPlug *fmemory.Plugin, cfg *config.Config) []tool.Tool {
	var out []tool.Tool
	for _, t := range memPlug.Tools(cfg) {
		switch t.Name() {
		case "remember", "memory_search":
			out = append(out, t)
		}
	}
	return out
}
