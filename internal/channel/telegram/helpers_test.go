package telegram

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/config"
	mavenlog "github.com/ageneralai/maven/pkg/log"
	"github.com/mymmrac/telego"
	ta "github.com/mymmrac/telego/telegoapi"
)

const fakeToken = "1234567890:ABCDEFGHIJKLMNOPQRSTUVWXYZ_abcdefgh"

var channelTestLog = mavenlog.Std()

type mockCaller struct {
	responses       map[string]*ta.Response
	calls           []mockCall
	callErr         error
	methodErrSeq    map[string][]error
	methodCallCount map[string]int
}

type mockCall struct {
	URL  string
	Data *ta.RequestData
}

func newMockCaller() *mockCaller {
	return &mockCaller{
		responses: map[string]*ta.Response{
			"getMe":              {Ok: true, Result: []byte(`{"id":1,"is_bot":true,"first_name":"Test","username":"testbot"}`)},
			"sendMessage":        {Ok: true, Result: []byte(`{"message_id":1,"date":0,"chat":{"id":123,"type":"private"}}`)},
			"editMessageText":    {Ok: true, Result: []byte(`{"message_id":1,"date":0,"chat":{"id":123,"type":"private"}}`)},
			"setMessageReaction": {Ok: true, Result: []byte(`true`)},
			"sendChatAction":     {Ok: true, Result: []byte(`true`)},
			"sendMessageDraft":   {Ok: true, Result: []byte(`true`)},
			"getFile":            {Ok: true, Result: []byte(`{"file_id":"f1","file_path":"photos/test.jpg"}`)},
			"getUpdates":         {Ok: true, Result: []byte(`[]`)},
		},
		methodCallCount: map[string]int{},
	}
}

func (m *mockCaller) Call(ctx context.Context, url string, data *ta.RequestData) (*ta.Response, error) {
	m.calls = append(m.calls, mockCall{URL: url, Data: data})
	if m.callErr != nil {
		return nil, m.callErr
	}
	parts := strings.Split(url, "/")
	method := parts[len(parts)-1]
	if seq := m.methodErrSeq[method]; len(seq) > 0 {
		idx := m.methodCallCount[method]
		m.methodCallCount[method] = idx + 1
		if idx < len(seq) && seq[idx] != nil {
			return nil, seq[idx]
		}
	}
	if resp, ok := m.responses[method]; ok {
		return resp, nil
	}
	return &ta.Response{Ok: true, Result: []byte(`true`)}, nil
}

func newTestBot(t *testing.T, caller *mockCaller) *telego.Bot {
	t.Helper()
	bot, err := telego.NewBot(fakeToken, telego.WithAPICaller(caller))
	if err != nil {
		t.Fatalf("newTestBot: %v", err)
	}
	return bot
}

func newTestChannel(t *testing.T, cfg config.TelegramConfig) (*TelegramChannel, *mockCaller) {
	return newTestChannelWithWorkspace(t, cfg, "")
}

func newTestChannelWithWorkspace(t *testing.T, cfg config.TelegramConfig, workspace string) (*TelegramChannel, *mockCaller) {
	t.Helper()
	b := bus.New(10, channelTestLog)
	if cfg.Token == "" {
		cfg.Token = fakeToken
	}
	ch, err := NewTelegramChannel(cfg, workspace, channelTestLog, b)
	if err != nil {
		t.Fatalf("newTestChannel: %v", err)
	}
	caller := newMockCaller()
	bot := newTestBot(t, caller)
	ch.bot = bot
	return ch, caller
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type retrySendCaller struct {
	inner     *mockCaller
	failFirst bool
	callCount int
}

func (r *retrySendCaller) Call(ctx context.Context, url string, data *ta.RequestData) (*ta.Response, error) {
	r.callCount++
	if r.failFirst && r.callCount == 1 && strings.HasSuffix(url, "/sendMessage") {
		return &ta.Response{Ok: false, Error: &ta.Error{Description: "HTML parse error", ErrorCode: 400}}, nil
	}
	return r.inner.Call(ctx, url, data)
}
