package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/plugin"
)

type rememberTool struct {
	plug *Plugin
	cfg  *config.Config
}

var rememberSchema = &tool.JSONSchema{
	Type: "object",
	Properties: map[string]any{
		"content": map[string]any{"type": "string", "description": "The information to store."},
		"kind": map[string]any{
			"type":        "string",
			"enum":        []string{"fact", "event", "preference"},
			"description": `"fact" replaces MEMORY.md (long-term), "event" appends to today's journal, "preference" appends a preference note.`,
		},
	},
	Required: []string{"content", "kind"},
}

func (p *Plugin) Tools(cfg *config.Config) []tool.Tool {
	return []tool.Tool{&rememberTool{plug: p, cfg: cfg}}
}

func (t *rememberTool) Name() string { return "remember" }

func (t *rememberTool) Description() string {
	return "Store information to long-term memory. Use kind='fact' for persistent facts about the user or context, 'event' for things that happened today, 'preference' for user preferences."
}

func (t *rememberTool) Schema() *tool.JSONSchema { return rememberSchema }

func (t *rememberTool) Execute(ctx context.Context, params map[string]any) (*tool.ToolResult, error) {
	content := strings.TrimSpace(stringParam(params, "content"))
	kindStr := strings.TrimSpace(stringParam(params, "kind"))
	if content == "" {
		return nil, fmt.Errorf("remember: content is required")
	}
	kind := plugin.MemoryKind(kindStr)
	if err := t.plug.Write(ctx, t.cfg, plugin.MemoryEntry{
		Content:   content,
		Kind:      kind,
		Timestamp: time.Now(),
		Source:    "agent",
	}); err != nil {
		return &tool.ToolResult{Success: false, Output: err.Error()}, err
	}
	if t.plug.refreshFn != nil {
		if err := t.plug.refreshFn(ctx); err != nil {
			_ = err
		}
	}
	return &tool.ToolResult{Success: true, Output: fmt.Sprintf("Stored (%s).", kind)}, nil
}

func stringParam(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return strings.TrimSpace(fmt.Sprint(v))
	}
	return strings.TrimSpace(s)
}
