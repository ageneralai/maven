package voice

import (
	"context"
	"testing"
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

func TestSession_RunTTS(t *testing.T) {
	var got [][]byte
	tts := &chunkTTS{chunks: [][]byte{[]byte("a"), []byte("b")}}
	s := NewSession(context.Background(), nil, tts)
	textCh := make(chan string, 2)
	textCh <- "Hello."
	close(textCh)
	agentCtx := s.NewAgentCtx()
	err := s.RunTTS(agentCtx, textCh, func(b []byte) error {
		cp := make([]byte, len(b))
		copy(cp, b)
		got = append(got, cp)
		return nil
	})
	if err != nil {
		t.Fatalf("RunTTS: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("writes = %d, want 1", len(got))
	}
	if string(got[0]) != "ab" {
		t.Fatalf("write = %q, want ab", string(got[0]))
	}
}
