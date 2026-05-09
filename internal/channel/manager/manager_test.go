package manager

import (
	"context"
	"fmt"
	"testing"

	"github.com/ageneralai/maven/internal/bus"
	chann "github.com/ageneralai/maven/internal/channel"
	mavenlog "github.com/ageneralai/maven/pkg/log"
)

var mgrTestLog = mavenlog.Std()

type mockManagedChannel struct {
	name     string
	started  bool
	stopped  bool
	startErr error
	stopErr  error
	sentMsgs []bus.OutboundMessage
}

func (m *mockManagedChannel) Name() string { return m.name }
func (m *mockManagedChannel) Start(ctx context.Context) error {
	m.started = true
	return m.startErr
}
func (m *mockManagedChannel) Stop() error {
	m.stopped = true
	return m.stopErr
}
func (m *mockManagedChannel) Send(ctx context.Context, msg bus.OutboundMessage) error {
	_ = ctx
	m.sentMsgs = append(m.sentMsgs, msg)
	return nil
}
func (m *mockManagedChannel) Capabilities() chann.CapabilitySet { return chann.CapabilitySet{} }

func TestChannelManager_Empty(t *testing.T) {
	b := bus.NewMessageBus(10, mgrTestLog)
	m := NewChannelManager(b, mgrTestLog, nil)
	if len(m.EnabledChannels()) != 0 {
		t.Errorf("expected 0 enabled channels, got %d", len(m.EnabledChannels()))
	}
}

func TestChannelManager_WithMockChannel(t *testing.T) {
	b := bus.NewMessageBus(10, mgrTestLog)
	mock := &mockManagedChannel{name: "mock"}
	m := &ChannelManager{
		channels: map[string]chann.Channel{"mock": mock},
		bus:      b,
		log:      mgrTestLog,
	}
	ctx := context.Background()
	if err := m.StartAll(ctx); err != nil {
		t.Errorf("StartAll error: %v", err)
	}
	if !mock.started {
		t.Error("mock channel should be started")
	}
	names := m.EnabledChannels()
	if len(names) != 1 || names[0] != "mock" {
		t.Errorf("EnabledChannels = %v, want [mock]", names)
	}
	if err := m.StopAll(); err != nil {
		t.Errorf("StopAll error: %v", err)
	}
	if !mock.stopped {
		t.Error("mock channel should be stopped")
	}
}

func TestChannelManager_StartAll_Empty(t *testing.T) {
	b := bus.NewMessageBus(10, mgrTestLog)
	m := NewChannelManager(b, mgrTestLog, nil)
	if err := m.StartAll(context.Background()); err != nil {
		t.Errorf("StartAll error: %v", err)
	}
}

func TestChannelManager_StopAll_Empty(t *testing.T) {
	b := bus.NewMessageBus(10, mgrTestLog)
	m := NewChannelManager(b, mgrTestLog, nil)
	if err := m.StopAll(); err != nil {
		t.Errorf("StopAll error: %v", err)
	}
}

func TestChannelManager_StartAll_Error(t *testing.T) {
	b := bus.NewMessageBus(10, mgrTestLog)
	mock := &mockManagedChannel{name: "mock", startErr: fmt.Errorf("start failed")}
	m := &ChannelManager{
		channels: map[string]chann.Channel{"mock": mock},
		bus:      b,
		log:      mgrTestLog,
	}
	if err := m.StartAll(context.Background()); err == nil {
		t.Error("expected error from StartAll")
	}
}

func TestChannelManager_StopAll_Error(t *testing.T) {
	b := bus.NewMessageBus(10, mgrTestLog)
	mock := &mockManagedChannel{name: "mock", stopErr: fmt.Errorf("stop failed")}
	m := &ChannelManager{
		channels: map[string]chann.Channel{"mock": mock},
		bus:      b,
		log:      mgrTestLog,
	}
	if err := m.StopAll(); err != nil {
		t.Errorf("StopAll should not return error: %v", err)
	}
}
