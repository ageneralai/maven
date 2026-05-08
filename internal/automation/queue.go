package automation

import (
	"context"
	"fmt"

	"golang.org/x/sync/semaphore"
)

// Queue bounds concurrent work for one logical lane (e.g. cron-only or heartbeat-only).
// Inbound chat does not use Queue; see pipeline.Pipeline and turnMu for reload/shutdown.
//
// Run blocks until a slot is free or ctx is cancelled. With maxConcurrent==1, behavior
// matches a mutex. With maxConcurrent>1, up to that many runs proceed in parallel;
// additional callers wait in FIFO order on the semaphore.
type Queue struct {
	sem *semaphore.Weighted
}

// NewQueue allows at most maxConcurrent concurrent runs per lane. If maxConcurrent < 1, uses 1.
func NewQueue(maxConcurrent int) *Queue {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &Queue{sem: semaphore.NewWeighted(int64(maxConcurrent))}
}

// Run acquires a slot, runs fn, releases the slot. Returns ctx error if cancelled while waiting.
func (q *Queue) Run(ctx context.Context, fn func(context.Context) error) error {
	if err := q.sem.Acquire(ctx, 1); err != nil {
		return fmt.Errorf("automation queue acquire: %w", err)
	}
	defer q.sem.Release(1)
	return fn(ctx)
}

// TryRun acquires a slot only if one is available immediately; otherwise returns ran=false.
func (q *Queue) TryRun(ctx context.Context, fn func(context.Context) error) (ran bool, err error) {
	if !q.sem.TryAcquire(1) {
		return false, nil
	}
	defer q.sem.Release(1)
	return true, fn(ctx)
}
