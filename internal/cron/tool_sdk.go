package cron

import (
	"context"
	"fmt"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
)

func Tools(svc *Service) []tool.Tool {
	if svc == nil {
		return nil
	}
	return []tool.Tool{
		&scheduleTool{svc: svc},
		&listTool{svc: svc},
		&removeTool{svc: svc},
	}
}

type scheduleTool struct{ svc *Service }

func (t *scheduleTool) Name() string { return "CronSchedule" }

func (t *scheduleTool) Description() string {
	return "Schedule a persisted gateway job. When it fires, the gateway runs the agent with your message. " +
		"Use exactly one of expr (six-field cron with seconds), in (duration e.g. 1m), or at_ms. " +
		"From a gateway chat, delivery defaults to the current conversation if you omit deliver/channel/to; set deliver false for a silent run. " +
		"You may also set deliver_to_incoming_chat true explicitly instead of guessing channel/to."
}

func (t *scheduleTool) Schema() *tool.JSONSchema { return cronScheduleToolSchema }

func (t *scheduleTool) Execute(ctx context.Context, params map[string]interface{}) (*tool.ToolResult, error) {
	job, err := AddFromToolMap(t.svc, ctx, params, time.Now())
	if err != nil {
		return nil, err
	}
	return &tool.ToolResult{Success: true, Output: FormatJobAdded(job)}, nil
}

type listTool struct{ svc *Service }

func (t *listTool) Name() string { return "CronList" }

func (t *listTool) Description() string {
	return "List all persisted gateway cron jobs (id, schedule, delivery targets)."
}

func (t *listTool) Schema() *tool.JSONSchema { return cronListToolSchema }

func (t *listTool) Execute(_ context.Context, _ map[string]interface{}) (*tool.ToolResult, error) {
	out := FormatList(t.svc.ListJobs())
	return &tool.ToolResult{Success: true, Output: out}, nil
}

type removeTool struct{ svc *Service }

func (t *removeTool) Name() string { return "CronRemove" }

func (t *removeTool) Description() string {
	return "Remove a gateway cron job by id (use CronList to see ids)."
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
