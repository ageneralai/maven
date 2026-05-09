package webui

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/voice"
	"github.com/coder/websocket"
)

const voiceClearSentinel = byte(0)

// voiceBinding attaches a voice.Session to one WebSocket (transport adapter only).
type voiceBinding struct {
	session *voice.Session
	conn    *websocket.Conn
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

func (w *WebUIChannel) consumeTranscripts(ctx context.Context, sess *voice.Session, conn *websocket.Conn, clientID string, audio <-chan []byte) {
	err := sess.ConsumeTranscripts(ctx, audio, func(ctx context.Context, t string) error {
		if !w.IsAllowed(clientID) {
			w.Log.Printf("[webui] rejected voice transcript from %s", clientID)
			return nil
		}
		if sess.Speaking() {
			_ = sess.InterruptPlayback(ctx, func(c context.Context) error {
				return conn.Write(c, websocket.MessageBinary, []byte{voiceClearSentinel})
			})
		}
		return w.Bus.PublishInbound(ctx, bus.InboundMessage{
			Channel:   webUIChannelName,
			SenderID:  clientID,
			ChatID:    clientID,
			Content:   t,
			Timestamp: time.Now(),
		})
	})
	if err != nil && err != context.Canceled {
		w.Log.Printf("[webui] voice transcript loop: %v", err)
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
	sess := voice.NewSession(stt, tts)
	clientID := fmt.Sprintf("webui-%d", w.nextID.Add(1))
	vb := &voiceBinding{session: sess, conn: conn}
	w.voiceSessions.Store(clientID, vb)
	w.Log.Printf("[webui] voice client connected: %s", clientID)
	defer func() {
		w.voiceSessions.Delete(clientID)
		_ = conn.CloseNow()
		w.Log.Printf("[webui] voice client disconnected: %s", clientID)
	}()
	ctx := r.Context()
	audioCh := make(chan []byte, 64)
	go w.pumpVoiceAudio(ctx, conn, audioCh)
	w.consumeTranscripts(ctx, sess, conn, clientID, audioCh)
}
