package feishu

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/model"
	"log/slog"

	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/channel"
	"github.com/ageneralai/maven/internal/channel/allowlist"
	"github.com/ageneralai/maven/internal/config"
	"github.com/ageneralai/maven/pkg/httpc"
)

const feishuChannelName = "feishu"

// feishuInboundImageMaxBytes is a self-imposed download cap; Feishu has no documented limit.
const feishuInboundImageMaxBytes = 10 << 20

type feishuClient struct {
	appID      string
	appSecret  string
	baseURL    string
	httpClient *http.Client
	mu         sync.RWMutex
	token      string
	tokenExp   time.Time
}

func newFeishuClient(appID, appSecret string, httpClient *http.Client) *feishuClient {
	return &feishuClient{
		appID:      appID,
		appSecret:  appSecret,
		baseURL:    "https://open.feishu.cn",
		httpClient: httpClient,
	}
}

func (c *feishuClient) GetTenantAccessToken(ctx context.Context) (string, error) {
	c.mu.RLock()
	if c.token != "" && time.Now().Before(c.tokenExp) {
		token := c.token
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if c.token != "" && time.Now().Before(c.tokenExp) {
		return c.token, nil
	}

	body := fmt.Sprintf(`{"app_id":"%s","app_secret":"%s"}`, c.appID, c.appSecret)
	req, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/open-apis/auth/v3/tenant_access_token/internal",
		strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("get tenant token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("feishu token error: %s", result.Msg)
	}

	c.token = result.TenantAccessToken
	c.tokenExp = time.Now().Add(time.Duration(result.Expire-60) * time.Second)
	return c.token, nil
}

func feishuTextMessagePayload(chatID, content string) ([]byte, error) {
	textJSON, err := json.Marshal(map[string]string{"text": content})
	if err != nil {
		return nil, fmt.Errorf("marshal text content: %w", err)
	}
	payload := map[string]any{
		"receive_id": chatID,
		"msg_type":   "text",
		"content":    string(textJSON),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	return data, nil
}

func (c *feishuClient) SendMessage(ctx context.Context, chatID, content string) error {
	token, err := c.GetTenantAccessToken(ctx)
	if err != nil {
		return err
	}

	data, err := feishuTextMessagePayload(chatID, content)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/open-apis/im/v1/messages?receive_id_type=chat_id",
		strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("create send request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send feishu message: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode send response: %w", err)
	}
	if result.Code != 0 {
		return fmt.Errorf("feishu send error: %s", result.Msg)
	}
	return nil
}

type FeishuImageDownloader func(ctx context.Context, tenantAccessToken, imageKey string) (string, string, error)

type FeishuChannel struct {
	name            string
	log             *slog.Logger
	bus             *bus.MessageBus
	allow           allowlist.Matcher
	cfg             config.FeishuConfig
	client          *feishuClient
	httpClient      *http.Client
	server          *http.Server
	cancel          context.CancelFunc
	imageDownloader FeishuImageDownloader
}

func NewFeishuChannel(cfg config.FeishuConfig, lg *slog.Logger, b *bus.MessageBus) (*FeishuChannel, error) {
	httpClient, err := httpc.ClientFromProxy(cfg.Proxy)
	if err != nil {
		return nil, fmt.Errorf("feishu proxy: %w", err)
	}
	client := newFeishuClient(cfg.AppID, cfg.AppSecret, httpClient)
	ch := &FeishuChannel{
		name:  feishuChannelName,
		log:   lg,
		bus:   b,
		allow: allowlist.NewMatcher(cfg.AllowFrom),
		cfg:   cfg,
		client:      client,
		httpClient:  httpClient,
	}
	ch.imageDownloader = func(ctx context.Context, tenantAccessToken, imageKey string) (string, string, error) {
		return ch.client.downloadImageAsBase64(ctx, tenantAccessToken, imageKey)
	}
	return ch, nil
}

func (f *FeishuChannel) Name() string {
	return f.name
}

func (f *FeishuChannel) IsAllowed(senderID string) bool {
	return f.allow.Allow(senderID)
}

func (f *FeishuChannel) Start(ctx context.Context) error {
	ctx, f.cancel = context.WithCancel(ctx)

	port := f.cfg.Port
	if port == 0 {
		port = 9876
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/feishu/webhook", f.handleWebhook)

	f.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		f.log.Info("feishu webhook server listening", "port", port)
		if err := f.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			f.log.Error("feishu server error", "err", err)
		}
	}()

	go func() {
		<-ctx.Done()
		_ = f.server.Close()
	}()

	return nil
}

func (f *FeishuChannel) Stop() error {
	if f.cancel != nil {
		f.cancel()
	}
	var err error
	if f.server != nil {
		err = f.server.Close()
	}
	f.log.Info("feishu stopped")
	return err
}

func (f *FeishuChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	return f.client.SendMessage(ctx, msg.ChatID, msg.Content)
}

func (f *FeishuChannel) Capabilities() channel.CapabilitySet {
	return channel.CapabilitySet{FileUpload: true}
}

func (f *FeishuChannel) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	var event struct {
		Challenge string `json:"challenge"`
		Type      string `json:"type"`
		Header    struct {
			EventType string `json:"event_type"`
			Token     string `json:"token"`
		} `json:"header"`
		Event struct {
			Sender struct {
				SenderID struct {
					OpenID string `json:"open_id"`
				} `json:"sender_id"`
			} `json:"sender"`
			Message struct {
				ChatID      string `json:"chat_id"`
				MessageType string `json:"message_type"`
				Content     string `json:"content"`
			} `json:"message"`
		} `json:"event"`
	}

	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// URL verification challenge
	if event.Challenge != "" {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(map[string]string{"challenge": event.Challenge}); err != nil {
			http.Error(w, "encode challenge", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = buf.WriteTo(w)
		return
	}

	// Verify token
	if f.cfg.VerificationToken != "" && event.Header.Token != f.cfg.VerificationToken {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	w.WriteHeader(http.StatusOK)

	// Only handle message events
	if event.Header.EventType != "im.message.receive_v1" {
		return
	}

	senderID := event.Event.Sender.SenderID.OpenID
	if !f.IsAllowed(senderID) {
		f.log.Info("feishu rejected message", "sender", senderID)
		return
	}

	messageType := strings.ToLower(strings.TrimSpace(event.Event.Message.MessageType))
	content, contentBlocks, messageMetadata, err := f.parseFeishuInboundMessage(
		r.Context(),
		messageType,
		event.Event.Message.Content,
	)
	if err != nil {
		f.log.Error("feishu parse message error", "err", err)
		return
	}
	if content == "" && len(contentBlocks) == 0 {
		return
	}

	metadata := map[string]any{"message_type": event.Event.Message.MessageType}
	for k, v := range messageMetadata {
		metadata[k] = v
	}

	_ = f.bus.PublishInbound(r.Context(), bus.InboundMessage{
		Channel:       feishuChannelName,
		SenderID:      senderID,
		ChatID:        event.Event.Message.ChatID,
		Content:       content,
		Timestamp:     time.Now(),
		ContentBlocks: contentBlocks,
		TransportMeta: metadata,
	})
}

func (f *FeishuChannel) parseFeishuInboundMessage(ctx context.Context, messageType, rawContent string) (string, []model.ContentBlock, map[string]any, error) {
	if messageType == "" {
		return "", nil, nil, nil
	}

	switch messageType {
	case "text":
		var textContent struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(rawContent), &textContent); err != nil {
			return "", nil, nil, fmt.Errorf("parse text content: %w", err)
		}
		content := strings.TrimSpace(textContent.Text)
		if content == "" {
			return "", nil, nil, nil
		}
		return content, nil, nil, nil

	case "image":
		var imageContent struct {
			ImageKey string `json:"image_key"`
		}
		if err := json.Unmarshal([]byte(rawContent), &imageContent); err != nil {
			return "", nil, nil, fmt.Errorf("parse image content: %w", err)
		}

		imageKey := strings.TrimSpace(imageContent.ImageKey)
		if imageKey == "" {
			return "", nil, nil, fmt.Errorf("missing image_key")
		}

		block, err := f.buildFeishuImageContentBlock(ctx, imageKey)
		if err != nil {
			f.log.Warn("feishu image download warning", "err", err)
		}
		if block == nil {
			return "[image]", nil, map[string]any{"image_key": imageKey}, nil
		}
		return "[image]", []model.ContentBlock{*block}, map[string]any{"image_key": imageKey}, nil

	default:
		return "", nil, nil, nil
	}
}

func (f *FeishuChannel) buildFeishuImageContentBlock(ctx context.Context, imageKey string) (*model.ContentBlock, error) {
	tenantAccessToken, err := f.client.GetTenantAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("get tenant access token: %w", err)
	}

	downloader := f.imageDownloader
	if downloader == nil {
		downloader = func(ctx context.Context, tenantAccessToken, imageKey string) (string, string, error) {
			return f.client.downloadImageAsBase64(ctx, tenantAccessToken, imageKey)
		}
	}
	base64Data, mediaType, err := downloader(ctx, tenantAccessToken, imageKey)
	if err != nil {
		return &model.ContentBlock{
			Type: model.ContentBlockImage,
			URL:  f.client.imageDownloadURL(imageKey),
		}, fmt.Errorf("download image %q: %w", imageKey, err)
	}

	return &model.ContentBlock{
		Type:      model.ContentBlockImage,
		MediaType: mediaType,
		Data:      base64Data,
	}, nil
}

func (c *feishuClient) imageDownloadURL(imageKey string) string {
	return fmt.Sprintf("%s/open-apis/im/v1/images/%s?image_type=message", c.baseURL, url.PathEscape(strings.TrimSpace(imageKey)))
}

func (c *feishuClient) downloadImageAsBase64(ctx context.Context, tenantAccessToken, imageKey string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.imageDownloadURL(imageKey), nil)
	if err != nil {
		return "", "", fmt.Errorf("create image request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tenantAccessToken)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("request image: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, feishuInboundImageMaxBytes+1))
	if err != nil {
		return "", "", fmt.Errorf("read image response: %w", err)
	}
	if int64(len(body)) > feishuInboundImageMaxBytes {
		return "", "", fmt.Errorf("image exceeds %d bytes", feishuInboundImageMaxBytes)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("image request failed with status %d", resp.StatusCode)
	}
	mediaType := normalizeFeishuMediaType(resp.Header.Get("Content-Type"))
	if mediaType == "" {
		mediaType = http.DetectContentType(body)
	}
	return base64.StdEncoding.EncodeToString(body), mediaType, nil
}

func normalizeFeishuMediaType(value string) string {
	contentType := strings.TrimSpace(value)
	if contentType == "" {
		return ""
	}
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = contentType[:idx]
	}
	return strings.TrimSpace(contentType)
}

var _ channel.Channel = (*FeishuChannel)(nil)
