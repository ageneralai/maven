package matrix

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/ageneralai/maven/internal/bus"
	chann "github.com/ageneralai/maven/internal/channel"
	mavenlog "github.com/ageneralai/maven/pkg/log"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type mockMatrixSender struct {
	sent      []string
	room      id.RoomID
	joinRooms []id.RoomID
}

func (m *mockMatrixSender) SendText(ctx context.Context, roomID id.RoomID, text string) (*mautrix.RespSendEvent, error) {
	m.room = roomID
	m.sent = append(m.sent, text)
	return &mautrix.RespSendEvent{}, nil
}

func (m *mockMatrixSender) JoinRoomByID(ctx context.Context, roomID id.RoomID) (*mautrix.RespJoinRoom, error) {
	m.joinRooms = append(m.joinRooms, roomID)
	return &mautrix.RespJoinRoom{}, nil
}

func (m *mockMatrixSender) SyncWithContext(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func testMatrixChannel(t *testing.T, allowFrom, allowRooms []string) (*MatrixChannel, *bus.MessageBus) {
	t.Helper()
	b := bus.New(8, mavenlog.Std())
	ch := &MatrixChannel{
		BaseChannel: chann.NewBaseChannel(matrixChannelName, b, allowFrom, mavenlog.Std()),
		userID:      "@agent:example.org",
		client:      &mockMatrixSender{},
		allowRooms:  buildAllowRooms(allowRooms),
	}
	return ch, b
}

func textEvent(sender, room, body string) *event.Event {
	return &event.Event{
		Sender:    id.UserID(sender),
		RoomID:    id.RoomID(room),
		Timestamp: time.Now().UnixMilli(),
		ID:        id.EventID("$evt"),
		Type:      event.EventMessage,
		Content: event.Content{
			Parsed: &event.MessageEventContent{
				MsgType: event.MsgText,
				Body:    body,
			},
		},
	}
}

func TestHandleMessageEvent_SkipsSelf(t *testing.T) {
	ch, b := testMatrixChannel(t, nil, nil)
	defer b.Close()
	done := make(chan struct{})
	go func() {
		select {
		case <-b.InboundChan():
			t.Error("unexpected inbound for self message")
		case <-time.After(50 * time.Millisecond):
		}
		close(done)
	}()
	ch.handleMessageEvent(context.Background(), textEvent("@agent:example.org", "!room:example.org", "hi"))
	<-done
}

func TestHandleMessageEvent_AllowFrom(t *testing.T) {
	ch, b := testMatrixChannel(t, []string{"@alice:example.org"}, nil)
	defer b.Close()
	received := make(chan bus.InboundMessage, 2)
	go func() {
		for msg := range b.InboundChan() {
			received <- msg
		}
	}()
	ch.handleMessageEvent(context.Background(), textEvent("@bob:example.org", "!room:example.org", "nope"))
	select {
	case msg := <-received:
		t.Fatalf("unexpected inbound for disallowed sender: %+v", msg)
	case <-time.After(50 * time.Millisecond):
	}
	ch.handleMessageEvent(context.Background(), textEvent("@alice:example.org", "!room:example.org", "yes"))
	select {
	case msg := <-received:
		if msg.SenderID != "@alice:example.org" || msg.Content != "yes" {
			t.Fatalf("unexpected inbound: %+v", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for inbound")
	}
}

func TestHandleMessageEvent_AllowRooms(t *testing.T) {
	ch, b := testMatrixChannel(t, nil, []string{"!allowed:example.org"})
	defer b.Close()
	received := make(chan bus.InboundMessage, 2)
	go func() {
		for msg := range b.InboundChan() {
			received <- msg
		}
	}()
	ch.handleMessageEvent(context.Background(), textEvent("@alice:example.org", "!blocked:example.org", "nope"))
	select {
	case msg := <-received:
		t.Fatalf("unexpected inbound for disallowed room: %+v", msg)
	case <-time.After(50 * time.Millisecond):
	}
	ch.handleMessageEvent(context.Background(), textEvent("@alice:example.org", "!allowed:example.org", "ok"))
	select {
	case msg := <-received:
		if msg.ChatID != "!allowed:example.org" {
			t.Fatalf("chat id = %q", msg.ChatID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for inbound")
	}
}

func TestHandleMessageEvent_SkipsNonText(t *testing.T) {
	ch, b := testMatrixChannel(t, nil, nil)
	defer b.Close()
	done := make(chan struct{})
	go func() {
		select {
		case <-b.InboundChan():
			t.Error("unexpected inbound for image message")
		case <-time.After(50 * time.Millisecond):
		}
		close(done)
	}()
	evt := textEvent("@alice:example.org", "!room:example.org", "img")
	evt.Content.Parsed = &event.MessageEventContent{MsgType: event.MsgImage, Body: "img"}
	ch.handleMessageEvent(context.Background(), evt)
	<-done
}

func TestHandleMemberEvent_JoinsInvite(t *testing.T) {
	mock := &mockMatrixSender{}
	ch := &MatrixChannel{
		BaseChannel: chann.NewBaseChannel(matrixChannelName, bus.New(1, mavenlog.Std()), nil, mavenlog.Std()),
		userID:      "@agent:example.org",
		client:      mock,
	}
	evt := &event.Event{
		RoomID: id.RoomID("!room:example.org"),
		Sender: id.UserID("@alice:example.org"),
		Type:   event.StateMember,
		Content: event.Content{
			Parsed: &event.MemberEventContent{Membership: event.MembershipInvite},
		},
	}
	stateKey := ch.userID.String()
	evt.StateKey = &stateKey
	ch.handleMemberEvent(context.Background(), evt)
	if len(mock.joinRooms) != 1 || mock.joinRooms[0] != "!room:example.org" {
		t.Fatalf("join rooms = %v", mock.joinRooms)
	}
}

func TestChunkText(t *testing.T) {
	long := strings.Repeat("a", 35000)
	chunks := chunkText(long, matrixSendChunkSize)
	if len(chunks) != 2 {
		t.Fatalf("chunks = %d, want 2", len(chunks))
	}
	if len(chunks[0]) != matrixSendChunkSize {
		t.Fatalf("first chunk len = %d", len(chunks[0]))
	}
}

func TestChunkText_NewlineRoundTrip(t *testing.T) {
	original := strings.Repeat("a", matrixSendChunkSize-10) + "\n\n" + strings.Repeat("b", matrixSendChunkSize+50)
	chunks := chunkText(original, matrixSendChunkSize)
	if strings.Join(chunks, "") != original {
		t.Fatalf("chunks do not reconstruct original string byte-for-byte")
	}
}

func TestChunkText_RuneSafe(t *testing.T) {
	// Each 日 is 3 bytes. Build a string that crosses the chunk boundary mid-rune.
	rune3 := "日"
	base := strings.Repeat("a", matrixSendChunkSize-1) + rune3 + strings.Repeat("a", 100)
	chunks := chunkText(base, matrixSendChunkSize)
	for i, c := range chunks {
		if !utf8.ValidString(c) {
			t.Fatalf("chunk %d is invalid UTF-8", i)
		}
	}
	joined := strings.Join(chunks, "")
	if joined != base {
		t.Fatalf("chunks do not reconstruct original string")
	}
}

func TestMatrixChannel_Send_Chunks(t *testing.T) {
	mock := &mockMatrixSender{}
	ch := &MatrixChannel{
		BaseChannel: chann.NewBaseChannel(matrixChannelName, bus.New(1, mavenlog.Std()), nil, mavenlog.Std()),
		client:      mock,
	}
	content := strings.Repeat("x", matrixSendChunkSize+100)
	if err := ch.Send(context.Background(), bus.OutboundMessage{ChatID: "!room:example.org", Content: content}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(mock.sent) != 2 {
		t.Fatalf("sent chunks = %d, want 2", len(mock.sent))
	}
}
