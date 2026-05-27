package task

import (
	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/kernel/config"
)

// Tools returns the in-process Task tool when tools.task is enabled.
func Tools(cfg config.TaskToolConfig, holder *RuntimeHolder) []tool.Tool {
	if !cfg.Enabled || holder == nil {
		return nil
	}
	tk := New(holder)
	if tk == nil {
		return nil
	}
	return []tool.Tool{tk}
}
