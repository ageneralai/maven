package bus

import (
	"time"

	"github.com/cexll/agentsdk-go/pkg/model"
)

type InboundMessage struct {
	Channel       string
	SenderID      string
	ChatID        string
	Content       string
	Timestamp     time.Time
	Media         []string
	Hints         RoutingHints
	TransportMeta map[string]any
	ContentBlocks []model.ContentBlock // 多模态内容（图片、文档等）
}

// StableRouteKey is the persistent conversation key (channel + chat) for session
// router rotation and post-actions. It is not the agentsdk SessionID.
func (m *InboundMessage) StableRouteKey() string {
	return m.Channel + ":" + m.ChatID
}

type OutboundMessage struct {
	Channel       string
	ChatID        string
	Content       string
	ReplyTo       string
	Media         []string
	Metadata      map[string]any
	ContentBlocks []model.ContentBlock // 多模态内容
}
