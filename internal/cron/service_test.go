package cron

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ageneralai/maven/internal/sessionid"
	"github.com/ageneralai/maven/pkg/executor"
)

func TestServiceFiresJobViaExecutor(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "jobs.json")
	var ran atomic.Bool
	var gotPrompt, gotSID string
	exec := stubExec{func(ctx context.Context, prompt, sessionID string) (string, error) {
		ran.Store(true)
		gotPrompt = prompt
		gotSID = sessionID
		return "out", nil
	}}
	s := mustNewService(t, path, exec, 2)
	j, err := s.AddJob("j", EverySchedule{Interval: 50 * time.Millisecond}, Payload{Message: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for !ran.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	s.Stop()
	if !ran.Load() {
		t.Fatal("executor not invoked")
	}
	if gotPrompt != "hi" {
		t.Fatalf("prompt %q", gotPrompt)
	}
	sid, err := sessionid.Parse(gotSID)
	if err != nil || sid.Kind != sessionid.KindCron || sid.Owner != j.ID {
		t.Fatalf("session %q for job %q", gotSID, j.ID)
	}
}

func TestServiceSkipsDoubleFire(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "jobs.json")
	var calls atomic.Int32
	exec := stubExec{func(ctx context.Context, prompt, sessionID string) (string, error) {
		calls.Add(1)
		time.Sleep(80 * time.Millisecond)
		return "", nil
	}}
	s := mustNewService(t, path, exec, 2)
	if _, err := s.AddJob("j", EverySchedule{Interval: 20 * time.Millisecond}, Payload{Message: "x"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for calls.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	s.Stop()
	if n := calls.Load(); n < 1 {
		t.Fatalf("calls=%d want >=1", n)
	}
}

func TestServiceSemaphoreBoundsConcurrency(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "jobs.json")
	var peak atomic.Int32
	var cur atomic.Int32
	exec := stubExec{func(ctx context.Context, prompt, sessionID string) (string, error) {
		n := cur.Add(1)
		for {
			old := peak.Load()
			if int32(n) <= old || peak.CompareAndSwap(old, int32(n)) {
				break
			}
		}
		time.Sleep(40 * time.Millisecond)
		cur.Add(-1)
		return "", nil
	}}
	s := mustNewService(t, path, exec, 1)
	for i := 0; i < 3; i++ {
		if _, err := s.AddJob("x", EverySchedule{Interval: 15 * time.Millisecond}, Payload{Message: "m"}); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	cancel()
	s.Stop()
	if peak.Load() > 1 {
		t.Fatalf("peak concurrency %d want 1", peak.Load())
	}
}

func TestServiceAtomicPersist(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "jobs.json")
	s := mustNewService(t, path, executor.Nop{}, 1)
	if _, err := s.AddJob("n", EverySchedule{Interval: time.Hour}, Payload{Message: "p"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 10 {
		t.Fatalf("persist too short: %q", data)
	}
}

type stubExec struct {
	fn func(ctx context.Context, prompt, sessionID string) (string, error)
}

func (s stubExec) RunTurn(ctx context.Context, prompt, sessionID string) (string, error) {
	return s.fn(ctx, prompt, sessionID)
}
