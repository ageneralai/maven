package task

import (
	"context"
	"strings"

	turnctx "github.com/ageneralai/maven/kernel/turnctx"
	"github.com/ageneralai/maven/kernel/sessionid"
)

const parentSessionMetadataKey = "session_id"

func parentSessionID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if tc, ok := turnctx.From(ctx); ok && tc.Metadata != nil {
		if v, ok := tc.Metadata[parentSessionMetadataKey].(string); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func childSessionID(_ string) string {
	return sessionid.New(sessionid.KindTask, "").String()
}

func isNestedTaskSession(sessionID string) bool {
	id, err := sessionid.Parse(sessionID)
	return err == nil && id.Kind == sessionid.KindTask
}
