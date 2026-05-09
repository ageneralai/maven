package acp

import (
	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/config"
)

// Tools returns custom tools for ACP delegation when enabled with at least one valid agent entry.
func Tools(cfg config.ACPToolConfig, workspace string, restrict bool) []tool.Tool {
	if !cfg.Enabled || len(cfg.Agents) == 0 {
		return nil
	}
	dt := NewDelegateTaskTool(workspace, restrict, cfg.Agents)
	if len(dt.agents) == 0 {
		return nil
	}
	return []tool.Tool{dt}
}
