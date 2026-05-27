package web

import "testing"

func TestMavenResponseSessionMap(t *testing.T) {
	resetMavenResponseSessionsForTest()
	storeMavenResponseSession("resp_aaa", "web-sess1")
	got, ok := lookupMavenResponseSession("resp_aaa")
	if !ok || got != "web-sess1" {
		t.Fatalf("lookup: got %q ok=%v", got, ok)
	}
	if _, ok := lookupMavenResponseSession("resp_missing"); ok {
		t.Fatal("expected missing id")
	}
}

func TestIsMavenResponseID(t *testing.T) {
	if !isMavenResponseID("resp_abc") {
		t.Fatal("expected resp_ prefix valid")
	}
	if isMavenResponseID("tSmdLA67S1YT") {
		t.Fatal("platform session id is not a response id")
	}
}
