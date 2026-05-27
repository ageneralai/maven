package matrix

import (
	"context"
	"strings"
	"time"

	"github.com/ageneralai/maven/kernel/bus"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

func (m *MatrixChannel) handleMessageEvent(ctx context.Context, evt *event.Event) {
	if evt == nil {
		return
	}
	if evt.Sender == m.userID {
		return
	}
	roomID := evt.RoomID.String()
	if !m.isRoomAllowed(roomID) {
		m.log.Debug("matrix rejected room", "room", roomID)
		return
	}
	senderID := evt.Sender.String()
	if !m.IsAllowed(senderID) {
		m.log.Debug("matrix rejected message", "sender", senderID)
		return
	}
	msg := evt.Content.AsMessage()
	if !msg.MsgType.IsText() {
		return
	}
	body := strings.TrimSpace(msg.Body)
	if body == "" {
		return
	}
	ts := time.Now().UTC()
	if evt.Timestamp > 0 {
		ts = time.UnixMilli(evt.Timestamp).UTC()
	}
	_ = m.bus.PublishInbound(ctx, bus.InboundMessage{
		Channel:   matrixChannelName,
		SenderID:  senderID,
		ChatID:    roomID,
		Content:   body,
		Timestamp: ts,
		TransportMeta: map[string]any{
			"event_id": evt.ID.String(),
			"room_id":  roomID,
			"msg_type": string(msg.MsgType),
		},
	})
}

func (m *MatrixChannel) handleMemberEvent(ctx context.Context, evt *event.Event) {
	if evt == nil || evt.GetStateKey() != m.userID.String() {
		return
	}
	member := evt.Content.AsMember()
	if member.Membership != event.MembershipInvite {
		return
	}
	if _, err := m.client.JoinRoomByID(ctx, evt.RoomID); err != nil {
		m.log.Error("matrix join room after invite", "room", evt.RoomID, "err", err)
		return
	}
	m.log.Info("matrix joined room", "room", evt.RoomID, "invited_by", evt.Sender)
}

func (m *MatrixChannel) isRoomAllowed(roomID string) bool {
	if len(m.allowRooms) == 0 {
		return true
	}
	return m.allowRooms[roomID]
}

func buildAllowRooms(allowRooms []string) map[string]bool {
	out := make(map[string]bool, len(allowRooms))
	for _, room := range allowRooms {
		room = strings.TrimSpace(room)
		if room != "" {
			out[room] = true
		}
	}
	return out
}

func parseUserID(raw string) (id.UserID, error) {
	userID := id.UserID(strings.TrimSpace(raw))
	if userID == "" {
		return "", errUserIDRequired
	}
	return userID, nil
}
