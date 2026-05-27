package gateway

import (
	"context"
	"fmt"

	"github.com/ageneralai/maven/internal/kernel/bus"
	"github.com/ageneralai/maven/internal/kernel/plugin"
	"github.com/ageneralai/maven/internal/plugins/trigger/cron"
)

func (g *Gateway) stopTriggers() {
	g.trigMu.Lock()
	defer g.trigMu.Unlock()
	for _, tr := range g.triggers {
		if err := tr.Stop(); err != nil {
			g.logger.Error("trigger stop error", "trigger", tr.Name(), "err", err)
		}
	}
	g.triggers = nil
}

func (g *Gateway) ensureCronService() error {
	if g.plugins == nil || g.pipe == nil {
		return nil
	}
	cp, ok := g.plugins.FindByName("cron").(*cron.Plugin)
	if !ok {
		return nil
	}
	_, err := cp.EnsureService(g.pipe)
	return err
}

func (g *Gateway) startTriggers(ctx context.Context) error {
	if g.plugins == nil || g.pipe == nil {
		return nil
	}
	g.stopTriggers()
	pub := bus.Publisher{Bus: g.bus}
	var started []plugin.Trigger
	for _, tr := range g.plugins.Triggers(g.cfg) {
		if err := tr.Start(ctx, g.pipe, pub); err != nil {
			for _, s := range started {
				_ = s.Stop()
			}
			return fmt.Errorf("trigger %q start: %w", tr.Name(), err)
		}
		started = append(started, tr)
	}
	g.trigMu.Lock()
	g.triggers = started
	g.trigMu.Unlock()
	return nil
}

func (g *Gateway) cronService() *cron.Service {
	return cron.ServiceFromRegistry(g.plugins)
}
