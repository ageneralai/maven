package cronschedule

import (
	"fmt"
	"strings"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/cron"
)

func FormatJobAdded(job *cron.CronJob) string {
	if job == nil {
		return ""
	}
	j := *job
	var sched string
	switch j.Schedule.Kind {
	case "cron":
		sched = fmt.Sprintf("cron expr=%q", j.Schedule.Expr)
	case "at":
		sched = fmt.Sprintf("at ms=%d (%s)", j.Schedule.AtMs, time.UnixMilli(j.Schedule.AtMs).UTC().Format(time.RFC3339))
	case "every":
		sched = fmt.Sprintf("every %d ms", j.Schedule.EveryMs)
	default:
		sched = fmt.Sprintf("kind=%s", j.Schedule.Kind)
	}
	del := ""
	if j.Payload.Deliver {
		del = fmt.Sprintf(" deliver→ %s:%s", j.Payload.Channel, j.Payload.To)
	}
	return fmt.Sprintf("Added job id=%s name=%q %s%s", j.ID, j.Name, sched, del)
}

func FormatJobLine(j cron.CronJob) string {
	en := "off"
	if j.Enabled {
		en = "on"
	}
	var sched string
	switch j.Schedule.Kind {
	case "cron":
		sched = fmt.Sprintf("expr=%q", j.Schedule.Expr)
	case "at":
		sched = fmt.Sprintf("at=%s", time.UnixMilli(j.Schedule.AtMs).UTC().Format(time.RFC3339))
	case "every":
		sched = fmt.Sprintf("everyMs=%d", j.Schedule.EveryMs)
	default:
		sched = fmt.Sprintf("kind=%s", j.Schedule.Kind)
	}
	d := ""
	if j.Payload.Deliver {
		d = fmt.Sprintf(" →%s:%s", j.Payload.Channel, j.Payload.To)
	}
	return fmt.Sprintf("%s name=%q enabled=%s %s msg=%q%s", j.ID, j.Name, en, sched, j.Payload.Message, d)
}

func FormatList(jobs []cron.CronJob) string {
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

var scheduleSchema = &tool.JSONSchema{
	Type: "object",
	Properties: map[string]interface{}{
		"name": map[string]interface{}{
			"type":        "string",
			"description": "Short label for this job (e.g. read-reminder).",
		},
		"message": map[string]interface{}{
			"type":        "string",
			"description": "Prompt passed to the agent when the job runs (what you want done or said).",
		},
		"expr": map[string]interface{}{
			"type":        "string",
			"description": "Optional. Six-field cron with seconds (e.g. 0 30 14 * * *). Mutually exclusive with in and at_ms.",
		},
		"in": map[string]interface{}{
			"type":        "string",
			"description": "Optional. Duration from now until one-shot run (e.g. 1m, 90s). Mutually exclusive with expr and at_ms.",
		},
		"at_ms": map[string]interface{}{
			"type":        "number",
			"description": "Optional. Unix milliseconds for one-shot run. Mutually exclusive with expr and in.",
		},
		"deliver": map[string]interface{}{
			"type":        "boolean",
			"description": "If true, send the agent output to channel+to when the job runs.",
		},
		"deliver_to_incoming_chat": map[string]interface{}{
			"type":        "boolean",
			"description": "If true (gateway chat only), deliver to the same channel/chat as the current conversation. Sets deliver implicitly. When omitted from a gateway chat with no channel/to, defaults to true.",
		},
		"channel": map[string]interface{}{
			"type":        "string",
			"description": "Outbound channel when deliver is true (e.g. telegram). Ignored if deliver_to_incoming_chat is true.",
		},
		"to": map[string]interface{}{
			"type":        "string",
			"description": "Recipient id when deliver is true. Ignored if deliver_to_incoming_chat is true.",
		},
	},
	Required: []string{"name", "message"},
}

var listSchema = &tool.JSONSchema{
	Type:       "object",
	Properties: map[string]interface{}{},
}

var removeSchema = &tool.JSONSchema{
	Type: "object",
	Properties: map[string]interface{}{
		"id": map[string]interface{}{
			"type":        "string",
			"description": "Job id from CronList output.",
		},
	},
	Required: []string{"id"},
}
