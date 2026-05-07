package automation

import (
	"context"
	"sync"
)

// Lane serializes unattended agent turns (cron + heartbeat). Inbound chat
// traffic does not use this lane.
type Lane struct {
	mu sync.Mutex
}

// TryRun executes fn while holding the lane lock. If the lock is not available,
// returns ran=false and err=nil.
func (l *Lane) TryRun(ctx context.Context, fn func(context.Context) error) (ran bool, err error) {
	if !l.mu.TryLock() {
		return false, nil
	}
	defer l.mu.Unlock()
	return true, fn(ctx)
}

// RunAlways holds the lane for the duration of fn.
func (l *Lane) RunAlways(ctx context.Context, fn func(context.Context) error) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return fn(ctx)
}
