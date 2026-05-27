// Package gateway bridges inbound chat channels and background scheduling.
//
// Session contract:
//   - Cron runs use a fresh SessionID per execution (see cron.SessionKey). Heartbeat uses a fresh SessionID per tick (see heartbeat.SessionKey).
//   - Cron admission (concurrency cap) lives in cron.Service (semaphore, gateway.cron.maxConcurrentRuns at process start, default 1; changing requires restart). Heartbeat admission is a try-once weight-1 semaphore inside heartbeat.Service: a tick skips only if the previous heartbeat turn is still running. Cron and heartbeat do not share an admission lane—they may run at the same time. Skipped heartbeat ticks log "skipped: previous tick still running".
//   - Inbound chat uses per-channel session keys and may call the runtime concurrently with cron and heartbeat. Inbound chat does not use the cron or heartbeat semaphores.
//   - Pipeline reload and shutdown drain via turnMu (pipeline.Pipeline).
//   - Gateway.Apply is the single declarative path: ChannelManager.Apply, new runtime via factory,
//     pipeline.Reload (no separate “first start” branch). Cron proactive delivery is cron.Deliver.AfterSuccessfulRun after a successful job run; TurnExecutor stays pipeline-only.
//   - The message bus (internal/bus.NewMessageBus) defaults to pkg/events.NoOp for EventPublisher.
//     Wire internal/bus.WithEventPublisher to observe events.EventBusPublishFailure and
//     events.EventBusClosed emits.
//   - Streaming: internal/bus.StreamDelegate defaults to noop; wire WithStreamDelegate or
//     MessageBus.SetStreamDelegate — pipeline wraps channel SendStream with OnStreamBegin/OnStreamEnd.
//   - Per-turn routing: pkg/context (package turnctx): pipeline attaches WithInbound;
//     tools resolve channel/chat with From / Channel / ChatID. TurnContext optionally carries Metadata
//     (trimmed-string keys after normalize).
//   - Liveness: internal/health.HealthReporter defaults to NoOp. gateway.Options.HealthReporter receives
//     SignalGatewayReady after the inbound pipeline goroutine starts; heartbeat.Service pulses
//     SignalHeartbeatTick on each ticker fire (before tick work). Both share the same reporter when wired from Options.
package gateway
