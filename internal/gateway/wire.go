package gateway

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/ageneralai/maven/internal/kernel/agent/postaction"
	"github.com/ageneralai/maven/internal/kernel/bus"
	"github.com/ageneralai/maven/internal/kernel/channel/manager"
	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/health"
	"github.com/ageneralai/maven/internal/kernel/pipeline"
	mavsession "github.com/ageneralai/maven/internal/kernel/session"
	"github.com/ageneralai/maven/internal/kernel/slash"
	"github.com/ageneralai/maven/internal/plugins/tool/acp"
	"github.com/ageneralai/maven/internal/plugins/voice/cartesia"
	"github.com/ageneralai/maven/internal/plugins/voice/deepgram"
	"github.com/ageneralai/maven/internal/plugins/voice/elevenlabs"
	voiceopenai "github.com/ageneralai/maven/internal/plugins/voice/openai"
	"github.com/ageneralai/maven/internal/kernel/executor"
	kmemory "github.com/ageneralai/maven/internal/kernel/memory"
	"github.com/ageneralai/maven/internal/kernel/plugin"
	fmemory "github.com/ageneralai/maven/internal/plugins/memory/file"
	"github.com/ageneralai/maven/internal/plugins/channel/feishu"
	skills "github.com/ageneralai/maven/internal/plugins/skill/file"
	"github.com/ageneralai/maven/internal/plugins/channel/matrix"
	"github.com/ageneralai/maven/internal/plugins/channel/telegram"
	"github.com/ageneralai/maven/internal/plugins/channel/web"
	"github.com/ageneralai/maven/internal/plugins/channel/wecom"
	"github.com/ageneralai/maven/internal/plugins/channel/whatsapp"
	"github.com/ageneralai/maven/internal/plugins/trigger/cron"
	"github.com/ageneralai/maven/internal/plugins/trigger/heartbeat"
)

type coreDeps struct {
	cfg            *config.Config
	logger         *slog.Logger
	bus            *bus.MessageBus
	sessions       *mavsession.Router
	historyStore   *mavsession.Store
	liveness       health.HealthReporter
	signalChan     chan os.Signal
	runtimeFactory RuntimeFactory
}

type planeDeps struct {
	channelMgr *manager.ChannelManager
	pipe       *pipeline.Pipeline
	plugins    *plugin.Registry
	memPlug    *fmemory.Plugin
	memReg     *kmemory.Registry
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
		liveness:       health.OrHealthReporter(opts.HealthReporter),
		signalChan:     opts.SignalChan,
		runtimeFactory: factory,
	}, nil
}

func wirePlanes(core *coreDeps) (*planeDeps, error) {
	webPlug := web.NewPlugin(core.bus, core.logger)
	channelMgr := manager.New(core.bus, core.logger, nil)
	sessRes := &mavsession.SessionResolver{Router: core.sessions}
	posts := postaction.New(core.sessions, core.cfg.Agent.Workspace)
	pipe := pipeline.New(core.logger, core.bus, nil, sessRes, posts, channelMgr, core.liveness)
	webPlug.SetStreamRunner(pipe)
	cronPlug := cron.NewPlugin(
		filepath.Join(config.ConfigDir(), "data", "cron", "jobs.json"),
		core.cfg.Gateway.Cron.MaxConcurrentRuns,
		channelMgr,
		core.logger,
	)
	hbPlug := heartbeat.NewPlugin(0, core.logger, heartbeat.WithHealthReporter(core.liveness))
	plugs := []plugin.Plugin{
		telegram.NewPlugin(core.bus, core.logger),
		feishu.NewPlugin(core.bus, core.logger),
		wecom.NewPlugin(core.bus, core.logger),
		whatsapp.NewPlugin(core.bus, core.logger),
		matrix.NewPlugin(core.bus, core.logger),
		webPlug,
		cronPlug,
		hbPlug,
		acp.NewPlugin(),
		skills.NewPlugin(core.logger),
		cartesia.NewPlugin(),
		deepgram.NewPlugin(),
		elevenlabs.NewPlugin(),
		voiceopenai.NewPlugin(),
	}
	plugins := plugin.NewRegistry(plugs...)
	webPlug.SetRegistry(plugins)
	channelMgr.SetRegistry(plugins)
	if _, err := cronPlug.EnsureService(pipe); err != nil {
		return nil, fmt.Errorf("cron ensure service: %w", err)
	}
	slashReg, err := slash.BuiltIns()
	if err != nil {
		return nil, err
	}
	if err := slash.RegisterPluginCommands(slashReg, plugins.SlashCommands(core.cfg)); err != nil {
		return nil, err
	}
	pipe.SetSlashRegistry(slashReg)
	memPlug := fmemory.NewPlugin(core.logger)
	memReg, err := kmemory.NewRegistry(core.logger, memPlug)
	if err != nil {
		return nil, fmt.Errorf("memory registry: %w", err)
	}
	return &planeDeps{
		channelMgr: channelMgr,
		pipe:       pipe,
		plugins:    plugins,
		memPlug:    memPlug,
		memReg:     memReg,
	}, nil
}
var (
	_ executor.TurnExecutor = (*pipeline.Pipeline)(nil)
	_ executor.StreamRunner = (*pipeline.Pipeline)(nil)
)
// Wire builds a production Gateway with all plugins registered.
func Wire(cfg *config.Config, lg *slog.Logger) (*Gateway, error) {
	return NewWithOptions(cfg, Options{Logger: lg})
}
