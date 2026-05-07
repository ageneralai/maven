package automation

import (
	"context"
	"testing"
)

func TestLane_TryRun_SkipsWhenBusy(t *testing.T) {
	l := &Lane{}
	holding := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = l.RunAlways(context.Background(), func(ctx context.Context) error {
			close(holding)
			<-release
			return nil
		})
	}()
	<-holding
	defer close(release)
	ran, err := l.TryRun(context.Background(), func(context.Context) error {
		t.Fatal("fn should not run when lane is held")
		return nil
	})
	if err != nil || ran {
		t.Fatalf("ran=%v err=%v", ran, err)
	}
}

func TestLane_TryRun_RunsFn(t *testing.T) {
	l := &Lane{}
	var ranFn bool
	ran, err := l.TryRun(context.Background(), func(context.Context) error {
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
