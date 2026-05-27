package gateway

import (
	"context"
	"log/slog"
	"os"
	"sync"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/kernel/agent"
	"github.com/ageneralai/maven/kernel/bus"
	"github.com/ageneralai/maven/kernel/channels/manager"
	"github.com/ageneralai/maven/kernel/config"
	"github.com/ageneralai/maven/kernel/health"
	"github.com/ageneralai/maven/kernel/pipeline"
	mavsession "github.com/ageneralai/maven/kernel/session"
	"github.com/ageneralai/maven/kernel/memory"
	"github.com/ageneralai/maven/kernel/plugin"
)

// RuntimeFactory builds the agent runtime used by the gateway pipeline.
type RuntimeFactory func(cfg *config.Config, sysPrompt string, skillRegs []api.SkillRegistration, pluginTools []tool.Tool, sessionStore api.SessionStore, lg *slog.Logger) (agent.Runtime, error)

// Options for creating a Gateway.
type Options struct {
	Logger         *slog.Logger
	RuntimeFactory RuntimeFactory
	SignalChan     chan os.Signal
	HealthReporter health.HealthReporter
}

// DefaultRuntimeFactory wires agentsdk-go with the given skills and plugin tools.
func DefaultRuntimeFactory(cfg *config.Config, sysPrompt string, skillRegs []api.SkillRegistration, pluginTools []tool.Tool, sessionStore api.SessionStore, lg *slog.Logger) (agent.Runtime, error) {
	return agent.NewSDKRuntime(cfg, sysPrompt, skillRegs, pluginTools, sessionStore, lg)
}

// Gateway is the plugin host: it owns the message bus, channel manager, pipeline, and trigger lifecycle.
type Gateway struct {
	cfg            *config.Config
	bus            *bus.MessageBus
	pipe           *pipeline.Pipeline
	pipeWg         sync.WaitGroup
	runCancel      context.CancelFunc
	channelMgr     *manager.ChannelManager
	runtimeFactory RuntimeFactory
	plugins        *plugin.Registry
	triggers       []plugin.Trigger
	trigMu         sync.Mutex
	mem            *memory.MemoryStore
	skillRegs      []api.SkillRegistration
	sessions       *mavsession.Router
	historyStore   *mavsession.Store
	signalChan     chan os.Signal
	logger         *slog.Logger
	liveness       health.HealthReporter
	applyMu        sync.Mutex
}

// New creates a Gateway. lg must be non-nil.
func New(cfg *config.Config, lg *slog.Logger) (*Gateway, error) {
	return NewWithOptions(cfg, Options{Logger: lg})
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
		plugins:        planes.plugins,
	}, nil
}
