package session

import (
	"testing"
)

func TestSessionResolver_WebChannel(t *testing.T) {
	r := &SessionResolver{}
	if got := r.ResolveSDKSessionID(WebChannelName, "web-1", WebChannelName+":web-1", SessionModeCurrent); got != "web-1" {
		t.Fatalf("ws chat session: got %q want web-1", got)
	}
}
