// Package gateway bridges inbound chat channels and background automation.
//
// Session contract:
//   - Cron runs use a fresh SessionID per execution (see package cronsession). Heartbeat uses a fresh SessionID per tick (see package heartbeatsession).
//   - Cron and heartbeat share one lane (agentMu); at most one unattended turn runs at a time.
//   - Inbound chat uses per-channel session keys and may call the runtime concurrently with automation. Inbound chat does not acquire agentMu.
//
// When heartbeat fires while the automation lane is busy, the tick skips and logs reason automation_lane_busy.
package gateway
