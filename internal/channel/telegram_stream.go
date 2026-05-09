package channel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/channel/telegram"
	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoapi"
	tu "github.com/mymmrac/telego/telegoutil"
)

const telegramStreamContentDraftID = 1

func telegramRetryAfter(err error) (time.Duration, bool) {
	if err == nil {
		return 0, false
	}
	var apiErr *telegoapi.Error
	if errors.As(err, &apiErr) && apiErr.Parameters != nil && apiErr.ErrorCode == http.StatusTooManyRequests {
		if apiErr.Parameters.RetryAfter > 0 {
			return time.Duration(apiErr.Parameters.RetryAfter) * time.Second, true
		}
	}
	const marker = "retry after "
	text := err.Error()
	idx := strings.LastIndex(text, marker)
	if idx < 0 {
		return 0, false
	}
	start := idx + len(marker)
	end := start
	for end < len(text) && text[end] >= '0' && text[end] <= '9' {
		end++
	}
	if end == start {
		return 0, false
	}
	secs, convErr := strconv.Atoi(text[start:end])
	if convErr != nil {
		return 0, false
	}
	return time.Duration(secs) * time.Second, true
}

func truncateForTelegramDraftText(s string) string {
	const maxRunes = 4096
	n := 0
	for i := range s {
		if n == maxRunes {
			return s[:i]
		}
		n++
	}
	return s
}

func (t *TelegramChannel) sendPlaceholder(chatID int64, text, parseMode string, silent bool) (int, error) {
	if t.bot == nil {
		return 0, fmt.Errorf("telegram bot not initialized")
	}
	msg := tu.Message(tu.ID(chatID), text)
	if parseMode != "" {
		msg = msg.WithParseMode(parseMode)
	}
	if silent {
		msg = msg.WithDisableNotification()
	}
	sent, err := t.bot.SendMessage(context.Background(), msg)
	if err != nil {
		return 0, err
	}
	return sent.MessageID, nil
}

func (t *TelegramChannel) deleteMessage(chatID int64, messageID int) error {
	if t.bot == nil {
		return fmt.Errorf("telegram bot not initialized")
	}
	return t.bot.DeleteMessage(context.Background(), &telego.DeleteMessageParams{
		ChatID:    tu.ID(chatID),
		MessageID: messageID,
	})
}

func (t *TelegramChannel) editMessage(chatID int64, messageID int, text string, parseMode string) error {
	if t.bot == nil {
		return fmt.Errorf("telegram bot not initialized")
	}
	edit := tu.EditMessageText(tu.ID(chatID), messageID, text)
	if parseMode != "" {
		edit = edit.WithParseMode(parseMode)
	}
	_, err := t.bot.EditMessageText(context.Background(), edit)
	if err != nil {
		if strings.Contains(err.Error(), "message is not modified") {
			return nil
		}
		return err
	}
	return nil
}

func (t *TelegramChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	if t.bot == nil {
		return fmt.Errorf("telegram bot not initialized")
	}
	chatID, err := strconv.ParseInt(msg.ChatID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid chat id %q: %w", msg.ChatID, err)
	}
	if placeholderID, ok := msg.Metadata["placeholder_id"]; ok {
		if pid, ok := placeholderID.(int); ok && pid != 0 {
			content := telegram.ToTelegramHTML(msg.Content)
			if err := t.editMessage(chatID, pid, content, telego.ModeHTML); err != nil {
				t.log.Printf("[telegram] edit placeholder failed: %v", err)
			} else {
				return nil
			}
		}
	}
	content := telegram.ToTelegramHTML(msg.Content)
	const maxLen = 4000
	for len(content) > 0 {
		chunk := content
		if len(chunk) > maxLen {
			idx := strings.LastIndex(chunk[:maxLen], "\n")
			if idx > 0 {
				chunk = chunk[:idx]
			} else {
				chunk = chunk[:maxLen]
			}
		}
		content = content[len(chunk):]
		tgMsg := tu.Message(tu.ID(chatID), chunk).WithParseMode(telego.ModeHTML)
		if _, err := t.bot.SendMessage(ctx, tgMsg); err != nil {
			plain := tu.Message(tu.ID(chatID), msg.Content)
			if _, err2 := t.bot.SendMessage(ctx, plain); err2 != nil {
				return fmt.Errorf("send telegram message: %w", err2)
			}
			return nil
		}
	}
	return nil
}

type streamMsg struct {
	id       int
	dirty    bool
	lastEdit time.Time
}

func (t *TelegramChannel) SendStream(ctx context.Context, chatID string, metadata map[string]any, events <-chan api.StreamEvent) error {
	if t.bot == nil {
		return fmt.Errorf("telegram bot not initialized")
	}
	numChatID, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid chat id %q: %w", chatID, err)
	}
	if !t.streaming {
		var sb strings.Builder
		for event := range events {
			if event.Type == api.EventContentBlockDelta && event.Delta != nil {
				sb.WriteString(event.Delta.Text)
			}
		}
		result := sb.String()
		if result == "" {
			return nil
		}
		return t.Send(ctx, bus.OutboundMessage{ChatID: chatID, Content: result, Metadata: metadata})
	}
	var statusMsg streamMsg
	var contentMsg streamMsg
	var textBuf strings.Builder
	var streamErr string
	var cooldownUntil time.Time
	useDraftStreaming := numChatID > 0
	const (
		statusMinGap         = 500 * time.Millisecond
		contentMinGap        = 1 * time.Second
		contentDraftMinGap   = 400 * time.Millisecond
		statusHeartbeatDelay = 5 * time.Second
	)
	card := telegram.NewStatusCard()
	showCard := t.feedback == "debug" || t.feedback == "normal"
	showCursor := t.feedback != "silent"
	var pendingToolInput map[string][]byte
	var blockToolID string

	setCooldown := func(now time.Time, err error) bool {
		delay, ok := telegramRetryAfter(err)
		if !ok {
			return false
		}
		until := now.Add(delay)
		if until.After(cooldownUntil) {
			cooldownUntil = until
		}
		return true
	}
	inCooldown := func(now time.Time) bool {
		return !cooldownUntil.IsZero() && now.Before(cooldownUntil)
	}
	waitCooldown := func() error {
		if cooldownUntil.IsZero() {
			return nil
		}
		delay := time.Until(cooldownUntil)
		if delay <= 0 {
			return nil
		}
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		}
	}

	upsertMessage := func(msg *streamMsg, text, parseMode string, silent bool, now time.Time) bool {
		if text == "" {
			return false
		}
		if inCooldown(now) {
			msg.dirty = true
			return false
		}
		if msg.id == 0 {
			pid, err := t.sendPlaceholder(numChatID, text, parseMode, silent)
			if err != nil {
				setCooldown(now, err)
				t.log.Printf("[telegram] stream placeholder failed: %v", err)
				msg.dirty = true
				return false
			}
			msg.id = pid
		} else {
			if err := t.editMessage(numChatID, msg.id, text, parseMode); err != nil {
				setCooldown(now, err)
				t.log.Printf("[telegram] stream edit failed: %v", err)
				msg.dirty = true
				return false
			}
		}
		msg.lastEdit = now
		msg.dirty = false
		return true
	}

	renderContent := func() string {
		text := textBuf.String()
		if text == "" {
			return ""
		}
		if showCursor {
			text += "▍"
		}
		return telegram.ToTelegramHTML(text)
	}

	contentFlushGap := func() time.Duration {
		if useDraftStreaming {
			return contentDraftMinGap
		}
		return contentMinGap
	}

	flushContent := func(now time.Time, force bool) {
		if !contentMsg.dirty {
			return
		}
		if !force {
			gap := contentFlushGap()
			if !contentMsg.lastEdit.IsZero() && now.Sub(contentMsg.lastEdit) < gap {
				return
			}
		}
		text := renderContent()
		if text == "" {
			contentMsg.dirty = false
			return
		}
		if inCooldown(now) {
			contentMsg.dirty = true
			return
		}
		if useDraftStreaming {
			draftText := truncateForTelegramDraftText(text)
			params := (&telego.SendMessageDraftParams{}).
				WithChatID(numChatID).
				WithDraftID(telegramStreamContentDraftID).
				WithText(draftText).
				WithParseMode(telego.ModeHTML)
			if err := t.bot.SendMessageDraft(ctx, params); err != nil {
				setCooldown(now, err)
				t.log.Printf("[telegram] sendMessageDraft failed: %v", err)
				contentMsg.dirty = true
				return
			}
		} else {
			if !upsertMessage(&contentMsg, text, telego.ModeHTML, true, now) {
				return
			}
			return
		}
		contentMsg.lastEdit = now
		contentMsg.dirty = false
	}

	tryUpdateStatus := func(now time.Time) {
		if !showCard {
			return
		}
		if inCooldown(now) {
			statusMsg.dirty = true
			return
		}
		if !statusMsg.lastEdit.IsZero() && now.Sub(statusMsg.lastEdit) < statusMinGap {
			statusMsg.dirty = true
			return
		}
		upsertMessage(&statusMsg, card.Render(), telego.ModeHTML, true, now)
	}

	tickFlush := func(now time.Time, forceContent bool) {
		if inCooldown(now) {
			return
		}
		if showCard && statusMsg.dirty && (statusMsg.lastEdit.IsZero() || now.Sub(statusMsg.lastEdit) >= statusMinGap) {
			upsertMessage(&statusMsg, card.Render(), telego.ModeHTML, true, now)
		}
		flushContent(now, forceContent)
		if showCard && statusMsg.id != 0 && !statusMsg.dirty && (statusMsg.lastEdit.IsZero() || now.Sub(statusMsg.lastEdit) >= statusHeartbeatDelay) {
			upsertMessage(&statusMsg, card.Render(), telego.ModeHTML, true, now)
		}
	}

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for events != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			tickFlush(time.Now(), false)
		case event, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			if t.feedback == "debug" && event.Type != api.EventContentBlockDelta && event.Type != api.EventContentBlockStop && event.Type != api.EventPing {
				t.log.Printf("[telegram] stream event: type=%s name=%s", event.Type, event.Name)
			}
			switch event.Type {
			case api.EventIterationStart:
				if event.Iteration != nil {
					card.SetIteration(*event.Iteration + 1)
				}
				if textBuf.Len() > 0 {
					textBuf.Reset()
				}
				tryUpdateStatus(time.Now())

			case api.EventContentBlockStart:
				if event.ContentBlock != nil && event.ContentBlock.Type == "tool_use" {
					blockToolID = event.ContentBlock.ID
				} else {
					blockToolID = ""
				}

			case api.EventContentBlockStop:
				blockToolID = ""

			case api.EventContentBlockDelta:
				if event.Delta == nil {
					continue
				}
				if event.Delta.Type == "input_json_delta" && blockToolID != "" {
					if pendingToolInput == nil {
						pendingToolInput = make(map[string][]byte)
					}
					var chunk string
					if json.Unmarshal(event.Delta.PartialJSON, &chunk) == nil {
						pendingToolInput[blockToolID] = append(pendingToolInput[blockToolID], []byte(chunk)...)
					}
					continue
				}
				if event.Delta.Text != "" {
					textBuf.WriteString(event.Delta.Text)
					contentMsg.dirty = true
					if useDraftStreaming {
						flushContent(time.Now(), false)
					}
				}

			case api.EventToolExecutionStart:
				var summary string
				if pendingToolInput != nil {
					if raw, ok := pendingToolInput[event.ToolUseID]; ok {
						summary = telegram.SummarizeToolInput(event.Name, json.RawMessage(raw))
						delete(pendingToolInput, event.ToolUseID)
					}
				}
				card.AddTool(event.ToolUseID, event.Name, summary)
				tryUpdateStatus(time.Now())

			case api.EventToolExecutionResult:
				failed := false
				if event.IsError != nil && *event.IsError {
					failed = true
				}
				card.FinishTool(event.ToolUseID, failed)
				tryUpdateStatus(time.Now())

			case api.EventError:
				streamErr = strings.TrimSpace(fmt.Sprintf("%v", event.Output))
				t.log.Printf("[telegram] stream error: %s", streamErr)
				tryUpdateStatus(time.Now())
			}
		}
	}
	drainNow := time.Now()
	tickFlush(drainNow, true)

	finalText := textBuf.String()
	if finalText == "" {
		if streamErr != "" {
			finalText = "stream failed: " + streamErr
		} else {
			finalText = "agent return null"
		}
	}

	finalMsg := bus.OutboundMessage{ChatID: chatID, Content: finalText, Metadata: metadata}
	if err := waitCooldown(); err != nil {
		return err
	}
	if err := t.Send(ctx, finalMsg); err != nil {
		if setCooldown(time.Now(), err) {
			if err := waitCooldown(); err != nil {
				return err
			}
			if err := t.Send(ctx, finalMsg); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	if statusMsg.id != 0 {
		if err := t.deleteMessage(numChatID, statusMsg.id); err != nil {
			t.log.Printf("[telegram] delete status message failed: %v", err)
		}
	}
	if contentMsg.id != 0 {
		if err := t.deleteMessage(numChatID, contentMsg.id); err != nil {
			t.log.Printf("[telegram] delete content message failed: %v", err)
		}
	}

	return nil
}
