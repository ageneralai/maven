package converse

import "context"

// NotifyOnDone wraps sink and signals done (non-blocking) after each reply that
// renders to natural completion, so a wake gate can start its idle window when
// playback finishes rather than when the user last spoke. Cancelled renders
// (barge-in, shutdown) do not signal.
func NotifyOnDone(sink Sink, done chan<- struct{}) Sink {
	return &doneSink{sink: sink, done: done}
}

type doneSink struct {
	sink Sink
	done chan<- struct{}
}

func (d *doneSink) Render(ctx context.Context, reply <-chan string) error {
	err := d.sink.Render(ctx, reply)
	if err == nil {
		select {
		case d.done <- struct{}{}:
		default:
		}
	}
	return err
}

var _ Sink = (*doneSink)(nil)
