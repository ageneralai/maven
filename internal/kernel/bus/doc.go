// Package bus provides the gateway message bus: inbound work for Pipeline and outbound
// fan-out to channels via MessageBus.DispatchOutbound.
//
// Backpressure (strict blocking): PublishInbound and PublishOutbound block until the
// message is enqueued on the internal buffered channel, the context is canceled or hits
// its deadline, or the bus is closed. Messages are not dropped at enqueue except on
// validation failure (ErrInvalidInbound, ErrInvalidOutbound). Latency-sensitive callers
// must use a bounded context deadline; otherwise a full buffer blocks until consumers drain.
//
// Publish failures from enqueue (ErrBusClosed, context.Canceled, context.DeadlineExceeded)
// are logged by the bus with stream and routing channel keys and emitted as
// events.EventBusPublishFailure when an EventPublisher is wired via WithEventPublisher.
//
// MessageBus construction and WithEventPublisher register that same publisher as
// kernel/events.Publish default (events.SetDefaultPublisher). Gateway code that
// emits lifecycle/diagnostic events should use events.Publish instead of reaching into
// the bus; the bus remains the only caller of SetDefaultPublisher.
//
// Streaming outbound: optional StreamDelegate ([WithStreamDelegate], [MessageBus.SetStreamDelegate]);
// the pipeline calls OnStreamBegin/OnStreamEnd around SendStream. Default is a noop delegate.
//
// Shutdown semantics are documented on MessageBus.Close.
package bus
