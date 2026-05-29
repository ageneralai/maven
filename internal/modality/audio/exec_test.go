package audio

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestExecCapturePlayback_RoundTrip(t *testing.T) {
	payload := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	tmp, err := os.CreateTemp(t.TempDir(), "pcm-*.bin")
	if err != nil {
		t.Fatal(err)
	}
	path := tmp.Name()
	if _, err := tmp.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatal(err)
	}
	capture := &ExecCapture{Command: "cat", Args: []string{path}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pcm, err := capture.Capture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	outPath := path + ".out"
	playback := &ExecPlayback{Command: "sh", Args: []string{"-c", "cat > " + outPath}}
	if err := playback.Play(ctx, pcm); err != nil && !errorsIsCtx(err) {
		t.Fatalf("Play: %v", err)
	}
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("round-trip mismatch: got %v want %v", got, payload)
	}
}

func errorsIsCtx(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func TestExecCapture_EmptyCommand(t *testing.T) {
	c := &ExecCapture{}
	if _, err := c.Capture(context.Background()); err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestExecPlayback_EmptyCommand(t *testing.T) {
	p := &ExecPlayback{}
	ch := make(chan []byte)
	close(ch)
	if err := p.Play(context.Background(), ch); err == nil {
		t.Fatal("expected error for empty command")
	}
}
