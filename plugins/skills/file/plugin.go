package skills

import (
	"context"
	"path/filepath"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/maven/kernel/config"
	"github.com/ageneralai/maven/kernel/plugin"
	"log/slog"
)

// Plugin loads SKILL.md files from the workspace skills directory.
type Plugin struct {
	log *slog.Logger
}

func NewPlugin(lg *slog.Logger) plugin.SkillPlugin {
	return &Plugin{log: lg}
}

func (p *Plugin) Name() string { return "skills-file" }

func (p *Plugin) Start(context.Context) error { return nil }

func (p *Plugin) Stop() error { return nil }

func (p *Plugin) Skills(cfg *config.Config) []api.SkillRegistration {
	if cfg == nil || !cfg.Skills.Enabled {
		return nil
	}
	dir := cfg.Skills.Dir
	if dir == "" {
		dir = filepath.Join(cfg.Agent.Workspace, "skills")
	}
	regs, err := LoadSkills(dir, p.log)
	if err != nil {
		p.log.Warn("skills load warning", "err", err)
		return nil
	}
	return regs
}
