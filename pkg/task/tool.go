package task

import (
	"context"
	"fmt"
	"strings"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/ageneral-agents-go/pkg/runtime/subagents"
	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
)

const toolDescription = `Delegate a scoped goal to an in-process SDK subagent (general-purpose, explore, plan). ` +
	`Use for research, exploration, or planning within the Maven workspace. ` +
	`Prefer DelegateTask for external coding CLIs configured under tools.acp.agents.`

var taskSchema = &tool.JSONSchema{
	Type: "object",
	Properties: map[string]any{
		"name": map[string]any{
			"type":        "string",
			"description": "Subagent type: general-purpose, explore, or plan.",
		},
		"goal": map[string]any{
			"type":        "string",
			"description": "Specific goal or instruction for the subagent.",
		},
		"model": map[string]any{
			"type":        "string",
			"description": "Optional model tier override (sonnet, haiku, low, mid, high).",
		},
	},
	Required: []string{"name", "goal"},
}

type taskTool struct {
	holder *RuntimeHolder
}

func New(holder *RuntimeHolder) tool.Tool {
	if holder == nil {
		return nil
	}
	return &taskTool{holder: holder}
}

func (t *taskTool) Name() string { return "Task" }

func (t *taskTool) Description() string { return toolDescription }

func (t *taskTool) Schema() *tool.JSONSchema { return taskSchema }

func (t *taskTool) Execute(ctx context.Context, params map[string]any) (*tool.ToolResult, error) {
	name := strings.ToLower(strings.TrimSpace(stringParam(params, "name")))
	goal := strings.TrimSpace(stringParam(params, "goal"))
	if name == "" || goal == "" {
		return nil, fmt.Errorf("name and goal are required")
	}
	def, ok := subagents.BuiltinDefinition(name)
	if !ok {
		return nil, fmt.Errorf("unknown subagent %q (builtin types: general-purpose, explore, plan)", name)
	}
	parentSession := parentSessionID(ctx)
	if isNestedTaskSession(parentSession) {
		return nil, fmt.Errorf("nested task delegation is not supported")
	}
	rt := t.holder.Get()
	if rt == nil {
		return nil, fmt.Errorf("task tool runtime is not ready")
	}
	req := api.Request{
		Prompt:         goal,
		SessionID:      childSessionID(parentSession),
		TargetSubagent: def.Name,
		ToolWhitelist:  append([]string(nil), def.BaseContext.ToolWhitelist...),
		Metadata: map[string]any{
			"parent_session": parentSession,
			"subagent":       def.Name,
		},
	}
	if tier := parseModelTier(stringParam(params, "model")); tier != "" {
		req.Model = tier
	}
	resp, err := rt.Run(ctx, req)
	if err != nil {
		return &tool.ToolResult{Success: false, Output: err.Error()}, err
	}
	output := ""
	if resp != nil && resp.Result != nil {
		output = strings.TrimSpace(resp.Result.Output)
	}
	if output == "" {
		output = "(subagent completed with no text output)"
	}
	return &tool.ToolResult{Success: true, Output: output}, nil
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

func parseModelTier(raw string) api.ModelTier {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "haiku", "low":
		return api.ModelTierLow
	case "sonnet", "mid", "medium":
		return api.ModelTierMid
	case "opus", "high":
		return api.ModelTierHigh
	default:
		return ""
	}
}
