package matrix

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ageneralai/maven/internal/bus"
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
		m.Log.Printf("[matrix] rejected room %s", roomID)
		return
	}
	senderID := evt.Sender.String()
	if !m.IsAllowed(senderID) {
		m.Log.Printf("[matrix] rejected message from %s", senderID)
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
	_ = m.Bus.PublishInbound(ctx, bus.InboundMessage{
		Channel:  matrixChannelName,
		SenderID: senderID,
		ChatID:   roomID,
		Content:  body,
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
		m.Log.Printf("[matrix] join room %s after invite: %v", evt.RoomID, err)
		return
	}
	m.Log.Printf("[matrix] joined room %s (invited by %s)", evt.RoomID, evt.Sender)
}

func (m *MatrixChannel) isRoomAllowed(roomID string) bool {
	if len(m.allowRooms) == 0 {
		return true
	}
	return m.allowRooms[roomID]
}

func chunkText(text string, maxLen int) []string {
	if maxLen <= 0 || len(text) <= maxLen {
		return []string{text}
	}
	var chunks []string
	for len(text) > maxLen {
		split := strings.LastIndex(text[:maxLen], "\n")
		if split <= 0 {
			split = runeAlignedSplit(text, maxLen)
		}
		chunks = append(chunks, text[:split])
		text = strings.TrimLeft(text[split:], "\n")
	}
	if text != "" {
		chunks = append(chunks, text)
	}
	return chunks
}

// runeAlignedSplit returns the largest byte offset <= maxLen that is a valid
// UTF-8 rune boundary, avoiding splits mid-codepoint.
func runeAlignedSplit(s string, maxLen int) int {
	if maxLen >= len(s) {
		return len(s)
	}
	for i := maxLen; i > 0; i-- {
		if utf8.RuneStart(s[i]) {
			return i
		}
	}
	return maxLen
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
