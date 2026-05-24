package task

import (
	"context"
	"strings"

	"github.com/ageneralai/ageneral-agents-go/pkg/middleware"
	"github.com/ageneralai/ageneral-agents-go/pkg/model"
	"github.com/google/uuid"
)

func parentSessionID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if st, ok := ctx.Value(model.MiddlewareStateKey).(*middleware.State); ok && st != nil {
		if v, ok := st.Values["session_id"].(string); ok {
			if s := strings.TrimSpace(v); s != "" {
				return s
			}
		}
	}
	if v, ok := ctx.Value(middleware.TraceSessionIDContextKey).(string); ok {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	if v, ok := ctx.Value(middleware.SessionIDContextKey).(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func childSessionID(parent string) string {
	parent = strings.TrimSpace(parent)
	if parent == "" {
		parent = "maven"
	}
	return parent + ":task:" + uuid.NewString()
}

func isNestedTaskSession(sessionID string) bool {
	return strings.Contains(sessionID, ":task:")
}
