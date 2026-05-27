package wecom

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/model"
	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/channel"
	"github.com/ageneralai/maven/internal/channel/allowlist"
	"github.com/ageneralai/maven/internal/channel/webhook"
	"github.com/ageneralai/maven/internal/config"
	"log/slog"

	"github.com/ageneralai/maven/pkg/httpc"
	"github.com/ageneralai/maven/pkg/stringutil"
)

const wecomChannelName = "wecom"

const (
	wecomDefaultPort         = 9886
	wecomInboundImageMaxBytes = 10 << 20 // 10MB
	wecomSendMaxRetries      = 3
)

// wecomMarkdownMaxBytes is the WeCom API hard limit on markdown message length.
const wecomMarkdownMaxBytes = 20480

type weComSendResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

type weComAPIError struct {
	Code int
	Msg  string
}

func (e *weComAPIError) Error() string {
	return fmt.Sprintf("wecom send error: %d %s", e.Code, e.Msg)
}

func (e *weComAPIError) IsRetryable() bool {
	return e.Code == -1 || e.Code == 6000
}

type weComHTTPStatusError struct {
	Code int
	Body string
}

func (e *weComHTTPStatusError) Error() string {
	return fmt.Sprintf("wecom response_url status %d: %s", e.Code, e.Body)
}

func (e *weComHTTPStatusError) IsRetryable() bool {
	return e.Code >= 500
}

type retryable interface {
	IsRetryable() bool
}

func wecomMarkdownSendPayload(content string) ([]byte, error) {
	payload := map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": content,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal wecom response_url payload: %w", err)
	}
	return body, nil
}

func (w *WeComChannel) sendMessage(ctx context.Context, responseURL string, msg bus.OutboundMessage) error {
	if strings.TrimSpace(responseURL) == "" {
		return fmt.Errorf("wecom response_url is required")
	}
	content := stringutil.TruncateBytes(msg.Content, wecomMarkdownMaxBytes)
	return w.sendTextWithRetry(ctx, responseURL, content)
}

func (w *WeComChannel) sendTextWithRetry(ctx context.Context, responseURL, content string) error {
	var lastErr error
	for attempt := 1; attempt <= wecomSendMaxRetries; attempt++ {
		err := w.sendTextOnce(ctx, responseURL, content)
		if err == nil {
			return nil
		}

		lastErr = err
		if !w.shouldRetry(err) || attempt == wecomSendMaxRetries {
			return err
		}

		backoff := time.Duration(attempt*attempt) * 100 * time.Millisecond
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}

	return lastErr
}

func (w *WeComChannel) shouldRetry(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var r retryable
	if errors.As(err, &r) {
		return r.IsRetryable()
	}
	return true
}

func (w *WeComChannel) sendTextOnce(ctx context.Context, responseURL, content string) error {
	body, err := wecomMarkdownSendPayload(content)
	if err != nil {
		return fmt.Errorf("marshal wecom response_url payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, responseURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create wecom response_url request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send wecom response_url message: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &weComHTTPStatusError{
			Code: resp.StatusCode,
			Body: strings.TrimSpace(string(raw)),
		}
	}

	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil
	}

	var result weComSendResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil
	}
	if result.ErrCode != 0 {
		return &weComAPIError{Code: result.ErrCode, Msg: result.ErrMsg}
	}

	return nil
}

type WeComChannel struct {
	name       string
	log        *slog.Logger
	bus        *bus.MessageBus
	allow      allowlist.Matcher
	cfg        config.WeComConfig
	httpClient *http.Client
	server     *http.Server
	cancel     context.CancelFunc
	runCtx     context.Context
	msgCache   *webhook.MsgIDCache
	replyCache *webhook.ReplyURLCache
	receiveID  string
}

func NewWeComChannel(cfg config.WeComConfig, lg *slog.Logger, b *bus.MessageBus) (*WeComChannel, error) {
	httpClient, err := httpc.ClientFromProxy(cfg.Proxy)
	if err != nil {
		return nil, fmt.Errorf("wecom proxy: %w", err)
	}
	receiveID := strings.TrimSpace(cfg.ReceiveID)
	return &WeComChannel{
		name:       wecomChannelName,
		log:        lg,
		bus:        b,
		allow:      allowlist.NewMatcher(cfg.AllowFrom),
		cfg:        cfg,
		httpClient: httpClient,
		msgCache:   webhook.NewMsgIDCache(0),
		replyCache: webhook.NewReplyURLCache(0),
		receiveID:  receiveID,
		runCtx:     context.Background(),
	}, nil
}

func (w *WeComChannel) Name() string {
	return w.name
}

func (w *WeComChannel) IsAllowed(senderID string) bool {
	return w.allow.Allow(senderID)
}

func (w *WeComChannel) Start(ctx context.Context) error {
	ctx, w.cancel = context.WithCancel(ctx)
	w.runCtx = ctx

	port := w.cfg.Port
	if port == 0 {
		port = wecomDefaultPort
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/wecom/bot", w.handleCallback)

	w.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		w.log.Info("wecom callback server listening", "port", port)
		if err := w.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			w.log.Error("wecom server error", "err", err)
		}
	}()

	go func() {
		<-ctx.Done()
		_ = w.server.Close()
	}()

	return nil
}

func (w *WeComChannel) Stop() error {
	if w.cancel != nil {
		w.cancel()
	}
	if w.server != nil {
		_ = w.server.Close()
	}
	w.log.Info("wecom stopped")
	return nil
}

func (w *WeComChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	chatID := strings.TrimSpace(msg.ChatID)
	if chatID == "" {
		return fmt.Errorf("wecom chat id is required")
	}
	responseURL, ok := w.replyCache.Get(chatID)
	if !ok {
		return fmt.Errorf("wecom response_url not found or expired for chat id %q", chatID)
	}
	return w.sendMessage(ctx, responseURL, msg)
}

func (w *WeComChannel) Capabilities() channel.CapabilitySet {
	return channel.CapabilitySet{FileUpload: true, ReactiveOnly: true}
}

type weComEncryptedEnvelope struct {
	Encrypt string `json:"-"`
}

func (e *weComEncryptedEnvelope) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for _, key := range []string{"encrypt", "Encrypt"} {
		if v, ok := raw[key]; ok {
			return json.Unmarshal(v, &e.Encrypt)
		}
	}
	return nil
}

func (e weComEncryptedEnvelope) CipherText() string {
	return strings.TrimSpace(e.Encrypt)
}

type weComFrom struct {
	UserID string `json:"userid"`
}

type weComText struct {
	Content string `json:"content"`
}

type weComMixedItem struct {
	MsgType string    `json:"msgtype"`
	Text    weComText `json:"text"`
}

type weComMixed struct {
	MsgItem []weComMixedItem `json:"msg_item"`
}

type weComVoice struct {
	Content string `json:"content"`
}

type weComImage struct {
	URL      string `json:"url"`
	PicURL   string `json:"pic_url"`
	ImageURL string `json:"image_url"`
	MediaID  string `json:"media_id"`
}

func (i weComImage) URLValue() string {
	for _, candidate := range []string{i.URL, i.PicURL, i.ImageURL} {
		if v := strings.TrimSpace(candidate); v != "" {
			return v
		}
	}
	return ""
}

type weComInboundMessage struct {
	MsgID       string     `json:"msgid"`
	AIBotID     string     `json:"aibotid"`
	ChatID      string     `json:"chatid"`
	ChatType    string     `json:"chattype"`
	From        weComFrom  `json:"from"`
	FromUserID  string     `json:"fromuserid"`
	ResponseURL string     `json:"response_url"`
	MsgType     string     `json:"msgtype"`
	Text        weComText  `json:"text"`
	Mixed       weComMixed `json:"mixed"`
	Voice       weComVoice `json:"voice"`
	Image       weComImage `json:"image"`
}

type weComReplyEnvelope struct {
	Encrypt         string `json:"encrypt"`
	MsgSignature    string `json:"msgsignature"`
	MsgSignatureAlt string `json:"msg_signature,omitempty"`
	Timestamp       string `json:"timestamp"`
	Nonce           string `json:"nonce"`
}

func (w *WeComChannel) handleCallback(resp http.ResponseWriter, req *http.Request) {
	sig := req.URL.Query().Get("msg_signature")
	timestamp := req.URL.Query().Get("timestamp")
	nonce := req.URL.Query().Get("nonce")

	if sig == "" || timestamp == "" || nonce == "" {
		http.Error(resp, "missing signature params", http.StatusBadRequest)
		return
	}

	switch req.Method {
	case http.MethodGet:
		w.verifyCallbackURL(resp, req, sig, timestamp, nonce)
	case http.MethodPost:
		w.handleIncomingMessage(resp, req, sig, timestamp, nonce)
	default:
		http.Error(resp, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (w *WeComChannel) verifyCallbackURL(resp http.ResponseWriter, req *http.Request, sig, timestamp, nonce string) {
	echostr := req.URL.Query().Get("echostr")
	if echostr == "" {
		http.Error(resp, "missing echostr", http.StatusBadRequest)
		return
	}

	if w.signature(timestamp, nonce, echostr) != sig {
		http.Error(resp, "invalid signature", http.StatusUnauthorized)
		return
	}

	plaintext, _, err := w.decrypt(echostr)
	if err != nil {
		http.Error(resp, "decrypt echostr failed", http.StatusBadRequest)
		return
	}

	resp.WriteHeader(http.StatusOK)
	_, _ = resp.Write([]byte(plaintext))
}

func (w *WeComChannel) handleIncomingMessage(resp http.ResponseWriter, req *http.Request, sig, timestamp, nonce string) {
	req.Body = http.MaxBytesReader(resp, req.Body, 1<<20) // 1MB limit
	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(resp, "read body failed", http.StatusBadRequest)
		return
	}

	var envelope weComEncryptedEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		http.Error(resp, "invalid json", http.StatusBadRequest)
		return
	}

	encrypt := envelope.CipherText()
	if encrypt == "" {
		http.Error(resp, "missing encrypt field", http.StatusBadRequest)
		return
	}

	if w.signature(timestamp, nonce, encrypt) != sig {
		http.Error(resp, "invalid signature", http.StatusUnauthorized)
		return
	}

	plaintext, receiveID, err := w.decrypt(encrypt)
	if err != nil {
		http.Error(resp, "decrypt message failed", http.StatusBadRequest)
		return
	}

	replyBody, err := w.buildEncryptedReply(timestamp, nonce, receiveID, "success")
	if err != nil {
		http.Error(resp, "encrypt reply failed", http.StatusInternalServerError)
		return
	}

	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(http.StatusOK)
	_, _ = resp.Write(replyBody)

	go w.processDecryptedMessage(plaintext)
}

func (w *WeComChannel) buildEncryptedReply(timestamp, nonce, receiveID string, payload any) ([]byte, error) {
	if payload == nil {
		payload = map[string]any{}
	}

	plainJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal reply payload: %w", err)
	}

	encrypt, err := w.encrypt(string(plainJSON), receiveID)
	if err != nil {
		return nil, fmt.Errorf("encrypt reply payload: %w", err)
	}

	msgSig := w.signature(timestamp, nonce, encrypt)
	reply := weComReplyEnvelope{
		Encrypt:         encrypt,
		MsgSignature:    msgSig,
		MsgSignatureAlt: msgSig,
		Timestamp:       timestamp,
		Nonce:           nonce,
	}

	body, err := json.Marshal(reply)
	if err != nil {
		return nil, fmt.Errorf("marshal reply envelope: %w", err)
	}

	return body, nil
}

func (w *WeComChannel) processDecryptedMessage(plaintext string) {
	var message weComInboundMessage
	if err := json.Unmarshal([]byte(plaintext), &message); err != nil {
		w.log.Error("wecom unmarshal message error", "err", err)
		return
	}

	senderID := w.resolveSenderID(message)
	if senderID == "" {
		return
	}

	if !w.allowMessageFrom(senderID) {
		w.log.Info("wecom rejected message", "sender", senderID)
		return
	}

	messageID := strings.TrimSpace(message.MsgID)
	if messageID != "" && w.msgCache.Seen(messageID) {
		w.log.Debug("wecom duplicate message dropped", "msg_id", messageID)
		return
	}

	chatID := w.resolveChatID(message, senderID)
	if chatID == "" {
		return
	}

	responseURL := strings.TrimSpace(message.ResponseURL)
	if responseURL != "" {
		w.replyCache.Set(chatID, responseURL)
	}

	content := extractWeComContent(message, w.log)
	contentBlocks := w.extractWeComContentBlocks(message)
	if content == "" && len(contentBlocks) == 0 {
		return
	}

	_ = w.bus.PublishInbound(w.runCtx, bus.InboundMessage{
		Channel:       wecomChannelName,
		SenderID:      senderID,
		ChatID:        chatID,
		Content:       content,
		Timestamp:     time.Now(),
		ContentBlocks: contentBlocks,
		TransportMeta: map[string]any{
			"msg_id":         messageID,
			"aibot_id":       strings.TrimSpace(message.AIBotID),
			"chat_id":        strings.TrimSpace(message.ChatID),
			"chat_type":      strings.TrimSpace(message.ChatType),
			"msg_type":       strings.TrimSpace(message.MsgType),
			"image_url":      message.Image.URLValue(),
			"image_media_id": strings.TrimSpace(message.Image.MediaID),
			"response_url":   responseURL,
		},
	})
}

func (w *WeComChannel) resolveSenderID(message weComInboundMessage) string {
	senderID := strings.TrimSpace(message.From.UserID)
	if senderID != "" {
		return senderID
	}
	return strings.TrimSpace(message.FromUserID)
}

func (w *WeComChannel) resolveChatID(message weComInboundMessage, senderID string) string {
	if strings.EqualFold(strings.TrimSpace(message.ChatType), "group") {
		if chatID := strings.TrimSpace(message.ChatID); chatID != "" {
			return chatID
		}
	}
	return senderID
}

func (w *WeComChannel) extractWeComContentBlocks(message weComInboundMessage) []model.ContentBlock {
	if !strings.EqualFold(strings.TrimSpace(message.MsgType), "image") {
		return nil
	}

	block, err := w.buildWeComImageContentBlock(w.runCtx, message)
	if err != nil {
		w.log.Warn("wecom process image warning", "err", err)
	}
	if block == nil {
		return nil
	}

	return []model.ContentBlock{*block}
}

func (w *WeComChannel) buildWeComImageContentBlock(ctx context.Context, message weComInboundMessage) (*model.ContentBlock, error) {
	imageURL := message.Image.URLValue()
	if imageURL == "" {
		mediaID := strings.TrimSpace(message.Image.MediaID)
		if mediaID != "" {
			return nil, fmt.Errorf("wecom image media_id %q requires access_token download", mediaID)
		}
		return nil, fmt.Errorf("wecom image payload missing url")
	}

	hc := w.httpClient
	if hc == nil {
		hc = http.DefaultClient
	}
	base64Data, mediaType, err := downloadWeComImageAsBase64(ctx, imageURL, hc)
	if err != nil {
		return &model.ContentBlock{
			Type: model.ContentBlockImage,
			URL:  imageURL,
		}, fmt.Errorf("download wecom image from %q: %w", imageURL, err)
	}

	return &model.ContentBlock{
		Type:      model.ContentBlockImage,
		MediaType: mediaType,
		Data:      base64Data,
	}, nil
}

func downloadWeComImageAsBase64(ctx context.Context, imageURL string, httpClient *http.Client) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("create image request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("request image: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, wecomInboundImageMaxBytes+1))
	if err != nil {
		return "", "", fmt.Errorf("read image response: %w", err)
	}
	if int64(len(body)) > wecomInboundImageMaxBytes {
		return "", "", fmt.Errorf("image exceeds %d bytes", wecomInboundImageMaxBytes)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("image request failed with status %d", resp.StatusCode)
	}

	mediaType := normalizeWeComMediaType(resp.Header.Get("Content-Type"))
	if mediaType == "" {
		mediaType = http.DetectContentType(body)
	}

	return base64.StdEncoding.EncodeToString(body), mediaType, nil
}

func normalizeWeComMediaType(value string) string {
	contentType := strings.TrimSpace(value)
	if contentType == "" {
		return ""
	}
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = contentType[:idx]
	}
	return strings.TrimSpace(contentType)
}

func extractWeComContent(message weComInboundMessage, lg *slog.Logger) string {
	switch strings.ToLower(strings.TrimSpace(message.MsgType)) {
	case "text":
		return strings.TrimSpace(message.Text.Content)
	case "voice":
		return strings.TrimSpace(message.Voice.Content)
	case "image":
		return "[image]"
	case "mixed":
		parts := make([]string, 0, len(message.Mixed.MsgItem))
		for _, item := range message.Mixed.MsgItem {
			if !strings.EqualFold(strings.TrimSpace(item.MsgType), "text") {
				continue
			}
			if text := strings.TrimSpace(item.Text.Content); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	default:
		lg.Warn("wecom unsupported message type", "msg_type", strings.TrimSpace(message.MsgType))
		return ""
	}
}

func (w *WeComChannel) allowMessageFrom(senderID string) bool {
	return w.allow.Allow(senderID)
}

var _ channel.Channel = (*WeComChannel)(nil)
