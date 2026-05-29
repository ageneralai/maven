package voice

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/converse"
	"github.com/ageneralai/maven/internal/kernel/converse/adapter"
	"github.com/ageneralai/maven/internal/kernel/executor"
	"github.com/ageneralai/maven/internal/kernel/plugin"
	"github.com/ageneralai/maven/internal/kernel/voice"
	"github.com/ageneralai/maven/internal/plugins/channel/web/wsession"
	"github.com/ageneralai/maven/internal/plugins/channel/web/wsmsg"
	"github.com/coder/websocket"
	"log/slog"
)

const voiceClearSentinel = byte(0)

type Transport struct {
	log              *slog.Logger
	voiceCfg         config.WebVoiceConfig
	appCfg           *config.Config
	plugins          *plugin.Registry
	runner           executor.StreamRunner
	responseSessions *wsession.ResponseSessions
	voiceClients     sync.Map
}

func NewTransport(voiceCfg config.WebVoiceConfig, appCfg *config.Config, plugins *plugin.Registry, lg *slog.Logger, runner executor.StreamRunner, responseSessions *wsession.ResponseSessions) *Transport {
	return &Transport{
		log:              lg,
		voiceCfg:         voiceCfg,
		appCfg:           appCfg,
		plugins:          plugins,
		runner:           runner,
		responseSessions: responseSessions,
	}
}

func (t *Transport) Enabled() bool {
	return t.voiceCfg.Enabled
}

func (t *Transport) Register(mux *http.ServeMux) {
	if t.voiceCfg.Enabled {
		mux.HandleFunc("/ws/voice", t.handleVoiceWS)
	}
}

func (t *Transport) Stop() {
	t.voiceClients.Range(func(key, value any) bool {
		vc, ok := value.(*client)
		if ok {
			vc.close()
			_ = vc.conn.CloseNow()
		}
		return true
	})
}

func (t *Transport) HasSession(chatID string) bool {
	_, ok := t.voiceClients.Load(chatID)
	return ok
}

type client struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
	tts     voice.TTS
	cancel  context.CancelFunc
}

func (c *client) close() {
	if c.cancel != nil {
		c.cancel()
	}
}

func (c *client) writeBinary(ctx context.Context, data []byte) error {
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.Write(writeCtx, websocket.MessageBinary, data)
}

func (c *client) writeText(ctx context.Context, data []byte) error {
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.Write(writeCtx, websocket.MessageText, data)
}

func (t *Transport) handleVoiceWS(wr http.ResponseWriter, r *http.Request) {
	if !t.voiceCfg.Enabled {
		http.NotFound(wr, r)
		return
	}
	sessionID, err := wsession.ResolveMavenSessionID(t.responseSessions, r, "")
	if err != nil {
		http.Error(wr, `{"error":{"message":"`+err.Error()+`","type":"invalid_request_error"}}`, http.StatusBadRequest)
		return
	}
	conn, err := websocket.Accept(wr, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		t.log.Error("web voice websocket accept error", "err", err)
		return
	}
	stt, err := voice.NewSTT(t.appCfg, t.plugins)
	if err != nil {
		t.log.Error("web voice stt init", "err", err)
		_ = conn.CloseNow()
		return
	}
	tts, err := voice.NewTTS(t.appCfg, t.plugins)
	if err != nil {
		t.log.Error("web voice tts init", "err", err)
		_ = conn.CloseNow()
		return
	}
	clientCtx, clientCancel := context.WithCancel(r.Context())
	defer clientCancel()
	vc := &client{
		conn:   conn,
		tts:    tts,
		cancel: clientCancel,
	}
	defer vc.close()
	t.voiceClients.Store(sessionID, vc)
	t.log.Info("web voice client connected", "session", sessionID)
	defer func() {
		t.voiceClients.Delete(sessionID)
		_ = conn.CloseNow()
		t.log.Info("web voice client disconnected", "session", sessionID)
	}()
	src := adapter.NewVoiceSource(adapter.VoiceSourceConfig{
		Open:    wsPCMOpener(conn),
		STT:     stt,
		Log:     t.log,
		Session: sessionID,
	})
	sink := &wsVoiceSink{w: vc, tts: tts, log: t.log, session: sessionID}
	ag := &adapter.StreamRunnerAgent{Runner: t.runner, SessionID: sessionID, Log: t.log}
	if err := converse.Converse(clientCtx, []converse.Source{src}, []converse.Sink{sink}, ag); err != nil && !errors.Is(err, context.Canceled) {
		t.log.Error("web voice converse", "session", sessionID, "err", err)
	}
}

func (t *Transport) Send(ctx context.Context, chatID string, content string) error {
	data, err := json.Marshal(wsmsg.Message{Type: "message", Content: content})
	if err != nil {
		return err
	}
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return t.writeClient(writeCtx, chatID, websocket.MessageText, data)
}

func (t *Transport) writeClient(ctx context.Context, chatID string, typ websocket.MessageType, data []byte) error {
	v, ok := t.voiceClients.Load(chatID)
	if !ok {
		return nil
	}
	vc, ok := v.(*client)
	if !ok {
		return nil
	}
	vc.writeMu.Lock()
	defer vc.writeMu.Unlock()
	return vc.conn.Write(ctx, typ, data)
}

func (t *Transport) SendStream(ctx context.Context, chatID string, events <-chan api.StreamEvent) error {
	v, ok := t.voiceClients.Load(chatID)
	if !ok {
		return nil
	}
	vc, ok := v.(*client)
	if !ok {
		return nil
	}
	evFwd := make(chan api.StreamEvent, 8)
	go func() {
		defer close(evFwd)
		for ev := range events {
			if ev.Type == api.EventError {
				return
			}
			select {
			case <-ctx.Done():
				return
			case evFwd <- ev:
			}
		}
	}()
	sink := &wsVoiceSink{w: vc, tts: vc.tts, log: t.log, session: chatID}
	err := sink.Render(ctx, adapter.Deltas(ctx, evFwd))
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return nil
}
