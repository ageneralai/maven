package acp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/config"
)

const delegateToolDescription = `Delegate a coding task to a configured external ACP agent subprocess (Codex, Claude Code, Gemini CLI, etc.). ` +
	`The subprocess runs only for this tool invocation; progress streams into the channel status card. ` +
	`Choose agent from configured tools.acp.agents keys; cwd defaults to the maven workspace and must stay inside it when workspace restriction is enabled.`

var delegateTaskSchema = &tool.JSONSchema{
	Type: "object",
	Properties: map[string]any{
		"agent": map[string]any{
			"type":        "string",
			"description": "Configured ACP agent name (tools.acp.agents key), e.g. codex, claude, gemini.",
		},
		"prompt": map[string]any{
			"type":        "string",
			"description": "Task instructions for the delegated agent.",
		},
		"cwd": map[string]any{
			"type":        "string",
			"description": "Working directory for the subprocess and ACP session cwd. Defaults to maven workspace.",
		},
		"timeout": map[string]any{
			"type":        "number",
			"description": "Optional timeout in seconds (default 120).",
		},
	},
	Required: []string{"agent", "prompt"},
}

type delegateTaskTool struct {
	workspace string
	restrict  bool
	agents    map[string]config.ACPAgent
}

// NewDelegateTaskTool builds DelegateTask for agents with non-empty commands.
func NewDelegateTaskTool(workspace string, restrict bool, agents map[string]config.ACPAgent) *delegateTaskTool {
	cp := make(map[string]config.ACPAgent, len(agents))
	for k, v := range agents {
		kk := strings.TrimSpace(k)
		if kk == "" || strings.TrimSpace(v.Command) == "" {
			continue
		}
		cp[kk] = v
	}
	return &delegateTaskTool{workspace: workspace, restrict: restrict, agents: cp}
}

func (t *delegateTaskTool) Name() string { return "DelegateTask" }

func (t *delegateTaskTool) Description() string { return delegateToolDescription }

func (t *delegateTaskTool) Schema() *tool.JSONSchema { return delegateTaskSchema }

func (t *delegateTaskTool) Metadata() tool.Metadata {
	return tool.Metadata{IsDestructive: true}
}

func (t *delegateTaskTool) Execute(ctx context.Context, params map[string]any) (*tool.ToolResult, error) {
	return t.StreamExecute(ctx, params, func(string, bool) {})
}

func (t *delegateTaskTool) StreamExecute(ctx context.Context, params map[string]any, emit func(string, bool)) (*tool.ToolResult, error) {
	if t == nil || len(t.agents) == 0 {
		return nil, fmt.Errorf("delegate task tool is not configured")
	}
	if emit == nil {
		emit = func(string, bool) {}
	}
	agentName := strings.TrimSpace(stringFromMap(params, "agent"))
	if agentName == "" {
		return nil, fmt.Errorf("agent is required")
	}
	agentCfg, ok := t.agents[agentName]
	if !ok {
		return nil, fmt.Errorf("unknown agent %q — add it under tools.acp.agents in config", agentName)
	}
	prompt := strings.TrimSpace(stringFromMap(params, "prompt"))
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	cwd := strings.TrimSpace(stringFromMap(params, "cwd"))
	if cwd == "" {
		cwd = t.workspace
	}
	cwdAbs, err := resolveWorkspacePath(t.workspace, t.restrict, cwd)
	if err != nil {
		return nil, err
	}
	timeoutSec := 120
	if v, ok := params["timeout"]; ok && v != nil {
		if n, ok := numberToInt(v); ok && n > 0 {
			timeoutSec = n
		}
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()
	final, err := runACPSession(runCtx, agentCfg, cwdAbs, prompt, t.workspace, t.restrict, emit)
	if err != nil {
		return &tool.ToolResult{Success: false, Output: err.Error()}, err
	}
	return &tool.ToolResult{Success: true, Output: final}, nil
}

func stringFromMap(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	default:
		return fmt.Sprint(s)
	}
}

func numberToInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	case float32:
		return int(x), true
	default:
		return 0, false
	}
}
