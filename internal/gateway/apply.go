package gateway

import (
	"context"
	"fmt"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/kernel/agent"
	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/prompt"
	kmemory "github.com/ageneralai/maven/internal/kernel/memory"
	"github.com/ageneralai/maven/internal/kernel/plugin"
	"github.com/ageneralai/maven/internal/kernel/slash"
)

func defaultMemoryQuery() plugin.MemoryQuery {
	return plugin.MemoryQuery{}
}

func buildSysPrompt(ctx context.Context, workspace string, memReg *kmemory.Registry, cfg *config.Config) (string, error) {
	template, err := prompt.BuildTemplate(workspace)
	if err != nil {
		return "", fmt.Errorf("system prompt: %w", err)
	}
	memCtx := memReg.Context(ctx, cfg, defaultMemoryQuery())
	if memCtx == "" {
		return template, nil
	}
	return template + "\n\n" + memCtx, nil
}

func (g *Gateway) loadSkillRegs(cfg *config.Config) []api.SkillRegistration {
	if g.plugins == nil {
		return nil
	}
	return g.plugins.Skills(cfg)
}

func (g *Gateway) validateReload(cfg *config.Config) error {
	if g.cfg != nil && cfg.Agent.Workspace != g.cfg.Agent.Workspace {
		return fmt.Errorf("reload: agent.workspace change not supported")
	}
	return nil
}

func (g *Gateway) buildRuntime(cfg *config.Config, sysPrompt string, skillRegs []api.SkillRegistration) (agent.Runtime, error) {
	var pluginTools []tool.Tool
	if g.plugins != nil {
		pluginTools = g.plugins.Tools(cfg)
	}
	return g.runtimeFactory(cfg, sysPrompt, skillRegs, pluginTools, g.historyStore, g.logger)
}

func (g *Gateway) reloadPipeline(ctx context.Context, cfg *config.Config, rt agent.Runtime, slashReg *slash.Registry) error {
	return g.pipe.Reload(func() error { return g.channelMgr.Apply(ctx, cfg) }, rt, cfg.Agent.Workspace, slashReg)
}

// Apply makes cfg the active gateway state: replaces channels via ChannelManager.Apply, builds a fresh
// runtime from the factory, swaps it into the pipeline under Reload semantics, refreshes slash commands,
// and restarts background triggers. Idempotent retries use the same path.
func (g *Gateway) Apply(ctx context.Context, cfg *config.Config) error {
	g.applyMu.Lock()
	defer g.applyMu.Unlock()
	if err := g.validateReload(cfg); err != nil {
		return err
	}
	g.stopTriggers()
	if err := g.ensureCronService(); err != nil {
		return fmt.Errorf("cron service: %w", err)
	}
	g.skillRegs = g.loadSkillRegs(cfg)
	sysPrompt, err := buildSysPrompt(ctx, cfg.Agent.Workspace, g.memReg, cfg)
	if err != nil {
		return err
	}
	slashReg, err := slash.BuiltIns()
	if err != nil {
		return err
	}
	if g.plugins != nil {
		if err := slash.RegisterPluginCommands(slashReg, g.plugins.SlashCommands(cfg)); err != nil {
			return fmt.Errorf("slash plugins: %w", err)
		}
	}
	rt, err := g.buildRuntime(cfg, sysPrompt, g.skillRegs)
	if err != nil {
		return fmt.Errorf("runtime factory: %w", err)
	}
	if err := g.reloadPipeline(ctx, cfg, rt, slashReg); err != nil {
		rt.Close()
		return fmt.Errorf("channels apply: %w", err)
	}
	g.cfg = cfg
	g.wirePostActionHooks()
	return g.startTriggers(ctx)
}

// wirePostActionHooks registers pre-compact flush callback on the postaction handler.
func (g *Gateway) wirePostActionHooks() {
	if g.pipe == nil {
		return
	}
	posts := g.pipe.Posts()
	if posts == nil {
		return
	}
	posts.SetPreCompactFlush(func(ctx context.Context, sessionID string) {
		const flushPrompt = "Before this conversation compacts, use the remember tool to save any important facts, preferences, or decisions that should persist to long-term memory. Be concise."
		_, _ = g.pipe.RunTurn(ctx, flushPrompt, sessionID)
	})
}
