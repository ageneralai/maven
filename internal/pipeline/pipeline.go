package pipeline

import (
	"context"
	"strconv"
	"sync"

	"github.com/ageneralai/maven/internal/agent"
	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/channel"
	"github.com/ageneralai/maven/internal/inboundctx"
	mavenlog "github.com/ageneralai/maven/internal/log"
	"github.com/ageneralai/maven/internal/slash"
	"github.com/ageneralai/maven/internal/stringutil"
)

const userErrMessage = "Sorry, I encountered an error processing your message."
const userErrCommand = "Sorry, I encountered an error processing your command."

type Pipeline struct {
	Log           mavenlog.PrintLogger
	Bus           *bus.MessageBus
	Channels      *channel.ChannelManager
	SlashRegistry *slash.Registry
	rtMu          sync.RWMutex
	Runtime       agent.Runtime
	Sessions      *agent.SessionResolver
	Posts         *agent.PostActionHandler
}

// New builds a pipeline with required dependencies. Optional fields (e.g. Channels)
// are left zero until wired by the gateway.
func New(log mavenlog.PrintLogger, b *bus.MessageBus, rt agent.Runtime, sessions *agent.SessionResolver, posts *agent.PostActionHandler) *Pipeline {
	return &Pipeline{Log: log, Bus: b, Runtime: rt, Sessions: sessions, Posts: posts}
}

// SetRuntime swaps the agent runtime used for subsequent turns (e.g. gateway reload).
func (p *Pipeline) SetRuntime(rt agent.Runtime) {
	p.rtMu.Lock()
	defer p.rtMu.Unlock()
	p.Runtime = rt
}

// CurrentRuntime returns the runtime used for agent turns. The pipeline owns this value;
// gateway reload swaps it with SetRuntime.
func (p *Pipeline) CurrentRuntime() agent.Runtime {
	p.rtMu.RLock()
	defer p.rtMu.RUnlock()
	return p.Runtime
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
		case msg := <-p.Bus.Inbound:
			p.handle(ctx, msg)
		case <-ctx.Done():
			return
		}
	}
}

// sendError logs err and delivers userMsg to the chat surface (single path for inbound failures).
func (p *Pipeline) sendError(chName, chatID, userMsg string, err error) {
	p.Log.Printf("[pipeline] %s/%s error: %v", chName, chatID, err)
	p.Bus.Outbound <- bus.OutboundMessage{
		Channel: chName,
		ChatID:  chatID,
		Content: userMsg,
	}
}

func (p *Pipeline) handle(ctx context.Context, msg bus.InboundMessage) {
	p.Log.Printf("[pipeline] inbound from %s/%s: %s", msg.Channel, msg.SenderID, stringutil.Truncate(msg.Content, 80))
	if handled, err := p.Posts.HandleBuiltin(msg); handled {
		if err != nil {
			p.sendError(msg.Channel, msg.ChatID, userErrCommand, err)
		} else {
			p.Bus.Outbound <- bus.OutboundMessage{
				Channel:  msg.Channel,
				ChatID:   msg.ChatID,
				Content:  "✅ Started a fresh session.",
				Metadata: cloneTransportMeta(msg.TransportMeta),
			}
		}
		return
	}
	msgCtx := inboundctx.With(ctx, msg.Channel, msg.ChatID)
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
	slashOut, err := slash.PreTurn(msgCtx, p.SlashRegistry, slash.Input{
		Text:              msg.Content,
		ExpectedSlashName: msg.Hints.SlashCommand,
	})
	if err != nil {
		p.sendError(msg.Channel, msg.ChatID, userErrCommand, err)
		return
	}
	if !slashOut.ContinueToModel {
		p.Bus.Outbound <- bus.OutboundMessage{
			Channel:  msg.Channel,
			ChatID:   msg.ChatID,
			Content:  slashOut.DirectReply,
			Metadata: cloneTransportMeta(msg.TransportMeta),
		}
		return
	}
	rt := p.CurrentRuntime()
	if ch != nil && !msg.Hints.ForceSync {
		if sc, ok := ch.(channel.StreamChannel); ok {
			events, err := agent.RunStreamWithMetadata(msgCtx, rt, msg.Content, sessionKey, msg.ContentBlocks, slashOut.RequestMetadata)
			if err != nil {
				p.sendError(msg.Channel, msg.ChatID, userErrMessage, err)
				return
			}
			meta := cloneTransportMeta(msg.TransportMeta)
			if err := sc.SendStream(ctx, msg.ChatID, meta, events); err != nil {
				p.sendError(msg.Channel, msg.ChatID, userErrMessage, err)
				return
			}
			return
		}
	}
	resp, err := agent.RunResponseWithMetadata(msgCtx, rt, msg.Content, sessionKey, msg.ContentBlocks, slashOut.RequestMetadata)
	if err != nil {
		p.sendError(msg.Channel, msg.ChatID, userErrMessage, err)
		return
	}
	result := ""
	if resp != nil && resp.Result != nil {
		result = resp.Result.Output
	}
	if postResult, handled, postErr := p.Posts.HandlePostResponse(msg.StableRouteKey(), resp, slashOut.Trail); handled || postErr != nil {
		if postErr != nil {
			p.sendError(msg.Channel, msg.ChatID, userErrCommand, postErr)
			return
		}
		result = postResult
	}
	if result != "" {
		p.Bus.Outbound <- bus.OutboundMessage{
			Channel:  msg.Channel,
			ChatID:   msg.ChatID,
			Content:  result,
			Metadata: cloneTransportMeta(msg.TransportMeta),
		}
	}
}
