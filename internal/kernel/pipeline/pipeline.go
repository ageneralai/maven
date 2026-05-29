package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/maven/internal/kernel/agent"
	"github.com/ageneralai/maven/internal/kernel/agent/postaction"
	"github.com/ageneralai/maven/internal/kernel/bus"
	"github.com/ageneralai/maven/internal/kernel/channels"
	"github.com/ageneralai/maven/internal/kernel/channels/manager"
	"github.com/ageneralai/maven/internal/kernel/events"
	"github.com/ageneralai/maven/internal/kernel/executor"
	"github.com/ageneralai/maven/internal/kernel/health"
	"github.com/ageneralai/maven/internal/kernel/session"
	"github.com/ageneralai/maven/internal/kernel/slash"
	"github.com/ageneralai/maven/internal/kernel/stringutil"
	turnctx "github.com/ageneralai/maven/internal/kernel/turnctx"
)

const userErrMessage = "Sorry, I encountered an error processing your message."
const userErrCommand = "Sorry, I encountered an error processing your command."

var errPostAction = errors.New("post-action handler failed")

// Pipeline runs the inbound loop and owns the agent runtime pointer. turnMu implements
// drain-safe reload: each handle and each automation RunTurn holds RLock for the full
// turn; Reload drains under Lock for the pointer swap only; applyChannels runs outside
// the lock so channel I/O does not stall inbound.
type Pipeline struct {
	log           *slog.Logger
	bus           *bus.MessageBus
	channels      *manager.ChannelManager
	slashRegistry atomic.Pointer[slash.Registry]
	sessions      session.Resolver
	posts         postaction.Handler
	liveness      health.HealthReporter
	eventBus      *events.Fanout
	turnMu        sync.RWMutex
	rt            agent.Runtime
}

// New builds a pipeline. rt may be nil only in tests that never run handles or RunTurn.
func New(log *slog.Logger, b *bus.MessageBus, rt agent.Runtime, sessions session.Resolver, posts postaction.Handler, channels *manager.ChannelManager, liveness health.HealthReporter) *Pipeline {
	return &Pipeline{log: log, bus: b, rt: rt, sessions: sessions, posts: posts, channels: channels, liveness: liveness, eventBus: events.NewFanout(nil)}
}

func (p *Pipeline) SetEventBus(f *events.Fanout) {
	if f != nil {
		p.eventBus = f
	}
}

// Posts returns the postaction handler for hook registration.
func (p *Pipeline) Posts() postaction.Handler {
	return p.posts
}

// SetSlashRegistry replaces the slash command registry. Called during gateway wiring and on Apply.
func (p *Pipeline) SetSlashRegistry(r *slash.Registry) {
	p.slashRegistry.Store(r)
}

// CurrentRuntime returns rt without holding the turn lock. Use only when no concurrent
// handle/RunTurn is possible (e.g. tests), or for inspection; Shutdown uses TakeRuntimeForShutdown.
func (p *Pipeline) CurrentRuntime() agent.Runtime {
	p.turnMu.RLock()
	defer p.turnMu.RUnlock()
	return p.rt
}

// RunTurn implements executor.TurnExecutor.
func (p *Pipeline) RunTurn(ctx context.Context, prompt, sessionID string) (string, error) {
	p.turnMu.RLock()
	defer p.turnMu.RUnlock()
	rt := p.rt
	prompt, blocks := mergePromptAndBlocks(prompt, nil)
	resp, err := rt.Run(ctx, api.Request{
		Prompt:        prompt,
		ContentBlocks: blocks,
		SessionID:     sessionID,
	})
	if err != nil {
		return "", err
	}
	if resp == nil || resp.Result == nil {
		return "", nil
	}
	return resp.Result.Output, nil
}

// RunStream runs a streaming agent turn while holding the turn lock.
func (p *Pipeline) RunStream(ctx context.Context, prompt, sessionID string) (<-chan api.StreamEvent, error) {
	p.turnMu.RLock()
	defer p.turnMu.RUnlock()
	rt := p.rt
	prompt, blocks := mergePromptAndBlocks(prompt, nil)
	return rt.RunStream(ctx, api.Request{
		Prompt:        prompt,
		ContentBlocks: blocks,
		SessionID:     sessionID,
	})
}

var _ executor.TurnExecutor = (*Pipeline)(nil)

// Reload runs applyChannels first (no lock; channels do not touch rt). Then it takes
// the write lock, swaps rt and workspace under exclusion, stores slashReg, unlocks,
// and closes the old runtime. Gateway closes newRt only when Reload returns an error
// from applyChannels.
func (p *Pipeline) Reload(applyChannels func() error, newRt agent.Runtime, workspace string, slashReg *slash.Registry) error {
	if err := applyChannels(); err != nil {
		return err
	}
	p.turnMu.Lock()
	old := p.rt
	p.rt = newRt
	if p.posts != nil {
		p.posts.SetWorkspace(workspace)
	}
	p.slashRegistry.Store(slashReg)
	p.turnMu.Unlock()
	if old != nil {
		old.Close()
	}
	return nil
}

// TakeRuntimeForShutdown clears rt under the write lock. Caller must have stopped new inbound
// (e.g. cancel pipeline ctx) so nothing observes nil mid-flight except after drain.
func (p *Pipeline) TakeRuntimeForShutdown() agent.Runtime {
	p.turnMu.Lock()
	defer p.turnMu.Unlock()
	old := p.rt
	p.rt = nil
	return old
}

func (p *Pipeline) Run(ctx context.Context) {
	for {
		select {
		case msg, ok := <-p.bus.InboundChan():
			if !ok {
				return
			}
			p.handle(ctx, msg)
		case <-ctx.Done():
			return
		}
	}
}

func (p *Pipeline) sendError(ctx context.Context, chName, chatID, userMsg string, err error) {
	p.log.Error("pipeline turn error", "channel", chName, "chat_id", chatID, "err", err)
	pubErr := p.bus.PublishOutbound(ctx, bus.OutboundMessage{
		Channel: chName,
		ChatID:  chatID,
		Content: userMsg,
	})
	if pubErr != nil {
		p.log.Error("pipeline error reply publish failed", "channel", chName, "chat_id", chatID, "err", pubErr)
		attrs := map[string]string{
			"channel": chName,
			"chat_id": chatID,
			"error":   pubErr.Error(),
		}
		if err != nil {
			attrs["cause"] = err.Error()
		}
		p.eventBus.Publish(ctx, events.Event{Type: events.EventOutboundDeliveryFailed, Attrs: attrs})
		if rep := health.OrHealthReporter(p.liveness); rep != nil {
			rep.Pulse(health.SignalDeliveryFailed)
		}
	}
}

func (p *Pipeline) reportStreamFailed(ctx context.Context, chName, chatID string, err error) {
	if err == nil {
		return
	}
	p.eventBus.Publish(ctx, events.Event{
		Type: events.EventStreamFailed,
		Attrs: map[string]string{
			"channel": chName,
			"chat_id": chatID,
			"error":   err.Error(),
		},
	})
	if rep := health.OrHealthReporter(p.liveness); rep != nil {
		rep.Pulse(health.SignalDeliveryFailed)
	}
}

func (p *Pipeline) turnContext(ctx context.Context, msg bus.InboundMessage, sessionKey string) context.Context {
	msgCtx := turnctx.WithInbound(ctx, msg.Channel, msg.ChatID)
	meta := map[string]any{"session_id": sessionKey}
	if msg.Hints.MessageID != 0 {
		meta["message_id"] = msg.Hints.MessageID
	}
	return turnctx.WithMetadata(msgCtx, meta)
}

func (p *Pipeline) handleBuiltin(ctx context.Context, msg bus.InboundMessage) bool {
	handled, err := p.posts.HandleBuiltin(msg)
	if !handled {
		return false
	}
	if err != nil {
		p.sendError(ctx, msg.Channel, msg.ChatID, userErrCommand, err)
		return true
	}
	if err := p.bus.PublishOutbound(ctx, bus.OutboundMessage{
		Channel:  msg.Channel,
		ChatID:   msg.ChatID,
		Content:  "✅ Started a fresh session.",
		Metadata: maps.Clone(msg.TransportMeta),
	}); err != nil {
		p.log.Error("pipeline publish session reset reply", "channel", msg.Channel, "err", err)
	}
	return true
}

func (p *Pipeline) runSlash(ctx context.Context, msg bus.InboundMessage, sessionKey, slashName string) (slash.Outcome, error) {
	msgCtx := p.turnContext(ctx, msg, sessionKey)
	reg := p.slashRegistry.Load()
	return slash.PreTurn(msgCtx, reg, slash.Input{
		Text:              msg.Content,
		ExpectedSlashName: slashName,
	})
}

func (p *Pipeline) runStream(ctx context.Context, rt agent.Runtime, msg bus.InboundMessage, sessionKey string, meta map[string]any, ch channels.StreamChannel, plan turnPlan) error {
	collectTurn := plan.sessionMode == session.SessionModeCurrent
	msgCtx := p.turnContext(ctx, msg, sessionKey)
	streamHints := bus.StreamHints{Channel: msg.Channel, ChatID: msg.ChatID}
	streamCtx := p.bus.OnStreamBegin(msgCtx, streamHints)
	streamEvents, err := runStreamWithMetadata(streamCtx, rt, msg.Content, sessionKey, msg.ContentBlocks, meta)
	if err != nil {
		p.bus.OnStreamEnd(streamCtx, streamHints, err)
		return err
	}
	var intercepted <-chan api.StreamEvent
	var outputCollector *strings.Builder
	if collectTurn {
		var sb strings.Builder
		outputCollector = &sb
		intercepted = collectStreamOutput(streamEvents, &sb)
	} else {
		intercepted = streamEvents
	}
	sendMeta := maps.Clone(msg.TransportMeta)
	sendErr := ch.SendStream(streamCtx, msg.ChatID, sendMeta, intercepted)
	if sendErr != nil {
		sendErr = channels.WrapDeliveryFailed(sendErr)
		p.reportStreamFailed(ctx, msg.Channel, msg.ChatID, sendErr)
	}
	p.bus.OnStreamEnd(streamCtx, streamHints, sendErr)
	if sendErr == nil && outputCollector != nil {
		p.publishTurnCompleted(ctx, msg, sessionKey, outputCollector.String())
	}
	return sendErr
}

func (p *Pipeline) publishTurnCompleted(ctx context.Context, msg bus.InboundMessage, sessionKey, assistantMsg string) {
	p.eventBus.Publish(ctx, events.Event{
		Type: events.EventTurnCompleted,
		Payload: events.TurnCompleted{
			UserMsg:      msg.Content,
			AssistantMsg: assistantMsg,
			SessionID:    sessionKey,
			Channel:      msg.Channel,
			ChatID:       msg.ChatID,
			At:           time.Now(),
		},
	})
}

func collectStreamOutput(in <-chan api.StreamEvent, sb *strings.Builder) <-chan api.StreamEvent {
	out := make(chan api.StreamEvent, cap(in))
	go func() {
		defer close(out)
		for ev := range in {
			if ev.Type == api.EventContentBlockDelta && ev.Delta != nil {
				sb.WriteString(ev.Delta.Text)
			}
			out <- ev
		}
	}()
	return out
}

func (p *Pipeline) runSync(ctx context.Context, rt agent.Runtime, msg bus.InboundMessage, sessionKey string, meta map[string]any, slashOut slash.Outcome, plan turnPlan) error {
	msgCtx := p.turnContext(ctx, msg, sessionKey)
	resp, err := runResponseWithMetadata(msgCtx, rt, msg.Content, sessionKey, msg.ContentBlocks, meta)
	if err != nil {
		return err
	}
	result := ""
	if resp != nil && resp.Result != nil {
		result = resp.Result.Output
	}
	if postResult, handled, postErr := p.posts.HandlePostResponse(msgCtx, msg.StableRouteKey(), resp, slashOut.Trail); handled || postErr != nil {
		if postErr != nil {
			return fmt.Errorf("%w: %w", errPostAction, postErr)
		}
		result = postResult
	}
	if result != "" {
		if err := p.bus.PublishOutbound(ctx, bus.OutboundMessage{
			Channel:  msg.Channel,
			ChatID:   msg.ChatID,
			Content:  result,
			Metadata: maps.Clone(msg.TransportMeta),
		}); err != nil {
			p.log.Error("pipeline publish sync reply", "channel", msg.Channel, "err", err)
		}
	}
	if plan.sessionMode == session.SessionModeCurrent {
		p.publishTurnCompleted(ctx, msg, sessionKey, result)
	}
	return nil
}

func (p *Pipeline) handle(ctx context.Context, msg bus.InboundMessage) {
	p.log.Debug("pipeline inbound", "channel", msg.Channel, "sender", msg.SenderID, "content", stringutil.Truncate(msg.Content, 80))
	if p.handleBuiltin(ctx, msg) {
		return
	}
	p.turnMu.RLock()
	defer p.turnMu.RUnlock()
	rt := p.rt
	p.eventBus.Publish(ctx, events.Event{
		Type: "pipeline.turn_start",
		Attrs: map[string]string{
			"channel": msg.Channel,
			"chat_id": msg.ChatID,
		},
	})
	var ch channels.Channel
	if p.channels != nil {
		ch = p.channels.GetChannel(msg.Channel)
	}
	plan := classifyTurn(msg, ch)
	sessionKey := p.sessions.ResolveSDKSessionID(msg.Channel, msg.ChatID, msg.StableRouteKey(), plan.sessionMode)
	if ch != nil {
		if ip, ok := ch.(channels.InboundPreprocessor); ok {
			if chatInt, err := strconv.ParseInt(msg.ChatID, 10, 64); err == nil {
				ip.PreProcessInbound(ctx, chatInt, msg.Hints)
			}
		}
	}
	slashOut, err := p.runSlash(ctx, msg, sessionKey, plan.slashName)
	if err != nil {
		p.sendError(ctx, msg.Channel, msg.ChatID, userErrCommand, err)
		return
	}
	if !slashOut.ContinueToModel {
		if err := p.bus.PublishOutbound(ctx, bus.OutboundMessage{
			Channel:  msg.Channel,
			ChatID:   msg.ChatID,
			Content:  slashOut.DirectReply,
			Metadata: maps.Clone(msg.TransportMeta),
		}); err != nil {
			p.log.Error("pipeline publish slash reply", "channel", msg.Channel, "err", err)
		}
		return
	}
	if plan.useStream {
		if sc, ok := ch.(channels.StreamChannel); ok {
			if err := p.runStream(ctx, rt, msg, sessionKey, slashOut.RequestMetadata, sc, plan); err != nil {
				if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
					p.sendError(ctx, msg.Channel, msg.ChatID, userErrMessage, err)
				}
				return
			}
			return
		}
	}
	if err := p.runSync(ctx, rt, msg, sessionKey, slashOut.RequestMetadata, slashOut, plan); err != nil {
		if errors.Is(err, errPostAction) {
			p.sendError(ctx, msg.Channel, msg.ChatID, userErrCommand, err)
			return
		}
		p.sendError(ctx, msg.Channel, msg.ChatID, userErrMessage, err)
		return
	}
}
