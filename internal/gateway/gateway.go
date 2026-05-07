package gateway

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/cexll/agentsdk-go/pkg/api"
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
	cfg        *config.Config
	bus        *bus.MessageBus
	pipe       *pipeline.Pipeline
	channels   *channel.ChannelManager
	cron       *cron.Service
	hb         *heartbeat.Service
	lane       *automation.Lane
	rt         agent.Runtime
	mem        *memory.MemoryStore
	skillRegs  []api.SkillRegistration
	sessions   *session.Router
	signalChan chan os.Signal
	logger     mavenlog.PrintLogger
}

// New creates a Gateway with default options.
func New(cfg *config.Config) (*Gateway, error) {
	return NewWithOptions(cfg, Options{})
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
	sysPrompt := prompt.Build(cfg.Agent.Workspace, g.mem.GetMemoryContext())
	if cfg.Skills.Enabled {
		skillDir := cfg.Skills.Dir
		if skillDir == "" {
			skillDir = filepath.Join(cfg.Agent.Workspace, "skills")
		}
		regs, err := skills.LoadSkills(skillDir, g.logger)
		if err != nil {
			g.logger.Printf("[gateway] skills load warning: %v", err)
		}
		g.skillRegs = regs
	}
	cronStorePath := filepath.Join(config.ConfigDir(), "data", "cron", "jobs.json")
	g.cron = cron.NewService(cronStorePath, g.logger)
	g.lane = &automation.Lane{}
	factory := opts.RuntimeFactory
	if factory == nil {
		factory = DefaultRuntimeFactory
	}
	rt, err := factory(cfg, sysPrompt, g.skillRegs, g.cron)
	if err != nil {
		return nil, err
	}
	g.rt = rt
	g.signalChan = opts.SignalChan
	sessRes := &agent.SessionResolver{Router: g.sessions}
	posts := &agent.PostActionHandler{Sessions: g.sessions, Workspace: cfg.Agent.Workspace}
	g.pipe = pipeline.New(g.logger, g.bus, rt, sessRes, posts)
	g.cron.OnJob = func(job cron.CronJob) (string, error) {
		if err := job.Payload.Validate(); err != nil {
			return "", err
		}
		var out string
		if err := g.lane.RunAlways(context.Background(), func(ctx context.Context) error {
			var runErr error
			out, runErr = agent.RunText(ctx, g.rt, job.Payload.Message, cronsession.SessionKey(job.ID), nil)
			return runErr
		}); err != nil {
			return "", err
		}
		if job.Payload.Deliver {
			g.bus.Outbound <- bus.OutboundMessage{
				Channel: job.Payload.Channel,
				ChatID:  job.Payload.To,
				Content: out,
			}
		}
		return out, nil
	}
	g.hb = heartbeat.New(cfg.Agent.Workspace, func(hbPrompt string) (string, error) {
		var out string
		ran, err := g.lane.TryRun(context.Background(), func(ctx context.Context) error {
			var runErr error
			out, runErr = agent.RunText(ctx, g.rt, hbPrompt, heartbeatsession.SessionKey(), nil)
			return runErr
		})
		if !ran {
			g.logger.Printf("[heartbeat] skipped: %s", heartbeatSkipReasonAutomationLaneBusy)
			return "", nil
		}
		return out, err
	}, 0, g.logger)
	chMgr, chErr := channel.NewChannelManagerWithGateway(cfg.Channels, cfg.Gateway, cfg.Agent.Workspace, g.bus, g.logger)
	if chErr != nil {
		return nil, fmt.Errorf("create channel manager: %w", chErr)
	}
	g.channels = chMgr
	g.pipe.Channels = chMgr
	return g, nil
}

// Run starts outbound dispatch, channels, cron, heartbeat, the inbound pipeline, and blocks on shutdown signal.
func (g *Gateway) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go g.bus.DispatchOutbound(ctx)
	if err := g.channels.StartAll(ctx); err != nil {
		return fmt.Errorf("start channels: %w", err)
	}
	g.logger.Printf("[gateway] channels started: %v", g.channels.EnabledChannels())
	if err := g.cron.Start(ctx); err != nil {
		g.logger.Printf("[gateway] cron start warning: %v", err)
	}
	go func() {
		if err := g.hb.Start(ctx); err != nil {
			g.logger.Printf("[gateway] heartbeat error: %v", err)
		}
	}()
	go g.pipe.Run(ctx)
	g.logger.Printf("[gateway] running on %s:%d", g.cfg.Gateway.Host, g.cfg.Gateway.Port)
	sigCh := g.signalChan
	if sigCh == nil {
		sigCh = make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	}
	<-sigCh
	g.logger.Printf("[gateway] shutting down...")
	return g.Shutdown()
}

// Shutdown stops cron, channels, and closes the agent runtime.
func (g *Gateway) Shutdown() error {
	g.cron.Stop()
	_ = g.channels.StopAll()
	if g.rt != nil {
		g.rt.Close()
	}
	g.logger.Printf("[gateway] shutdown complete")
	return nil
}

