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
// Gateway wiring injects a shared *events.Fanout via WithEventPublisher; pipeline and
// plugins receive the same instance for publish/subscribe. There is no process-global
// event registry.
//
// Streaming outbound: optional StreamDelegate ([WithStreamDelegate], [MessageBus.SetStreamDelegate]);
// the pipeline calls OnStreamBegin/OnStreamEnd around SendStream. Default is a noop delegate.
//
// Shutdown semantics are documented on MessageBus.Close.
package bus
