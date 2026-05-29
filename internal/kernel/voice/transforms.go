package voice

import (
	"context"
	"errors"
	"strings"
)

// Sentencize buffers text deltas and emits complete sentences (TakeCompleteSentences),
// flushing the remainder (FlushRemainder) when the input closes.
func Sentencize(ctx context.Context, in <-chan string) <-chan string {
	out := make(chan string, 8)
	go func() {
		defer close(out)
		buf := ""
		for {
			select {
			case <-ctx.Done():
				return
			case delta, ok := <-in:
				if !ok {
					tail := FlushRemainder(&buf)
					if tail != "" {
						select {
						case <-ctx.Done():
						case out <- tail:
						}
					}
					return
				}
				buf += delta
				for _, sent := range TakeCompleteSentences(&buf) {
					select {
					case <-ctx.Done():
						return
					case out <- sent:
					}
				}
			}
		}
	}()
	return out
}

// SynthesizeResult holds the first non-cancel TTS error from Synthesize.
type SynthesizeResult struct {
	Err  error
	done chan struct{}
}

// Wait blocks until the Synthesize goroutine has exited and Err is safe to read.
func (r *SynthesizeResult) Wait() {
	<-r.done
}

// Synthesize turns sentences into aligned PCM chunks (int16 LE). Stops the output stream on TTS error.
func Synthesize(ctx context.Context, tts TTS, sentences <-chan string) (<-chan []byte, *SynthesizeResult) {
	out := make(chan []byte, 16)
	res := &SynthesizeResult{done: make(chan struct{})}
	go func() {
		defer close(res.done)
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case text, ok := <-sentences:
				if !ok {
					return
				}
				text = strings.TrimSpace(text)
				if text == "" {
					continue
				}
				chunks, err := tts.Synthesize(ctx, text)
				if err != nil {
					if !errors.Is(err, context.Canceled) {
						res.Err = err
					}
					return
				}
				if err := emitAlignedPCM(ctx, chunks, out); err != nil {
					if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
						res.Err = err
					}
					return
				}
			}
		}
	}()
	return out, res
}

func emitAlignedPCM(ctx context.Context, chunks <-chan []byte, out chan<- []byte) error {
	var pending []byte
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-chunks:
			if !ok {
				return nil
			}
			if len(chunk) == 0 {
				continue
			}
			if len(pending) > 0 {
				chunk = append(pending, chunk...)
				pending = nil
			}
			if len(chunk)%2 != 0 {
				pending = append(pending, chunk[len(chunk)-1])
				chunk = chunk[:len(chunk)-1]
			}
			if len(chunk) == 0 {
				continue
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case out <- chunk:
			}
		}
	}
}
