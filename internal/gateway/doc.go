// Package gateway bridges inbound chat channels and background automation.
//
// Session contract:
//   - Cron runs use a fresh SessionID per execution (see package cronsession). Heartbeat uses a fresh SessionID per tick (see package heartbeatsession).
//   - Cron and heartbeat use separate automation.Queue instances. Cron waits for a slot (up to gateway.cron.maxConcurrentRuns fixed at gateway process start, default 1; changing it requires restart). Heartbeat uses try-once on the heartbeat queue only (one slot): it skips only if a previous heartbeat is still running, not because cron is busy—cron and heartbeat may run at the same time (different sessions, different queues). Skipped heartbeat ticks log automation_lane_busy.
//   - Inbound chat uses per-channel session keys and may call the runtime concurrently with automation. Inbound chat does not use automation.Queue.
//   - Pipeline reload and shutdown drain via turnMu (pipeline.Pipeline); queues are admission only.
package gateway
