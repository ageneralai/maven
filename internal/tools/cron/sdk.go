package cron

import (
	"context"
	"fmt"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	svcron "github.com/ageneralai/maven/internal/cron"
)

func Tools(s *svcron.Service) []tool.Tool {
	if s == nil {
		return nil
	}
	return []tool.Tool{
		&scheduleTool{svc: s},
		&listTool{svc: s},
		&removeTool{svc: s},
	}
}

type scheduleTool struct{ svc *svcron.Service }

func (t *scheduleTool) Name() string { return "cron-schedule" }

func (t *scheduleTool) Description() string {
	return "Schedule a persisted job. When it fires, the agent runs with your message. " +
		"Use exactly one of expr (six-field cron with seconds), in (duration e.g. 1m), or at_ms. " +
		"Omitting all delivery fields is silent by default; in an active inbound chat, omitting them auto-delivers to the current conversation. " +
		"Set deliver_to_incoming_chat=true to explicitly deliver to the current conversation, or deliver=true with channel+to for an explicit recipient."
}

func (t *scheduleTool) Schema() *tool.JSONSchema { return cronScheduleToolSchema }

func (t *scheduleTool) Execute(ctx context.Context, params map[string]interface{}) (*tool.ToolResult, error) {
	job, err := AddFromToolMap(t.svc, ctx, params, time.Now())
	if err != nil {
		return &tool.ToolResult{Success: false, Output: err.Error()}, nil
	}
	return &tool.ToolResult{Success: true, Output: FormatJobAdded(job)}, nil
}

type listTool struct{ svc *svcron.Service }

func (t *listTool) Name() string { return "cron-list" }

func (t *listTool) Description() string {
	return "List all persisted cron jobs (id, schedule, delivery targets)."
}

func (t *listTool) Schema() *tool.JSONSchema { return cronListToolSchema }

func (t *listTool) Execute(_ context.Context, _ map[string]interface{}) (*tool.ToolResult, error) {
	out := FormatList(t.svc.ListJobs())
	return &tool.ToolResult{Success: true, Output: out}, nil
}

type removeTool struct{ svc *svcron.Service }

func (t *removeTool) Name() string { return "cron-remove" }

func (t *removeTool) Description() string {
	return "Remove a persisted cron job by id (use cron-list to see ids)."
}

func (t *removeTool) Schema() *tool.JSONSchema { return cronRemoveToolSchema }

func (t *removeTool) Execute(_ context.Context, params map[string]interface{}) (*tool.ToolResult, error) {
	id := stringFromMap(params, "id")
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}
	if !t.svc.RemoveJob(id) {
		return nil, fmt.Errorf("no job with id %q", id)
	}
	return &tool.ToolResult{Success: true, Output: fmt.Sprintf("Removed job %q.", id)}, nil
}
