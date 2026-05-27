package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/model"
	"github.com/ageneralai/maven/kernel/bus"
	"github.com/ageneralai/maven/kernel/config"
)

var feishuTestLog = slog.New(slog.DiscardHandler)

func newFeishuTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(handler)
}

func newFeishuChannelWithBaseURL(t *testing.T, cfg config.FeishuConfig, baseURL string) *FeishuChannel {
	t.Helper()
	b := bus.New(10, feishuTestLog)
	ch, err := NewFeishuChannel(cfg, feishuTestLog, b)
	if err != nil {
		t.Fatalf("NewFeishuChannel error: %v", err)
	}
	ch.client.baseURL = baseURL
	return ch
}

func TestNewFeishuChannel_Valid(t *testing.T) {
	b := bus.New(10, feishuTestLog)
	ch, err := NewFeishuChannel(config.FeishuConfig{
		AppID:     "cli_test",
		AppSecret: "secret",
	}, feishuTestLog, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch.Name() != "feishu" {
		t.Errorf("Name = %q, want feishu", ch.Name())
	}
}

func TestFeishuConfig_Validate_MissingAppID(t *testing.T) {
	if err := (config.FeishuConfig{Enabled: true, AppSecret: "secret"}).Validate(); err == nil {
		t.Error("expected error for missing app_id")
	}
}

func TestFeishuConfig_Validate_MissingAppSecret(t *testing.T) {
	if err := (config.FeishuConfig{Enabled: true, AppID: "cli_test"}).Validate(); err == nil {
		t.Error("expected error for missing app_secret")
	}
}

func TestFeishuChannel_Send_Success(t *testing.T) {
	var sentChatID, sentContent string
	ts := newFeishuTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			_, _ = io.WriteString(w, `{"code":0,"tenant_access_token":"test-token","expire":7200}`)
		case "/open-apis/im/v1/messages":
			body, _ := io.ReadAll(r.Body)
			var payload map[string]any
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("invalid payload: %v", err)
			}
			sentChatID = payload["receive_id"].(string)
			var textContent map[string]string
			if err := json.Unmarshal([]byte(payload["content"].(string)), &textContent); err != nil {
				t.Fatalf("invalid text content: %v", err)
			}
			sentContent = textContent["text"]
			_, _ = io.WriteString(w, `{"code":0,"msg":"ok"}`)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer ts.Close()
	ch := newFeishuChannelWithBaseURL(t, config.FeishuConfig{
		AppID: "cli_test", AppSecret: "secret",
	}, ts.URL)
	err := ch.Send(context.Background(), bus.OutboundMessage{ChatID: "chat_123", Content: "hello"})
	if err != nil {
		t.Errorf("Send error: %v", err)
	}
	if sentChatID != "chat_123" {
		t.Errorf("chatID = %q, want chat_123", sentChatID)
	}
	if sentContent != "hello" {
		t.Errorf("content = %q, want hello", sentContent)
	}
}

func TestFeishuChannel_Send_Error(t *testing.T) {
	ts := newFeishuTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			_, _ = io.WriteString(w, `{"code":0,"tenant_access_token":"test-token","expire":7200}`)
		case "/open-apis/im/v1/messages":
			_, _ = io.WriteString(w, `{"code":1,"msg":"send failed"}`)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer ts.Close()
	ch := newFeishuChannelWithBaseURL(t, config.FeishuConfig{
		AppID: "cli_test", AppSecret: "secret",
	}, ts.URL)
	err := ch.Send(context.Background(), bus.OutboundMessage{ChatID: "chat_123", Content: "hello"})
	if err == nil {
		t.Error("expected error")
	}
}

func TestFeishuChannel_Stop_NotStarted(t *testing.T) {
	b := bus.New(10, feishuTestLog)
	ch, _ := NewFeishuChannel(config.FeishuConfig{
		AppID: "cli_test", AppSecret: "secret",
	}, feishuTestLog, b)
	err := ch.Stop()
	if err != nil {
		t.Errorf("Stop error: %v", err)
	}
}

func newTestFeishuChannel(t *testing.T, cfg config.FeishuConfig) (*FeishuChannel, *bus.MessageBus) {
	t.Helper()
	b := bus.New(10, feishuTestLog)
	ch, err := NewFeishuChannel(cfg, feishuTestLog, b)
	if err != nil {
		t.Fatalf("NewFeishuChannel error: %v", err)
	}
	ch.imageDownloader = func(ctx context.Context, tenantAccessToken, imageKey string) (string, string, error) {
		return "", "", fmt.Errorf("test image downloader not configured")
	}
	return ch, b
}

func TestFeishuWebhook_Challenge(t *testing.T) {
	ch, _ := newTestFeishuChannel(t, config.FeishuConfig{
		AppID: "cli_test", AppSecret: "secret",
	})
	body := `{"challenge":"test-challenge-token","token":"xxx","type":"url_verification"}`
	req := httptest.NewRequest(http.MethodPost, "/feishu/webhook", strings.NewReader(body))
	w := httptest.NewRecorder()
	ch.handleWebhook(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	raw := w.Body.Bytes()
	if !json.Valid(raw) {
		t.Fatalf("response body is not valid JSON: %q", raw)
	}
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if resp["challenge"] != "test-challenge-token" {
		t.Errorf("challenge = %q, want test-challenge-token", resp["challenge"])
	}
}

func TestFeishuWebhook_MethodNotAllowed(t *testing.T) {
	ch, _ := newTestFeishuChannel(t, config.FeishuConfig{
		AppID: "cli_test", AppSecret: "secret",
	})
	req := httptest.NewRequest(http.MethodGet, "/feishu/webhook", nil)
	w := httptest.NewRecorder()
	ch.handleWebhook(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestFeishuWebhook_InvalidJSON(t *testing.T) {
	ch, _ := newTestFeishuChannel(t, config.FeishuConfig{
		AppID: "cli_test", AppSecret: "secret",
	})
	req := httptest.NewRequest(http.MethodPost, "/feishu/webhook", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	ch.handleWebhook(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestFeishuWebhook_InvalidToken(t *testing.T) {
	ch, _ := newTestFeishuChannel(t, config.FeishuConfig{
		AppID:             "cli_test",
		AppSecret:         "secret",
		VerificationToken: "correct-token",
	})
	body := `{"header":{"event_type":"im.message.receive_v1","token":"wrong-token"},"event":{}}`
	req := httptest.NewRequest(http.MethodPost, "/feishu/webhook", strings.NewReader(body))
	w := httptest.NewRecorder()
	ch.handleWebhook(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestFeishuWebhook_ValidToken(t *testing.T) {
	ch, _ := newTestFeishuChannel(t, config.FeishuConfig{
		AppID:             "cli_test",
		AppSecret:         "secret",
		VerificationToken: "correct-token",
	})
	body := `{"header":{"event_type":"other.event","token":"correct-token"},"event":{}}`
	req := httptest.NewRequest(http.MethodPost, "/feishu/webhook", strings.NewReader(body))
	w := httptest.NewRecorder()
	ch.handleWebhook(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestFeishuWebhook_MessageReceive(t *testing.T) {
	ch, b := newTestFeishuChannel(t, config.FeishuConfig{
		AppID: "cli_test", AppSecret: "secret",
	})
	event := map[string]any{
		"header": map[string]any{
			"event_type": "im.message.receive_v1",
			"token":      "",
		},
		"event": map[string]any{
			"sender": map[string]any{
				"sender_id": map[string]any{
					"open_id": "ou_test123",
				},
			},
			"message": map[string]any{
				"chat_id":      "oc_chat456",
				"message_type": "text",
				"content":      `{"text":"hello maven"}`,
			},
		},
	}
	data, _ := json.Marshal(event)
	req := httptest.NewRequest(http.MethodPost, "/feishu/webhook", strings.NewReader(string(data)))
	w := httptest.NewRecorder()
	ch.handleWebhook(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	select {
	case msg := <-b.InboundChan():
		if msg.Content != "hello maven" {
			t.Errorf("content = %q, want 'hello maven'", msg.Content)
		}
		if msg.SenderID != "ou_test123" {
			t.Errorf("senderID = %q, want ou_test123", msg.SenderID)
		}
		if msg.ChatID != "oc_chat456" {
			t.Errorf("chatID = %q, want oc_chat456", msg.ChatID)
		}
		if msg.Channel != "feishu" {
			t.Errorf("channel = %q, want feishu", msg.Channel)
		}
	case <-time.After(time.Second):
		t.Error("expected inbound message")
	}
}

func TestFeishuWebhook_RejectedSender(t *testing.T) {
	ch, b := newTestFeishuChannel(t, config.FeishuConfig{
		AppID:     "cli_test",
		AppSecret: "secret",
		AllowFrom: []string{"ou_allowed"},
	})
	event := map[string]any{
		"header": map[string]any{
			"event_type": "im.message.receive_v1",
		},
		"event": map[string]any{
			"sender": map[string]any{
				"sender_id": map[string]any{
					"open_id": "ou_rejected",
				},
			},
			"message": map[string]any{
				"chat_id":      "oc_chat",
				"message_type": "text",
				"content":      `{"text":"hello"}`,
			},
		},
	}
	data, _ := json.Marshal(event)
	req := httptest.NewRequest(http.MethodPost, "/feishu/webhook", strings.NewReader(string(data)))
	w := httptest.NewRecorder()
	ch.handleWebhook(w, req)
	select {
	case <-b.InboundChan():
		t.Error("should not receive message from rejected sender")
	default:
	}
}

func TestFeishuWebhook_ImageMessage(t *testing.T) {
	ch, b := newTestFeishuChannel(t, config.FeishuConfig{
		AppID: "cli_test", AppSecret: "secret",
	})
	ch.imageDownloader = func(ctx context.Context, tenantAccessToken, imageKey string) (string, string, error) {
		if tenantAccessToken != "test-token" {
			t.Fatalf("tenantAccessToken = %q, want test-token", tenantAccessToken)
		}
		if imageKey != "img_xxx" {
			t.Fatalf("imageKey = %q, want img_xxx", imageKey)
		}
		return "iVBORw0KGgo=", "image/png", nil
	}
	ch.client.token = "test-token"
	ch.client.tokenExp = time.Now().Add(time.Hour)
	event := map[string]any{
		"header": map[string]any{
			"event_type": "im.message.receive_v1",
		},
		"event": map[string]any{
			"sender": map[string]any{
				"sender_id": map[string]any{
					"open_id": "ou_test",
				},
			},
			"message": map[string]any{
				"chat_id":      "oc_chat",
				"message_type": "image",
				"content":      `{"image_key":"img_xxx"}`,
			},
		},
	}
	data, _ := json.Marshal(event)
	req := httptest.NewRequest(http.MethodPost, "/feishu/webhook", strings.NewReader(string(data)))
	w := httptest.NewRecorder()
	ch.handleWebhook(w, req)
	select {
	case msg := <-b.InboundChan():
		if msg.Content != "[image]" {
			t.Errorf("content = %q, want [image]", msg.Content)
		}
		if len(msg.ContentBlocks) != 1 {
			t.Fatalf("content blocks len = %d, want 1", len(msg.ContentBlocks))
		}
		block := msg.ContentBlocks[0]
		if block.Type != model.ContentBlockImage {
			t.Errorf("content block type = %q, want %q", block.Type, model.ContentBlockImage)
		}
		if block.MediaType != "image/png" {
			t.Errorf("media type = %q, want image/png", block.MediaType)
		}
		if block.Data != "iVBORw0KGgo=" {
			t.Errorf("data = %q, want iVBORw0KGgo=", block.Data)
		}
		if got := msg.TransportMeta["image_key"]; got != "img_xxx" {
			t.Errorf("transport meta image_key = %v, want img_xxx", got)
		}
	default:
		t.Error("should receive image message")
	}
}

func TestFeishuWebhook_UnsupportedMessageType(t *testing.T) {
	ch, b := newTestFeishuChannel(t, config.FeishuConfig{
		AppID: "cli_test", AppSecret: "secret",
	})
	event := map[string]any{
		"header": map[string]any{
			"event_type": "im.message.receive_v1",
		},
		"event": map[string]any{
			"sender": map[string]any{
				"sender_id": map[string]any{
					"open_id": "ou_test",
				},
			},
			"message": map[string]any{
				"chat_id":      "oc_chat",
				"message_type": "post",
				"content":      `{}`,
			},
		},
	}
	data, _ := json.Marshal(event)
	req := httptest.NewRequest(http.MethodPost, "/feishu/webhook", strings.NewReader(string(data)))
	w := httptest.NewRecorder()
	ch.handleWebhook(w, req)
	select {
	case <-b.InboundChan():
		t.Error("should not receive unsupported message type")
	default:
	}
}

func TestFeishuWebhook_EmptyText(t *testing.T) {
	ch, b := newTestFeishuChannel(t, config.FeishuConfig{
		AppID: "cli_test", AppSecret: "secret",
	})
	event := map[string]any{
		"header": map[string]any{
			"event_type": "im.message.receive_v1",
		},
		"event": map[string]any{
			"sender": map[string]any{
				"sender_id": map[string]any{
					"open_id": "ou_test",
				},
			},
			"message": map[string]any{
				"chat_id":      "oc_chat",
				"message_type": "text",
				"content":      `{"text":""}`,
			},
		},
	}
	data, _ := json.Marshal(event)
	req := httptest.NewRequest(http.MethodPost, "/feishu/webhook", strings.NewReader(string(data)))
	w := httptest.NewRecorder()
	ch.handleWebhook(w, req)
	select {
	case <-b.InboundChan():
		t.Error("should not receive empty text message")
	default:
	}
}

func TestFeishuWebhook_InvalidContent(t *testing.T) {
	ch, b := newTestFeishuChannel(t, config.FeishuConfig{
		AppID: "cli_test", AppSecret: "secret",
	})
	event := map[string]any{
		"header": map[string]any{
			"event_type": "im.message.receive_v1",
		},
		"event": map[string]any{
			"sender": map[string]any{
				"sender_id": map[string]any{
					"open_id": "ou_test",
				},
			},
			"message": map[string]any{
				"chat_id":      "oc_chat",
				"message_type": "text",
				"content":      "not-valid-json",
			},
		},
	}
	data, _ := json.Marshal(event)
	req := httptest.NewRequest(http.MethodPost, "/feishu/webhook", strings.NewReader(string(data)))
	w := httptest.NewRecorder()
	ch.handleWebhook(w, req)
	select {
	case <-b.InboundChan():
		t.Error("should not receive message with invalid content JSON")
	default:
	}
}

func TestFeishuWebhook_NonMessageEvent(t *testing.T) {
	ch, b := newTestFeishuChannel(t, config.FeishuConfig{
		AppID: "cli_test", AppSecret: "secret",
	})
	body := `{"header":{"event_type":"im.chat.member.bot.added_v1"},"event":{}}`
	req := httptest.NewRequest(http.MethodPost, "/feishu/webhook", strings.NewReader(body))
	w := httptest.NewRecorder()
	ch.handleWebhook(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	select {
	case <-b.InboundChan():
		t.Error("should not receive non-message event")
	default:
	}
}

func TestFeishuWebhook_NoVerificationToken(t *testing.T) {
	ch, _ := newTestFeishuChannel(t, config.FeishuConfig{
		AppID:             "cli_test",
		AppSecret:         "secret",
		VerificationToken: "",
	})
	body := `{"header":{"event_type":"other.event","token":"any-token"},"event":{}}`
	req := httptest.NewRequest(http.MethodPost, "/feishu/webhook", strings.NewReader(body))
	w := httptest.NewRecorder()
	ch.handleWebhook(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (token verification should be skipped)", w.Code)
	}
}

func TestFeishuChannel_StartStop(t *testing.T) {
	b := bus.New(10, feishuTestLog)
	ch, err := NewFeishuChannel(config.FeishuConfig{
		AppID: "cli_test", AppSecret: "secret", Port: 0,
	}, feishuTestLog, b)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = ch.Start(ctx)
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	err = ch.Stop()
	if err != nil {
		t.Errorf("Stop error: %v", err)
	}
}
