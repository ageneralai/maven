package pipeline

import (
	"context"
	"strconv"

	"github.com/ageneralai/maven/internal/agent"
	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/channel"
	"github.com/ageneralai/maven/internal/inboundctx"
	mavenlog "github.com/ageneralai/maven/internal/log"
)

const userErrMessage = "Sorry, I encountered an error processing your message."
const userErrCommand = "Sorry, I encountered an error processing your command."

type Pipeline struct {
	Log      mavenlog.PrintLogger
	Bus      *bus.MessageBus
	Channels *channel.ChannelManager
	Runtime  agent.Runtime
	Sessions *agent.SessionResolver
	Posts    *agent.PostActionHandler
}

// New builds a pipeline with required dependencies. Optional fields (e.g. Channels)
// are left zero until wired by the gateway.
func New(log mavenlog.PrintLogger, b *bus.MessageBus, rt agent.Runtime, sessions *agent.SessionResolver, posts *agent.PostActionHandler) *Pipeline {
	return &Pipeline{Log: log, Bus: b, Runtime: rt, Sessions: sessions, Posts: posts}
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
	p.Log.Printf("[pipeline] inbound from %s/%s: %s", msg.Channel, msg.SenderID, truncate(msg.Content, 80))
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
	if ch != nil && !msg.Hints.ForceSync {
		if sc, ok := ch.(channel.StreamChannel); ok {
			events, err := agent.RunStream(msgCtx, p.Runtime, msg.Content, sessionKey, msg.ContentBlocks)
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
	resp, err := agent.RunResponse(msgCtx, p.Runtime, msg.Content, sessionKey, msg.ContentBlocks)
	if err != nil {
		p.sendError(msg.Channel, msg.ChatID, userErrMessage, err)
		return
	}
	result := ""
	if resp != nil && resp.Result != nil {
		result = resp.Result.Output
	}
	if postResult, handled, postErr := p.Posts.HandlePostResponse(msg.StableRouteKey(), resp); handled || postErr != nil {
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
