package telegram

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/ageneral-agents-go/pkg/model"
	"github.com/ageneralai/maven/internal/kernel/bus"
	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/stringutil"
	"github.com/mymmrac/telego"
	ta "github.com/mymmrac/telego/telegoapi"
)

// === Telegram Channel Constructor Tests ===
func TestNewTelegramChannel_Valid(t *testing.T) {
	b := bus.New(10, channelTestLog)
	ch, err := NewTelegramChannel(config.TelegramConfig{Token: fakeToken}, "", channelTestLog, b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch.Name() != "telegram" {
		t.Errorf("Name = %q, want telegram", ch.Name())
	}
}

func TestTelegramConfig_Validate_MissingToken(t *testing.T) {
	if err := (config.TelegramConfig{Enabled: true}).Validate(); err == nil {
		t.Error("expected error for empty token when enabled")
	}
}

// === toTelegramHTML Tests ===
func TestToTelegramHTML(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"**bold**", "<b>bold</b>"},
		{"`code`", "<code>code</code>"},
		{"a & b", "a &amp; b"},
		{"<tag>", "&lt;tag&gt;"},
	}
	for _, tt := range tests {
		got := ToTelegramHTML(tt.input)
		if got != tt.want {
			t.Errorf("ToTelegramHTML(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
func TestToTelegramHTML_CodeBlocks(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"code block with language", "```go\nfunc main() {}\n```", "<pre>func main() {}\n</pre>"},
		{"code block without language", "```\ncode here\n```", "<pre>\ncode here\n</pre>"},
		{"italic text", "*italic*", "<i>italic</i>"},
		{"mixed bold and italic", "**bold** and *italic*", "<b>bold</b> and <i>italic</i>"},
		{"unclosed code block", "```code", "```code"},
		{"unclosed inline code", "`code", "`code"},
		{"unclosed bold", "**bold", "**bold"},
		{"unclosed italic", "*italic", "*italic"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToTelegramHTML(tt.input)
			if got != tt.want {
				t.Errorf("ToTelegramHTML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// === Telegram Channel Tests ===
func TestTelegramChannel_Stop_NotStarted(t *testing.T) {
	b := bus.New(10, channelTestLog)
	ch, _ := NewTelegramChannel(config.TelegramConfig{Token: fakeToken}, "", channelTestLog, b)
	err := ch.Stop()
	if err != nil {
		t.Errorf("Stop error: %v", err)
	}
}
func TestTelegramChannel_Send_NilBot(t *testing.T) {
	b := bus.New(10, channelTestLog)
	ch, _ := NewTelegramChannel(config.TelegramConfig{Token: fakeToken}, "", channelTestLog, b)
	err := ch.Send(context.Background(), bus.OutboundMessage{ChatID: "123", Content: "test"})
	if err == nil {
		t.Error("expected error when bot is nil")
	}
}
func TestTelegramChannel_Send_InvalidChatID(t *testing.T) {
	ch, _, _ := newTestChannel(t, config.TelegramConfig{})
	err := ch.Send(context.Background(), bus.OutboundMessage{ChatID: "not-a-number", Content: "test"})
	if err == nil {
		t.Error("expected error for invalid chat ID")
	}
}
func TestTelegramChannel_Send_Success(t *testing.T) {
	ch, caller, _ := newTestChannel(t, config.TelegramConfig{})
	err := ch.Send(context.Background(), bus.OutboundMessage{ChatID: "123", Content: "hello"})
	if err != nil {
		t.Errorf("Send error: %v", err)
	}
	if len(caller.calls) == 0 {
		t.Error("expected at least one API call")
	}
}
func TestTelegramChannel_Send_LongMessage(t *testing.T) {
	ch, caller, _ := newTestChannel(t, config.TelegramConfig{})
	longContent := ""
	for i := 0; i < 100; i++ {
		longContent += "This is a long line of text that will be repeated.\n"
	}
	err := ch.Send(context.Background(), bus.OutboundMessage{ChatID: "123", Content: longContent})
	if err != nil {
		t.Errorf("Send error: %v", err)
	}
	// Count sendMessage calls
	sendCount := 0
	for _, c := range caller.calls {
		if strings.HasSuffix(c.URL, "/sendMessage") {
			sendCount++
		}
	}
	if sendCount < 2 {
		t.Errorf("expected multiple sent messages for long content, got %d", sendCount)
	}
}
func TestTelegramChannel_Send_LongMessageNoNewline(t *testing.T) {
	ch, caller, _ := newTestChannel(t, config.TelegramConfig{})
	longContent := strings.Repeat("x", 5000)
	err := ch.Send(context.Background(), bus.OutboundMessage{ChatID: "123", Content: longContent})
	if err != nil {
		t.Errorf("Send error: %v", err)
	}
	sendCount := 0
	for _, c := range caller.calls {
		if strings.HasSuffix(c.URL, "/sendMessage") {
			sendCount++
		}
	}
	if sendCount < 2 {
		t.Errorf("expected multiple messages, got %d", sendCount)
	}
}
func TestTelegramChannel_Send_HTMLError_Retry(t *testing.T) {
	ch, _, _ := newTestChannel(t, config.TelegramConfig{})
	retryCaller := &retrySendCaller{inner: newMockCaller(), failFirst: true}
	bot, _ := telego.NewBot(fakeToken, telego.WithAPICaller(retryCaller))
	ch.bot = bot
	err := ch.Send(context.Background(), bus.OutboundMessage{ChatID: "123", Content: "test"})
	if err != nil {
		t.Errorf("Send should succeed after retry: %v", err)
	}
}

func TestTelegramChannel_Send_BothFail(t *testing.T) {
	ch, caller, _ := newTestChannel(t, config.TelegramConfig{})
	caller.responses["sendMessage"] = &ta.Response{Ok: false, Error: &ta.Error{Description: "send failed", ErrorCode: 400}}
	err := ch.Send(context.Background(), bus.OutboundMessage{ChatID: "123", Content: "test"})
	if err == nil {
		t.Error("expected error when both sends fail")
	}
}

// === HandleMessage Tests ===
func TestTelegramChannel_HandleMessage_Allowed(t *testing.T) {
	b := bus.New(10, channelTestLog)
	ch, _ := NewTelegramChannel(config.TelegramConfig{Token: fakeToken}, "", channelTestLog, b)
	msg := &telego.Message{
		From: &telego.User{ID: 123, Username: "testuser"},
		Chat: telego.Chat{ID: 456, Type: "private"},
		Text: "hello",
		Date: 1234567890,
	}
	ch.handleMessage(msg)
	select {
	case inbound := <-b.InboundChan():
		if inbound.Content != "hello" {
			t.Errorf("content = %q, want hello", inbound.Content)
		}
		if inbound.SenderID != "123" {
			t.Errorf("senderID = %q, want 123", inbound.SenderID)
		}
		if inbound.ChatID != "456" {
			t.Errorf("chatID = %q, want 456", inbound.ChatID)
		}
	case <-time.After(2 * time.Second):
		t.Error("expected inbound message")
	}
}

func TestTelegramChannel_HandleMessage_Rejected(t *testing.T) {
	b := bus.New(10, channelTestLog)
	ch, _ := NewTelegramChannel(config.TelegramConfig{
		Token:     fakeToken,
		AllowFrom: []string{"999"},
	}, "", channelTestLog, b)
	msg := &telego.Message{
		From: &telego.User{ID: 123, Username: "testuser"},
		Chat: telego.Chat{ID: 456, Type: "private"},
		Text: "hello",
	}
	ch.handleMessage(msg)
	select {
	case <-b.InboundChan():
		t.Error("should not receive message from rejected user")
	default:
	}
}
func TestTelegramChannel_HandleMessage_EmptyText(t *testing.T) {
	b := bus.New(10, channelTestLog)
	ch, _ := NewTelegramChannel(config.TelegramConfig{Token: fakeToken}, "", channelTestLog, b)
	msg := &telego.Message{
		From: &telego.User{ID: 123},
		Chat: telego.Chat{ID: 456, Type: "private"},
		Text: "",
	}
	ch.handleMessage(msg)
	select {
	case <-b.InboundChan():
		t.Error("should not send message with empty content")
	default:
	}
}
func TestTelegramChannel_HandleMessage_Caption(t *testing.T) {
	b := bus.New(10, channelTestLog)
	ch, _ := NewTelegramChannel(config.TelegramConfig{Token: fakeToken}, "", channelTestLog, b)
	msg := &telego.Message{
		From:    &telego.User{ID: 123},
		Chat:    telego.Chat{ID: 456, Type: "private"},
		Text:    "",
		Caption: "image caption",
	}
	ch.handleMessage(msg)
	select {
	case inbound := <-b.InboundChan():
		if inbound.Content != "image caption" {
			t.Errorf("content = %q, want 'image caption'", inbound.Content)
		}
	case <-time.After(2 * time.Second):
		t.Error("expected inbound message")
	}
}
func TestTelegramChannel_HandleMessage_Photo(t *testing.T) {
	ch, _, b := newTestChannel(t, config.TelegramConfig{})
	photoData := []byte{0xff, 0xd8, 0xff, 0xd9}
	ch.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(photoData)),
			Header:     make(http.Header),
		}, nil
	})}
	msg := &telego.Message{
		From:    &telego.User{ID: 123},
		Chat:    telego.Chat{ID: 456, Type: "private"},
		Caption: "photo caption",
		Photo: []telego.PhotoSize{
			{FileID: "photo-small"},
			{FileID: "photo-large"},
		},
	}
	ch.handleMessage(msg)
	select {
	case inbound := <-b.InboundChan():
		if inbound.Content != "photo caption" {
			t.Errorf("content = %q, want 'photo caption'", inbound.Content)
		}
		if len(inbound.ContentBlocks) != 1 {
			t.Fatalf("content blocks len = %d, want 1", len(inbound.ContentBlocks))
		}
		block := inbound.ContentBlocks[0]
		if block.Type != model.ContentBlockImage {
			t.Errorf("content block type = %q, want %q", block.Type, model.ContentBlockImage)
		}
		if block.Data != base64.StdEncoding.EncodeToString(photoData) {
			t.Errorf("content block data mismatch")
		}
	case <-time.After(2 * time.Second):
		t.Error("expected inbound message")
	}
}
func TestTelegramChannel_HandleMessage_PhotoWithCaption(t *testing.T) {
	ch, _, b := newTestChannel(t, config.TelegramConfig{})
	photoData := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00}
	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(photoData)
	}))
	defer downloadServer.Close()
	serverURL, err := url.Parse(downloadServer.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	ch.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		clonedReq := req.Clone(req.Context())
		clonedReq.URL.Scheme = serverURL.Scheme
		clonedReq.URL.Host = serverURL.Host
		return transport.RoundTrip(clonedReq)
	})}
	msg := &telego.Message{
		From:    &telego.User{ID: 123},
		Chat:    telego.Chat{ID: 456, Type: "private"},
		Caption: "photo caption via server",
		Photo: []telego.PhotoSize{
			{FileID: "photo-small"},
			{FileID: "photo-large"},
		},
	}
	ch.handleMessage(msg)
	select {
	case inbound := <-b.InboundChan():
		if inbound.Content != "photo caption via server" {
			t.Errorf("content = %q, want 'photo caption via server'", inbound.Content)
		}
		if len(inbound.ContentBlocks) != 1 {
			t.Fatalf("content blocks len = %d, want 1", len(inbound.ContentBlocks))
		}
		block := inbound.ContentBlocks[0]
		if block.Type != model.ContentBlockImage {
			t.Errorf("content block type = %q, want %q", block.Type, model.ContentBlockImage)
		}
	case <-time.After(2 * time.Second):
		t.Error("expected inbound message")
	}
}
func TestTelegramChannel_HandleMessage_Document(t *testing.T) {
	workspace := t.TempDir()
	ch, _, b := newTestChannelWithWorkspace(t, config.TelegramConfig{}, workspace)
	pdfData := []byte("%PDF-1.4\n")
	ch.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(pdfData)),
			Header:     make(http.Header),
		}, nil
	})}
	msg := &telego.Message{
		From: &telego.User{ID: 123},
		Chat: telego.Chat{ID: 456, Type: "private"},
		Document: &telego.Document{
			FileID:   "doc-1",
			MimeType: "application/pdf",
			FileName: "test.pdf",
		},
	}
	ch.handleMessage(msg)
	select {
	case inbound := <-b.InboundChan():
		if !strings.Contains(inbound.Content, "[File saved to:") {
			t.Errorf("content = %q, want file path reference", inbound.Content)
		}
		if len(inbound.ContentBlocks) != 0 {
			t.Errorf("content blocks len = %d, want 0 (file saved to disk)", len(inbound.ContentBlocks))
		}
	case <-time.After(2 * time.Second):
		t.Error("expected inbound message")
	}
}

// === Reply Context Tests ===
func TestTelegramChannel_HandleMessage_ReplyContext(t *testing.T) {
	b := bus.New(10, channelTestLog)
	ch, _ := NewTelegramChannel(config.TelegramConfig{Token: fakeToken}, "", channelTestLog, b)
	msg := &telego.Message{
		From: &telego.User{ID: 123, Username: "testuser"},
		Chat: telego.Chat{ID: 456, Type: "private"},
		Text: "what does this mean?",
		Date: 1234567890,
		ReplyToMessage: &telego.Message{
			From: &telego.User{ID: 789, FirstName: "Alice", LastName: "B"},
			Text: "The quick brown fox jumps over the lazy dog",
		},
	}
	ch.handleMessage(msg)
	select {
	case inbound := <-b.InboundChan():
		if !strings.Contains(inbound.Content, "[Replying to Alice B]") {
			t.Errorf("missing reply context header, got: %q", inbound.Content)
		}
		if !strings.Contains(inbound.Content, "The quick brown fox") {
			t.Errorf("missing replied-to text, got: %q", inbound.Content)
		}
		if !strings.Contains(inbound.Content, "what does this mean?") {
			t.Errorf("missing user text, got: %q", inbound.Content)
		}
	case <-time.After(2 * time.Second):
		t.Error("expected inbound message")
	}
}
func TestTelegramChannel_HandleMessage_ReplyToPhoto(t *testing.T) {
	b := bus.New(10, channelTestLog)
	ch, _ := NewTelegramChannel(config.TelegramConfig{Token: fakeToken}, "", channelTestLog, b)
	msg := &telego.Message{
		From: &telego.User{ID: 123},
		Chat: telego.Chat{ID: 456, Type: "private"},
		Text: "describe this image",
		Date: 1234567890,
		ReplyToMessage: &telego.Message{
			From:  &telego.User{ID: 789, FirstName: "Bob"},
			Photo: []telego.PhotoSize{{FileID: "photo-1"}},
		},
	}
	ch.handleMessage(msg)
	select {
	case inbound := <-b.InboundChan():
		if !strings.Contains(inbound.Content, "[Replying to Bob]") {
			t.Errorf("missing reply header, got: %q", inbound.Content)
		}
		if !strings.Contains(inbound.Content, "[Photo]") {
			t.Errorf("missing photo indicator, got: %q", inbound.Content)
		}
	case <-time.After(2 * time.Second):
		t.Error("expected inbound message")
	}
}
func TestTelegramChannel_HandleMessage_ExternalReply(t *testing.T) {
	b := bus.New(10, channelTestLog)
	ch, _ := NewTelegramChannel(config.TelegramConfig{Token: fakeToken}, "", channelTestLog, b)
	msg := &telego.Message{
		From: &telego.User{ID: 123},
		Chat: telego.Chat{ID: 456, Type: "private"},
		Text: "reply ping",
		Date: 1234567890,
		ExternalReply: &telego.ExternalReplyInfo{
			Origin: &telego.MessageOriginChannel{
				Chat: telego.Chat{Title: "Tech News"},
			},
		},
		Quote: &telego.TextQuote{Text: "Breaking news content here"},
	}
	ch.handleMessage(msg)
	select {
	case inbound := <-b.InboundChan():
		if !strings.Contains(inbound.Content, "[Replying to channel: Tech News]") {
			t.Errorf("missing external reply header, got: %q", inbound.Content)
		}
		if !strings.Contains(inbound.Content, "Breaking news content here") {
			t.Errorf("missing quote text, got: %q", inbound.Content)
		}
		if !strings.Contains(inbound.Content, "reply ping") {
			t.Errorf("missing user text, got: %q", inbound.Content)
		}
	case <-time.After(2 * time.Second):
		t.Error("expected inbound message")
	}
}
func TestTelegramChannel_HandleMessage_ExternalReplyWithPhoto(t *testing.T) {
	ch, _, b := newTestChannel(t, config.TelegramConfig{})
	photoData := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00}
	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(photoData)
	}))
	defer downloadServer.Close()
	serverURL, err := url.Parse(downloadServer.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	ch.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		clonedReq := req.Clone(req.Context())
		clonedReq.URL.Scheme = serverURL.Scheme
		clonedReq.URL.Host = serverURL.Host
		return transport.RoundTrip(clonedReq)
	})}
	msg := &telego.Message{
		From: &telego.User{ID: 123},
		Chat: telego.Chat{ID: 456, Type: "private"},
		Text: "what is this image about",
		Date: 1234567890,
		ExternalReply: &telego.ExternalReplyInfo{
			Origin: &telego.MessageOriginChannel{
				Chat: telego.Chat{Title: "Linux.do 热门话题"},
			},
			Photo: []telego.PhotoSize{
				{FileID: "ext-photo-small", Width: 76, Height: 998},
				{FileID: "ext-photo-large", Width: 760, Height: 9980},
			},
		},
		Quote: &telego.TextQuote{Text: "some quoted text"},
	}
	ch.handleMessage(msg)
	select {
	case inbound := <-b.InboundChan():
		if !strings.Contains(inbound.Content, "[Replying to channel: Linux.do 热门话题]") {
			t.Errorf("missing external reply header, got: %q", inbound.Content)
		}
		if !strings.Contains(inbound.Content, "some quoted text") {
			t.Errorf("missing quote text, got: %q", inbound.Content)
		}
		if len(inbound.ContentBlocks) != 1 {
			t.Fatalf("expected 1 content block (photo), got %d", len(inbound.ContentBlocks))
		}
		if inbound.ContentBlocks[0].Type != model.ContentBlockImage {
			t.Errorf("block type = %q, want image", inbound.ContentBlocks[0].Type)
		}
	case <-time.After(2 * time.Second):
		t.Error("expected inbound message")
	}
}

// === Forward Message Tests ===
func TestTelegramChannel_HandleMessage_ForwardWithText(t *testing.T) {
	b := bus.New(10, channelTestLog)
	ch, _ := NewTelegramChannel(config.TelegramConfig{Token: fakeToken}, "", channelTestLog, b)
	msg := &telego.Message{
		From: &telego.User{ID: 123},
		Chat: telego.Chat{ID: 456, Type: "private"},
		Text: "Check this out",
		Date: 1234567890,
		ForwardOrigin: &telego.MessageOriginUser{
			SenderUser: telego.User{FirstName: "Charlie", LastName: "D"},
		},
	}
	ch.handleMessage(msg)
	select {
	case inbound := <-b.InboundChan():
		if !strings.Contains(inbound.Content, "[Forwarded from Charlie D]") {
			t.Errorf("missing forward label, got: %q", inbound.Content)
		}
		if !strings.Contains(inbound.Content, "Check this out") {
			t.Errorf("missing forwarded text, got: %q", inbound.Content)
		}
	case <-time.After(2 * time.Second):
		t.Error("expected inbound message")
	}
}
func TestTelegramChannel_HandleMessage_ForwardNoComment(t *testing.T) {
	b := bus.New(10, channelTestLog)
	ch, _ := NewTelegramChannel(config.TelegramConfig{Token: fakeToken}, "", channelTestLog, b)
	msg := &telego.Message{
		From: &telego.User{ID: 123},
		Chat: telego.Chat{ID: 456, Type: "private"},
		Date: 1234567890,
		ForwardOrigin: &telego.MessageOriginChannel{
			Chat: telego.Chat{Title: "Tech News"},
		},
	}
	ch.handleMessage(msg)
	select {
	case inbound := <-b.InboundChan():
		if !strings.Contains(inbound.Content, "[Forwarded from channel: Tech News]") {
			t.Errorf("missing forward label, got: %q", inbound.Content)
		}
		if !strings.Contains(inbound.Content, "Summarize or process") {
			t.Errorf("missing summarize hint, got: %q", inbound.Content)
		}
	case <-time.After(2 * time.Second):
		t.Error("expected inbound message")
	}
}

// === Media Group Tests ===
func TestTelegramChannel_HandleMessage_MediaGroup(t *testing.T) {
	ch, _, b := newTestChannel(t, config.TelegramConfig{})
	photoData := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00}
	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(photoData)
	}))
	defer downloadServer.Close()
	serverURL, err := url.Parse(downloadServer.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	ch.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		clonedReq := req.Clone(req.Context())
		clonedReq.URL.Scheme = serverURL.Scheme
		clonedReq.URL.Host = serverURL.Host
		return transport.RoundTrip(clonedReq)
	})}
	// Simulate 3 photos in a media group (Telegram sends each as separate Message).
	for i := 0; i < 3; i++ {
		msg := &telego.Message{
			From:         &telego.User{ID: 123},
			Chat:         telego.Chat{ID: 456, Type: "private"},
			Date:         1234567890,
			MediaGroupID: "mg-abc-123",
			Photo:        []telego.PhotoSize{{FileID: fmt.Sprintf("photo-%d", i)}},
		}
		if i == 0 {
			msg.Caption = "album caption"
		}
		ch.handleMessage(msg)
	}
	// Should produce exactly ONE inbound message after flush.
	select {
	case inbound := <-b.InboundChan():
		if !strings.Contains(inbound.Content, "album caption") {
			t.Errorf("missing caption, got: %q", inbound.Content)
		}
		if len(inbound.ContentBlocks) != 3 {
			t.Fatalf("expected 3 content blocks (photos), got %d", len(inbound.ContentBlocks))
		}
		for i, block := range inbound.ContentBlocks {
			if block.Type != model.ContentBlockImage {
				t.Errorf("block[%d] type = %q, want image", i, block.Type)
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected exactly one inbound message from media group")
	}
	// Verify no second message arrives.
	select {
	case extra := <-b.InboundChan():
		t.Fatalf("unexpected second inbound message: %+v", extra)
	case <-time.After(300 * time.Millisecond):
		// Good — no duplicate.
	}
}

func TestTelegramChannel_FlushMediaGroup_ImageOnly(t *testing.T) {
	ch, _, b := newTestChannel(t, config.TelegramConfig{})
	photoData := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00}
	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(photoData)
	}))
	defer downloadServer.Close()
	serverURL, err := url.Parse(downloadServer.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	ch.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		clonedReq := req.Clone(req.Context())
		clonedReq.URL.Scheme = serverURL.Scheme
		clonedReq.URL.Host = serverURL.Host
		return transport.RoundTrip(clonedReq)
	})}
	for i := 0; i < 2; i++ {
		ch.handleMessage(&telego.Message{
			From:         &telego.User{ID: 123},
			Chat:         telego.Chat{ID: 456, Type: "private"},
			Date:         1234567890,
			MediaGroupID: "mg-images-only",
			Photo:        []telego.PhotoSize{{FileID: fmt.Sprintf("photo-%d", i)}},
		})
	}
	select {
	case inbound := <-b.InboundChan():
		if inbound.Content != "" {
			t.Fatalf("expected empty content for image-only group, got %q", inbound.Content)
		}
		if len(inbound.ContentBlocks) != 2 {
			t.Fatalf("expected 2 image blocks, got %d", len(inbound.ContentBlocks))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected inbound message from image-only media group")
	}
}

func TestTelegramChannel_Send_UTF8ChunkBoundaries(t *testing.T) {
	ch, caller, _ := newTestChannel(t, config.TelegramConfig{})
	const maxLen = 4000
	text := strings.Repeat("é", maxLen+1)
	err := ch.Send(context.Background(), bus.OutboundMessage{ChatID: "123", Content: text})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	var bodies []string
	for _, c := range caller.calls {
		if !strings.HasSuffix(c.URL, "/sendMessage") || c.Data == nil {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(c.Data.BodyRaw, &payload); err != nil {
			continue
		}
		if body, ok := payload["text"].(string); ok {
			bodies = append(bodies, body)
		}
	}
	if len(bodies) < 2 {
		t.Fatalf("expected chunked sends, got %d", len(bodies))
	}
	for i, body := range bodies {
		if !utf8.ValidString(body) {
			t.Fatalf("chunk %d is not valid UTF-8", i)
		}
	}
	if strings.Join(bodies, "") != ToTelegramHTML(text) {
		t.Fatal("chunk join round-trip failed")
	}
}

// === WeChat Image Test (unchanged, no tgbotapi dependency) ===
func TestTelegramChannel_RegisteredBotCommands(t *testing.T) {
	ch, _, _ := newTestChannel(t, config.TelegramConfig{Token: fakeToken})
	ch.pipelineSlashes = map[string]string{
		"memory": "Show long-term memory",
		"jobs":   "List cron jobs",
	}
	ch.slashCommands = map[string]Command{
		"status": {Name: "status", Description: "Check bot status"},
		"help":   {Name: "help", Description: "  Show\navailable commands  "},
		"echo":   {Name: "echo"},
	}

	commands := ch.registeredBotCommands()
	if len(commands) != 6 {
		t.Fatalf("registeredBotCommands len = %d, want 6", len(commands))
	}

	gotNames := make([]string, 0, len(commands))
	gotDesc := make(map[string]string, len(commands))
	for _, cmd := range commands {
		gotNames = append(gotNames, cmd.Command)
		gotDesc[cmd.Command] = cmd.Description
	}

	if strings.Join(gotNames, ",") != "echo,help,jobs,memory,new,status" {
		t.Fatalf("registered command names = %v, want [echo help jobs memory new status]", gotNames)
	}
	if gotDesc["new"] != "Start a fresh session" {
		t.Fatalf("/new description = %q", gotDesc["new"])
	}
	if gotDesc["help"] != "Show available commands" {
		t.Fatalf("/help description = %q", gotDesc["help"])
	}
	if gotDesc["echo"] != "Run /echo" {
		t.Fatalf("/echo description = %q", gotDesc["echo"])
	}
	if gotDesc["memory"] != "Show long-term memory" {
		t.Fatalf("/memory description = %q", gotDesc["memory"])
	}
}

func TestTelegramChannel_RegisteredBotCommands_SkipsInvalidNames(t *testing.T) {
	ch, _, _ := newTestChannel(t, config.TelegramConfig{Token: fakeToken})
	ch.pipelineSlashes = map[string]string{
		"cron-add":    "Schedule a job",
		"cron-list":   "List jobs",
		"cron-remove": "Remove a job",
		"memory":      "Show memory",
		"jobs":        "List cron jobs",
	}
	commands := ch.registeredBotCommands()
	names := make([]string, 0, len(commands))
	for _, cmd := range commands {
		names = append(names, cmd.Command)
	}
	if strings.Join(names, ",") != "jobs,memory,new" {
		t.Fatalf("registered command names = %v, want [jobs memory new]", names)
	}
}

func TestValidTelegramBotCommand(t *testing.T) {
	tests := map[string]bool{
		"new": true, "compact": true, "memory": true, "jobs": true, "cron_add": true,
		"": false, "cron-add": false, "Cron": false, "a-b": false, strings.Repeat("a", 33): false,
	}
	for name, want := range tests {
		if got := validTelegramBotCommand(name); got != want {
			t.Fatalf("validTelegramBotCommand(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestTelegramChannel_TelegramRootOverride(t *testing.T) {
	b := bus.New(10, channelTestLog)
	ch, err := NewTelegramChannel(config.TelegramConfig{Token: fakeToken, RootDir: "/tmp/custom-telegram"}, "", channelTestLog, b)
	if err != nil {
		t.Fatalf("NewTelegramChannel error: %v", err)
	}
	if got := ch.telegramRoot(); got != "/tmp/custom-telegram" {
		t.Fatalf("telegramRoot = %q, want /tmp/custom-telegram", got)
	}
}

func TestTelegramChannel_SyncBotCommands(t *testing.T) {
	ch, caller, _ := newTestChannel(t, config.TelegramConfig{Token: fakeToken})
	ch.pipelineSlashes = map[string]string{"memory": "Show long-term memory"}
	ch.slashCommands = map[string]Command{
		"compact": {Name: "compact", Description: "Compress conversation history"},
		"status":  {Name: "status", Description: "Check bot status"},
	}

	if err := ch.syncBotCommands(context.Background()); err != nil {
		t.Fatalf("syncBotCommands error: %v", err)
	}

	var payloads []struct {
		Commands []struct {
			Command     string `json:"command"`
			Description string `json:"description"`
		} `json:"commands"`
		Scope *struct {
			Type string `json:"type"`
		} `json:"scope,omitempty"`
	}

	for _, call := range caller.calls {
		if !strings.HasSuffix(call.URL, "/setMyCommands") {
			continue
		}
		var payload struct {
			Commands []struct {
				Command     string `json:"command"`
				Description string `json:"description"`
			} `json:"commands"`
			Scope *struct {
				Type string `json:"type"`
			} `json:"scope,omitempty"`
		}
		if err := json.Unmarshal(call.Data.BodyRaw, &payload); err != nil {
			t.Fatalf("unmarshal setMyCommands payload: %v", err)
		}
		payloads = append(payloads, payload)
	}

	if len(payloads) != 2 {
		t.Fatalf("setMyCommands call count = %d, want 2", len(payloads))
	}

	for i, payload := range payloads {
		if len(payload.Commands) != 4 {
			t.Fatalf("payload %d command count = %d, want 4", i, len(payload.Commands))
		}
		names := []string{payload.Commands[0].Command, payload.Commands[1].Command, payload.Commands[2].Command, payload.Commands[3].Command}
		if strings.Join(names, ",") != "compact,memory,new,status" {
			t.Fatalf("payload %d commands = %v, want [compact memory new status]", i, names)
		}
	}

	if payloads[0].Scope != nil {
		t.Fatalf("first payload scope = %+v, want nil", payloads[0].Scope)
	}
	if payloads[1].Scope == nil || payloads[1].Scope.Type != "all_private_chats" {
		t.Fatalf("second payload scope = %+v, want all_private_chats", payloads[1].Scope)
	}
}

// === Status Card Tests ===

func TestStatusCard_Render_Empty(t *testing.T) {
	card := NewStatusCard()
	html := card.Render()
	if !strings.Contains(html, "Working...") {
		t.Errorf("expected Working... in card, got %q", html)
	}
	if !strings.Contains(html, "⏱") {
		t.Errorf("expected timer in card, got %q", html)
	}
}

func TestStatusCard_Render_WithTools(t *testing.T) {
	card := NewStatusCard()
	card.AddTool("t1", "Read", "config.go")
	card.AddTool("t2", "Grep", "handleAuth")
	card.FinishTool("t1", false)
	html := card.Render()
	if !strings.Contains(html, "✅") {
		t.Error("expected ✅ for finished tool")
	}
	if !strings.Contains(html, "⏳") {
		t.Error("expected ⏳ for running tool")
	}
	if !strings.Contains(html, "Read") {
		t.Error("expected tool name Read")
	}
	if !strings.Contains(html, "config.go") {
		t.Error("expected tool summary config.go")
	}
}
func TestStatusCard_Render_WithIteration(t *testing.T) {
	card := NewStatusCard()
	card.SetIteration(3)
	card.AddTool("t1", "Bash", "ls -la")
	html := card.Render()
	if !strings.Contains(html, "Iteration 3") {
		t.Errorf("expected Iteration 3 in card, got %q", html)
	}
}
func TestStatusCard_FinishTool_Error(t *testing.T) {
	card := NewStatusCard()
	card.AddTool("t1", "Edit", "main.go")
	card.FinishTool("t1", true)
	html := card.Render()
	if !strings.Contains(html, "❌") {
		t.Error("expected ❌ for failed tool")
	}
}
func TestSummarizeToolInput(t *testing.T) {
	tests := []struct {
		name  string
		tool  string
		input string
		want  string
	}{
		{"file path", "Read", `{"filePath":"/src/main.go"}`, "/src/main.go"},
		{"command", "Bash", `{"command":"ls -la"}`, "ls -la"},
		{"query", "Grep", `{"query":"handleAuth","include":"*.go"}`, "handleAuth"},
		{"empty", "Read", `{}`, ""},
		{"invalid json", "Read", `not json`, ""},
		{"long path", "Read", `{"filePath":"/very/long/path/that/exceeds/forty/characters/limit/file.go"}`, "/very/long/path/that/exceeds/forty/ch..."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SummarizeToolInput(tt.tool, json.RawMessage(tt.input))
			if got != tt.want {
				t.Errorf("SummarizeToolInput(%s) = %q, want %q", tt.tool, got, tt.want)
			}
		})
	}
}
func TestTelegramChannel_SendStream_PureText(t *testing.T) {
	ch, caller, _ := newTestChannel(t, config.TelegramConfig{Streaming: true, Feedback: "normal"})
	events := make(chan api.StreamEvent, 10)
	// Pure text — no tool events, should skip status card
	events <- api.StreamEvent{Type: api.EventContentBlockDelta, Delta: &api.Delta{Type: "text_delta", Text: "hello "}}
	events <- api.StreamEvent{Type: api.EventContentBlockDelta, Delta: &api.Delta{Type: "text_delta", Text: "world"}}
	close(events)
	err := ch.SendStream(context.Background(), "123", nil, events)
	if err != nil {
		t.Fatalf("SendStream error: %v", err)
	}
	var sendCount, deleteCount int
	var silentSend, normalSend int
	for _, c := range caller.calls {
		if strings.HasSuffix(c.URL, "/sendMessage") {
			sendCount++
			if c.Data != nil && len(c.Data.BodyRaw) > 0 {
				var payload map[string]any
				if err := json.Unmarshal(c.Data.BodyRaw, &payload); err == nil {
					if v, ok := payload["disable_notification"].(bool); ok && v {
						silentSend++
					} else {
						normalSend++
					}
				}
			}
		}
		if strings.HasSuffix(c.URL, "/deleteMessage") {
			deleteCount++
		}
	}
	// Content message is now ticker-driven (not created on first delta),
	// so with synchronous events only the final report is sent.
	if sendCount < 1 {
		t.Errorf("expected at least 1 sendMessage call (final), got %d", sendCount)
	}
	if normalSend == 0 {
		t.Error("expected final report to be sent as a normal notification message")
	}
}
func TestTelegramChannel_SendStream_WithTools(t *testing.T) {
	ch, caller, _ := newTestChannel(t, config.TelegramConfig{Streaming: true, Feedback: "debug"})
	events := make(chan api.StreamEvent, 20)
	iter := 0
	// Simulate: iteration_start -> content_block(tool_use) -> tool_execution -> text
	events <- api.StreamEvent{Type: api.EventIterationStart, Iteration: &iter}
	events <- api.StreamEvent{Type: api.EventContentBlockStart, ContentBlock: &api.ContentBlock{Type: "tool_use", ID: "t1", Name: "Read"}}
	events <- api.StreamEvent{Type: api.EventContentBlockStop}
	events <- api.StreamEvent{Type: api.EventToolExecutionStart, ToolUseID: "t1", Name: "Read", Iteration: &iter}
	events <- api.StreamEvent{Type: api.EventToolExecutionResult, ToolUseID: "t1", Name: "Read"}
	events <- api.StreamEvent{Type: api.EventContentBlockDelta, Delta: &api.Delta{Type: "text_delta", Text: "result"}}
	close(events)
	err := ch.SendStream(context.Background(), "123", nil, events)
	if err != nil {
		t.Fatalf("SendStream error: %v", err)
	}
	var sendCount, deleteCount int
	var silentSend, normalSend int
	for _, c := range caller.calls {
		if strings.HasSuffix(c.URL, "/sendMessage") {
			sendCount++
			if c.Data != nil && len(c.Data.BodyRaw) > 0 {
				var payload map[string]any
				if err := json.Unmarshal(c.Data.BodyRaw, &payload); err == nil {
					if v, ok := payload["disable_notification"].(bool); ok && v {
						silentSend++
					} else {
						normalSend++
					}
				}
			}
		}
		if strings.HasSuffix(c.URL, "/deleteMessage") {
			deleteCount++
		}
	}
	// Status card is event-driven; content message is ticker-driven.
	// With synchronous events, only status card + final report are sent.
	if sendCount < 2 {
		t.Errorf("expected at least 2 sendMessage calls (status + final), got %d", sendCount)
	}
	if deleteCount < 1 {
		t.Errorf("expected at least 1 deleteMessage call for status cleanup, got %d", deleteCount)
	}
	if silentSend < 1 {
		t.Errorf("expected at least 1 silent intermediate message, got %d", silentSend)
	}
	if normalSend == 0 {
		t.Error("expected final report to be sent as a normal notification message")
	}
}

func TestStreamState_ToolExecutionOutputAppendsToCard(t *testing.T) {
	ch, _, _ := newTestChannel(t, config.TelegramConfig{Streaming: true, Feedback: "normal"})
	st := newStreamState(context.Background(), ch, "123", 123, nil)
	iter := 0
	st.handleEvent(api.StreamEvent{Type: api.EventIterationStart, Iteration: &iter})
	st.handleEvent(api.StreamEvent{Type: api.EventToolExecutionStart, ToolUseID: "t1", Name: "DelegateTask"})
	st.handleEvent(api.StreamEvent{Type: api.EventToolExecutionOutput, ToolUseID: "t1", Output: "💭 thinking…\n"})
	tr := true
	st.handleEvent(api.StreamEvent{Type: api.EventToolExecutionOutput, ToolUseID: "t1", Output: "err", IsStderr: &tr})
	rendered := st.card.Render()
	if !strings.Contains(rendered, "💭 thinking…") {
		t.Fatalf("card missing streamed output: %s", rendered)
	}
	if !strings.Contains(rendered, "[stderr] err") {
		t.Fatalf("card missing stderr prefix: %s", rendered)
	}
}

func TestTelegramChannel_SendStream_Disabled(t *testing.T) {
	ch, caller, _ := newTestChannel(t, config.TelegramConfig{Streaming: false})
	events := make(chan api.StreamEvent, 5)
	events <- api.StreamEvent{Type: api.EventContentBlockDelta, Delta: &api.Delta{Type: "text_delta", Text: "buffered"}}
	close(events)
	err := ch.SendStream(context.Background(), "123", nil, events)
	if err != nil {
		t.Fatalf("SendStream error: %v", err)
	}
	var hasSend bool
	for _, c := range caller.calls {
		if strings.HasSuffix(c.URL, "/sendMessage") {
			hasSend = true
		}
	}
	if !hasSend {
		t.Error("expected sendMessage for non-streaming mode")
	}
}

func TestTelegramChannel_SendStream_ErrorVisibility(t *testing.T) {
	ch, caller, _ := newTestChannel(t, config.TelegramConfig{Streaming: true, Feedback: "normal"})
	events := make(chan api.StreamEvent, 5)
	events <- api.StreamEvent{Type: api.EventError, Output: "unexpected end of JSON input"}
	close(events)

	if err := ch.SendStream(context.Background(), "123", nil, events); err != nil {
		t.Fatalf("SendStream error: %v", err)
	}

	finalText := ""
	for i := len(caller.calls) - 1; i >= 0; i-- {
		c := caller.calls[i]
		if !strings.HasSuffix(c.URL, "/sendMessage") || c.Data == nil || len(c.Data.BodyRaw) == 0 {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(c.Data.BodyRaw, &payload); err != nil {
			continue
		}
		if text, ok := payload["text"].(string); ok {
			finalText = text
			break
		}
	}

	if !strings.Contains(finalText, "stream failed: unexpected end of JSON input") {
		t.Fatalf("final message = %q, want stream error visible", finalText)
	}
	if strings.Contains(finalText, "agent return null") {
		t.Fatalf("final message should not fallback to null, got %q", finalText)
	}
}

func TestTelegramChannel_SendStream_FinalSendFailureKeepsIntermediateMessages(t *testing.T) {
	ch, caller, _ := newTestChannel(t, config.TelegramConfig{Streaming: true, Feedback: "debug"})
	caller.methodErrSeq = map[string][]error{
		"sendMessage": {nil, errors.New("final send failed"), errors.New("final send failed")},
	}

	events := make(chan api.StreamEvent, 10)
	iter := 0
	events <- api.StreamEvent{Type: api.EventIterationStart, Iteration: &iter}
	events <- api.StreamEvent{Type: api.EventToolExecutionStart, ToolUseID: "t1", Name: "Read", Iteration: &iter}
	events <- api.StreamEvent{Type: api.EventContentBlockDelta, Delta: &api.Delta{Type: "text_delta", Text: "partial result"}}
	close(events)

	err := ch.SendStream(context.Background(), "123", nil, events)
	if err == nil || !strings.Contains(err.Error(), "final send failed") {
		t.Fatalf("SendStream error = %v, want final send failure", err)
	}

	var sendCount, deleteCount int
	for _, c := range caller.calls {
		if strings.HasSuffix(c.URL, "/sendMessage") {
			sendCount++
		}
		if strings.HasSuffix(c.URL, "/deleteMessage") {
			deleteCount++
		}
	}
	if sendCount < 3 {
		t.Fatalf("expected intermediate + final send attempts, got %d", sendCount)
	}
	if deleteCount != 0 {
		t.Fatalf("expected no deleteMessage when final send fails, got %d", deleteCount)
	}
}

func TestTelegramRetryAfter(t *testing.T) {
	delay, ok := telegramRetryAfter(&ta.Error{ErrorCode: 429, Parameters: &ta.ResponseParameters{RetryAfter: 3}})
	if !ok || delay != 3*time.Second {
		t.Fatalf("api retry after = (%v, %v), want (%v, true)", delay, ok, 3*time.Second)
	}

	delay, ok = telegramRetryAfter(errors.New(`telego: editMessageText: api: 429 "Too Many Requests: retry after 717", migrate to chat ID: 0, retry after: 717`))
	if !ok {
		t.Fatal("expected retry after to be detected")
	}
	if delay != 717*time.Second {
		t.Fatalf("delay = %v, want %v", delay, 717*time.Second)
	}
	if _, ok := telegramRetryAfter(errors.New("plain error")); ok {
		t.Fatal("unexpected retry after for plain error")
	}
}

func TestTelegramChannel_SendStream_PrivateUsesMessageDraftWhenTickerFlushes(t *testing.T) {
	ch, caller, _ := newTestChannel(t, config.TelegramConfig{Streaming: true, Feedback: "normal"})
	events := make(chan api.StreamEvent, 10)
	events <- api.StreamEvent{Type: api.EventContentBlockDelta, Delta: &api.Delta{Type: "text_delta", Text: "hello"}}
	go func() {
		time.Sleep(450 * time.Millisecond)
		close(events)
	}()
	if err := ch.SendStream(context.Background(), "123", nil, events); err != nil {
		t.Fatalf("SendStream error: %v", err)
	}
	var draftCount int
	for _, c := range caller.calls {
		if strings.HasSuffix(c.URL, "/sendMessageDraft") {
			draftCount++
		}
	}
	if draftCount < 1 {
		t.Fatalf("expected at least 1 sendMessageDraft for private chat, got %d", draftCount)
	}
}

// TestTelegramChannel_SendStream_GroupDoesNotUseMessageDraft ensures negative chat IDs (basic groups
// and supergroups such as -100…) never call sendMessageDraft; only private DMs use drafts.
func TestTelegramChannel_SendStream_GroupDoesNotUseMessageDraft(t *testing.T) {
	ch, caller, _ := newTestChannel(t, config.TelegramConfig{Streaming: true, Feedback: "normal"})
	events := make(chan api.StreamEvent, 10)
	events <- api.StreamEvent{Type: api.EventContentBlockDelta, Delta: &api.Delta{Type: "text_delta", Text: "hello"}}
	go func() {
		time.Sleep(450 * time.Millisecond)
		close(events)
	}()
	if err := ch.SendStream(context.Background(), "-1001234567890", nil, events); err != nil {
		t.Fatalf("SendStream error: %v", err)
	}
	for _, c := range caller.calls {
		if strings.HasSuffix(c.URL, "/sendMessageDraft") {
			t.Fatalf("group chats must not call sendMessageDraft, saw %s", c.URL)
		}
	}
}

func TestTruncateForTelegramDraftText(t *testing.T) {
	const maxRunes = telegramStreamDraftMaxRunes
	runes := strings.Repeat("é", maxRunes+10) // 2-byte runes
	got := stringutil.TruncateRunes(runes, maxRunes)
	if utf8.RuneCountInString(got) != maxRunes {
		t.Fatalf("rune count = %d, want %d", utf8.RuneCountInString(got), maxRunes)
	}
	short := "hello"
	if stringutil.TruncateRunes(short, maxRunes) != short {
		t.Fatalf("truncate short = %q", stringutil.TruncateRunes(short, maxRunes))
	}
}

func TestTelegramChannel_SendStream_FinalSend429RetriesOnce(t *testing.T) {
	ch, caller, _ := newTestChannel(t, config.TelegramConfig{Streaming: true, Feedback: "debug"})
	caller.methodErrSeq = map[string][]error{
		"sendMessage": {nil, errors.New(`telego: sendMessage: api: 429 "Too Many Requests: retry after 0", migrate to chat ID: 0, retry after: 0`), nil},
	}

	events := make(chan api.StreamEvent, 10)
	iter := 0
	events <- api.StreamEvent{Type: api.EventIterationStart, Iteration: &iter}
	events <- api.StreamEvent{Type: api.EventToolExecutionStart, ToolUseID: "t1", Name: "Read", Iteration: &iter}
	events <- api.StreamEvent{Type: api.EventContentBlockDelta, Delta: &api.Delta{Type: "text_delta", Text: "partial result"}}
	close(events)

	if err := ch.SendStream(context.Background(), "123", nil, events); err != nil {
		t.Fatalf("SendStream error: %v", err)
	}

	var sendCount int
	for _, c := range caller.calls {
		if strings.HasSuffix(c.URL, "/sendMessage") {
			sendCount++
		}
	}
	if sendCount < 3 {
		t.Fatalf("expected retry after final send 429, got %d sendMessage calls", sendCount)
	}
}

func TestTelegramChannel_SaveFile_SanitizesName(t *testing.T) {
	tmpDir := t.TempDir()
	ch, _, _ := newTestChannelWithWorkspace(t, config.TelegramConfig{}, tmpDir)

	payload := []byte("hello")
	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer downloadServer.Close()

	serverURL, err := url.Parse(downloadServer.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	ch.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		clonedReq := req.Clone(req.Context())
		clonedReq.URL.Scheme = serverURL.Scheme
		clonedReq.URL.Host = serverURL.Host
		return transport.RoundTrip(clonedReq)
	})}

	savedPath, err := ch.saveFile("f1", "../nested/evil.txt")
	if err != nil {
		t.Fatalf("saveFile error: %v", err)
	}
	if strings.Contains(savedPath, "..") {
		t.Fatalf("saved path must not contain parent segments: %s", savedPath)
	}
	base := filepath.Base(savedPath)
	if !strings.Contains(base, "evil.txt") {
		t.Fatalf("sanitized basename %q should retain evil.txt", base)
	}
}

func TestTelegramChannel_SendReaction_SkipsMissingMessageID(t *testing.T) {
	ch, _, _ := newTestChannel(t, config.TelegramConfig{Feedback: "debug"})
	ch.PreProcessFeedback(123, 0)
}
