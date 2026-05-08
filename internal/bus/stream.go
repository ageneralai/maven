package bus

import "context"

// StreamHints identifies the chat route for a surfaced streaming turn (model tokens → channel).
type StreamHints struct {
	Channel string
	ChatID  string
}

// StreamDelegate observes streamed outbound replies before/after runtime + SendStream wiring.
// Methods must return quickly; heavier work belongs in async handlers.
type StreamDelegate interface {
	// NotifyStreamBegin runs before RunStream consumes the runtime; return wrapped ctx if needed for deadlines/values downstream.
	NotifyStreamBegin(ctx context.Context, h StreamHints) context.Context
	NotifyStreamEnd(ctx context.Context, h StreamHints, err error)
}

type noopStreamDelegate struct{}

func (noopStreamDelegate) NotifyStreamBegin(ctx context.Context, _ StreamHints) context.Context {
	return ctx
}

func (noopStreamDelegate) NotifyStreamEnd(context.Context, StreamHints, error) {}

// OrStreamDelegate returns a non-nil delegate; nil becomes the default noop.
func OrStreamDelegate(d StreamDelegate) StreamDelegate {
	if d == nil {
		return noopStreamDelegate{}
	}
	return d
}

// SetStreamDelegate replaces the delegate (nil ⇒ noop delegate).
func (b *MessageBus) SetStreamDelegate(d StreamDelegate) {
	b.streamMu.Lock()
	defer b.streamMu.Unlock()
	b.streamDel = OrStreamDelegate(d)
}

// OnStreamBegin notifies the delegate before a streamed outbound turn consumes the runtime.
func (b *MessageBus) OnStreamBegin(ctx context.Context, h StreamHints) context.Context {
	b.streamMu.RLock()
	del := b.streamDel
	b.streamMu.RUnlock()
	return del.NotifyStreamBegin(ctx, h)
}

// OnStreamEnd notifies the delegate after RunStream (+SendStream path) finishes; err reflects runtime or SendStream failure.
func (b *MessageBus) OnStreamEnd(ctx context.Context, h StreamHints, streamErr error) {
	b.streamMu.RLock()
	del := b.streamDel
	b.streamMu.RUnlock()
	del.NotifyStreamEnd(ctx, h, streamErr)
}
