package telegram

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/maven/internal/bus"
	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoapi"
	tu "github.com/mymmrac/telego/telegoutil"
)

// telegramStreamContentDraftID is the Bot API draft slot id; only one draft per chat, always slot 1.
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

const telegramStreamDraftMaxRunes = 4096

func (t *TelegramChannel) sendPlaceholder(ctx context.Context, chatID int64, text, parseMode string, silent bool) (int, error) {
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
	sent, err := t.bot.SendMessage(ctx, msg)
	if err != nil {
		return 0, err
	}
	return sent.MessageID, nil
}

func (t *TelegramChannel) deleteMessage(ctx context.Context, chatID int64, messageID int) error {
	if t.bot == nil {
		return fmt.Errorf("telegram bot not initialized")
	}
	return t.bot.DeleteMessage(ctx, &telego.DeleteMessageParams{
		ChatID:    tu.ID(chatID),
		MessageID: messageID,
	})
}

func (t *TelegramChannel) editMessage(ctx context.Context, chatID int64, messageID int, text string, parseMode string) error {
	if t.bot == nil {
		return fmt.Errorf("telegram bot not initialized")
	}
	edit := tu.EditMessageText(tu.ID(chatID), messageID, text)
	if parseMode != "" {
		edit = edit.WithParseMode(parseMode)
	}
	_, err := t.bot.EditMessageText(ctx, edit)
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
			content := ToTelegramHTML(msg.Content)
			if err := t.editMessage(ctx, chatID, pid, content, telego.ModeHTML); err != nil {
				t.log.Error("telegram edit placeholder failed", "err", err)
			} else {
				return nil
			}
		}
	}
	content := ToTelegramHTML(msg.Content)
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
	st := newStreamState(ctx, t, chatID, numChatID, metadata)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for events != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			st.tickFlush(time.Now(), false)
		case event, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			st.handleEvent(event)
		}
	}
	st.tickFlush(time.Now(), true)
	return st.finalizeSend(ctx)
}
