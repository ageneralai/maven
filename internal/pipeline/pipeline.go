package pipeline

import (
	"context"
	"errors"
	"strconv"
	"sync"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/ageneral-agents-go/pkg/model"
	"github.com/ageneralai/maven/internal/agent"
	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/channel"
	"github.com/ageneralai/maven/internal/channel/manager"
	turnctx "github.com/ageneralai/maven/pkg/context"
	"github.com/ageneralai/maven/pkg/events"
	"github.com/ageneralai/maven/pkg/executor"
	"log/slog"

	"github.com/ageneralai/maven/internal/session"
	"github.com/ageneralai/maven/internal/slash"
	"github.com/ageneralai/maven/pkg/stringutil"
)

const userErrMessage = "Sorry, I encountered an error processing your message."
const userErrCommand = "Sorry, I encountered an error processing your command."

type errPostActionHandle struct {
	err error
}

func (e errPostActionHandle) Error() string {
	return e.err.Error()
}

func (e errPostActionHandle) Unwrap() error {
	return e.err
}

// Pipeline runs the inbound loop and owns the agent runtime pointer. turnMu implements
// drain-safe reload: each handle and each automation RunText holds RLock for the full
// turn; Reload drains under Lock for the pointer swap only; applyChannels runs outside
// the lock so channel I/O does not stall inbound.
type Pipeline struct {
	Log           *slog.Logger
	Bus           *bus.MessageBus
	Channels      *manager.ChannelManager
	SlashRegistry *slash.Registry
	Sessions      session.Resolver
	Posts         *agent.PostActionHandler
	turnMu sync.RWMutex
	rt             agent.Runtime
}

// New builds a pipeline. rt may be nil only in tests that never run handles or RunText.
func New(log *slog.Logger, b *bus.MessageBus, rt agent.Runtime, sessions session.Resolver, posts *agent.PostActionHandler) *Pipeline {
	return &Pipeline{Log: log, Bus: b, rt: rt, Sessions: sessions, Posts: posts}
}

// CurrentRuntime returns rt without holding the turn lock. Use only when no concurrent
// handle/RunText is possible (e.g. tests), or for inspection; Shutdown uses TakeRuntimeForShutdown.
func (p *Pipeline) CurrentRuntime() agent.Runtime {
	p.turnMu.RLock()
	defer p.turnMu.RUnlock()
	return p.rt
}

// RunText runs one unattended agent turn (cron, heartbeat) while holding the same turn
// lock as inbound, so reload cannot Close the runtime mid-call.
func (p *Pipeline) RunText(ctx context.Context, prompt, sessionID string, contentBlocks []model.ContentBlock) (string, error) {
	p.turnMu.RLock()
	defer p.turnMu.RUnlock()
	rt := p.rt
	return agent.RunText(ctx, rt, prompt, sessionID, contentBlocks)
}

// RunTurn implements executor.TurnExecutor.
func (p *Pipeline) RunTurn(ctx context.Context, prompt, sessionID string) (string, error) {
	return p.RunText(ctx, prompt, sessionID, nil)
}

// RunStream runs a streaming agent turn while holding the turn lock, so reload cannot
// Close the runtime mid-call. Safe to call concurrently with RunText and inbound pipeline.
func (p *Pipeline) RunStream(ctx context.Context, prompt, sessionID string) (<-chan api.StreamEvent, error) {
	p.turnMu.RLock()
	defer p.turnMu.RUnlock()
	rt := p.rt
	return agent.RunStream(ctx, rt, prompt, sessionID, nil)
}

var _ executor.TurnExecutor = (*Pipeline)(nil)

// Reload runs applyChannels first (no lock; channels do not touch rt). Then it takes
// the write lock, swaps rt and workspace under exclusion, unlocks, and closes the old
// runtime. Gateway closes newRt only when Reload returns an error from applyChannels.
func (p *Pipeline) Reload(applyChannels func() error, newRt agent.Runtime, workspace string) error {
	if err := applyChannels(); err != nil {
		return err
	}
	p.turnMu.Lock()
	old := p.rt
	p.rt = newRt
	if p.Posts != nil {
		p.Posts.Workspace = workspace
	}
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

func cloneTransportMeta(meta map[string]any) map[string]any {
	if len(meta) == 0 {
		return nil
	}
	out := make(map[string]any, len(meta))
	for k, v := range meta {
		out[k] = v
	}
	return out
}

func (p *Pipeline) Run(ctx context.Context) {
	for {
		select {
		case msg, ok := <-p.Bus.InboundChan():
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
	p.Log.Error("pipeline turn error", "channel", chName, "chat_id", chatID, "err", err)
	// TODO(mvp): add dead-letter or delivery-failure counters before external launch; callers cannot observe PublishOutbound failures.
	pubErr := p.Bus.PublishOutbound(ctx, bus.OutboundMessage{
		Channel: chName,
		ChatID:  chatID,
		Content: userMsg,
	})
	if pubErr != nil {
		p.Log.Error("pipeline error reply publish failed", "channel", chName, "chat_id", chatID, "err", pubErr)
	}
}

func (p *Pipeline) turnContext(ctx context.Context, msg bus.InboundMessage) context.Context {
	msgCtx := turnctx.WithInbound(ctx, msg.Channel, msg.ChatID)
	if msg.Hints.MessageID != 0 {
		msgCtx = turnctx.WithMetadata(msgCtx, map[string]any{
			"message_id": msg.Hints.MessageID,
		})
	}
	return msgCtx
}

func (p *Pipeline) handleBuiltin(ctx context.Context, msg bus.InboundMessage) bool {
	handled, err := p.Posts.HandleBuiltin(msg)
	if !handled {
		return false
	}
	if err != nil {
		p.sendError(ctx, msg.Channel, msg.ChatID, userErrCommand, err)
		return true
	}
	if err := p.Bus.PublishOutbound(ctx, bus.OutboundMessage{
		Channel:  msg.Channel,
		ChatID:   msg.ChatID,
		Content:  "✅ Started a fresh session.",
		Metadata: cloneTransportMeta(msg.TransportMeta),
	}); err != nil {
		p.Log.Error("pipeline publish session reset reply", "channel", msg.Channel, "err", err)
	}
	return true
}

func (p *Pipeline) runSlash(ctx context.Context, msg bus.InboundMessage) (slash.Outcome, error) {
	msgCtx := p.turnContext(ctx, msg)
	return slash.PreTurn(msgCtx, p.SlashRegistry, slash.Input{
		Text:              msg.Content,
		ExpectedSlashName: msg.Hints.SlashCommand,
	})
}

func (p *Pipeline) runStream(ctx context.Context, rt agent.Runtime, msg bus.InboundMessage, sessionKey string, meta map[string]any, ch channel.StreamChannel) error {
	msgCtx := p.turnContext(ctx, msg)
	streamHints := bus.StreamHints{Channel: msg.Channel, ChatID: msg.ChatID}
	streamCtx := p.Bus.OnStreamBegin(msgCtx, streamHints)
	streamEvents, err := agent.RunStreamWithMetadata(streamCtx, rt, msg.Content, sessionKey, msg.ContentBlocks, meta)
	if err != nil {
		p.Bus.OnStreamEnd(streamCtx, streamHints, err)
		return err
	}
	sendMeta := cloneTransportMeta(msg.TransportMeta)
	sendErr := ch.SendStream(streamCtx, msg.ChatID, sendMeta, streamEvents)
	p.Bus.OnStreamEnd(streamCtx, streamHints, sendErr)
	return sendErr
}

func (p *Pipeline) runSync(ctx context.Context, rt agent.Runtime, msg bus.InboundMessage, sessionKey string, meta map[string]any, slashOut slash.Outcome) error {
	msgCtx := p.turnContext(ctx, msg)
	resp, err := agent.RunResponseWithMetadata(msgCtx, rt, msg.Content, sessionKey, msg.ContentBlocks, meta)
	if err != nil {
		return err
	}
	result := ""
	if resp != nil && resp.Result != nil {
		result = resp.Result.Output
	}
	if postResult, handled, postErr := p.Posts.HandlePostResponse(msgCtx, msg.StableRouteKey(), resp, slashOut.Trail); handled || postErr != nil {
		if postErr != nil {
			return errPostActionHandle{postErr}
		}
		result = postResult
	}
	if result != "" {
		if err := p.Bus.PublishOutbound(ctx, bus.OutboundMessage{
			Channel:  msg.Channel,
			ChatID:   msg.ChatID,
			Content:  result,
			Metadata: cloneTransportMeta(msg.TransportMeta),
		}); err != nil {
			p.Log.Error("pipeline publish sync reply", "channel", msg.Channel, "err", err)
		}
	}
	return nil
}

func (p *Pipeline) handle(ctx context.Context, msg bus.InboundMessage) {
	p.Log.Debug("pipeline inbound", "channel", msg.Channel, "sender", msg.SenderID, "content", stringutil.Truncate(msg.Content, 80))
	if p.handleBuiltin(ctx, msg) {
		return
	}
	p.turnMu.RLock()
	defer p.turnMu.RUnlock()
	rt := p.rt
	events.Publish(ctx, events.Event{
		Type: "pipeline.turn_start",
		Attrs: map[string]string{
			"channel": msg.Channel,
			"chat_id": msg.ChatID,
		},
	})
	sessionKey := p.Sessions.ResolveSDKSessionID(msg)
	var ch channel.Channel
	if p.Channels != nil {
		ch = p.Channels.GetChannel(msg.Channel)
	}
	if ch != nil {
		if ip, ok := ch.(channel.InboundPreprocessor); ok {
			if chatInt, err := strconv.ParseInt(msg.ChatID, 10, 64); err == nil {
				ip.PreProcessInbound(ctx, chatInt, msg.Hints)
			}
		}
	}
	slashOut, err := p.runSlash(ctx, msg)
	if err != nil {
		p.sendError(ctx, msg.Channel, msg.ChatID, userErrCommand, err)
		return
	}
	if !slashOut.ContinueToModel {
		if err := p.Bus.PublishOutbound(ctx, bus.OutboundMessage{
			Channel:  msg.Channel,
			ChatID:   msg.ChatID,
			Content:  slashOut.DirectReply,
			Metadata: cloneTransportMeta(msg.TransportMeta),
		}); err != nil {
			p.Log.Error("pipeline publish slash reply", "channel", msg.Channel, "err", err)
		}
		return
	}
	if ch != nil && !msg.Hints.ForceSync {
		if sc, ok := ch.(channel.StreamChannel); ok {
			if err := p.runStream(ctx, rt, msg, sessionKey, slashOut.RequestMetadata, sc); err != nil {
				if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
					p.sendError(ctx, msg.Channel, msg.ChatID, userErrMessage, err)
				}
				return
			}
			return
		}
	}
	if err := p.runSync(ctx, rt, msg, sessionKey, slashOut.RequestMetadata, slashOut); err != nil {
		var ep errPostActionHandle
		if errors.As(err, &ep) {
			p.sendError(ctx, msg.Channel, msg.ChatID, userErrCommand, ep.err)
			return
		}
		p.sendError(ctx, msg.Channel, msg.ChatID, userErrMessage, err)
		return
	}
}
