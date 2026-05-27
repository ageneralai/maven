package bus

import (
	"errors"
	"strings"
	"time"

	"github.com/ageneralai/ageneral-agents-go/pkg/model"
)

var (
	ErrInvalidInbound  = errors.New("bus: inbound message has empty channel")
	ErrInvalidOutbound = errors.New("bus: outbound message has empty channel")
)

func normalizeInboundMessage(m InboundMessage) (InboundMessage, error) {
	m.Channel = strings.TrimSpace(m.Channel)
	m.ChatID = strings.TrimSpace(m.ChatID)
	m.SenderID = strings.TrimSpace(m.SenderID)
	if m.Channel == "" {
		return InboundMessage{}, ErrInvalidInbound
	}
	return m, nil
}

func normalizeOutboundMessage(m OutboundMessage) (OutboundMessage, error) {
	m.Channel = strings.TrimSpace(m.Channel)
	m.ChatID = strings.TrimSpace(m.ChatID)
	m.ReplyTo = strings.TrimSpace(m.ReplyTo)
	if m.Channel == "" {
		return OutboundMessage{}, ErrInvalidOutbound
	}
	return m, nil
}

type InboundMessage struct {
	Channel       string
	SenderID      string
	ChatID        string
	Content       string
	Timestamp     time.Time
	Media         []string
	Hints         RoutingHints
	TransportMeta map[string]any
	ContentBlocks []model.ContentBlock // Multimodal content (images, documents, etc.)
}

// StableRouteKey is the persistent conversation key (channel + chat) for session
// router rotation and post-actions. It is not the agentsdk SessionID.
func (m *InboundMessage) StableRouteKey() string {
	return m.Channel + ":" + m.ChatID
}

// OutboundMessage is queued for MessageBus.DispatchOutbound. Enqueue uses the same
// strict blocking contract as inbound (see package bus). Delivery to chat is
// best-effort: subscribers invoke channel Send and log failures; there is no
// ack or retry path on the bus today. Evolving this should use a structured
// outbound result (errors, dead-letter, or user-visible fallback) instead of
// growing ad-hoc logging alone.
type OutboundMessage struct {
	Channel       string
	ChatID        string
	Content       string
	ReplyTo       string
	Media         []string
	Metadata      map[string]any
	ContentBlocks []model.ContentBlock // Multimodal content.
}
