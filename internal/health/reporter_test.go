package health

import (
	"testing"

	"github.com/ageneralai/maven/internal/health/healthtest"
)

func TestNoOp_Pulse(t *testing.T) {
	var n NoOp
	n.Pulse(SignalGatewayReady)
	n.Pulse(SignalHeartbeatTick)
}

func TestOrHealthReporter_nil(t *testing.T) {
	h := OrHealthReporter(nil)
	h.Pulse(SignalGatewayReady)
	if _, ok := h.(NoOp); !ok {
		t.Fatalf("want NoOp, got %T", h)
	}
}

func TestOrHealthReporter_preserves(t *testing.T) {
	var rec healthtest.PulseRecorder
	h := OrHealthReporter(&rec)
	h.Pulse(SignalHeartbeatTick)
	snaps := rec.Snapshot()
	if len(snaps) != 1 || snaps[0] != SignalHeartbeatTick {
		t.Fatalf("signals=%v", snaps)
	}
}
