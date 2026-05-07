// Package gateway bridges inbound chat channels and background automation.
//
// Session contract:
//   - Automation (cron, heartbeat) always uses SessionID "system". Cron and heartbeat share one lane (agentMu); at most one runs at a time.
//   - Inbound chat uses per-channel session keys and may call the runtime concurrently with automation. Inbound chat does not acquire agentMu.
//
// When heartbeat fires while the automation lane is busy, the tick skips and logs reason automation_lane_busy.
package gateway
