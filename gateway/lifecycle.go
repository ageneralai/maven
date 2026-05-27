package gateway

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ageneralai/maven/kernel/config"
	"github.com/ageneralai/maven/kernel/health"
)

func (g *Gateway) interruptRunLoops() {
	if g.runCancel != nil {
		g.runCancel()
		g.runCancel = nil
	}
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

// Shutdown cancels triggers and the pipeline/dispatch ctx, drains the inbound loop, stops channels/closes runtime and bus (order-sensitive).
func (g *Gateway) Shutdown() error {
	g.interruptRunLoops()
	g.stopTriggers()
	g.pipeWg.Wait()
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
