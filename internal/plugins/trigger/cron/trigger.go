package cron

import (
	"context"

	"github.com/ageneralai/maven/internal/kernel/executor"
	"github.com/ageneralai/maven/internal/kernel/plugin"
)

type runner struct {
	p *Plugin
}

func (r *runner) Name() string { return "cron" }

func (r *runner) Start(ctx context.Context, exec executor.TurnExecutor, pub plugin.OutboundPublisher) error {
	deliver := &Deliver{Pub: pub, Channels: r.p.channels, Log: r.p.log}
	return r.p.startLoop(ctx, exec, deliver)
}

func (r *runner) Stop() error {
	r.p.stopLoop()
	return nil
}
