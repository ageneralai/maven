package gateway

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/ageneralai/maven/internal/agent/postaction"
	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/channel/manager"
	"github.com/ageneralai/maven/internal/config"
	"github.com/ageneralai/maven/internal/cron"
	"github.com/ageneralai/maven/internal/health"
	"github.com/ageneralai/maven/internal/heartbeat"
	"github.com/ageneralai/maven/internal/pipeline"
	mavsession "github.com/ageneralai/maven/internal/session"
	"github.com/ageneralai/maven/internal/slash"
	mavoice "github.com/ageneralai/maven/internal/voice"
	"github.com/ageneralai/maven/pkg/acp"
	"github.com/ageneralai/maven/pkg/executor"
	"github.com/ageneralai/maven/pkg/memory"
	"github.com/ageneralai/maven/pkg/plugin"
)

type coreDeps struct {
	cfg            *config.Config
	logger         *slog.Logger
	bus            *bus.MessageBus
	sessions       *mavsession.Router
	historyStore   *mavsession.Store
	mem            *memory.MemoryStore
	liveness       health.HealthReporter
	signalChan     chan os.Signal
	runtimeFactory RuntimeFactory
}

type planeDeps struct {
	channelMgr *manager.ChannelManager
	pipe       *pipeline.Pipeline
	cron       *cron.Service
	hb         *heartbeat.Service
	plugins    *plugin.Registry
}

func wireCore(cfg *config.Config, opts Options) (*coreDeps, error) {
	if opts.Logger == nil {
		return nil, fmt.Errorf("gateway: logger is required")
	}
	factory := opts.RuntimeFactory
	if factory == nil {
		factory = DefaultRuntimeFactory
	}
	router, routerErr := mavsession.New(filepath.Join(cfg.Agent.Workspace, ".maven", "session-router.json"))
	if routerErr != nil {
		return nil, routerErr
	}
	histStore, err := mavsession.NewStore(filepath.Join(config.ConfigDir(), "sessions"))
	if err != nil {
		return nil, fmt.Errorf("session store: %w", err)
	}
	return &coreDeps{
		cfg:            cfg,
		logger:         opts.Logger,
		bus:            bus.New(config.DefaultBufSize, opts.Logger, bus.WithHealthReporter(health.OrHealthReporter(opts.HealthReporter))),
		sessions:       router,
		historyStore:   histStore,
		mem:            memory.NewMemoryStore(cfg.Agent.Workspace),
		liveness:       health.OrHealthReporter(opts.HealthReporter),
		signalChan:     opts.SignalChan,
		runtimeFactory: factory,
	}, nil
}

func wirePlanes(core *coreDeps) (*planeDeps, error) {
	channelMgr := manager.New(core.bus, core.logger, nil, nil)
	sessRes := &mavsession.SessionResolver{Router: core.sessions}
	posts := postaction.New(core.sessions, core.cfg.Agent.Workspace)
	pipe := pipeline.New(core.logger, core.bus, nil, sessRes, posts, channelMgr, core.liveness)
	channelMgr.SetStreamRunner(pipe)
	cronDeliver := &cron.Deliver{Bus: core.bus, Channels: channelMgr, Log: core.logger}
	cronSvc, err := cron.NewService(filepath.Join(config.ConfigDir(), "data", "cron", "jobs.json"), pipe, core.cfg.Gateway.Cron.MaxConcurrentRuns, core.logger, cronDeliver)
	if err != nil {
		return nil, fmt.Errorf("cron service: %w", err)
	}
	slashReg, err := slash.BuiltIns(cronSvc)
	if err != nil {
		return nil, err
	}
	pipe.SetSlashRegistry(slashReg)
	hb, err := heartbeat.New(core.cfg.Agent.Workspace, pipe, 0, core.logger, heartbeat.WithHealthReporter(core.liveness))
	if err != nil {
		return nil, fmt.Errorf("heartbeat: %w", err)
	}
	plugs := []plugin.Plugin{acp.NewPlugin()}
	plugs = append(plugs, mavoice.VoicePlugins()...)
	plugins := plugin.NewRegistry(plugs...)
	channelMgr.SetPlugins(plugins)
	return &planeDeps{
		channelMgr: channelMgr,
		pipe:       pipe,
		cron:       cronSvc,
		hb:         hb,
		plugins:    plugins,
	}, nil
}

var (
	_ executor.TurnExecutor = (*pipeline.Pipeline)(nil)
	_ executor.StreamRunner = (*pipeline.Pipeline)(nil)
)
