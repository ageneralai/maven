package gateway

import (
	"context"

	"github.com/ageneralai/maven/internal/kernel/config"
)

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
