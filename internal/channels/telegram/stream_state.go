package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/maven/internal/bus"
	"github.com/mymmrac/telego"
)

type streamState struct {
	ctx                  context.Context
	t                    *TelegramChannel
	chatID               string
	numChatID            int64
	metadata             map[string]any
	useDraft             bool
	showCard             bool
	showCursor           bool
	debugLogEvents       bool
	cooldownUntil        time.Time
	statusMsg            streamMsg
	contentMsg           streamMsg
	textBuf              strings.Builder
	streamErr            string
	card                 *StatusCard
	pendingToolInput     map[string][]byte
	blockToolID          string
	statusMinGap         time.Duration
	contentMinGap        time.Duration
	contentDraftMinGap   time.Duration
	statusHeartbeatDelay time.Duration
}

func newStreamState(ctx context.Context, t *TelegramChannel, chatID string, numChatID int64, metadata map[string]any) *streamState {
	return &streamState{
		ctx:                  ctx,
		t:                    t,
		chatID:               chatID,
		numChatID:            numChatID,
		metadata:             metadata,
		useDraft:             numChatID > 0,
		showCard:             t.feedback == "debug" || t.feedback == "normal",
		showCursor:           t.feedback != "silent",
		debugLogEvents:       t.feedback == "debug",
		card:                 NewStatusCard(),
		statusMinGap:         500 * time.Millisecond,
		contentMinGap:        time.Second,
		contentDraftMinGap:   400 * time.Millisecond,
		statusHeartbeatDelay: 5 * time.Second,
	}
}

func (s *streamState) setCooldown(now time.Time, err error) bool {
	delay, ok := telegramRetryAfter(err)
	if !ok {
		return false
	}
	until := now.Add(delay)
	if until.After(s.cooldownUntil) {
		s.cooldownUntil = until
	}
	return true
}

func (s *streamState) inCooldown(now time.Time) bool {
	return !s.cooldownUntil.IsZero() && now.Before(s.cooldownUntil)
}

func (s *streamState) waitCooldown() error {
	if s.cooldownUntil.IsZero() {
		return nil
	}
	delay := time.Until(s.cooldownUntil)
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *streamState) upsertMessage(sm *streamMsg, text, parseMode string, silent bool, now time.Time) bool {
	if text == "" {
		return false
	}
	if s.inCooldown(now) {
		sm.dirty = true
		return false
	}
	if sm.id == 0 {
		pid, err := s.t.sendPlaceholder(s.numChatID, text, parseMode, silent)
		if err != nil {
			s.setCooldown(now, err)
			s.t.Log.Printf("[telegram] stream placeholder failed: %v", err)
			sm.dirty = true
			return false
		}
		sm.id = pid
	} else {
		if err := s.t.editMessage(s.numChatID, sm.id, text, parseMode); err != nil {
			s.setCooldown(now, err)
			s.t.Log.Printf("[telegram] stream edit failed: %v", err)
			sm.dirty = true
			return false
		}
	}
	sm.lastEdit = now
	sm.dirty = false
	return true
}

func (s *streamState) renderContent() string {
	text := s.textBuf.String()
	if text == "" {
		return ""
	}
	if s.showCursor {
		text += "▍"
	}
	return ToTelegramHTML(text)
}

func (s *streamState) contentFlushGap() time.Duration {
	if s.useDraft {
		return s.contentDraftMinGap
	}
	return s.contentMinGap
}

func (s *streamState) flushContent(now time.Time, force bool) {
	if !s.contentMsg.dirty {
		return
	}
	if !force {
		gap := s.contentFlushGap()
		if !s.contentMsg.lastEdit.IsZero() && now.Sub(s.contentMsg.lastEdit) < gap {
			return
		}
	}
	text := s.renderContent()
	if text == "" {
		s.contentMsg.dirty = false
		return
	}
	if s.inCooldown(now) {
		s.contentMsg.dirty = true
		return
	}
	if s.useDraft {
		draftText := truncateForTelegramDraftText(text)
		params := (&telego.SendMessageDraftParams{}).
			WithChatID(s.numChatID).
			WithDraftID(telegramStreamContentDraftID).
			WithText(draftText).
			WithParseMode(telego.ModeHTML)
		if err := s.t.bot.SendMessageDraft(s.ctx, params); err != nil {
			s.setCooldown(now, err)
			s.t.Log.Printf("[telegram] sendMessageDraft failed: %v", err)
			s.contentMsg.dirty = true
			return
		}
	} else if !s.upsertMessage(&s.contentMsg, text, telego.ModeHTML, true, now) {
		return
	}
	s.contentMsg.lastEdit = now
	s.contentMsg.dirty = false
}

func (s *streamState) tryUpdateStatus(now time.Time) {
	if !s.showCard {
		return
	}
	if s.inCooldown(now) {
		s.statusMsg.dirty = true
		return
	}
	if !s.statusMsg.lastEdit.IsZero() && now.Sub(s.statusMsg.lastEdit) < s.statusMinGap {
		s.statusMsg.dirty = true
		return
	}
	s.upsertMessage(&s.statusMsg, s.card.Render(), telego.ModeHTML, true, now)
}

func (s *streamState) tickFlush(now time.Time, forceContent bool) {
	if s.inCooldown(now) {
		return
	}
	if s.showCard && s.statusMsg.dirty && (s.statusMsg.lastEdit.IsZero() || now.Sub(s.statusMsg.lastEdit) >= s.statusMinGap) {
		s.upsertMessage(&s.statusMsg, s.card.Render(), telego.ModeHTML, true, now)
	}
	s.flushContent(now, forceContent)
	if s.showCard && s.statusMsg.id != 0 && !s.statusMsg.dirty && (s.statusMsg.lastEdit.IsZero() || now.Sub(s.statusMsg.lastEdit) >= s.statusHeartbeatDelay) {
		s.upsertMessage(&s.statusMsg, s.card.Render(), telego.ModeHTML, true, now)
	}
}

func (s *streamState) handleEvent(event api.StreamEvent) {
	if s.debugLogEvents && event.Type != api.EventContentBlockDelta && event.Type != api.EventContentBlockStop && event.Type != api.EventPing {
		s.t.Log.Printf("[telegram] stream event: type=%s name=%s", event.Type, event.Name)
	}
	switch event.Type {
	case api.EventIterationStart:
		if event.Iteration != nil {
			s.card.SetIteration(*event.Iteration + 1)
		}
		if s.textBuf.Len() > 0 {
			s.textBuf.Reset()
		}
		s.tryUpdateStatus(time.Now())
	case api.EventContentBlockStart:
		if event.ContentBlock != nil && event.ContentBlock.Type == "tool_use" {
			s.blockToolID = event.ContentBlock.ID
		} else {
			s.blockToolID = ""
		}
	case api.EventContentBlockStop:
		s.blockToolID = ""
	case api.EventContentBlockDelta:
		if event.Delta == nil {
			return
		}
		if event.Delta.Type == "input_json_delta" && s.blockToolID != "" {
			if s.pendingToolInput == nil {
				s.pendingToolInput = make(map[string][]byte)
			}
			var chunk string
			if json.Unmarshal(event.Delta.PartialJSON, &chunk) == nil {
				s.pendingToolInput[s.blockToolID] = append(s.pendingToolInput[s.blockToolID], []byte(chunk)...)
			}
			return
		}
		if event.Delta.Text != "" {
			s.textBuf.WriteString(event.Delta.Text)
			s.contentMsg.dirty = true
			if s.useDraft {
				s.flushContent(time.Now(), false)
			}
		}
	case api.EventToolExecutionStart:
		var summary string
		if s.pendingToolInput != nil {
			if raw, ok := s.pendingToolInput[event.ToolUseID]; ok {
				summary = SummarizeToolInput(event.Name, json.RawMessage(raw))
				delete(s.pendingToolInput, event.ToolUseID)
			}
		}
		s.card.AddTool(event.ToolUseID, event.Name, summary)
		s.tryUpdateStatus(time.Now())
	case api.EventToolExecutionResult:
		failed := false
		if event.IsError != nil && *event.IsError {
			failed = true
		}
		s.card.FinishTool(event.ToolUseID, failed)
		s.tryUpdateStatus(time.Now())
	case api.EventError:
		s.streamErr = strings.TrimSpace(fmt.Sprintf("%v", event.Output))
		s.t.Log.Printf("[telegram] stream error: %s", s.streamErr)
		s.tryUpdateStatus(time.Now())
	}
}

func (s *streamState) finalizeSend(ctx context.Context) error {
	finalText := s.textBuf.String()
	if finalText == "" {
		if s.streamErr != "" {
			finalText = "stream failed: " + s.streamErr
		} else {
			finalText = "agent return null"
		}
	}
	finalMsg := bus.OutboundMessage{ChatID: s.chatID, Content: finalText, Metadata: s.metadata}
	if err := s.waitCooldown(); err != nil {
		return err
	}
	if err := s.t.Send(ctx, finalMsg); err != nil {
		if s.setCooldown(time.Now(), err) {
			if err := s.waitCooldown(); err != nil {
				return err
			}
			if err := s.t.Send(ctx, finalMsg); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	if s.statusMsg.id != 0 {
		if err := s.t.deleteMessage(s.numChatID, s.statusMsg.id); err != nil {
			s.t.Log.Printf("[telegram] delete status message failed: %v", err)
		}
	}
	if s.contentMsg.id != 0 {
		if err := s.t.deleteMessage(s.numChatID, s.contentMsg.id); err != nil {
			s.t.Log.Printf("[telegram] delete content message failed: %v", err)
		}
	}
	return nil
}
