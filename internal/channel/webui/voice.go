package webui

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/voice"
	pkgvoice "github.com/ageneralai/maven/pkg/voice"
	"github.com/coder/websocket"
)

const voiceClearSentinel = byte(0)

type voiceSessionState struct {
	mu        sync.Mutex
	conn      *websocket.Conn
	ttsCancel context.CancelFunc
	speaking  atomic.Bool
	stt       pkgvoice.STT
	tts       pkgvoice.TTS
}

func (vs *voiceSessionState) interruptPlayback(ctx context.Context) {
	vs.mu.Lock()
	if vs.ttsCancel != nil {
		vs.ttsCancel()
		vs.ttsCancel = nil
	}
	vs.mu.Unlock()
	_ = vs.conn.Write(ctx, websocket.MessageBinary, []byte{voiceClearSentinel})
}

func (w *WebUIChannel) pumpVoiceAudio(ctx context.Context, conn *websocket.Conn, audioCh chan<- []byte) {
	defer close(audioCh)
	for {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if typ != websocket.MessageBinary || len(data) == 0 {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case audioCh <- data:
		}
	}
}

func (w *WebUIChannel) consumeTranscripts(ctx context.Context, vs *voiceSessionState, clientID string, audio <-chan []byte) {
	txtCh, err := vs.stt.Transcribe(ctx, audio)
	if err != nil {
		w.Log.Printf("[webui] voice transcribe: %v", err)
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case t, ok := <-txtCh:
			if !ok {
				return
			}
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			if !w.IsAllowed(clientID) {
				w.Log.Printf("[webui] rejected voice transcript from %s", clientID)
				continue
			}
			if vs.speaking.Load() {
				vs.interruptPlayback(ctx)
			}
			_ = w.Bus.PublishInbound(ctx, bus.InboundMessage{
				Channel:   webUIChannelName,
				SenderID:  clientID,
				ChatID:    clientID,
				Content:   t,
				Timestamp: time.Now(),
			})
		}
	}
}

func (w *WebUIChannel) handleVoiceWS(wr http.ResponseWriter, r *http.Request) {
	if !w.voiceCfg.Enabled {
		http.NotFound(wr, r)
		return
	}
	conn, err := websocket.Accept(wr, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		w.Log.Printf("[webui] voice websocket accept error: %v", err)
		return
	}
	keys := voice.MergeKeys(w.appCfg)
	stt, err := voice.NewSTT(w.voiceCfg, keys)
	if err != nil {
		w.Log.Printf("[webui] voice stt init: %v", err)
		_ = conn.CloseNow()
		return
	}
	tts, err := voice.NewTTS(w.voiceCfg, keys)
	if err != nil {
		w.Log.Printf("[webui] voice tts init: %v", err)
		_ = conn.CloseNow()
		return
	}
	clientID := fmt.Sprintf("webui-%d", w.nextID.Add(1))
	vs := &voiceSessionState{conn: conn, stt: stt, tts: tts}
	w.voiceSessions.Store(clientID, vs)
	w.Log.Printf("[webui] voice client connected: %s", clientID)
	defer func() {
		w.voiceSessions.Delete(clientID)
		_ = conn.CloseNow()
		w.Log.Printf("[webui] voice client disconnected: %s", clientID)
	}()
	ctx := r.Context()
	audioCh := make(chan []byte, 64)
	go w.pumpVoiceAudio(ctx, conn, audioCh)
	w.consumeTranscripts(ctx, vs, clientID, audioCh)
}
