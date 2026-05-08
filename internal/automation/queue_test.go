package automation

import (
	"context"
	"testing"
)

func TestQueue_TryRun_SkipsWhenBusy(t *testing.T) {
	q := NewQueue(1)
	holding := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = q.Run(context.Background(), func(ctx context.Context) error {
			close(holding)
			<-release
			return nil
		})
	}()
	<-holding
	defer close(release)
	ran, err := q.TryRun(context.Background(), func(context.Context) error {
		t.Fatal("fn should not run when slot is held")
		return nil
	})
	if err != nil || ran {
		t.Fatalf("ran=%v err=%v", ran, err)
	}
}

func TestQueue_TryRun_RunsFn(t *testing.T) {
	q := NewQueue(1)
	var ranFn bool
	ran, err := q.TryRun(context.Background(), func(context.Context) error {
		ranFn = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ran || !ranFn {
		t.Fatalf("ran=%v ranFn=%v", ran, ranFn)
	}
}

func TestQueue_Run_ContextCancel(t *testing.T) {
	q := NewQueue(1)
	holding := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = q.Run(context.Background(), func(ctx context.Context) error {
			close(holding)
			<-release
			return nil
		})
	}()
	<-holding
	defer close(release)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := q.Run(ctx, func(context.Context) error {
		t.Fatal("fn should not run with cancelled ctx")
		return nil
	})
	if err == nil {
		t.Fatal("expected error from cancelled ctx acquire")
	}
}

func TestQueue_NewQueue_zeroIsOne(t *testing.T) {
	q := NewQueue(0)
	holding := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = q.Run(context.Background(), func(context.Context) error {
			close(holding)
			<-release
			return nil
		})
	}()
	<-holding
	defer close(release)
	ran, _ := q.TryRun(context.Background(), func(context.Context) error { return nil })
	if ran {
		t.Fatal("maxConcurrent 0 normalizes to 1: try should fail while holder runs")
	}
}

func TestQueue_N_ConcurrentRuns(t *testing.T) {
	const n = 3
	q := NewQueue(n)
	started := make(chan struct{}, n)
	release := make(chan struct{})
	for i := 0; i < n; i++ {
		go func() {
			_ = q.Run(context.Background(), func(ctx context.Context) error {
				started <- struct{}{}
				<-release
				return nil
			})
		}()
	}
	for i := 0; i < n; i++ {
		<-started
	}
	ran, _ := q.TryRun(context.Background(), func(context.Context) error { return nil })
	if ran {
		t.Fatal("should not acquire when all n slots held")
	}
	close(release)
}
