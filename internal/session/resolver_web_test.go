package session

import (
	"testing"

	"github.com/ageneralai/maven/internal/sessionid"
)

func TestSessionResolver_WebChannel(t *testing.T) {
	r := &SessionResolver{}
	if got := r.ResolveSDKSessionID(sessionid.WebChannelName, "web-1", sessionid.WebChannelName+":web-1", SessionModeCurrent); got != "web-1" {
		t.Fatalf("ws chat session: got %q want web-1", got)
	}
}
