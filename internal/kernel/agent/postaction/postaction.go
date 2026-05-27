package postaction

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/ageneral-agents-go/pkg/message"
	"github.com/ageneralai/maven/internal/kernel/bus"
	"github.com/ageneralai/maven/internal/kernel/session"
	"github.com/ageneralai/maven/internal/kernel/slash"
)

// Handler applies gateway post-turn effects using slash.PreTurn trail metadata.
type Handler interface {
	SetWorkspace(workspace string)
	HandleBuiltin(msg bus.InboundMessage) (bool, error)
	HandlePostResponse(ctx context.Context, chatRouteKey string, resp *api.Response, trail []slash.Execution) (string, bool, error)
}

type handler struct {
	sessions  *session.Router
	workspace string
}

func New(sessions *session.Router, workspace string) Handler {
	if sessions == nil {
		panic("postaction: sessions router is required")
	}
	return &handler{sessions: sessions, workspace: workspace}
}

func (h *handler) SetWorkspace(workspace string) {
	h.workspace = workspace
}

func (h *handler) HandleBuiltin(msg bus.InboundMessage) (bool, error) {
	switch strings.TrimSpace(msg.Hints.BuiltinCommand) {
	case "new":
		_, _, err := h.sessions.Rotate(msg.StableRouteKey())
		return true, err
	default:
		return false, nil
	}
}

func (h *handler) HandlePostResponse(ctx context.Context, chatRouteKey string, resp *api.Response, trail []slash.Execution) (string, bool, error) {
	_ = ctx
	action := extractPostAction(trail)
	switch a := action.(type) {
	case slash.CompactRotateAction:
		summary := strings.TrimSpace(resultOutput(resp))
		if summary == "" {
			return "", true, fmt.Errorf("compact summary is empty")
		}
		oldSessionID, newSessionID, err := h.sessions.Rotate(chatRouteKey)
		if err != nil {
			return "", true, err
		}
		if err := seedSessionSummary(h.workspace, newSessionID, summary); err != nil {
			_ = h.sessions.Set(chatRouteKey, oldSessionID)
			return "", true, err
		}
		if a.ResponseMode == slash.CompactResponseModeAck {
			return "✅ Conversation compacted and continued in a fresh session.", true, nil
		}
		return summary, true, nil
	default:
		return "", false, nil
	}
}

func extractPostAction(trail []slash.Execution) slash.PostAction {
	for _, ex := range trail {
		if ex.Result.PostAction != nil {
			return ex.Result.PostAction
		}
	}
	return nil
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
	historyDir := filepath.Join(workspace, ".maven", "history")
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
