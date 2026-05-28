package gateway

import (
	"context"

	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/slash"
)

func (g *Gateway) registerReloadSlash(slashReg *slash.Registry) error {
	return slashReg.Register(
		slash.Definition{
			Name:        "reload",
			Description: "Re-read config, AGENTS.md, SOUL.md, MEMORY.md, skills; rebuild runtime.",
		},
		slash.HandlerFunc(func(context.Context, slash.Invocation) (slash.Result, error) {
			g.requestReload()
			return slash.Result{Output: "Reloading…"}, nil
		}),
	)
}

func (g *Gateway) requestReload() {
	select {
	case g.manualReloadCh <- struct{}{}:
	default:
	}
}

func (g *Gateway) reloadFromConfig(ctx context.Context) {
	newCfg, lerr := config.LoadConfig()
	if lerr != nil {
		g.logger.Error("gateway reload load config error", "err", lerr)
		return
	}
	if aerr := g.Apply(ctx, newCfg); aerr != nil {
		g.logger.Error("gateway reload apply error", "err", aerr)
		return
	}
	g.logger.Info("gateway reloaded", "host", newCfg.Gateway.Host, "port", newCfg.Gateway.Port, "channels", g.channelMgr.EnabledChannels())
}
