package gateway

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

func (g *Gateway) registerGatewaySlashes(slashReg *slash.Registry) error {
	if err := g.registerReloadSlash(slashReg); err != nil {
		return fmt.Errorf("slash reload: %w", err)
	}
	if err := slashReg.Register(
		slash.Definition{Name: "status", Description: "Show gateway status: cron jobs, memory size."},
		slash.HandlerFunc(func(ctx context.Context, inv slash.Invocation) (slash.Result, error) {
			return g.slashStatus(ctx), nil
		}),
	); err != nil {
		return fmt.Errorf("slash status: %w", err)
	}
	return nil
}

func (g *Gateway) slashStatus(_ context.Context) slash.Result {
	var parts []string
	if svc := g.cronService(); svc != nil {
		jobs := svc.ListJobs()
		enabled := 0
		for _, j := range jobs {
			if j.Enabled {
				enabled++
			}
		}
		parts = append(parts, fmt.Sprintf("🕐 Cron jobs: %d active / %d total", enabled, len(jobs)))
	}
	if g.cfg != nil {
		memPath := filepath.Join(g.cfg.Agent.Workspace, "memory", "MEMORY.md")
		if fi, err := os.Stat(memPath); err == nil {
			parts = append(parts, fmt.Sprintf("🧠 MEMORY.md: %d bytes", fi.Size()))
		} else {
			parts = append(parts, "🧠 MEMORY.md: empty")
		}
	}
	if len(parts) == 0 {
		return slash.Result{Output: "No status available."}
	}
	return slash.Result{Output: strings.Join(parts, "\n")}
}
