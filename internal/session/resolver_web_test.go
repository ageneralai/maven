package session

import (
	"testing"

	"github.com/ageneralai/maven/internal/bus"
)

func TestSessionResolver_WebChannel(t *testing.T) {
	r := &SessionResolver{}
	msg := bus.InboundMessage{Channel: WebChannelName, ChatID: "web-1"}
	if got := r.ResolveSDKSessionID(msg); got != "web-1" {
		t.Fatalf("ws chat session: got %q want web-1", got)
	}
}
