package health

import (
	"sync"
	"testing"
)

func TestNoOp_Pulse(t *testing.T) {
	var n NoOp
	n.Pulse(SignalGatewayReady)
	n.Pulse(SignalHeartbeatTick)
}

func TestOrHealthReporter_nil(t *testing.T) {
	var h HealthReporter = OrHealthReporter(nil)
	h.Pulse(SignalGatewayReady)
	if _, ok := h.(NoOp); !ok {
		t.Fatalf("want NoOp, got %T", h)
	}
}

func TestOrHealthReporter_preserves(t *testing.T) {
	var c capture
	h := OrHealthReporter(&c)
	h.Pulse(SignalHeartbeatTick)
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.signals) != 1 || c.signals[0] != SignalHeartbeatTick {
		t.Fatalf("signals=%v", c.signals)
	}
}

type capture struct {
	mu      sync.Mutex
	signals []string
}

func (c *capture) Pulse(s string) {
	c.mu.Lock()
	c.signals = append(c.signals, s)
	c.mu.Unlock()
}
