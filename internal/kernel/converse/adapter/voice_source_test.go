package adapter

import (
	"context"
	"encoding/binary"
	"testing"
	"time"

	"github.com/ageneralai/maven/internal/kernel/converse"
)

func loudPCM(level float64) []byte {
	v := int16(level * 32767)
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, uint16(v))
	return b
}

func silentPCM() []byte {
	return make([]byte, 2)
}

type fakeSTT struct {
	transcript string
}

func (f *fakeSTT) Transcribe(ctx context.Context, audio <-chan []byte) (<-chan string, error) {
	out := make(chan string, 1)
	go func() {
		defer close(out)
		for range audio {
		}
		// Let the VAD goroutine emit SpeechStart before the utterance path closes out.
		select {
		case <-ctx.Done():
			return
		case <-time.After(20 * time.Millisecond):
		}
		if f.transcript == "" {
			return
		}
		select {
		case <-ctx.Done():
		case out <- f.transcript:
		}
	}()
	return out, nil
}

func fakeOpener(frames ...[]byte) PCMOpener {
	return func(ctx context.Context) (<-chan []byte, error) {
		ch := make(chan []byte, len(frames))
		go func() {
			defer close(ch)
			for _, frame := range frames {
				select {
				case <-ctx.Done():
					return
				case ch <- frame:
				}
			}
		}()
		return ch, nil
	}
}

func TestNewVoiceSource(t *testing.T) {
	t.Parallel()
	src := NewVoiceSource(VoiceSourceConfig{
		Open: fakeOpener(silentPCM(), loudPCM(1.0)),
		STT:  &fakeSTT{transcript: "hello"},
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var got []converse.Event
	for ev := range src.Listen(ctx) {
		got = append(got, ev)
	}
	if len(got) != 2 {
		t.Fatalf("events = %d, want 2: %#v", len(got), got)
	}
	if _, ok := got[0].(converse.SpeechStart); !ok {
		t.Fatalf("event[0] = %T, want SpeechStart", got[0])
	}
	u, ok := got[1].(converse.Utterance)
	if !ok || u.Text != "hello" {
		t.Fatalf("event[1] = %#v, want Utterance{hello}", got[1])
	}
}
