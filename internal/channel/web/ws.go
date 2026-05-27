package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/channel"
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
		w.log.Error("web websocket accept error", "err", err)
		return
	}
	clientID := fmt.Sprintf("web-%d", w.nextID.Add(1))
	client := &wsClient{conn: conn, id: clientID}
	w.clients.Store(clientID, client)
	w.log.Info("web client connected", "client", clientID)
	defer func() {
		w.clients.Delete(clientID)
		_ = conn.CloseNow()
		w.log.Info("web client disconnected", "client", clientID)
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
			w.log.Debug("web rejected message", "client", clientID)
			continue
		}
		_ = w.bus.PublishInbound(r.Context(), bus.InboundMessage{
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
			c, ok := value.(*wsClient)
			if ok {
				_ = c.conn.Write(ctx, websocket.MessageText, data)
			}
			return true
		})
		return nil
	}
	c, ok := client.(*wsClient)
	if !ok {
		return nil
	}
	return c.conn.Write(ctx, websocket.MessageText, data)
}

func (w *WebChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	if w.voice != nil && w.voice.HasSession(msg.ChatID) {
		return w.voice.Send(ctx, msg.ChatID, msg.Content)
	}
	data, err := json.Marshal(wsMessage{Type: "message", Content: msg.Content})
	if err != nil {
		return err
	}
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return w.writeToClient(writeCtx, msg.ChatID, data)
}

func (w *WebChannel) SendStream(ctx context.Context, chatID string, metadata map[string]any, events <-chan api.StreamEvent) error {
	_ = metadata
	if w.voice != nil && w.voice.HasSession(chatID) {
		return w.voice.SendStream(ctx, chatID, events)
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

func streamEventError(ev api.StreamEvent) error {
	msg := strings.TrimSpace(fmt.Sprintf("%v", ev.Output))
	if msg == "" {
		msg = "stream error"
	}
	return fmt.Errorf("%s", msg)
}

func (w *WebChannel) Capabilities() channel.CapabilitySet {
	return channel.CapabilitySet{}
}
