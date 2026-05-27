package wsession

import (
	"net/http/httptest"
	"testing"
)

func TestResolveMavenSessionID(t *testing.T) {
	ResetMavenResponseSessionsForTest()
	StoreMavenResponseSession("resp_aaa", "web-sess1")
	req := httptest.NewRequest("POST", "/v1/responses", nil)
	req.Header.Set(HeaderMavenSessionID, "sess1")
	got, err := ResolveMavenSessionID(req, "")
	if err != nil || got != "web-sess1" {
		t.Fatalf("header session: got %q err=%v", got, err)
	}
	req2 := httptest.NewRequest("POST", "/v1/responses", nil)
	got, err = ResolveMavenSessionID(req2, "resp_aaa")
	if err != nil || got != "web-sess1" {
		t.Fatalf("previous_response_id: got %q err=%v", got, err)
	}
	req3 := httptest.NewRequest("POST", "/v1/responses", nil)
	_, err = ResolveMavenSessionID(req3, "tSmdLA67S1YT")
	if err == nil {
		t.Fatal("expected invalid previous_response_id")
	}
	req4 := httptest.NewRequest("POST", "/v1/responses", nil)
	_, err = ResolveMavenSessionID(req4, "")
	if err == nil {
		t.Fatal("expected Maven-Session-Id required")
	}
	req5 := httptest.NewRequest("GET", "/ws/voice?session=sess-query", nil)
	got, err = ResolveMavenSessionID(req5, "")
	if err != nil || got != "web-sess-query" {
		t.Fatalf("query session: got %q err=%v", got, err)
	}
}
