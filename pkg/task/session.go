package task

import (
	"context"
	"strings"

	"github.com/ageneralai/ageneral-agents-go/pkg/middleware"
	"github.com/google/uuid"
)

const taskSessionKeyPrefix = "task-"

// parentSessionID reads the session id injected by the SDK middleware.
// TraceSessionIDContextKey is the single source of truth.
// If the SDK renames or removes this key this function returns "" and
// childSessionID generates a fresh session — safe but orphaned from parent.
// TODO: ask SDK owners for a stable exported accessor rather than reading
// context keys directly.
func parentSessionID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(middleware.TraceSessionIDContextKey).(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func childSessionID(_ string) string {
	return taskSessionKeyPrefix + uuid.NewString()
}

func isNestedTaskSession(sessionID string) bool {
	return strings.HasPrefix(strings.TrimSpace(sessionID), taskSessionKeyPrefix)
}
