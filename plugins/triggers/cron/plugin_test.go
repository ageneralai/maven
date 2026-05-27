package cron

import (
	"testing"

	"github.com/ageneralai/maven/kernel/plugin"
)

func TestPlugin_ImplementsAxes(t *testing.T) {
	var _ plugin.TriggerPlugin = (*Plugin)(nil)
	var _ plugin.ToolPlugin = (*Plugin)(nil)
	var _ plugin.SlashPlugin = (*Plugin)(nil)
}
