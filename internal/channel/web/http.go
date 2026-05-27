package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/maven/internal/bus"
	chann "github.com/ageneralai/maven/internal/channel"
	"github.com/ageneralai/maven/internal/config"
	"log/slog"

	"github.com/ageneralai/maven/pkg/plugin"
	"github.com/google/uuid"
)

//go:embed static
var staticFiles embed.FS

const webChannelName = "web"

type WebChannel struct {
	chann.BaseChannel
	port          int
	server        *http.Server
	clients       sync.Map
	voiceSessions sync.Map
	nextID        atomic.Int64
	voiceCfg      config.WebVoiceConfig
	appCfg        *config.Config
	plugins       *plugin.Registry
	runner        StreamRunner
}

func NewWebChannel(cfg config.WebConfig, gwCfg config.GatewayConfig, appCfg *config.Config, plugins *plugin.Registry, lg *slog.Logger, b *bus.MessageBus, runner StreamRunner) (*WebChannel, error) {
	port := gwCfg.Port
	if port == 0 {
		port = config.DefaultPort
	}
	return &WebChannel{
		BaseChannel: chann.NewBaseChannel(webChannelName, b, cfg.AllowFrom, lg),
		port:        port,
		voiceCfg:    cfg.Voice,
		appCfg:      appCfg,
		plugins:     plugins,
		runner:      runner,
	}, nil
}

func (w *WebChannel) Start(ctx context.Context) error {
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("embed static fs: %w", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/web/config", w.handleWebConfig)
	mux.HandleFunc("/ws", w.handleWS)
	if w.voiceCfg.Enabled {
		mux.HandleFunc("/ws/voice", w.handleVoiceWS)
	}
	if w.runner != nil {
		mux.HandleFunc("/v1/responses", w.handleResponses)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))
	w.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", w.port),
		Handler: mux,
	}
	go func() {
		w.Log.Info("web channel listening", "port", w.port)
		if err := w.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			w.Log.Error("web server error", "err", err)
		}
	}()
	return nil
}

func (w *WebChannel) Stop() error {
	if w.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := w.server.Shutdown(ctx); err != nil {
			w.Log.Error("web shutdown error", "err", err)
		}
	}
	w.clients.Range(func(key, value any) bool {
		c := value.(*wsClient)
		_ = c.conn.CloseNow()
		return true
	})
	w.voiceSessions.Range(func(key, value any) bool {
		vc := value.(*voiceClient)
		vc.sess.Close()
		_ = vc.conn.CloseNow()
		return true
	})
	w.Log.Info("web stopped")
	return nil
}

func (w *WebChannel) handleWebConfig(wr http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(wr, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	wr.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(wr).Encode(struct {
		VoiceEnabled bool `json:"voiceEnabled"`
	}{VoiceEnabled: w.voiceCfg.Enabled})
}

// handleResponses implements POST /v1/responses per the OpenResponses spec.
// https://www.openresponses.org/specification
func (w *WebChannel) handleResponses(wr http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(wr, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Input              any    `json:"input"`
		PreviousResponseID string `json:"previous_response_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(wr, `{"error":{"message":"invalid request","type":"invalid_request_error"}}`, http.StatusBadRequest)
		return
	}
	// Extract text from input — string or [{type:message,role:user,content:...}]
	prompt := extractPrompt(req.Input)
	if strings.TrimSpace(prompt) == "" {
		http.Error(wr, `{"error":{"message":"input is required","type":"invalid_request_error"}}`, http.StatusBadRequest)
		return
	}
	sessionID, err := resolveMavenSessionID(r, req.PreviousResponseID)
	if err != nil {
		http.Error(wr, `{"error":{"message":"`+err.Error()+`","type":"invalid_request_error"}}`, http.StatusBadRequest)
		return
	}

	events, err := w.runner.RunStream(r.Context(), prompt, sessionID)
	if err != nil {
		http.Error(wr, `{"error":{"message":"agent error","type":"server_error"}}`, http.StatusInternalServerError)
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
		b, _ := json.Marshal(data)
		fmt.Fprintf(wr, "event: %s\ndata: %s\n\n", eventType, b)
		fl.Flush()
	}

	writeEvent("response.created", map[string]any{
		"type":     "response.created",
		"response": map[string]any{"id": responseID, "status": "in_progress"},
	})
	writeEvent("response.output_item.added", map[string]any{
		"type":  "response.output_item.added",
		"index": 0,
		"item":  map[string]any{"type": "message", "status": "in_progress"},
	})
	writeEvent("response.content_part.added", map[string]any{
		"type":          "response.content_part.added",
		"item_index":    0,
		"content_index": 0,
	})

	for ev := range events {
		if ev.Type == api.EventContentBlockDelta && ev.Delta != nil && ev.Delta.Text != "" {
			writeEvent("response.output_text.delta", map[string]any{
				"type":          "response.output_text.delta",
				"item_index":    0,
				"content_index": 0,
				"delta":         ev.Delta.Text,
			})
		}
	}

	writeEvent("response.output_text.done", map[string]any{
		"type": "response.output_text.done", "item_index": 0, "content_index": 0,
	})
	writeEvent("response.content_part.done", map[string]any{
		"type": "response.content_part.done", "item_index": 0, "content_index": 0,
	})
	writeEvent("response.output_item.done", map[string]any{
		"type": "response.output_item.done", "index": 0,
	})
	writeEvent("response.completed", map[string]any{
		"type":     "response.completed",
		"response": map[string]any{"id": responseID, "status": "completed"},
	})
	storeMavenResponseSession(responseID, sessionID)
	fmt.Fprint(wr, "data: [DONE]\n\n")
	fl.Flush()
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
