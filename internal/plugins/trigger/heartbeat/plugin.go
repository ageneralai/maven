package heartbeat

import (
	"context"
	"sync"
	"time"

	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/executor"
	"github.com/ageneralai/maven/internal/kernel/health"
	"github.com/ageneralai/maven/internal/kernel/plugin"
	"log/slog"
)

// Plugin runs periodic HEARTBEAT.md checks via the shared TurnExecutor.
type Plugin struct {
	interval time.Duration
	log      *slog.Logger
	rep      health.HealthReporter
	mu       sync.Mutex
	svc      *Service
	runCtx   context.CancelFunc
}

func NewPlugin(interval time.Duration, lg *slog.Logger, opts ...Option) *Plugin {
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	p := &Plugin{interval: interval, log: lg, rep: health.NoOp{}}
	for _, o := range opts {
		if o == nil {
			continue
		}
		s := &Service{log: lg, rep: p.rep, interval: interval}
		o(s)
		p.rep = s.rep
	}
	return p
}

func (p *Plugin) Name() string { return "heartbeat" }

func (p *Plugin) Start(context.Context) error { return nil }

func (p *Plugin) Stop() error { return nil }

func (p *Plugin) Triggers(cfg *config.Config) []plugin.Trigger {
	if cfg == nil {
		return nil
	}
	return []plugin.Trigger{&hbRunner{plugin: p, workspace: cfg.Agent.Workspace}}
}

type hbRunner struct {
	plugin    *Plugin
	workspace string
}

func (r *hbRunner) Name() string { return "heartbeat" }

func (r *hbRunner) Start(ctx context.Context, exec executor.TurnExecutor, _ plugin.OutboundPublisher) error {
	svc, err := New(r.workspace, exec, r.plugin.interval, r.plugin.log, WithHealthReporter(r.plugin.rep))
	if err != nil {
		return err
	}
	r.plugin.mu.Lock()
	if r.plugin.runCtx != nil {
		r.plugin.runCtx()
	}
	runCtx, cancel := context.WithCancel(ctx)
	r.plugin.runCtx = cancel
	r.plugin.svc = svc
	r.plugin.mu.Unlock()
	go func() { _ = svc.Start(runCtx) }()
	return nil
}

func (r *hbRunner) Stop() error {
	r.plugin.mu.Lock()
	if r.plugin.runCtx != nil {
		r.plugin.runCtx()
		r.plugin.runCtx = nil
	}
	svc := r.plugin.svc
	r.plugin.svc = nil
	r.plugin.mu.Unlock()
	if svc != nil {
		svc.Stop()
	}
	return nil
}
