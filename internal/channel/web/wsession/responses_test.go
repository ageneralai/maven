package wsession

import "testing"

func TestMavenResponseSessionMap(t *testing.T) {
	ResetMavenResponseSessionsForTest()
	StoreMavenResponseSession("resp_aaa", "web-sess1")
	got, ok := LookupMavenResponseSession("resp_aaa")
	if !ok || got != "web-sess1" {
		t.Fatalf("lookup: got %q ok=%v", got, ok)
	}
	if _, ok := LookupMavenResponseSession("resp_missing"); ok {
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
