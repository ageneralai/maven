package gateway

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/kernel/agent"
	"github.com/ageneralai/maven/internal/kernel/channel"
	"github.com/ageneralai/maven/internal/kernel/config"
	mavenlog "github.com/ageneralai/maven/internal/kernel/log"
	kmemory "github.com/ageneralai/maven/internal/kernel/memory"
	"github.com/ageneralai/maven/internal/kernel/plugin"
	"github.com/ageneralai/maven/internal/kernel/prompt"
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

func (g *Gateway) applyLogLevel(cfg *config.Config) error {
	level, err := config.ParseLogLevel(cfg.Logging.Level)
	if err != nil {
		return err
	}
	mavenlog.SetLevel(level)
	return nil
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
	g.channelMgr.SetPipelineSlashCommands(pipelineSlashDefinitions(slashReg.Definitions()))
	return g.pipe.Reload(func() error { return g.channelMgr.Apply(ctx, cfg) }, rt, cfg.Agent.Workspace, slashReg)
}

func pipelineSlashDefinitions(defs []slash.Definition) []channels.PipelineSlashDefinition {
	if len(defs) == 0 {
		return nil
	}
	out := make([]channels.PipelineSlashDefinition, 0, len(defs))
	for _, def := range defs {
		out = append(out, channels.PipelineSlashDefinition{
			Name:        def.Name,
			Description: def.Description,
		})
	}
	return out
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
	if err := g.applyLogLevel(cfg); err != nil {
		return fmt.Errorf("logging.level: %w", err)
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
	if err := g.registerReloadSlash(slashReg); err != nil {
		return fmt.Errorf("slash reload: %w", err)
	}
	if err := slashReg.Register(
		slash.Definition{Name: "status", Description: "Show gateway status: cron jobs, memory size."},
		slash.HandlerFunc(func(ctx context.Context, inv slash.Invocation) (slash.Result, error) {
			return g.slashStatus(ctx), nil
		}),
	); err != nil {
		return fmt.Errorf("slash status: %w", err)
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

func (g *Gateway) slashStatus(_ context.Context) slash.Result {
	var parts []string
	if svc := g.cronService(); svc != nil {
		jobs := svc.ListJobs()
		enabled := 0
		for _, j := range jobs {
			if j.Enabled {
				enabled++
			}
		}
		parts = append(parts, fmt.Sprintf("🕐 Cron jobs: %d active / %d total", enabled, len(jobs)))
	}
	if g.cfg != nil {
		memPath := filepath.Join(g.cfg.Agent.Workspace, "memory", "MEMORY.md")
		if fi, err := os.Stat(memPath); err == nil {
			parts = append(parts, fmt.Sprintf("🧠 MEMORY.md: %d bytes", fi.Size()))
		} else {
			parts = append(parts, "🧠 MEMORY.md: empty")
		}
	}
	if len(parts) == 0 {
		return slash.Result{Output: "No status available."}
	}
	return slash.Result{Output: strings.Join(parts, "\n")}
}

// wirePostActionHooks registers pre-compact flush and post-turn journaling callbacks.
func (g *Gateway) wirePostActionHooks() {
	if g.pipe == nil {
		return
	}
	posts := g.pipe.Posts()
	if posts != nil {
		posts.SetPreCompactFlush(func(ctx context.Context, sessionID string) {
			const flushPrompt = "Before this conversation compacts, use the remember tool to save any important facts, preferences, or decisions that should persist to long-term memory. Be concise."
			_, _ = g.pipe.RunTurn(ctx, flushPrompt, sessionID)
		})
	}
	if g.plugins != nil {
		g.plugins.ConfigureTurnJournals(g.cfg)
	}
}
