package terminal

import (
	"context"
	"fmt"

	"github.com/ageneralai/maven/internal/kernel/converse"
)

// voiceLine overwrites the open "you ▸" line with the STT transcript and forwards events.
type voiceLine struct {
	source  converse.Source
	session *Session
}

func (v *voiceLine) Listen(ctx context.Context) <-chan converse.Event {
	inner := v.source.Listen(ctx)
	out := make(chan converse.Event, 4)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-inner:
				if !ok {
					return
				}
				if u, ok := ev.(converse.Utterance); ok && u.Text != "" {
					_, _ = fmt.Fprintf(v.session.Transcript, "\r%s%s\n", userLabel, u.Text)
				}
				select {
				case <-ctx.Done():
					return
				case out <- ev:
				}
			}
		}
	}()
	return out
}

var _ converse.Source = (*voiceLine)(nil)
