// Package health is Maven's minimal liveness backbone: coarse Pulse signals for probes
// or external observers. Implementations must finish quickly (no gateway re-entry).
package health

// Signal names for HealthReporter.Pulse; treat as stable telemetry keys.
const (
	SignalGatewayReady     = "gateway.ready"
	SignalHeartbeatTick    = "heartbeat.tick"
	SignalDeliveryFailed   = "delivery.failed"
)

// HealthReporter receives liveness taps (gateway started main loops, heartbeat ticker fired, etc.).
type HealthReporter interface {
	Pulse(signal string)
}

// NoOp drops all pulses.
type NoOp struct{}

func (NoOp) Pulse(string) {}

// OrHealthReporter returns a non-nil reporter; nil becomes NoOp.
func OrHealthReporter(h HealthReporter) HealthReporter {
	if h == nil {
		return NoOp{}
	}
	return h
}
