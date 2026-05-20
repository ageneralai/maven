package web

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

// voiceControlDetect: client signals mic energy above threshold — interrupt agent turn early.
const voiceControlDetect = byte(1)

// voiceClient binds a voice Session to one WebSocket (transport only).
type voiceClient struct {
	sess *voice.Session
	conn *websocket.Conn
}

func (w *WebChannel) handleVoiceWS(wr http.ResponseWriter, r *http.Request) {
	if !w.voiceCfg.Enabled {
		http.NotFound(wr, r)
		return
	}
	conn, err := websocket.Accept(wr, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		w.Log.Printf("[web] voice websocket accept error: %v", err)
		return
	}
	stt, err := voice.NewSTT(w.appCfg, w.plugins)
	if err != nil {
		w.Log.Printf("[web] voice stt init: %v", err)
		_ = conn.CloseNow()
		return
	}
	tts, err := voice.NewTTS(w.appCfg, w.plugins)
	if err != nil {
		w.Log.Printf("[web] voice tts init: %v", err)
		_ = conn.CloseNow()
		return
	}
	sess := voice.NewSession(r.Context(), stt, tts)
	defer sess.Close()
	clientID := fmt.Sprintf("webui-%d", w.nextID.Add(1))
	vc := &voiceClient{sess: sess, conn: conn}
	w.voiceSessions.Store(clientID, vc)
	w.Log.Printf("[web] voice client connected: %s", clientID)
	defer func() {
		w.voiceSessions.Delete(clientID)
		_ = conn.CloseNow()
		w.Log.Printf("[web] voice client disconnected: %s", clientID)
	}()
	audioCh := make(chan []byte, 64)
	go func() {
		defer close(audioCh)
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
			if len(data) == 1 && data[0] == voiceControlDetect {
				sess.Interrupt()
				writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = conn.Write(writeCtx, websocket.MessageBinary, []byte{voiceClearSentinel})
				cancel()
				continue
			}
			select {
			case <-r.Context().Done():
				return
			case audioCh <- data:
			}
		}
	}()
	go func() {
		err := sess.RunSTT(audioCh, func(t string) {
			if !w.IsAllowed(clientID) {
				w.Log.Printf("[web] rejected voice transcript from %s", clientID)
				return
			}
			sess.Interrupt()
			writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = conn.Write(writeCtx, websocket.MessageBinary, []byte{voiceClearSentinel})
			cancel()
			_ = w.Bus.PublishInbound(r.Context(), bus.InboundMessage{
				Channel:   webChannelName,
				SenderID:  clientID,
				ChatID:    clientID,
				Content:   t,
				Timestamp: time.Now(),
			})
		})
		if err != nil && err != context.Canceled {
			w.Log.Printf("[web] voice STT: %v", err)
		}
	}()
	<-r.Context().Done()
}
