package cron

import (
	"fmt"
	"strings"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
)

func FormatJobAdded(job *CronJob) string {
	if job == nil {
		return ""
	}
	j := *job
	sched := formatSchedule(j.Schedule)
	del := ""
	if j.Payload.Deliver {
		del = fmt.Sprintf(" deliver→ %s:%s", j.Payload.Channel, j.Payload.To)
	}
	return fmt.Sprintf("Added job id=%s name=%q %s%s", j.ID, j.Name, sched, del)
}

func FormatJobLine(j CronJob) string {
	en := "off"
	if j.Enabled {
		en = "on"
	}
	d := ""
	if j.Payload.Deliver {
		d = fmt.Sprintf(" →%s:%s", j.Payload.Channel, j.Payload.To)
	}
	return fmt.Sprintf("%s name=%q enabled=%s %s msg=%q%s", j.ID, j.Name, en, formatScheduleLine(j.Schedule), j.Payload.Message, d)
}

func formatSchedule(s Schedule) string {
	switch v := s.(type) {
	case CronSchedule:
		return fmt.Sprintf("cron expr=%q", v.Expr)
	case AtSchedule:
		return fmt.Sprintf("at ms=%d (%s)", v.At.UnixMilli(), v.At.UTC().Format(time.RFC3339))
	case EverySchedule:
		return fmt.Sprintf("every %d ms", v.Interval.Milliseconds())
	default:
		return fmt.Sprintf("schedule=%T", s)
	}
}

func formatScheduleLine(s Schedule) string {
	switch v := s.(type) {
	case CronSchedule:
		return fmt.Sprintf("expr=%q", v.Expr)
	case AtSchedule:
		return fmt.Sprintf("at=%s", v.At.UTC().Format(time.RFC3339))
	case EverySchedule:
		return fmt.Sprintf("everyMs=%d", v.Interval.Milliseconds())
	default:
		return fmt.Sprintf("schedule=%T", s)
	}
}

func FormatList(jobs []CronJob) string {
	if len(jobs) == 0 {
		return "No cron jobs."
	}
	var b strings.Builder
	for _, j := range jobs {
		b.WriteString(FormatJobLine(j))
		b.WriteByte('\n')
	}
	return strings.TrimSuffix(b.String(), "\n")
}

var cronScheduleToolSchema = &tool.JSONSchema{
	Type: "object",
	Properties: map[string]any{
		"name": map[string]any{
			"type":        "string",
			"description": "Short label for this job (e.g. read-reminder).",
		},
		"message": map[string]any{
			"type":        "string",
			"description": "Prompt passed to the agent when the job runs (what you want done or said).",
		},
		"expr": map[string]any{
			"type":        "string",
			"description": "Optional. Six-field cron with seconds (e.g. 0 30 14 * * *). Mutually exclusive with in and at_ms.",
		},
		"in": map[string]any{
			"type":        "string",
			"description": "Optional. Duration from now until one-shot run (e.g. 1m, 90s). Mutually exclusive with expr and at_ms.",
		},
		"at_ms": map[string]any{
			"type":        "number",
			"description": "Optional. Unix milliseconds for one-shot run. Mutually exclusive with expr and in.",
		},
		"deliver": map[string]any{
			"type":        "boolean",
			"description": "If true, send the agent output to channel+to when the job runs.",
		},
		"deliver_to_incoming_chat": map[string]any{
			"type":        "boolean",
			"description": "If true, deliver to the same channel/chat as the current conversation. Sets deliver implicitly. When omitted in an active chat with no channel/to, defaults to true.",
		},
		"channel": map[string]any{
			"type":        "string",
			"description": "Outbound channel when deliver is true (e.g. telegram). Ignored if deliver_to_incoming_chat is true.",
		},
		"to": map[string]any{
			"type":        "string",
			"description": "Recipient id when deliver is true. Ignored if deliver_to_incoming_chat is true.",
		},
	},
	Required: []string{"name", "message"},
}

var cronListToolSchema = &tool.JSONSchema{
	Type:       "object",
	Properties: map[string]any{},
}

var cronRemoveToolSchema = &tool.JSONSchema{
	Type: "object",
	Properties: map[string]any{
		"id": map[string]any{
			"type":        "string",
			"description": "Job id from cron-list output.",
		},
	},
	Required: []string{"id"},
}
