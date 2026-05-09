package voice

import (
	"context"
	"testing"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
)

type chunkTTS struct {
	chunks [][]byte
}

func (c *chunkTTS) Synthesize(ctx context.Context, text string) (<-chan []byte, error) {
	if text == "" {
		ch := make(chan []byte)
		close(ch)
		return ch, nil
	}
	out := make(chan []byte, 4)
	go func() {
		defer close(out)
		for _, b := range c.chunks {
			if len(b) == 0 {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case out <- b:
			}
		}
	}()
	return out, nil
}

func TestSession_StreamEventsToTTS(t *testing.T) {
	var got [][]byte
	tts := &chunkTTS{chunks: [][]byte{[]byte("a"), []byte("b")}}
	s := NewSession(nil, tts) // nil STT: ConsumeTranscripts not exercised here
	events := make(chan api.StreamEvent, 3)
	events <- api.StreamEvent{Type: api.EventContentBlockDelta, Delta: &api.Delta{Type: "text_delta", Text: "Hello."}}
	close(events)
	ctx := context.Background()
	err := s.StreamEventsToTTS(ctx, events, func(_ context.Context, b []byte) error {
		cp := make([]byte, len(b))
		copy(cp, b)
		got = append(got, cp)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamEventsToTTS: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("writes = %d, want 2", len(got))
	}
}
