package bus

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/ageneralai/maven/internal/events"
	mavenlog "github.com/ageneralai/maven/pkg/log"
)

var ErrBusClosed = errors.New("bus closed")

// MessageBus carries pipeline inbound work and fans out outbound to at most one
// subscriber per channel name ([SetOutboundSubscriber] keys match trimmed [OutboundMessage.Channel]).
//
// Publish methods use strict blocking backpressure — see package comment.
//
// Streaming: optional [StreamDelegate] via [WithStreamDelegate] or [MessageBus.SetStreamDelegate];
// the pipeline invokes [MessageBus.OnStreamBegin] / [MessageBus.OnStreamEnd] around channel SendStream.
type MessageBus struct {
	inbound   chan InboundMessage
	outbound  chan OutboundMessage
	closeOnce sync.Once
	done      chan struct{}
	closed    atomic.Bool
	wg        sync.WaitGroup
	pubMu     sync.Mutex

	publisher events.EventPublisher

	streamMu sync.RWMutex
	streamDel StreamDelegate

	mu   sync.RWMutex
	subs map[string]func(OutboundMessage)
	log  mavenlog.PrintLogger
}

// Option configures MessageBus construction.
type Option func(*MessageBus)

// WithEventPublisher wires lifecycle/diagnostic emits (publish failures, bus closed).
// Passing nil behaves like internal/events.NoOp.
func WithEventPublisher(p events.EventPublisher) Option {
	return func(b *MessageBus) {
		b.publisher = events.OrPublisher(p)
		events.SetDefaultPublisher(b.publisher)
	}
}

// WithStreamDelegate wires streaming lifecycle hooks ([OnStreamBegin] / [OnStreamEnd]); nil ⇒ noop delegate.
func WithStreamDelegate(d StreamDelegate) Option {
	return func(b *MessageBus) {
		b.streamDel = OrStreamDelegate(d)
	}
}

func NewMessageBus(bufSize int, log mavenlog.PrintLogger, opts ...Option) *MessageBus {
	if bufSize <= 0 {
		bufSize = 100
	}
	b := &MessageBus{
		inbound:   make(chan InboundMessage, bufSize),
		outbound:  make(chan OutboundMessage, bufSize),
		done:      make(chan struct{}),
		log:       log,
		subs:      make(map[string]func(OutboundMessage)),
		publisher: events.NoOp{},
		streamDel: noopStreamDelegate{},
	}
	for _, o := range opts {
		if o != nil {
			o(b)
		}
	}
	events.SetDefaultPublisher(b.publisher)
	return b
}

func (b *MessageBus) InboundChan() <-chan InboundMessage {
	return b.inbound
}

func (b *MessageBus) OutboundChan() <-chan OutboundMessage {
	return b.outbound
}

// PublishInbound enqueues msg on the inbound buffer (strict blocking until space, ctx done, or Close).
func (b *MessageBus) PublishInbound(ctx context.Context, msg InboundMessage) error {
	msg, normErr := normalizeInboundMessage(msg)
	if normErr != nil {
		return normErr
	}
	return publishEnqueue(b, ctx, "inbound", msg.Channel, b.inbound, msg)
}

// PublishOutbound enqueues msg on the outbound buffer (same blocking contract as PublishInbound).
// Downstream latency is bounded only by ctx; use [context.WithTimeout] when callers require it.
func (b *MessageBus) PublishOutbound(ctx context.Context, msg OutboundMessage) error {
	msg, normErr := normalizeOutboundMessage(msg)
	if normErr != nil {
		return normErr
	}
	return publishEnqueue(b, ctx, "outbound", msg.Channel, b.outbound, msg)
}

func publishEnqueue[T any](b *MessageBus, ctx context.Context, stream, routingKey string, ch chan T, msg T) error {
	b.pubMu.Lock()
	if b.closed.Load() {
		b.pubMu.Unlock()
		b.reportPublishFailure(stream, routingKey, ErrBusClosed)
		return ErrBusClosed
	}
	b.wg.Add(1)
	b.pubMu.Unlock()
	defer b.wg.Done()
	select {
	case <-ctx.Done():
		err := ctx.Err()
		b.reportPublishFailure(stream, routingKey, err)
		return err
	case <-b.done:
		b.reportPublishFailure(stream, routingKey, ErrBusClosed)
		return ErrBusClosed
	case ch <- msg:
		return nil
	}
}

func (b *MessageBus) reportPublishFailure(stream, routingKey string, err error) {
	b.log.Printf("[bus] metric=publish_failure stream=%s channel=%q err=%v", stream, routingKey, err)
	b.publisher.Publish(context.Background(), busPublishFailureEvent(stream, routingKey, err))
}

func busPublishFailureEvent(stream, routingKey string, err error) events.Event {
	return events.Event{
		Type: events.EventBusPublishFailure,
		Attrs: map[string]string{
			"stream":  stream,
			"channel": routingKey,
			"error":   err.Error(),
		},
	}
}

func busClosedEvent() events.Event {
	return events.Event{Type: events.EventBusClosed}
}

// Close shuts down the bus: rejects new publishes with [ErrBusClosed], unblocks blocked
// publishers via the internal done signal, waits for every in-flight [publishEnqueue]
// goroutine (each holds a [sync.WaitGroup] count from publish to select completion), then
// closes inbound and outbound channels.
//
// After Close returns, [PublishInbound] and [PublishOutbound] always return ErrBusClosed
// (before or inside publish). Readers on [InboundChan] or [OutboundChan] observe channel
// close: receive yields (zero value, ok=false).
//
// [DispatchOutbound] exits when ctx is canceled, or after Close closes outbound (receive !ok).
// Emits events.EventBusClosed after queues are closed.
func (b *MessageBus) Close() {
	b.closeOnce.Do(func() {
		b.pubMu.Lock()
		b.closed.Store(true)
		close(b.done)
		b.pubMu.Unlock()
		b.wg.Wait()
		close(b.inbound)
		close(b.outbound)
		b.publisher.Publish(context.Background(), busClosedEvent())
	})
}

// SetOutboundSubscriber registers the single outbound handler for channel (trimmed for map keys).
// Passing nil removes the subscriber.
func (b *MessageBus) SetOutboundSubscriber(channel string, fn func(OutboundMessage)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	channel = trim(channel)
	if fn == nil {
		delete(b.subs, channel)
		return
	}
	b.subs[channel] = fn
}

func (b *MessageBus) DispatchOutbound(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-b.outbound:
			if !ok {
				return
			}
			b.mu.RLock()
			cb := b.subs[msg.Channel]
			b.mu.RUnlock()
			if cb != nil {
				func() {
					defer func() {
						if rec := recover(); rec != nil {
							b.log.Printf("[bus] panic in outbound subscriber: %v", rec)
						}
					}()
					cb(msg)
				}()
			} else {
				b.log.Printf("[bus] no subscriber for channel %q, dropping message", msg.Channel)
			}
		}
	}
}
