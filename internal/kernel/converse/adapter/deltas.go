package adapter

import (
	"context"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
)

// Deltas extracts assistant text deltas from a runtime stream.
func Deltas(ctx context.Context, events <-chan api.StreamEvent) <-chan string {
	out := make(chan string, 64)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-events:
				if !ok {
					return
				}
				if ev.Type == api.EventContentBlockDelta && ev.Delta != nil && ev.Delta.Text != "" {
					select {
					case <-ctx.Done():
						return
					case out <- ev.Delta.Text:
					}
				}
			}
		}
	}()
	return out
}
