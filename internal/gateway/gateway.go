package gateway

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/maven/internal/agent"
	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/channel/manager"
	"github.com/ageneralai/maven/internal/config"
	"github.com/ageneralai/maven/internal/cron"
	"github.com/ageneralai/maven/internal/health"
	"github.com/ageneralai/maven/internal/heartbeat"
	"github.com/ageneralai/maven/pkg/memory"
	"github.com/ageneralai/maven/internal/pipeline"
	"github.com/ageneralai/maven/pkg/prompt"
	"github.com/ageneralai/maven/internal/session"
	"github.com/ageneralai/maven/internal/skills"
	"github.com/ageneralai/maven/internal/slash"
	mavenlog "github.com/ageneralai/maven/pkg/log"
)

// RuntimeFactory builds the agent runtime used by the gateway pipeline.
type RuntimeFactory func(cfg *config.Config, sysPrompt string, skillRegs []api.SkillRegistration, cronSvc *cron.Service) (agent.Runtime, error)

// Options for creating a Gateway.
type Options struct {
	RuntimeFactory RuntimeFactory
	SignalChan     chan os.Signal
	HealthReporter health.HealthReporter
}

// DefaultRuntimeFactory wires agentsdk-go with the given skills and cron command/tool registration.
func DefaultRuntimeFactory(cfg *config.Config, sysPrompt string, skillRegs []api.SkillRegistration, cronSvc *cron.Service) (agent.Runtime, error) {
	return agent.NewSDKRuntime(cfg, sysPrompt, skillRegs, cronSvc)
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
	mem            *memory.MemoryStore
	skillRegs      []api.SkillRegistration
	sessions       *session.Router
	signalChan     chan os.Signal
	logger         mavenlog.PrintLogger
	liveness       health.HealthReporter
	hbCancel       context.CancelFunc
	applyMu        sync.Mutex
}

// New creates a Gateway with default options.
func New(cfg *config.Config) (*Gateway, error) {
	return NewWithOptions(cfg, Options{})
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
		g.logger.Printf("[gateway] skills load warning: %v", err)
	}
	return regs
}

// NewWithOptions creates a Gateway with a custom runtime factory (for tests).
// Pipeline runtime is unset until Apply; Run calls Apply before starting cron/pipeline goroutines.
func NewWithOptions(cfg *config.Config, opts Options) (*Gateway, error) {
	g := &Gateway{cfg: cfg, logger: mavenlog.Std()}
	g.bus = bus.NewMessageBus(config.DefaultBufSize, g.logger)
	g.mem = memory.NewMemoryStore(cfg.Agent.Workspace)
	router, routerErr := session.New(filepath.Join(cfg.Agent.Workspace, ".maven", "session-router.json"))
	if routerErr != nil {
		return nil, routerErr
	}
	g.sessions = router
	factory := opts.RuntimeFactory
	if factory == nil {
		factory = DefaultRuntimeFactory
	}
	g.runtimeFactory = factory
	g.channelMgr = manager.NewChannelManager(g.bus, g.logger)
	var pipe *pipeline.Pipeline
	exec := &gatewayTurnExecutor{pipeFn: func() *pipeline.Pipeline { return pipe }}
	cronDeliver := &cron.Deliver{Bus: g.bus, Channels: g.channelMgr, Log: g.logger}
	g.cron = cron.NewService(filepath.Join(config.ConfigDir(), "data", "cron", "jobs.json"), exec, cfg.Gateway.Cron.MaxConcurrentRuns, g.logger, cronDeliver)
	g.signalChan = opts.SignalChan
	liveness := health.OrHealthReporter(opts.HealthReporter)
	g.liveness = liveness
	sessRes := &session.SessionResolver{Router: g.sessions}
	posts := &agent.PostActionHandler{Sessions: g.sessions, Workspace: cfg.Agent.Workspace}
	pipe = pipeline.New(g.logger, g.bus, nil, sessRes, posts)
	pipe.Channels = g.channelMgr
	pipe.SlashRegistry = slash.BuiltIns(g.cron)
	g.pipe = pipe
	g.hb = heartbeat.New(cfg.Agent.Workspace, exec, 0, g.logger, heartbeat.WithHealthReporter(liveness))
	return g, nil
}

func (g *Gateway) validateReload(cfg *config.Config) error {
	if g.cfg != nil && cfg.Agent.Workspace != g.cfg.Agent.Workspace {
		return fmt.Errorf("reload: agent.workspace change not supported")
	}
	return nil
}

func (g *Gateway) buildRuntime(cfg *config.Config, sysPrompt string, skillRegs []api.SkillRegistration) (agent.Runtime, error) {
	return g.runtimeFactory(cfg, sysPrompt, skillRegs, g.cron)
}

func (g *Gateway) reloadPipeline(ctx context.Context, cfg *config.Config, rt agent.Runtime) error {
	return g.pipe.Reload(func() error { return g.channelMgr.Apply(ctx, cfg) }, rt, cfg.Agent.Workspace)
}

// Apply makes cfg the active gateway state: replaces channels via ChannelManager.Apply, builds a fresh
// runtime from the factory, swaps it into the pipeline under Reload semantics, refreshes SlashRegistry from cron,
// and restarts the heartbeat ticker tree. Idempotent retries use the same path.
func (g *Gateway) Apply(ctx context.Context, cfg *config.Config) error {
	g.applyMu.Lock()
	defer g.applyMu.Unlock()
	if err := g.validateReload(cfg); err != nil {
		return err
	}
	g.skillRegs = g.loadSkillRegs(cfg)
	sysPrompt := prompt.Build(cfg.Agent.Workspace, g.mem.GetMemoryContext())
	rt, err := g.buildRuntime(cfg, sysPrompt, g.skillRegs)
	if err != nil {
		return fmt.Errorf("runtime factory: %w", err)
	}
	if err := g.reloadPipeline(ctx, cfg, rt); err != nil {
		rt.Close()
		return fmt.Errorf("channels apply: %w", err)
	}
	g.cfg = cfg
	g.pipe.SlashRegistry = slash.BuiltIns(g.cron)
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
			g.logger.Printf("[gateway] heartbeat error: %v", err)
		}
	}()
}

// Run wires the gateway lifecycle: outbound dispatch goroutine → Apply desired config → cron → inbound pipeline goroutine → block on reload/signals/shutdown.
func (g *Gateway) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	g.runCancel = cancel
	go g.bus.DispatchOutbound(ctx)
	if err := g.Apply(ctx, g.cfg); err != nil {
		return fmt.Errorf("initial apply: %w", err)
	}
	g.logger.Printf("[gateway] channels started: %v", g.channelMgr.EnabledChannels())
	if err := g.cron.Start(ctx); err != nil {
		g.logger.Printf("[gateway] cron start warning: %v", err)
	}
	g.pipeWg.Add(1)
	go func() {
		defer g.pipeWg.Done()
		g.pipe.Run(ctx)
	}()
	g.liveness.Pulse(health.SignalGatewayReady)
	g.logger.Printf("[gateway] running on %s:%d", g.cfg.Gateway.Host, g.cfg.Gateway.Port)
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
				g.logger.Printf("[gateway] shutting down...")
				return g.Shutdown()
			case <-sigCh:
				g.logger.Printf("[gateway] shutting down...")
				return g.Shutdown()
			}
		}
		select {
		case <-ctx.Done():
			g.logger.Printf("[gateway] shutting down...")
			return g.Shutdown()
		case <-sigCh:
			g.logger.Printf("[gateway] shutting down...")
			return g.Shutdown()
		case <-reloadCh:
			newCfg, lerr := config.LoadConfig()
			if lerr != nil {
				g.logger.Printf("[gateway] reload load config: %v", lerr)
				continue
			}
			if aerr := g.Apply(ctx, newCfg); aerr != nil {
				g.logger.Printf("[gateway] reload apply: %v", aerr)
			} else {
				g.logger.Printf("[gateway] reloaded; gateway %s:%d; channels: %v", newCfg.Gateway.Host, newCfg.Gateway.Port, g.channelMgr.EnabledChannels())
			}
		}
	}
}

// Shutdown cancels heartbeat and the pipeline/dispatch ctx, drains the inbound loop, stops cron/channels/closes runtime and bus (order-sensitive).
func (g *Gateway) Shutdown() error {
	g.interruptRunLoops()
	g.pipeWg.Wait()
	g.cron.Stop()
	_ = g.channelMgr.StopAll()
	if g.pipe != nil {
		if rt := g.pipe.TakeRuntimeForShutdown(); rt != nil {
			rt.Close()
		}
	}
	if g.bus != nil {
		g.bus.Close()
	}
	g.logger.Printf("[gateway] shutdown complete")
	return nil
}
