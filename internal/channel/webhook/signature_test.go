package webhook

import "testing"

func TestSignatureGolden(t *testing.T) {
	got := Signature("t1", "time1", "n1", "data1")
	want := "9ed1f1192b806800384a4bbf59377766edec0aea"
	if got != want {
		t.Fatalf("signature = %q, want %q (sorted join data1n1t1time1)", got, want)
	}
	if got2 := Signature("t1", "time1", "n1", "data1"); got2 != got {
		t.Fatal("signature not deterministic")
	}
}
