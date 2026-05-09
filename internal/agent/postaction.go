package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/ageneral-agents-go/pkg/message"
	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/session"
	"github.com/ageneralai/maven/internal/slash"
)

// PostActionHandler applies gateway post-turn effects using slash.PreTurn trail metadata.
type PostActionHandler struct {
	Sessions  *session.Router
	Workspace string
}

func (h *PostActionHandler) HandleBuiltin(msg bus.InboundMessage) (bool, error) {
	switch strings.TrimSpace(msg.Hints.BuiltinCommand) {
	case "new":
		if h == nil || h.Sessions == nil {
			return true, nil
		}
		_, _, err := h.Sessions.Rotate(msg.StableRouteKey())
		return true, err
	default:
		return false, nil
	}
}

func (h *PostActionHandler) HandlePostResponse(ctx context.Context, chatRouteKey string, resp *api.Response, trail []slash.Execution) (string, bool, error) {
	// turnctx.From(ctx) is available here for future post-action use of Metadata/Budget.
	action := trailMeta(trail, slash.MetaPostAction)
	if action == "" {
		return "", false, nil
	}
	switch action {
	case slash.PostActionCompactRotate:
		summary := strings.TrimSpace(resultOutput(resp))
		if summary == "" {
			return "", true, fmt.Errorf("compact summary is empty")
		}
		if h == nil || h.Sessions == nil {
			return "", true, fmt.Errorf("session router is not configured")
		}
		oldSessionID, newSessionID, err := h.Sessions.Rotate(chatRouteKey)
		if err != nil {
			return "", true, err
		}
		if err := seedSessionSummary(h.Workspace, newSessionID, summary); err != nil {
			_ = h.Sessions.Set(chatRouteKey, oldSessionID)
			return "", true, err
		}
		if trailMeta(trail, slash.MetaResponse) == slash.ResponseCompactAck {
			return "✅ Conversation compacted and continued in a fresh session.", true, nil
		}
		return summary, true, nil
	default:
		return "", false, nil
	}
}

func trailMeta(trail []slash.Execution, key string) string {
	if key == "" {
		return ""
	}
	for _, ex := range trail {
		if ex.Result.Metadata == nil {
			continue
		}
		if value, ok := ex.Result.Metadata[key]; ok {
			if text := strings.TrimSpace(fmt.Sprint(value)); text != "" {
				return text
			}
		}
	}
	return ""
}

func resultOutput(resp *api.Response) string {
	if resp == nil || resp.Result == nil {
		return ""
	}
	return resp.Result.Output
}

func seedSessionSummary(workspace, sessionID, summary string) error {
	workspace = strings.TrimSpace(workspace)
	sessionID = strings.TrimSpace(sessionID)
	summary = strings.TrimSpace(summary)
	if workspace == "" || sessionID == "" || summary == "" {
		return fmt.Errorf("invalid compact seed payload")
	}
	historyDir := filepath.Join(workspace, ".claude", "history")
	if err := os.MkdirAll(historyDir, 0o700); err != nil {
		return fmt.Errorf("mkdir compact history dir: %w", err)
	}
	payload := []message.Message{{
		Role:    "system",
		Content: "Previous conversation summary:\n" + summary,
	}}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode compact seed: %w", err)
	}
	path := filepath.Join(historyDir, sessionID+".json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write compact seed: %w", err)
	}
	return nil
}
