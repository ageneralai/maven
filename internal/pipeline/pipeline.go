package pipeline

import (
	"context"
	"strconv"
	"sync"

	"github.com/ageneralai/ageneral-agents-go/pkg/model"
	"github.com/ageneralai/maven/internal/agent"
	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/channel"
	turnctx "github.com/ageneralai/maven/internal/context"
	"github.com/ageneralai/maven/internal/events"
	"github.com/ageneralai/maven/internal/executor"
	"github.com/ageneralai/maven/internal/session"
	"github.com/ageneralai/maven/internal/slash"
	mavenlog "github.com/ageneralai/maven/pkg/log"
	"github.com/ageneralai/maven/pkg/stringutil"
)

const userErrMessage = "Sorry, I encountered an error processing your message."
const userErrCommand = "Sorry, I encountered an error processing your command."

// Pipeline runs the inbound loop and owns the agent runtime pointer. turnMu implements
// drain-safe reload: each handle and each automation RunText holds RLock for the full
// turn; Reload drains under Lock for the pointer swap only; applyChannels runs outside
// the lock so channel I/O does not stall inbound.
type Pipeline struct {
	Log           mavenlog.PrintLogger
	Bus           *bus.MessageBus
	Channels      *channel.ChannelManager
	SlashRegistry *slash.Registry
	Sessions      session.Resolver
	Posts         *agent.PostActionHandler
	turnMu        sync.RWMutex
	rt            agent.Runtime
}

// New builds a pipeline. rt may be nil only in tests that never run handles or RunText.
func New(log mavenlog.PrintLogger, b *bus.MessageBus, rt agent.Runtime, sessions session.Resolver, posts *agent.PostActionHandler) *Pipeline {
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
	p.Log.Printf("[pipeline] %s/%s error: %v", chName, chatID, err)
	_ = p.Bus.PublishOutbound(ctx, bus.OutboundMessage{
		Channel: chName,
		ChatID:  chatID,
		Content: userMsg,
	})
}

func (p *Pipeline) handle(ctx context.Context, msg bus.InboundMessage) {
	p.Log.Printf("[pipeline] inbound from %s/%s: %s", msg.Channel, msg.SenderID, stringutil.Truncate(msg.Content, 80))
	if handled, err := p.Posts.HandleBuiltin(msg); handled {
		if err != nil {
			p.sendError(ctx, msg.Channel, msg.ChatID, userErrCommand, err)
		} else {
			_ = p.Bus.PublishOutbound(ctx, bus.OutboundMessage{
				Channel:  msg.Channel,
				ChatID:   msg.ChatID,
				Content:  "✅ Started a fresh session.",
				Metadata: cloneTransportMeta(msg.TransportMeta),
			})
		}
		return
	}
	p.turnMu.RLock()
	defer p.turnMu.RUnlock()
	rt := p.rt
	msgCtx := turnctx.WithInbound(ctx, msg.Channel, msg.ChatID)
	if msg.Hints.MessageID != 0 {
		msgCtx = turnctx.WithMetadata(msgCtx, map[string]any{
			"message_id": msg.Hints.MessageID,
		})
	}
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
	// msgCtx holds per-turn routing and metadata for slash.PreTurn (read via turnctx.From inside slash).
	slashOut, err := slash.PreTurn(msgCtx, p.SlashRegistry, slash.Input{
		Text:              msg.Content,
		ExpectedSlashName: msg.Hints.SlashCommand,
	})
	if err != nil {
		p.sendError(ctx, msg.Channel, msg.ChatID, userErrCommand, err)
		return
	}
	if !slashOut.ContinueToModel {
		_ = p.Bus.PublishOutbound(ctx, bus.OutboundMessage{
			Channel:  msg.Channel,
			ChatID:   msg.ChatID,
			Content:  slashOut.DirectReply,
			Metadata: cloneTransportMeta(msg.TransportMeta),
		})
		return
	}
	if ch != nil && !msg.Hints.ForceSync {
		if sc, ok := ch.(channel.StreamChannel); ok {
			streamHints := bus.StreamHints{Channel: msg.Channel, ChatID: msg.ChatID}
			streamCtx := p.Bus.OnStreamBegin(msgCtx, streamHints)
			streamEvents, err := agent.RunStreamWithMetadata(streamCtx, rt, msg.Content, sessionKey, msg.ContentBlocks, slashOut.RequestMetadata)
			if err != nil {
				p.Bus.OnStreamEnd(streamCtx, streamHints, err)
				p.sendError(ctx, msg.Channel, msg.ChatID, userErrMessage, err)
				return
			}
			meta := cloneTransportMeta(msg.TransportMeta)
			sendErr := sc.SendStream(ctx, msg.ChatID, meta, streamEvents)
			p.Bus.OnStreamEnd(streamCtx, streamHints, sendErr)
			if sendErr != nil {
				p.sendError(ctx, msg.Channel, msg.ChatID, userErrMessage, sendErr)
				return
			}
			return
		}
	}
	resp, err := agent.RunResponseWithMetadata(msgCtx, rt, msg.Content, sessionKey, msg.ContentBlocks, slashOut.RequestMetadata)
	if err != nil {
		p.sendError(ctx, msg.Channel, msg.ChatID, userErrMessage, err)
		return
	}
	result := ""
	if resp != nil && resp.Result != nil {
		result = resp.Result.Output
	}
	if postResult, handled, postErr := p.Posts.HandlePostResponse(msgCtx, msg.StableRouteKey(), resp, slashOut.Trail); handled || postErr != nil {
		if postErr != nil {
			p.sendError(ctx, msg.Channel, msg.ChatID, userErrCommand, postErr)
			return
		}
		result = postResult
	}
	if result != "" {
		_ = p.Bus.PublishOutbound(ctx, bus.OutboundMessage{
			Channel:  msg.Channel,
			ChatID:   msg.ChatID,
			Content:  result,
			Metadata: cloneTransportMeta(msg.TransportMeta),
		})
	}
}
