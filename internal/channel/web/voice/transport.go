package voice

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/maven/internal/channel/web/wsession"
	"github.com/ageneralai/maven/internal/channel/web/wsmsg"
	"github.com/ageneralai/maven/internal/config"
	"github.com/ageneralai/maven/internal/voice"
	"github.com/ageneralai/maven/pkg/executor"
	"github.com/ageneralai/maven/pkg/plugin"
	pkgvoice "github.com/ageneralai/maven/pkg/voice"
	"github.com/coder/websocket"
	"log/slog"
)

const voiceClearSentinel = byte(0)

const voiceDetectRMSThreshold = 0.01

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
			vc.sess.Close()
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
	sess    *voice.Session
	conn    *websocket.Conn
	writeMu sync.Mutex
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
	sess := voice.NewSession(r.Context(), stt, tts)
	defer sess.Close()
	vc := &client{sess: sess, conn: conn}
	t.voiceClients.Store(sessionID, vc)
	t.log.Info("web voice client connected", "session", sessionID)
	defer func() {
		t.voiceClients.Delete(sessionID)
		_ = conn.CloseNow()
		t.log.Info("web voice client disconnected", "session", sessionID)
	}()
	audioCh := make(chan []byte, 64)
	go func() {
		defer close(audioCh)
		voiceDetectArmed := true
		for {
			typ, data, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			if typ != websocket.MessageBinary {
				continue
			}
			if len(data) == 0 {
				continue
			}
			speaking := pcmRMS(data) > voiceDetectRMSThreshold
			if speaking && voiceDetectArmed {
				voiceDetectArmed = false
				sess.Interrupt()
				vc.writeCancel()
			} else if !speaking {
				voiceDetectArmed = true
			}
			select {
			case <-r.Context().Done():
				return
			case audioCh <- data:
			}
		}
	}()
	go func() {
		err := sess.RunSTT(audioCh, func(text string) {
			sess.Interrupt()
			vc.writeCancel()
			events, err := t.runner.RunStream(r.Context(), text, sessionID)
			if err != nil {
				t.log.Error("web voice agent stream", "session", sessionID, "err", err)
				return
			}
			if err := t.SendStream(r.Context(), sessionID, events); err != nil {
				t.log.Error("web voice tts stream", "session", sessionID, "err", err)
			}
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			t.log.Error("web voice STT", "err", err)
		}
	}()
	<-r.Context().Done()
}

func (c *client) writeCancel() {
	writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.Write(writeCtx, websocket.MessageBinary, []byte{voiceClearSentinel})
}

func pcmRMS(pcm []byte) float64 {
	samples := len(pcm) / 2
	if samples == 0 {
		return 0
	}
	var sum float64
	for i := 0; i+1 < len(pcm); i += 2 {
		v := int16(binary.LittleEndian.Uint16(pcm[i:]))
		x := float64(v) / 32768
		sum += x * x
	}
	return math.Sqrt(sum / float64(samples))
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

func streamEventError(ev api.StreamEvent) error {
	msg := strings.TrimSpace(fmt.Sprintf("%v", ev.Output))
	if msg == "" {
		msg = "stream error"
	}
	return fmt.Errorf("%s", msg)
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
	sess := vc.sess
	conn := vc.conn
	micAgentCtx := sess.NewAgentCtx()
	agentCtx, cancelAgent := context.WithCancel(micAgentCtx)
	defer cancelAgent()
	go func() {
		<-ctx.Done()
		cancelAgent()
	}()
	textCh := make(chan string, 8)
	var wg sync.WaitGroup
	var drainErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(textCh)
		buf := ""
		for {
			select {
			case <-agentCtx.Done():
				return
			case ev, ok := <-events:
				if !ok {
					tail := pkgvoice.FlushRemainder(&buf)
					if tail != "" {
						select {
						case textCh <- tail:
						case <-agentCtx.Done():
							return
						}
					}
					return
				}
				if ev.Type == api.EventError {
					drainErr = streamEventError(ev)
					sess.Interrupt()
					return
				}
				if ev.Type == api.EventContentBlockDelta && ev.Delta != nil && ev.Delta.Text != "" {
					buf += ev.Delta.Text
					for _, sent := range pkgvoice.TakeCompleteSentences(&buf) {
						select {
						case textCh <- sent:
						case <-agentCtx.Done():
							return
						}
					}
				}
			}
		}
	}()
	writeAudio := func(b []byte) error {
		writeCtx, cancel := context.WithTimeout(agentCtx, 5*time.Second)
		defer cancel()
		vc.writeMu.Lock()
		defer vc.writeMu.Unlock()
		return conn.Write(writeCtx, websocket.MessageBinary, b)
	}
	ttsErr := sess.RunTTS(agentCtx, textCh, writeAudio)
	if ttsErr != nil {
		sess.Interrupt()
	}
	wg.Wait()
	done, err := json.Marshal(wsmsg.Message{Type: "stream_done"})
	if err != nil {
		return err
	}
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	vc.writeMu.Lock()
	defer vc.writeMu.Unlock()
	if werr := conn.Write(writeCtx, websocket.MessageText, done); werr != nil {
		if drainErr == nil && ttsErr == nil {
			return werr
		}
	}
	if drainErr != nil {
		return drainErr
	}
	if ttsErr != nil && !errors.Is(ttsErr, context.Canceled) && !errors.Is(ttsErr, context.DeadlineExceeded) {
		return ttsErr
	}
	return nil
}
