package slash

import (
	"context"
	"strings"
)

func handleCompact(_ context.Context, inv Invocation) (Result, error) {
	focus := strings.TrimSpace(strings.Join(inv.Args, " "))
	var b strings.Builder
	b.WriteString("Create a compact continuation summary for this conversation. ")
	b.WriteString("This output is for the assistant, not for the end user. ")
	b.WriteString("Write only the summary text. Capture goals, decisions, constraints, preferences, important file paths, open questions, and the best next steps. ")
	b.WriteString("Do not address the user, do not mention compacting, and do not include markdown fences.")
	if focus != "" {
		b.WriteString(" Give extra attention to: ")
		b.WriteString(focus)
		b.WriteString(".")
	}
	return Result{
		Metadata: map[string]any{
			"api.prepend_prompt": b.String(),
		},
		PostAction: CompactRotateAction{ResponseMode: CompactResponseModeAck},
	}, nil
}
