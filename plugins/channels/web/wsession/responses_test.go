package wsession

import "testing"

func TestMavenResponseSessionMap(t *testing.T) {
	sessions := NewResponseSessions()
	sessions.StoreMavenResponseSession("resp_aaa", "web-sess1")
	got, ok := sessions.lookupMavenResponseSession("resp_aaa")
	if !ok || got != "web-sess1" {
		t.Fatalf("lookup: got %q ok=%v", got, ok)
	}
	if _, ok := sessions.lookupMavenResponseSession("resp_missing"); ok {
		t.Fatal("expected missing id")
	}
}

func TestIsMavenResponseID(t *testing.T) {
	if !IsMavenResponseID("resp_abc") {
		t.Fatal("expected resp_ prefix valid")
	}
	if IsMavenResponseID("tSmdLA67S1YT") {
		t.Fatal("platform session id is not a response id")
	}
}
