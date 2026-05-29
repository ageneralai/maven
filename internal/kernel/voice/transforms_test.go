package voice

import (
	"context"
	"testing"
	"time"
)

type fakeSTT struct {
	texts []string
}

func (f *fakeSTT) Transcribe(ctx context.Context, audio <-chan []byte) (<-chan string, error) {
	out := make(chan string, len(f.texts))
	go func() {
		defer close(out)
		for _, t := range f.texts {
			select {
			case <-ctx.Done():
				return
			case out <- t:
			}
		}
	}()
	return out, nil
}

type chunkTTS struct {
	chunks [][]byte
	err    error
}

func (c *chunkTTS) Synthesize(ctx context.Context, text string) (<-chan []byte, error) {
	if c.err != nil {
		return nil, c.err
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

func TestSentencize(t *testing.T) {
	t.Parallel()
	in := make(chan string, 4)
	in <- "Hello there. More"
	in <- " text!"
	close(in)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var got []string
	for s := range Sentencize(ctx, in) {
		got = append(got, s)
	}
	if len(got) != 2 {
		t.Fatalf("sentences = %#v", got)
	}
	if got[0] != "Hello there." || got[1] != "More text!" {
		t.Fatalf("sentences = %#v", got)
	}
}

func TestSynthesize_alignsOddByte(t *testing.T) {
	t.Parallel()
	tts := &chunkTTS{chunks: [][]byte{[]byte("a"), []byte("b")}}
	sent := make(chan string, 1)
	sent <- "Hello."
	close(sent)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	pcm, res := Synthesize(ctx, tts, sent)
	var got [][]byte
	for b := range pcm {
		cp := make([]byte, len(b))
		copy(cp, b)
		got = append(got, cp)
	}
	res.Wait()
	if res.Err != nil {
		t.Fatalf("Synthesize err: %v", res.Err)
	}
	if len(got) != 1 || string(got[0]) != "ab" {
		t.Fatalf("pcm writes = %#v", got)
	}
}

func TestSynthesize_ttsError(t *testing.T) {
	t.Parallel()
	tts := &chunkTTS{err: context.DeadlineExceeded}
	sent := make(chan string, 1)
	sent <- "Hi."
	close(sent)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	pcm, res := Synthesize(ctx, tts, sent)
	for range pcm {
	}
	res.Wait()
	if res.Err == nil {
		t.Fatal("want TTS error")
	}
}
