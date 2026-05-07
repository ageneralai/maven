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

	"github.com/ageneralai/maven/internal/agent"
	"github.com/ageneralai/maven/internal/automation"
	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/channel"
	"github.com/ageneralai/maven/internal/config"
	"github.com/ageneralai/maven/internal/cron"
	"github.com/ageneralai/maven/internal/cronsession"
	"github.com/ageneralai/maven/internal/heartbeat"
	"github.com/ageneralai/maven/internal/heartbeatsession"
	mavenlog "github.com/ageneralai/maven/internal/log"
	"github.com/ageneralai/maven/internal/memory"
	"github.com/ageneralai/maven/internal/pipeline"
	"github.com/ageneralai/maven/internal/prompt"
	"github.com/ageneralai/maven/internal/session"
	"github.com/ageneralai/maven/internal/skills"
	"github.com/cexll/agentsdk-go/pkg/api"
)

const heartbeatSkipReasonAutomationLaneBusy = "automation_lane_busy"

// RuntimeFactory builds the agent runtime used by the gateway pipeline and automation lane.
type RuntimeFactory func(cfg *config.Config, sysPrompt string, skillRegs []api.SkillRegistration, cronSvc *cron.Service) (agent.Runtime, error)

// Options for creating a Gateway.
type Options struct {
	RuntimeFactory RuntimeFactory
	SignalChan     chan os.Signal
}

// DefaultRuntimeFactory wires agentsdk-go with the given skills and cron command/tool registration.
func DefaultRuntimeFactory(cfg *config.Config, sysPrompt string, skillRegs []api.SkillRegistration, cronSvc *cron.Service) (agent.Runtime, error) {
	return agent.NewSDKRuntime(cfg, sysPrompt, skillRegs, cronSvc)
}

// Gateway wires channels, bus, cron, heartbeat, and the inbound pipeline. Business logic lives in internal/pipeline.
type Gateway struct {
	cfg                 *config.Config
	bus                 *bus.MessageBus
	pipe                *pipeline.Pipeline
	channels            *channel.ChannelManager
	cron                *cron.Service
	hb                  *heartbeat.Service
	lane                *automation.Lane
	runtimeFactory      RuntimeFactory
	mem                 *memory.MemoryStore
	skillRegs           []api.SkillRegistration
	sessions            *session.Router
	signalChan          chan os.Signal
	logger              mavenlog.PrintLogger
	hbCancel            context.CancelFunc
	applyMu             sync.Mutex
	initialChannelsDone bool
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
func NewWithOptions(cfg *config.Config, opts Options) (*Gateway, error) {
	g := &Gateway{cfg: cfg, logger: mavenlog.Std()}
	g.bus = bus.NewMessageBus(config.DefaultBufSize, g.logger)
	g.mem = memory.NewMemoryStore(cfg.Agent.Workspace)
	router, routerErr := session.New(filepath.Join(cfg.Agent.Workspace, ".maven", "session-router.json"))
	if routerErr != nil {
		return nil, routerErr
	}
	g.sessions = router
	g.skillRegs = g.loadSkillRegs(cfg)
	sysPrompt := prompt.Build(cfg.Agent.Workspace, g.mem.GetMemoryContext())
	cronStorePath := filepath.Join(config.ConfigDir(), "data", "cron", "jobs.json")
	g.cron = cron.NewService(cronStorePath, g.logger)
	g.lane = &automation.Lane{}
	factory := opts.RuntimeFactory
	if factory == nil {
		factory = DefaultRuntimeFactory
	}
	g.runtimeFactory = factory
	rt, err := factory(cfg, sysPrompt, g.skillRegs, g.cron)
	if err != nil {
		return nil, err
	}
	g.signalChan = opts.SignalChan
	sessRes := &agent.SessionResolver{Router: g.sessions}
	posts := &agent.PostActionHandler{Sessions: g.sessions, Workspace: cfg.Agent.Workspace}
	g.pipe = pipeline.New(g.logger, g.bus, rt, sessRes, posts)
	g.channels = channel.NewChannelManager(g.bus, g.logger)
	g.pipe.Channels = g.channels
	g.cron.OnJob = func(job cron.CronJob) (string, error) {
		if err := job.Payload.Validate(); err != nil {
			return "", err
		}
		var out string
		if err := g.lane.RunAlways(context.Background(), func(ctx context.Context) error {
			var runErr error
			out, runErr = agent.RunText(ctx, g.pipe.CurrentRuntime(), job.Payload.Message, cronsession.SessionKey(job.ID), nil)
			return runErr
		}); err != nil {
			return "", err
		}
		if job.Payload.Deliver {
			ch := g.channels.GetChannel(job.Payload.Channel)
			if ch != nil && ch.Capabilities().ReactiveOnly {
				g.logger.Printf("[gateway] cron deliver skipped: channel %q is reactive-only", job.Payload.Channel)
			} else {
				g.bus.Outbound <- bus.OutboundMessage{
					Channel: job.Payload.Channel,
					ChatID:  job.Payload.To,
					Content: out,
				}
			}
		}
		return out, nil
	}
	g.hb = heartbeat.New(cfg.Agent.Workspace, func(hbPrompt string) (string, error) {
		var out string
		ran, err := g.lane.TryRun(context.Background(), func(ctx context.Context) error {
			var runErr error
			out, runErr = agent.RunText(ctx, g.pipe.CurrentRuntime(), hbPrompt, heartbeatsession.SessionKey(), nil)
			return runErr
		})
		if !ran {
			g.logger.Printf("[heartbeat] skipped: %s", heartbeatSkipReasonAutomationLaneBusy)
			return "", nil
		}
		return out, err
	}, 0, g.logger)
	return g, nil
}

// Apply rebuilds the runtime from cfg, swaps the pipeline runtime, replaces channels, and restarts heartbeat.
func (g *Gateway) Apply(ctx context.Context, cfg *config.Config) error {
	g.applyMu.Lock()
	defer g.applyMu.Unlock()
	if g.cfg != nil && cfg.Agent.Workspace != g.cfg.Agent.Workspace {
		return fmt.Errorf("reload: agent.workspace change not supported")
	}
	if !g.initialChannelsDone {
		if err := g.channels.Apply(ctx, cfg); err != nil {
			return fmt.Errorf("channels apply: %w", err)
		}
		g.skillRegs = g.loadSkillRegs(cfg)
		g.cfg = cfg
		g.initialChannelsDone = true
		g.startHeartbeat(ctx)
		return nil
	}
	skillRegs := g.loadSkillRegs(cfg)
	sysPrompt := prompt.Build(cfg.Agent.Workspace, g.mem.GetMemoryContext())
	newRt, err := g.runtimeFactory(cfg, sysPrompt, skillRegs, g.cron)
	if err != nil {
		return fmt.Errorf("runtime factory: %w", err)
	}
	oldRt := g.pipe.CurrentRuntime()
	g.pipe.SetRuntime(newRt)
	if g.pipe.Posts != nil {
		g.pipe.Posts.Workspace = cfg.Agent.Workspace
	}
	if err := g.channels.Apply(ctx, cfg); err != nil {
		g.pipe.SetRuntime(oldRt)
		newRt.Close()
		return fmt.Errorf("channels apply: %w", err)
	}
	if oldRt != nil {
		oldRt.Close()
	}
	g.skillRegs = skillRegs
	g.cfg = cfg
	g.startHeartbeat(ctx)
	return nil
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

// Run starts outbound dispatch, applies config (channels + runtime alignment), cron, heartbeat, the inbound pipeline, and blocks until shutdown or hot reload errors.
func (g *Gateway) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go g.bus.DispatchOutbound(ctx)
	if err := g.Apply(ctx, g.cfg); err != nil {
		return fmt.Errorf("initial apply: %w", err)
	}
	g.logger.Printf("[gateway] channels started: %v", g.channels.EnabledChannels())
	if err := g.cron.Start(ctx); err != nil {
		g.logger.Printf("[gateway] cron start warning: %v", err)
	}
	go g.pipe.Run(ctx)
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
				g.logger.Printf("[gateway] reloaded; gateway %s:%d; channels: %v", newCfg.Gateway.Host, newCfg.Gateway.Port, g.channels.EnabledChannels())
			}
		}
	}
}

// Shutdown stops cron, channels, heartbeat, and closes the agent runtime.
func (g *Gateway) Shutdown() error {
	if g.hbCancel != nil {
		g.hbCancel()
		g.hbCancel = nil
	}
	g.cron.Stop()
	_ = g.channels.StopAll()
	if g.pipe != nil {
		if rt := g.pipe.CurrentRuntime(); rt != nil {
			rt.Close()
		}
	}
	g.logger.Printf("[gateway] shutdown complete")
	return nil
}
