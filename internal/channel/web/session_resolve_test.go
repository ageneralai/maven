package web

import (
	"net/http/httptest"
	"testing"
)

func TestResolveMavenSessionID(t *testing.T) {
	resetMavenResponseSessionsForTest()
	storeMavenResponseSession("resp_aaa", "sess1")
	req := httptest.NewRequest("POST", "/v1/responses", nil)
	req.Header.Set(HeaderMavenSessionID, "sess1")
	got, err := resolveMavenSessionID(req, "")
	if err != nil || got != "sess1" {
		t.Fatalf("header session: got %q err=%v", got, err)
	}
	req2 := httptest.NewRequest("POST", "/v1/responses", nil)
	got, err = resolveMavenSessionID(req2, "resp_aaa")
	if err != nil || got != "sess1" {
		t.Fatalf("previous_response_id: got %q err=%v", got, err)
	}
	req3 := httptest.NewRequest("POST", "/v1/responses", nil)
	_, err = resolveMavenSessionID(req3, "tSmdLA67S1YT")
	if err == nil {
		t.Fatal("expected invalid previous_response_id")
	}
	req4 := httptest.NewRequest("POST", "/v1/responses", nil)
	_, err = resolveMavenSessionID(req4, "")
	if err == nil {
		t.Fatal("expected Maven-Session-Id required")
	}
}
