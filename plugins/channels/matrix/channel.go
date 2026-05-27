package matrix

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"log/slog"

	"github.com/ageneralai/maven/kernel/bus"
	"github.com/ageneralai/maven/kernel/channels"
	"github.com/ageneralai/maven/kernel/channels/allowlist"
	"github.com/ageneralai/maven/kernel/config"
	"github.com/ageneralai/maven/kernel/stringutil"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

const matrixChannelName = "matrix"

const matrixSendChunkSize = 32000

var errUserIDRequired = errors.New("matrix user id is required")

type matrixSender interface {
	SendText(ctx context.Context, roomID id.RoomID, text string) (*mautrix.RespSendEvent, error)
	JoinRoomByID(ctx context.Context, roomID id.RoomID) (*mautrix.RespJoinRoom, error)
	SyncWithContext(ctx context.Context) error
}

type MatrixChannel struct {
	name       string
	log        *slog.Logger
	bus        *bus.MessageBus
	allow      allowlist.Matcher
	userID     id.UserID
	client     matrixSender
	allowRooms map[string]bool
	cancel     context.CancelFunc
	syncWG     sync.WaitGroup
}

func NewMatrixChannel(cfg config.MatrixConfig, workspace string, lg *slog.Logger, b *bus.MessageBus) (*MatrixChannel, error) {
	userID, err := parseUserID(cfg.UserID)
	if err != nil {
		return nil, err
	}
	ws := strings.TrimSpace(workspace)
	if ws == "" {
		return nil, fmt.Errorf("agent workspace is required for matrix state")
	}
	store, err := openFileSyncStore(ws, userID, strings.TrimSpace(cfg.DeviceID))
	if err != nil {
		return nil, err
	}
	client, err := mautrix.NewClient(cfg.Homeserver, userID, cfg.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("create matrix client: %w", err)
	}
	client.Store = store
	if deviceID := store.DeviceID(); deviceID != "" {
		client.DeviceID = id.DeviceID(deviceID)
	}
	ch := &MatrixChannel{
		name:       matrixChannelName,
		log:        lg,
		bus:        b,
		allow:      allowlist.NewMatcher(cfg.AllowFrom),
		userID:     userID,
		client:     client,
		allowRooms: buildAllowRooms(cfg.AllowRooms),
	}
	if err := ch.registerSyncHandlers(client); err != nil {
		return nil, err
	}
	return ch, nil
}

func (m *MatrixChannel) registerSyncHandlers(client *mautrix.Client) error {
	syncer, ok := client.Syncer.(*mautrix.DefaultSyncer)
	if !ok {
		return fmt.Errorf("matrix: expected DefaultSyncer, got %T", client.Syncer)
	}
	syncer.OnEventType(event.EventMessage, func(ctx context.Context, evt *event.Event) {
		m.handleMessageEvent(ctx, evt)
	})
	syncer.OnEventType(event.StateMember, func(ctx context.Context, evt *event.Event) {
		m.handleMemberEvent(ctx, evt)
	})
	return nil
}

func (m *MatrixChannel) Name() string {
	return m.name
}

func (m *MatrixChannel) IsAllowed(senderID string) bool {
	return m.allow.Allow(senderID)
}

func (m *MatrixChannel) Capabilities() channels.CapabilitySet {
	return channels.CapabilitySet{}
}

func (m *MatrixChannel) Start(ctx context.Context) error {
	ctx, m.cancel = context.WithCancel(ctx)
	m.syncWG.Add(1)
	go func() {
		defer m.syncWG.Done()
		m.log.Info("matrix sync started", "user_id", m.userID)
		if err := m.client.SyncWithContext(ctx); err != nil && !errors.Is(err, context.Canceled) {
			m.log.Error("matrix sync stopped", "err", err)
		}
	}()
	return nil
}

func (m *MatrixChannel) Stop() error {
	if m.cancel != nil {
		m.cancel()
	}
	m.syncWG.Wait()
	m.log.Info("matrix stopped")
	return nil
}

func (m *MatrixChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	roomID := id.RoomID(strings.TrimSpace(msg.ChatID))
	if roomID == "" {
		return fmt.Errorf("matrix room id is required")
	}
	content := strings.TrimSpace(msg.Content)
	if content == "" {
		return nil
	}
	for _, chunk := range stringutil.ChunkBytes(content, matrixSendChunkSize) {
		if _, err := m.client.SendText(ctx, roomID, chunk); err != nil {
			return fmt.Errorf("matrix send to %s: %w", roomID, err)
		}
	}
	return nil
}
