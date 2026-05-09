package voice

import "testing"

func TestTakeCompleteSentences(t *testing.T) {
	buf := "Hello there. Next bit! Final?"
	got := TakeCompleteSentences(&buf)
	if len(got) != 3 {
		t.Fatalf("got %d sentences: %q", len(got), got)
	}
	if got[0] != "Hello there." || got[1] != "Next bit!" || got[2] != "Final?" {
		t.Fatalf("sentences: %#v", got)
	}
	if buf != "" {
		t.Fatalf("remainder buf = %q", buf)
	}
}
