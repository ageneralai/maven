// Package gateway bridges inbound chat channels and background scheduling.
//
// Session contract:
//   - Cron runs use a fresh SessionID per execution (see sessionid.KindCron). Heartbeat uses a fresh SessionID per tick (see sessionid.KindHeartbeat).
//   - Cron admission (concurrency cap) lives in cron.Service (semaphore, gateway.cron.maxConcurrentRuns at process start, default 1; changing requires restart). Heartbeat admission is a try-once weight-1 semaphore inside heartbeat.Service: a tick skips only if the previous heartbeat turn is still running. Cron and heartbeat do not share an admission lane—they may run at the same time. Skipped heartbeat ticks log "skipped: previous tick still running".
//   - Inbound chat uses per-channel session keys and may call the runtime concurrently with cron and heartbeat. Inbound chat does not use the cron or heartbeat semaphores.
//   - Pipeline reload and shutdown drain via turnMu (kernel/pipeline.Pipeline).
//   - Gateway.Apply is the single declarative path: ChannelManager.Apply, new runtime via factory,
//     pipeline.Reload (no separate "first start" branch). Cron proactive delivery is cron.Deliver.AfterSuccessfulRun after a successful job run; TurnExecutor stays pipeline-only.
//   - Events: wireCore builds one kernel/events.Fanout and injects it into the bus
//     (kernel/bus.WithEventPublisher), the pipeline (Pipeline.SetEventBus), and the plugin
//     registry (Registry.SetEventBus); there is no process-global registry. The bus emits
//     events.EventBusPublishFailure and events.EventBusClosed; the pipeline emits
//     events.EventStreamFailed, events.EventOutboundDeliveryFailed, and events.EventTurnCompleted.
//     The memory plugin subscribes to events.EventTurnCompleted via plugin.EventAwarePlugin.
//   - Streaming: kernel/bus.StreamDelegate defaults to noop; wire WithStreamDelegate or
//     MessageBus.SetStreamDelegate — pipeline wraps channel SendStream with OnStreamBegin/OnStreamEnd.
//   - Per-turn routing: kernel/turnctx: pipeline attaches WithInbound;
//     tools resolve channel/chat with From / Channel / ChatID. TurnContext optionally carries Metadata
//     (trimmed-string keys after normalize).
//   - Liveness: kernel/health.HealthReporter defaults to NoOp. gateway.Options.HealthReporter receives
//     SignalGatewayReady after the inbound pipeline goroutine starts; heartbeat.Service pulses
//     SignalHeartbeatTick on each ticker fire (before tick work). Both share the same reporter when wired from Options.
package gateway
