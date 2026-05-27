package scheduling

import (
	"context"

	"golang.org/x/sync/semaphore"
)

// Lane caps concurrent scheduled work (cron, heartbeat) via a weighted semaphore.
type Lane struct {
	*semaphore.Weighted
}

func NewLane(maxConcurrent int64) *Lane {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &Lane{Weighted: semaphore.NewWeighted(maxConcurrent)}
}

func (l *Lane) TryAcquire() bool {
	return l.Weighted.TryAcquire(1)
}

func (l *Lane) Acquire(ctx context.Context) error {
	return l.Weighted.Acquire(ctx, 1)
}

func (l *Lane) Release() {
	l.Weighted.Release(1)
}
