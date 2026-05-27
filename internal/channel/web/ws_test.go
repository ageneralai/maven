package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/config"
	"github.com/coder/websocket"
	"log/slog"
)

var webTestLog = slog.New(slog.DiscardHandler)

func TestNewWebChannel(t *testing.T) {
	b := bus.New(10, webTestLog)
	cfg := config.WebConfig{Enabled: true}
	gwCfg := config.GatewayConfig{Port: 0}

	ch, err := NewWebChannel(cfg, gwCfg, nil, nil, webTestLog, b, nil)
	if err != nil {
		t.Fatalf("NewWebChannel: %v", err)
	}
	if ch.Name() != "web" {
		t.Errorf("Name() = %q, want %q", ch.Name(), "web")
	}
}

func TestWebChannel_StartStop(t *testing.T) {
	b := bus.New(10, webTestLog)
	cfg := config.WebConfig{Enabled: true}
	gwCfg := config.GatewayConfig{Port: 19876}

	ch, err := NewWebChannel(cfg, gwCfg, nil, nil, webTestLog, b, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ch.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://localhost:19876/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("GET / status = %d, want 200", resp.StatusCode)
	}

	if err := ch.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestWebChannel_WebSocket(t *testing.T) {
	b := bus.New(10, webTestLog)
	cfg := config.WebConfig{Enabled: true}
	gwCfg := config.GatewayConfig{Port: 19877}

	ch, err := NewWebChannel(cfg, gwCfg, nil, nil, webTestLog, b, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ch.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ch.Stop() }()

	time.Sleep(100 * time.Millisecond)

	conn, _, err := websocket.Dial(ctx, "ws://localhost:19877/ws", nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()

	msg := wsMessage{Type: "message", Content: "hello from test"}
	data, _ := json.Marshal(msg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("ws write: %v", err)
	}

	select {
	case inbound := <-b.InboundChan():
		if inbound.Channel != "web" {
			t.Errorf("channel = %q, want %q", inbound.Channel, "web")
		}
		if inbound.Content != "hello from test" {
			t.Errorf("content = %q, want %q", inbound.Content, "hello from test")
		}
		if !strings.HasPrefix(inbound.ChatID, "web-") {
			t.Errorf("chatID = %q, want prefix %q", inbound.ChatID, "web-")
		}

		if err := ch.Send(ctx, bus.OutboundMessage{
			Channel: "web",
			ChatID:  inbound.ChatID,
			Content: "reply from bot",
		}); err != nil {
			t.Fatalf("Send: %v", err)
		}

		_, respData, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("ws read: %v", err)
		}
		var resp wsMessage
		if err := json.Unmarshal(respData, &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if resp.Type != "message" {
			t.Errorf("resp type = %q, want %q", resp.Type, "message")
		}
		if resp.Content != "reply from bot" {
			t.Errorf("resp content = %q, want %q", resp.Content, "reply from bot")
		}

	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for inbound message")
	}
}

func TestWebChannel_SendBroadcast(t *testing.T) {
	b := bus.New(10, webTestLog)
	cfg := config.WebConfig{Enabled: true}
	gwCfg := config.GatewayConfig{Port: 19878}

	ch, err := NewWebChannel(cfg, gwCfg, nil, nil, webTestLog, b, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ch.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ch.Stop() }()

	time.Sleep(100 * time.Millisecond)

	conn1, _, err := websocket.Dial(ctx, "ws://localhost:19878/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn1.CloseNow() }()

	conn2, _, err := websocket.Dial(ctx, "ws://localhost:19878/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn2.CloseNow() }()

	time.Sleep(100 * time.Millisecond)

	if err := ch.Send(ctx, bus.OutboundMessage{
		Channel: "web",
		ChatID:  "unknown-id",
		Content: "broadcast msg",
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	for i, conn := range []*websocket.Conn{conn1, conn2} {
		readCtx, readCancel := context.WithTimeout(ctx, 3*time.Second)
		_, data, err := conn.Read(readCtx)
		readCancel()
		if err != nil {
			t.Fatalf("client %d read: %v", i+1, err)
		}
		var msg wsMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("client %d unmarshal: %v", i+1, err)
		}
		if msg.Content != "broadcast msg" {
			t.Errorf("client %d content = %q, want %q", i+1, msg.Content, "broadcast msg")
		}
	}
}

func TestWebChannel_SendStream(t *testing.T) {
	b := bus.New(10, webTestLog)
	cfg := config.WebConfig{Enabled: true}
	gwCfg := config.GatewayConfig{Port: 19879}

	ch, err := NewWebChannel(cfg, gwCfg, nil, nil, webTestLog, b, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := ch.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ch.Stop() }()

	time.Sleep(100 * time.Millisecond)

	conn, _, err := websocket.Dial(ctx, "ws://localhost:19879/ws", nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()

	msg := wsMessage{Type: "message", Content: "ping"}
	data, _ := json.Marshal(msg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("ws write: %v", err)
	}

	var chatID string
	select {
	case inbound := <-b.InboundChan():
		chatID = inbound.ChatID
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for inbound")
	}

	events := make(chan api.StreamEvent, 4)
	go func() {
		events <- api.StreamEvent{Type: api.EventContentBlockDelta, Delta: &api.Delta{Type: "text_delta", Text: "aa"}}
		events <- api.StreamEvent{Type: api.EventContentBlockDelta, Delta: &api.Delta{Type: "text_delta", Text: "bb"}}
		close(events)
	}()

	if err := ch.SendStream(ctx, chatID, nil, events); err != nil {
		t.Fatalf("SendStream: %v", err)
	}

	readCtx, readCancel := context.WithTimeout(ctx, 3*time.Second)
	defer readCancel()

	wantTypes := []string{"stream", "stream", "stream_done"}
	for _, want := range wantTypes {
		_, payload, err := conn.Read(readCtx)
		if err != nil {
			t.Fatalf("ws read type %q: %v", want, err)
		}
		var got wsMessage
		if err := json.Unmarshal(payload, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got.Type != want {
			t.Errorf("type = %q, want %q", got.Type, want)
		}
		if want == "stream" && got.Delta == "" {
			t.Errorf("stream delta empty")
		}
	}
}
