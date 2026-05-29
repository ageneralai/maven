package responses

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/maven/internal/kernel/executor"
	"github.com/ageneralai/maven/internal/plugins/channel/web/wsession"
	"github.com/google/uuid"
)

type Handler struct {
	Runner   executor.StreamRunner
	Sessions *wsession.ResponseSessions
}

func (h *Handler) ServeHTTP(wr http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(wr, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(wr, "invalid request", "invalid_request_error", http.StatusBadRequest)
		return
	}
	prompt := extractPrompt(req.Input)
	if strings.TrimSpace(prompt) == "" {
		writeJSONError(wr, "input is required", "invalid_request_error", http.StatusBadRequest)
		return
	}
	sessionID, err := wsession.ResolveMavenSessionID(h.Sessions, r, req.PreviousResponseID)
	if err != nil {
		writeJSONError(wr, err.Error(), "invalid_request_error", http.StatusBadRequest)
		return
	}
	events, err := h.Runner.RunStream(r.Context(), prompt, sessionID)
	if err != nil {
		writeJSONError(wr, "agent error", "server_error", http.StatusInternalServerError)
		return
	}
	responseID := "resp_" + uuid.NewString()
	wr.Header().Set("Content-Type", "text/event-stream")
	wr.Header().Set("Cache-Control", "no-cache")
	wr.Header().Set("Connection", "keep-alive")
	fl, ok := wr.(http.Flusher)
	if !ok {
		http.Error(wr, "streaming not supported", http.StatusInternalServerError)
		return
	}
	writeEvent := func(eventType string, data any) {
		if err := writeSSE(wr, fl, eventType, data); err != nil {
			return
		}
	}
	writeEvent("response.created", CreatedEvent{
		Type:     "response.created",
		Response: ResponseRef{ID: responseID, Status: "in_progress"},
	})
	writeEvent("response.output_item.added", OutputItemAddedEvent{
		Type:  "response.output_item.added",
		Index: 0,
		Item:  OutputItem{Type: "message", Status: "in_progress"},
	})
	writeEvent("response.content_part.added", ContentPartAddedEvent{
		Type:         "response.content_part.added",
		ItemIndex:    0,
		ContentIndex: 0,
	})
	for ev := range events {
		if ev.Type == api.EventContentBlockDelta && ev.Delta != nil && ev.Delta.Text != "" {
			writeEvent("response.output_text.delta", OutputTextDeltaEvent{
				Type:         "response.output_text.delta",
				ItemIndex:    0,
				ContentIndex: 0,
				Delta:        ev.Delta.Text,
			})
		}
	}
	writeEvent("response.output_text.done", ItemIndexEvent{
		Type: "response.output_text.done", ItemIndex: 0, ContentIndex: 0,
	})
	writeEvent("response.content_part.done", ItemIndexEvent{
		Type: "response.content_part.done", ItemIndex: 0, ContentIndex: 0,
	})
	writeEvent("response.output_item.done", ItemIndexEvent{
		Type: "response.output_item.done", Index: 0,
	})
	h.Sessions.StoreMavenResponseSession(responseID, sessionID)
	writeEvent("response.completed", CompletedEvent{
		Type:     "response.completed",
		Response: ResponseRef{ID: responseID, Status: "completed"},
	})
	_ = writeDone(wr, fl)
}

func writeJSONError(wr http.ResponseWriter, message, errType string, status int) {
	var body ErrorBody
	body.Error.Message = message
	body.Error.Type = errType
	b, _ := json.Marshal(body)
	wr.Header().Set("Content-Type", "application/json; charset=utf-8")
	wr.WriteHeader(status)
	_, _ = wr.Write(b)
}

func extractPrompt(input any) string {
	switch v := input.(type) {
	case string:
		return v
	case []any:
		last := ""
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if m["type"] == "message" && m["role"] == "user" {
				if c, ok := m["content"].(string); ok {
					last = c
				}
			}
		}
		return last
	}
	return ""
}
