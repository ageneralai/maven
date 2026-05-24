package web

import (
	"context"
	"encoding/binary"
	"math"
	"net/http"
	"time"

	"github.com/ageneralai/maven/internal/voice"
	"github.com/coder/websocket"
)

const voiceClearSentinel = byte(0)

const voiceDetectRMSThreshold = 0.01

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
	sessionID, err := resolveMavenSessionID(r, "")
	if err != nil {
		http.Error(wr, `{"error":{"message":"`+err.Error()+`","type":"invalid_request_error"}}`, http.StatusBadRequest)
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
	vc := &voiceClient{sess: sess, conn: conn}
	w.voiceSessions.Store(sessionID, vc)
	w.Log.Printf("[web] voice client connected: session=%s", sessionID)
	defer func() {
		w.voiceSessions.Delete(sessionID)
		_ = conn.CloseNow()
		w.Log.Printf("[web] voice client disconnected: session=%s", sessionID)
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
				writeVoiceCancel(conn)
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
		err := sess.RunSTT(audioCh, func(t string) {
			sess.Interrupt()
			writeVoiceCancel(conn)
			events, err := w.runner.RunStream(r.Context(), t, sessionID)
			if err != nil {
				w.Log.Printf("[web] voice agent stream session=%s: %v", sessionID, err)
				return
			}
			if err := w.sendStreamVoice(r.Context(), sessionID, events); err != nil {
				w.Log.Printf("[web] voice tts stream session=%s: %v", sessionID, err)
			}
		})
		if err != nil && err != context.Canceled {
			w.Log.Printf("[web] voice STT: %v", err)
		}
	}()
	<-r.Context().Done()
}

func writeVoiceCancel(conn *websocket.Conn) {
	writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = conn.Write(writeCtx, websocket.MessageBinary, []byte{voiceClearSentinel})
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
