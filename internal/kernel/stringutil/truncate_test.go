package stringutil

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		n     int
		want  string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is longer than ten", 10, "this is lo..."},
		{"", 5, ""},
	}
	for _, tt := range tests {
		got := Truncate(tt.input, tt.n)
		if got != tt.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
		}
	}
}

func TestTruncateBytes(t *testing.T) {
	t.Parallel()
	if got := TruncateBytes("hello", 10); got != "hello" {
		t.Fatalf("short = %q", got)
	}
	if got := TruncateBytes("hello", 0); got != "hello" {
		t.Fatalf("zero limit = %q", got)
	}
	s := strings.Repeat("A", 25000)
	got := TruncateBytes(s, 20480)
	if len([]byte(got)) > 20480 {
		t.Fatalf("bytes = %d, want <= 20480", len([]byte(got)))
	}
	mixed := strings.Repeat("a", 20478) + "日"
	got = TruncateBytes(mixed, 20480)
	if !utf8.ValidString(got) {
		t.Fatalf("invalid UTF-8: %q", got)
	}
	if len([]byte(got)) > 20480 {
		t.Fatalf("bytes = %d, want <= 20480", len([]byte(got)))
	}
	if strings.Contains(got, "日") {
		t.Fatalf("partial rune kept: %q", got)
	}
}

func TestTruncateRunes(t *testing.T) {
	t.Parallel()
	const maxRunes = 4096
	runes := strings.Repeat("é", maxRunes+10)
	got := TruncateRunes(runes, maxRunes)
	if utf8.RuneCountInString(got) != maxRunes {
		t.Fatalf("rune count = %d, want %d", utf8.RuneCountInString(got), maxRunes)
	}
	if TruncateRunes("hello", 10) != "hello" {
		t.Fatalf("short string changed")
	}
	if TruncateRunes("hello", 0) != "hello" {
		t.Fatalf("zero limit changed string")
	}
}

func TestTruncateRunesTail(t *testing.T) {
	t.Parallel()
	const maxRunes = 1200
	long := strings.Repeat("x", 5000)
	got := TruncateRunesTail(long, maxRunes, "… ")
	if n := utf8.RuneCountInString(got); n > maxRunes {
		t.Fatalf("rune count = %d, want <= %d", n, maxRunes)
	}
	if !strings.HasPrefix(got, "… ") {
		t.Fatalf("missing marker prefix: %q", got)
	}
	if TruncateRunesTail("short", maxRunes, "… ") != "short" {
		t.Fatalf("short string changed")
	}
}

func TestChunkBytes(t *testing.T) {
	t.Parallel()
	const chunkSize = 32000
	long := strings.Repeat("a", 35000)
	chunks := ChunkBytes(long, chunkSize)
	if len(chunks) != 2 {
		t.Fatalf("chunks = %d, want 2", len(chunks))
	}
	if len(chunks[0]) != chunkSize {
		t.Fatalf("first chunk len = %d", len(chunks[0]))
	}
}

func TestChunkBytes_NewlineRoundTrip(t *testing.T) {
	t.Parallel()
	const chunkSize = 32000
	original := strings.Repeat("a", chunkSize-10) + "\n\n" + strings.Repeat("b", chunkSize+50)
	chunks := ChunkBytes(original, chunkSize)
	if strings.Join(chunks, "") != original {
		t.Fatalf("chunks do not reconstruct original string byte-for-byte")
	}
}

func TestChunkBytes_RuneSafe(t *testing.T) {
	t.Parallel()
	const chunkSize = 32000
	rune3 := "日"
	base := strings.Repeat("a", chunkSize-1) + rune3 + strings.Repeat("a", 100)
	chunks := ChunkBytes(base, chunkSize)
	for i, c := range chunks {
		if !utf8.ValidString(c) {
			t.Fatalf("chunk %d is invalid UTF-8", i)
		}
	}
	if strings.Join(chunks, "") != base {
		t.Fatalf("chunks do not reconstruct original string")
	}
}

func TestRuneAlignedCut(t *testing.T) {
	t.Parallel()
	const chunkSize = 32000
	rune3 := "日"
	base := strings.Repeat("a", chunkSize-1) + rune3
	cut := RuneAlignedCut(base, chunkSize)
	if !utf8.ValidString(base[:cut]) {
		t.Fatalf("cut produces invalid UTF-8 at %d", cut)
	}
	if cut > chunkSize {
		t.Fatalf("cut = %d, want <= %d", cut, chunkSize)
	}
	if cut != chunkSize-1 {
		t.Fatalf("cut = %d, want %d", cut, chunkSize-1)
	}
}
