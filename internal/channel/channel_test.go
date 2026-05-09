package channel

import (
	"testing"

	"github.com/ageneralai/maven/internal/bus"
	mavenlog "github.com/ageneralai/maven/pkg/log"
)

var channelTestLog = mavenlog.Std()

func TestBaseChannel_Name(t *testing.T) {
	b := bus.NewMessageBus(10, channelTestLog)
	ch := NewBaseChannel("test", b, nil, channelTestLog)
	if ch.Name() != "test" {
		t.Errorf("Name = %q, want test", ch.Name())
	}
}

func TestBaseChannel_IsAllowed_NoFilter(t *testing.T) {
	b := bus.NewMessageBus(10, channelTestLog)
	ch := NewBaseChannel("test", b, nil, channelTestLog)
	if !ch.IsAllowed("anyone") {
		t.Error("should allow anyone when allowFrom is empty")
	}
}

func TestBaseChannel_IsAllowed_WithFilter(t *testing.T) {
	b := bus.NewMessageBus(10, channelTestLog)
	ch := NewBaseChannel("test", b, []string{"user1", "user2"}, channelTestLog)
	if !ch.IsAllowed("user1") {
		t.Error("should allow user1")
	}
	if !ch.IsAllowed("user2") {
		t.Error("should allow user2")
	}
	if ch.IsAllowed("user3") {
		t.Error("should reject user3")
	}
}
