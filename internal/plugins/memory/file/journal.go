package memory

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	sdkapi "github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/kernel/agent"
	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/hook"
	"github.com/google/uuid"
)

const (
	shadowSystemPrompt = `Given the conversation exchange below, journal any new facts, decisions, goals, tools, or context worth keeping. Use memory_search() first to check if the fact is already in today's journal, then call remember() only for net-new information. If nothing new was shared, do not call remember().`
	shadowTurnTimeout  = 60 * time.Second
)

type shadowRuntime interface {
	Run(ctx context.Context, req sdkapi.Request) (*sdkapi.Response, error)
	Close() error
}

type shadowRuntimeFactory func(cfg *config.Config, prompt string, tools []tool.Tool) (shadowRuntime, error)

func defaultShadowRuntime(cfg *config.Config, prompt string, tools []tool.Tool) (shadowRuntime, error) {
	return sdkapi.New(context.Background(), sdkapi.Options{
		ProjectRoot:   cfg.Agent.Workspace,
		ModelFactory:  agent.NewProvider(cfg),
		SystemPrompt:  prompt,
		MaxIterations: 2,
		MaxSessions:   1,
		CustomTools:   tools,
	})
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

func runShadowTurn(ctx context.Context, rt shadowRuntime, log *slog.Logger, ev hook.PostTurnEvent) {
	prompt := fmt.Sprintf("User: %s\n\nAssistant: %s", ev.UserMsg, ev.AssistantMsg)
	resp, err := rt.Run(ctx, sdkapi.Request{Prompt: prompt, SessionID: uuid.New().String()})
	if err != nil {
		log.Debug("memory-file: shadow turn failed", "err", err)
		return
	}
	if resp == nil || resp.Result == nil {
		log.Debug("memory-file: nothing to journal")
		return
	}
	remembered := false
	for _, tc := range resp.Result.ToolCalls {
		if tc.Name != "remember" {
			continue
		}
		remembered = true
		log.Info("memory-file: remembered", "content", tc.Arguments["content"])
	}
	if !remembered {
		log.Debug("memory-file: nothing to journal")
	}
}
