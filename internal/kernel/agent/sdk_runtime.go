package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/ageneral-agents-go/pkg/model"
	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/task"
)

type runtimeAdapter struct {
	rt *api.Runtime
}

// api.Runtime does not satisfy Runtime: Close() is error-returning on *api.Runtime.
var _ Runtime = (*runtimeAdapter)(nil)

func (r *runtimeAdapter) Run(ctx context.Context, req api.Request) (*api.Response, error) {
	return r.rt.Run(ctx, req)
}

func (r *runtimeAdapter) RunStream(ctx context.Context, req api.Request) (<-chan api.StreamEvent, error) {
	return r.rt.RunStream(ctx, req)
}

func (r *runtimeAdapter) Close() {
	_ = r.rt.Close()
}

// NewProvider constructs the model factory for the given config.
func NewProvider(cfg *config.Config) api.ModelFactory {
	switch cfg.Provider.Type {
	case "openai":
		return &model.OpenAIProvider{
			APIKey:    cfg.Provider.APIKey,
			BaseURL:   cfg.Provider.BaseURL,
			ModelName: cfg.Agent.Model,
			MaxTokens: cfg.Agent.MaxTokens,
		}
	default:
		return &model.AnthropicProvider{
			APIKey:    cfg.Provider.APIKey,
			BaseURL:   cfg.Provider.BaseURL,
			ModelName: cfg.Agent.Model,
			MaxTokens: cfg.Agent.MaxTokens,
		}
	}
}

// NewSDKRuntime constructs the default ageneral-agents-go runtime. Slash commands are handled in the gateway pipeline (kernel/slash), not via api.Options. pluginTools come from the gateway registry (e.g. ACP).
func NewSDKRuntime(cfg *config.Config, sysPrompt string, skillRegs []api.SkillRegistration, pluginTools []tool.Tool, sessionStore api.SessionStore, lg *slog.Logger) (Runtime, error) {
	provider := NewProvider(cfg)
	taskHolder := &task.RuntimeHolder{}
	customTools := append([]tool.Tool{}, pluginTools...)
	customTools = append(customTools, task.Tools(cfg.Tools.Task, taskHolder)...)
	rt, err := api.New(context.Background(), api.Options{
		ProjectRoot:   cfg.Agent.Workspace,
		ModelFactory:  provider,
		SystemPrompt:  sysPrompt,
		MaxIterations: cfg.Agent.MaxToolIterations,
		MCPServers:    cfg.MCP.Servers,
		AutoCompact: api.CompactConfig{
			Enabled:       cfg.AutoCompact.Enabled,
			Threshold:     cfg.AutoCompact.Threshold,
			PreserveCount: cfg.AutoCompact.PreserveCount,
		},
		Skills:       skillRegs,
		CustomTools:  customTools,
		SessionStore: sessionStore,
	})
	if err != nil {
		return nil, fmt.Errorf("create runtime: %w", err)
	}
	taskHolder.Set(rt)
	return &runtimeAdapter{rt: rt}, nil
}
