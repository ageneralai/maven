// Package testutil holds tiny helpers shared by internal tests.
package testutil

import "sync"

// PulseRecorder implements health.HealthReporter-style Pulse for tests only.
type PulseRecorder struct {
	mu   sync.Mutex
	sigs []string
}

func (r *PulseRecorder) Pulse(s string) {
	r.mu.Lock()
	r.sigs = append(r.sigs, s)
	r.mu.Unlock()
}

// Has reports whether s was recorded at least once.
func (r *PulseRecorder) Has(s string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.sigs {
		if x == s {
			return true
		}
	}
	return false
}

// Count returns how many pulses equal s were recorded.
func (r *PulseRecorder) Count(s string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, x := range r.sigs {
		if x == s {
			n++
		}
	}
	return n
}

// Snapshot returns a copy of recorded pulses in order.
func (r *PulseRecorder) Snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.sigs))
	copy(out, r.sigs)
	return out
}
