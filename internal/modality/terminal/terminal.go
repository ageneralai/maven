package terminal

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/ageneralai/maven/internal/kernel/converse"
)

// KeyboardSource emits utterances from the session's stdin line channel.
type KeyboardSource struct {
	lines <-chan string
}

func (k *KeyboardSource) Listen(ctx context.Context) <-chan converse.Event {
	out := make(chan converse.Event)
	go func() {
		defer close(out)
		for {
			var line string
			select {
			case <-ctx.Done():
				return
			case l, ok := <-k.lines:
				if !ok {
					return
				}
				line = l
			}
			text := strings.TrimSpace(line)
			if text == "" {
				continue
			}
			if text == "exit" || text == "quit" {
				return
			}
			select {
			case <-ctx.Done():
				return
			case out <- converse.Utterance{Text: text}:
			}
		}
	}()
	return out
}

// ScreenSink streams reply deltas to Out, prefixed with Label on the first delta.
type ScreenSink struct {
	Out   io.Writer
	Label string
}

func (s *ScreenSink) Render(ctx context.Context, reply <-chan string) error {
	w := s.Out
	if w == nil {
		return nil
	}
	prefixed := false
	for {
		select {
		case <-ctx.Done():
			if prefixed {
				_, _ = fmt.Fprintln(w)
			}
			return ctx.Err()
		case delta, ok := <-reply:
			if !ok {
				_, err := fmt.Fprintln(w)
				return err
			}
			if !prefixed && s.Label != "" {
				if _, err := fmt.Fprint(w, "\n"+s.Label); err != nil {
					return err
				}
				prefixed = true
			}
			if _, err := fmt.Fprint(w, delta); err != nil {
				return err
			}
		}
	}
}
