package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/maven/internal/bus"
	chann "github.com/ageneralai/maven/internal/channel"
	pkgvoice "github.com/ageneralai/maven/pkg/voice"
	"github.com/coder/websocket"
)

type wsMessage struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	Delta   string `json:"delta,omitempty"`
}

type wsClient struct {
	conn *websocket.Conn
	id   string
}

func (w *WebChannel) handleWS(wr http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(wr, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		w.Log.Printf("[web] websocket accept error: %v", err)
		return
	}

	clientID := fmt.Sprintf("web-%d", w.nextID.Add(1))
	client := &wsClient{conn: conn, id: clientID}
	w.clients.Store(clientID, client)
	w.Log.Printf("[web] client connected: %s", clientID)

	defer func() {
		w.clients.Delete(clientID)
		_ = conn.CloseNow()
		w.Log.Printf("[web] client disconnected: %s", clientID)
	}()

	for {
		_, data, err := conn.Read(r.Context())
		if err != nil {
			return
		}
		var msg wsMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		if msg.Type != "message" || msg.Content == "" {
			continue
		}
		if !w.IsAllowed(clientID) {
			w.Log.Printf("[web] rejected message from %s", clientID)
			continue
		}
		_ = w.Bus.PublishInbound(r.Context(), bus.InboundMessage{
			Channel:   webChannelName,
			SenderID:  clientID,
			ChatID:    clientID,
			Content:   msg.Content,
			Timestamp: time.Now(),
		})
	}
}

func (w *WebChannel) writeToClient(ctx context.Context, chatID string, data []byte) error {
	client, ok := w.clients.Load(chatID)
	if !ok {
		w.clients.Range(func(key, value any) bool {
			c := value.(*wsClient)
			_ = c.conn.Write(ctx, websocket.MessageText, data)
			return true
		})
		return nil
	}
	c := client.(*wsClient)
	return c.conn.Write(ctx, websocket.MessageText, data)
}

func (w *WebChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	if _, ok := w.voiceSessions.Load(msg.ChatID); ok {
		data, err := json.Marshal(wsMessage{Type: "message", Content: msg.Content})
		if err != nil {
			return err
		}
		writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		return w.writeVoiceClient(writeCtx, msg.ChatID, websocket.MessageText, data)
	}
	data, err := json.Marshal(wsMessage{Type: "message", Content: msg.Content})
	if err != nil {
		return err
	}
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return w.writeToClient(writeCtx, msg.ChatID, data)
}

func (w *WebChannel) writeVoiceClient(ctx context.Context, chatID string, typ websocket.MessageType, data []byte) error {
	v, ok := w.voiceSessions.Load(chatID)
	if !ok {
		return nil
	}
	vc := v.(*voiceClient)
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

func (w *WebChannel) SendStream(ctx context.Context, chatID string, metadata map[string]any, events <-chan api.StreamEvent) error {
	_ = metadata
	if _, ok := w.voiceSessions.Load(chatID); ok {
		return w.sendStreamVoice(ctx, chatID, events)
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-events:
			if !ok {
				done, err := json.Marshal(wsMessage{Type: "stream_done"})
				if err != nil {
					return err
				}
				return w.writeToClient(ctx, chatID, done)
			}
			if ev.Type == api.EventError {
				return streamEventError(ev)
			}
			if ev.Type == api.EventContentBlockDelta && ev.Delta != nil && ev.Delta.Text != "" {
				payload, err := json.Marshal(wsMessage{Type: "stream", Delta: ev.Delta.Text})
				if err != nil {
					return err
				}
				if err := w.writeToClient(ctx, chatID, payload); err != nil {
					return err
				}
			}
		}
	}
}

func (w *WebChannel) sendStreamVoice(ctx context.Context, chatID string, events <-chan api.StreamEvent) error {
	v, ok := w.voiceSessions.Load(chatID)
	if !ok {
		return nil
	}
	vc := v.(*voiceClient)
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
	done, err := json.Marshal(wsMessage{Type: "stream_done"})
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

func (w *WebChannel) Capabilities() chann.CapabilitySet {
	return chann.CapabilitySet{}
}
