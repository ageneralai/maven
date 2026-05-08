package runtimecmd

import (
	"context"
	"strings"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/ageneral-agents-go/pkg/runtime/commands"
	"github.com/ageneralai/maven/internal/cron"
)

const (
	MetaPostAction = "maven.post_action"
	MetaResponse   = "maven.response_mode"

	PostActionCompactRotate = "compact_rotate"
	ResponseCompactAck      = "compact_ack"
)

func Build(cronSvc *cron.Service) []api.CommandRegistration {
	regs := []api.CommandRegistration{{
		Definition: commands.Definition{
			Name:        "compact",
			Description: "Compress the current conversation into a fresh continuation context.",
		},
		Handler: commands.HandlerFunc(handleCompact),
	}}
	regs = append(regs, cronRegistrations(cronSvc)...)
	return regs
}

func handleCompact(_ context.Context, inv commands.Invocation) (commands.Result, error) {
	focus := strings.TrimSpace(strings.Join(inv.Args, " "))
	var prompt strings.Builder
	prompt.WriteString("Create a compact continuation summary for this conversation. ")
	prompt.WriteString("This output is for the assistant, not for the end user. ")
	prompt.WriteString("Write only the summary text. Capture goals, decisions, constraints, preferences, important file paths, open questions, and the best next steps. ")
	prompt.WriteString("Do not address the user, do not mention compacting, and do not include markdown fences.")
	if focus != "" {
		prompt.WriteString(" Give extra attention to: ")
		prompt.WriteString(focus)
		prompt.WriteString(".")
	}

	return commands.Result{
		Metadata: map[string]any{
			"api.prepend_prompt": prompt.String(),
			MetaPostAction:       PostActionCompactRotate,
			MetaResponse:         ResponseCompactAck,
		},
	}, nil
}
