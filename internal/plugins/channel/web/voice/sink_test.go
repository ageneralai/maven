package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/ageneralai/maven/internal/kernel/voice"
	"github.com/ageneralai/maven/internal/plugins/channel/web/wsmsg"
)

type fakeFrameWriter struct {
	mu      sync.Mutex
	binary  [][]byte
	text    [][]byte
	binErr  error
	textErr error
}

func (f *fakeFrameWriter) writeBinary(_ context.Context, data []byte) error {
	if f.binErr != nil {
		return f.binErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.binary = append(f.binary, bytes.Clone(data))
	return nil
}

func (f *fakeFrameWriter) writeText(_ context.Context, data []byte) error {
	if f.textErr != nil {
		return f.textErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.text = append(f.text, bytes.Clone(data))
	return nil
}

type fakeTTS struct {
	chunks [][]byte
}

func (f *fakeTTS) Synthesize(_ context.Context, _ string) (<-chan []byte, error) {
	out := make(chan []byte, len(f.chunks))
	go func() {
		defer close(out)
		for _, c := range f.chunks {
			out <- c
		}
	}()
	return out, nil
}

func TestWsVoiceSink_RenderComplete(t *testing.T) {
	t.Parallel()
	fw := &fakeFrameWriter{}
	tts := &fakeTTS{chunks: [][]byte{{0x01, 0x02}, {0x03, 0x04}}}
	sink := &wsVoiceSink{w: fw, tts: tts}
	reply := make(chan string, 1)
	reply <- "Hello world."
	close(reply)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := sink.Render(ctx, reply); err != nil {
		t.Fatalf("Render: %v", err)
	}
	fw.mu.Lock()
	defer fw.mu.Unlock()
	if len(fw.binary) != 2 {
		t.Fatalf("binary frames = %d, want 2", len(fw.binary))
	}
	if len(fw.text) != 1 {
		t.Fatalf("text frames = %d, want 1", len(fw.text))
	}
	var msg wsmsg.Message
	if err := json.Unmarshal(fw.text[0], &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Type != "stream_done" {
		t.Fatalf("type = %q, want stream_done", msg.Type)
	}
}

type stallTTS struct{}

func (stallTTS) Synthesize(ctx context.Context, _ string) (<-chan []byte, error) {
	out := make(chan []byte)
	go func() {
		<-ctx.Done()
		close(out)
	}()
	return out, nil
}

func TestWsVoiceSink_RenderInterrupt(t *testing.T) {
	t.Parallel()
	fw := &fakeFrameWriter{}
	sink := &wsVoiceSink{w: fw, tts: stallTTS{}}
	reply := make(chan string, 1)
	reply <- "Hello world."
	close(reply)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	err := sink.Render(ctx, reply)
	if err == nil {
		t.Fatal("expected context error")
	}
	fw.mu.Lock()
	defer fw.mu.Unlock()
	gotSentinel := false
	for _, frame := range fw.binary {
		if len(frame) == 1 && frame[0] == voiceClearSentinel {
			gotSentinel = true
		}
	}
	if !gotSentinel {
		t.Fatalf("binary = %#v, want 0x00 sentinel", fw.binary)
	}
	if len(fw.text) != 0 {
		t.Fatalf("text frames = %d, want 0 on interrupt", len(fw.text))
	}
}

var _ voice.TTS = (*fakeTTS)(nil)
