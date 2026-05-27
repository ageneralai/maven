package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/agent"
	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/channel/manager"
	"github.com/ageneralai/maven/internal/config"
	"github.com/ageneralai/maven/internal/cron"
	"github.com/ageneralai/maven/internal/health"
	"github.com/ageneralai/maven/internal/heartbeat"
	"github.com/ageneralai/maven/internal/pipeline"
	mavsession "github.com/ageneralai/maven/internal/session"
	"github.com/ageneralai/maven/internal/skills"
	"github.com/ageneralai/maven/internal/slash"
	"github.com/ageneralai/maven/pkg/memory"
	"github.com/ageneralai/maven/pkg/plugin"
	"github.com/ageneralai/maven/pkg/prompt"
)

// RuntimeFactory builds the agent runtime used by the gateway pipeline.
type RuntimeFactory func(cfg *config.Config, sysPrompt string, skillRegs []api.SkillRegistration, cronSvc *cron.Service, pluginTools []tool.Tool, sessionStore api.SessionStore, lg *slog.Logger) (agent.Runtime, error)

// Options for creating a Gateway.
type Options struct {
	Logger         *slog.Logger
	RuntimeFactory RuntimeFactory
	SignalChan     chan os.Signal
	HealthReporter health.HealthReporter
}

// DefaultRuntimeFactory wires agentsdk-go with the given skills, cron command/tool registration, and gateway plugin tools.
func DefaultRuntimeFactory(cfg *config.Config, sysPrompt string, skillRegs []api.SkillRegistration, cronSvc *cron.Service, pluginTools []tool.Tool, sessionStore api.SessionStore, lg *slog.Logger) (agent.Runtime, error) {
	return agent.NewSDKRuntime(cfg, sysPrompt, skillRegs, cronSvc, pluginTools, sessionStore, lg)
}

// Gateway wires channels, bus, cron, heartbeat, and the inbound pipeline. Business logic lives in internal/pipeline.
type Gateway struct {
	cfg            *config.Config
	bus            *bus.MessageBus
	pipe           *pipeline.Pipeline
	pipeWg         sync.WaitGroup
	runCancel      context.CancelFunc
	channelMgr     *manager.ChannelManager
	cron           *cron.Service
	hb             *heartbeat.Service
	runtimeFactory RuntimeFactory
	plugins        *plugin.Registry
	mem            *memory.MemoryStore
	skillRegs      []api.SkillRegistration
	sessions       *mavsession.Router
	historyStore   *mavsession.Store
	signalChan     chan os.Signal
	logger         *slog.Logger
	liveness       health.HealthReporter
	hbCancel       context.CancelFunc
	applyMu        sync.Mutex
}

// New creates a Gateway. lg must be non-nil.
func New(cfg *config.Config, lg *slog.Logger) (*Gateway, error) {
	return NewWithOptions(cfg, Options{Logger: lg})
}

func (g *Gateway) loadSkillRegs(cfg *config.Config) []api.SkillRegistration {
	if !cfg.Skills.Enabled {
		return nil
	}
	skillDir := cfg.Skills.Dir
	if skillDir == "" {
		skillDir = filepath.Join(cfg.Agent.Workspace, "skills")
	}
	regs, err := skills.LoadSkills(skillDir, g.logger)
	if err != nil {
		g.logger.Warn("gateway skills load warning", "err", err)
	}
	return regs
}

// NewWithOptions creates a Gateway with a custom runtime factory (for tests).
// Pipeline runtime is unset until Apply; Run calls Apply before starting cron/pipeline goroutines.
func NewWithOptions(cfg *config.Config, opts Options) (*Gateway, error) {
	core, err := wireCore(cfg, opts)
	if err != nil {
		return nil, err
	}
	planes, err := wirePlanes(core)
	if err != nil {
		return nil, err
	}
	return &Gateway{
		cfg:            core.cfg,
		logger:         core.logger,
		bus:            core.bus,
		sessions:       core.sessions,
		historyStore:   core.historyStore,
		mem:            core.mem,
		liveness:       core.liveness,
		signalChan:     core.signalChan,
		runtimeFactory: core.runtimeFactory,
		channelMgr:     planes.channelMgr,
		pipe:           planes.pipe,
		cron:           planes.cron,
		hb:             planes.hb,
		plugins:        planes.plugins,
	}, nil
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
	return g.runtimeFactory(cfg, sysPrompt, skillRegs, g.cron, pluginTools, g.historyStore, g.logger)
}

func (g *Gateway) reloadPipeline(ctx context.Context, cfg *config.Config, rt agent.Runtime, slashReg *slash.Registry) error {
	return g.pipe.Reload(func() error { return g.channelMgr.Apply(ctx, cfg) }, rt, cfg.Agent.Workspace, slashReg)
}

// Apply makes cfg the active gateway state: replaces channels via ChannelManager.Apply, builds a fresh
// runtime from the factory, swaps it into the pipeline under Reload semantics, refreshes slash builtins from cron,
// and restarts the heartbeat ticker tree. Idempotent retries use the same path.
func (g *Gateway) Apply(ctx context.Context, cfg *config.Config) error {
	g.applyMu.Lock()
	defer g.applyMu.Unlock()
	if err := g.validateReload(cfg); err != nil {
		return err
	}
	g.skillRegs = g.loadSkillRegs(cfg)
	sysPrompt, err := prompt.Build(cfg.Agent.Workspace, g.mem.GetMemoryContext())
	if err != nil {
		return fmt.Errorf("system prompt: %w", err)
	}
	slashReg, err := slash.BuiltIns(g.cron)
	if err != nil {
		return fmt.Errorf("slash builtins: %w", err)
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
	g.startHeartbeat(ctx)
	return nil
}

func (g *Gateway) interruptRunLoops() {
	if g.hbCancel != nil {
		g.hbCancel()
		g.hbCancel = nil
	}
	if g.runCancel != nil {
		g.runCancel()
		g.runCancel = nil
	}
}

func (g *Gateway) startHeartbeat(ctx context.Context) {
	if g.hbCancel != nil {
		g.hbCancel()
		g.hbCancel = nil
	}
	hbCtx, cancel := context.WithCancel(ctx)
	g.hbCancel = cancel
	go func() {
		if err := g.hb.Start(hbCtx); err != nil {
			g.logger.Error("gateway heartbeat error", "err", err)
		}
	}()
}

// Run wires the gateway lifecycle: outbound dispatch goroutine → Apply desired config → cron → inbound pipeline goroutine → block on reload/signals/shutdown.
func (g *Gateway) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	g.runCancel = cancel
	go g.bus.DispatchOutbound(ctx)
	if g.plugins != nil {
		if err := g.plugins.Start(ctx); err != nil {
			return fmt.Errorf("plugins start: %w", err)
		}
	}
	if err := g.Apply(ctx, g.cfg); err != nil {
		return fmt.Errorf("initial apply: %w", err)
	}
	g.logger.Info("gateway channels started", "channels", g.channelMgr.EnabledChannels())
	if err := g.cron.Start(ctx); err != nil {
		g.logger.Warn("gateway cron start warning", "err", err)
	}
	g.pipeWg.Add(1)
	go func() {
		defer g.pipeWg.Done()
		g.pipe.Run(ctx)
	}()
	g.liveness.Pulse(health.SignalGatewayReady)
	g.logger.Info("gateway running", "host", g.cfg.Gateway.Host, "port", g.cfg.Gateway.Port)
	debounce := time.Duration(g.cfg.Gateway.ReloadDebounceMs) * time.Millisecond
	var reloadCh <-chan struct{}
	var stopReload func()
	if g.cfg.Gateway.HotReload {
		reloadCh, stopReload = config.WatchConfigPath(ctx, config.ConfigPath(), debounce)
		defer stopReload()
	}
	sigCh := g.signalChan
	if sigCh == nil {
		sigCh = make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	}
	for {
		if !g.cfg.Gateway.HotReload {
			select {
			case <-ctx.Done():
				g.logger.Info("gateway shutting down")
				return g.Shutdown()
			case <-sigCh:
				g.logger.Info("gateway shutting down")
				return g.Shutdown()
			}
		}
		select {
		case <-ctx.Done():
			g.logger.Info("gateway shutting down")
			return g.Shutdown()
		case <-sigCh:
			g.logger.Info("gateway shutting down")
			return g.Shutdown()
		case <-reloadCh:
			newCfg, lerr := config.LoadConfig()
			if lerr != nil {
				g.logger.Error("gateway reload load config error", "err", lerr)
				continue
			}
			if aerr := g.Apply(ctx, newCfg); aerr != nil {
				g.logger.Error("gateway reload apply error", "err", aerr)
			} else {
				g.logger.Info("gateway reloaded", "host", newCfg.Gateway.Host, "port", newCfg.Gateway.Port, "channels", g.channelMgr.EnabledChannels())
			}
		}
	}
}

// Shutdown cancels heartbeat and the pipeline/dispatch ctx, drains the inbound loop, stops cron/channels/closes runtime and bus (order-sensitive).
func (g *Gateway) Shutdown() error {
	g.interruptRunLoops()
	if g.hb != nil {
		g.hb.Stop()
	}
	g.pipeWg.Wait()
	g.cron.Stop()
	if g.plugins != nil {
		if err := g.plugins.Stop(); err != nil {
			g.logger.Error("gateway plugins stop error", "err", err)
		}
	}
	_ = g.channelMgr.StopAll()
	if g.pipe != nil {
		if rt := g.pipe.TakeRuntimeForShutdown(); rt != nil {
			rt.Close()
		}
	}
	if g.bus != nil {
		g.bus.Close()
	}
	g.logger.Info("gateway shutdown complete")
	return nil
}
