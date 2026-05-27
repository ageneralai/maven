package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/channel"
	"github.com/ageneralai/maven/internal/channel/allowlist"
	"github.com/ageneralai/maven/internal/channel/web/responses"
	webvoice "github.com/ageneralai/maven/internal/channel/web/voice"
	"github.com/ageneralai/maven/internal/channel/web/wsession"
	"github.com/ageneralai/maven/internal/config"
	"log/slog"

	"github.com/ageneralai/maven/pkg/plugin"
)

//go:embed static
var staticFiles embed.FS

const webChannelName = "web"

type WebChannel struct {
	name      string
	log       *slog.Logger
	bus       *bus.MessageBus
	allow     allowlist.Matcher
	port      int
	server    *http.Server
	clients   sync.Map
	nextID    atomic.Int64
	voiceCfg  config.WebVoiceConfig
	voice     *webvoice.Transport
	sessions  *wsession.ResponseSessions
	responses *responses.Handler
}

func NewWebChannel(cfg config.WebConfig, gwCfg config.GatewayConfig, appCfg *config.Config, plugins *plugin.Registry, lg *slog.Logger, b *bus.MessageBus, runner StreamRunner) (*WebChannel, error) {
	port := gwCfg.Port
	if port == 0 {
		port = config.DefaultPort
	}
	w := &WebChannel{
		name:     webChannelName,
		log:      lg,
		bus:      b,
		allow:    allowlist.NewMatcher(cfg.AllowFrom),
		port:     port,
		voiceCfg: cfg.Voice,
		sessions: wsession.NewResponseSessions(),
	}
	if cfg.Voice.Enabled {
		w.voice = webvoice.NewTransport(cfg.Voice, appCfg, plugins, lg, runner, w.sessions)
	}
	if runner != nil {
		w.responses = &responses.Handler{Runner: runner, Sessions: w.sessions}
	}
	return w, nil
}

func (w *WebChannel) Name() string {
	return w.name
}

func (w *WebChannel) IsAllowed(senderID string) bool {
	return w.allow.Allow(senderID)
}

func (w *WebChannel) Start(ctx context.Context) error {
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("embed static fs: %w", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/web/config", w.handleWebConfig)
	mux.HandleFunc("/ws", w.handleWS)
	if w.voice != nil {
		w.voice.Register(mux)
	}
	if w.responses != nil {
		mux.Handle("/v1/responses", w.responses)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))
	w.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", w.port),
		Handler: mux,
	}
	go func() {
		w.log.Info("web channel listening", "port", w.port)
		if err := w.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			w.log.Error("web server error", "err", err)
		}
	}()
	return nil
}

func (w *WebChannel) Stop() error {
	if w.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := w.server.Shutdown(ctx); err != nil {
			w.log.Error("web shutdown error", "err", err)
		}
	}
	w.clients.Range(func(key, value any) bool {
		c, ok := value.(*wsClient)
		if ok {
			_ = c.conn.CloseNow()
		}
		return true
	})
	if w.voice != nil {
		w.voice.Stop()
	}
	w.log.Info("web stopped")
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

var (
	_ channel.Channel       = (*WebChannel)(nil)
	_ channel.StreamChannel = (*WebChannel)(nil)
)
