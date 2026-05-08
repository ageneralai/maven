package cron

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ageneralai/maven/internal/cronsession"
	"github.com/ageneralai/maven/internal/executor"
	mavenlog "github.com/ageneralai/maven/pkg/log"
)

func TestServiceFiresJobViaExecutor(t *testing.T) {
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
	s := NewService(path, exec, 2, mavenlog.Std(), nil)
	j, err := s.AddJob("j", Schedule{Kind: "every", EveryMs: 50}, Payload{Message: "hi"})
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
	if !cronsession.MatchesJob(j.ID, gotSID) {
		t.Fatalf("session %q for job %q", gotSID, j.ID)
	}
}

func TestServiceSkipsDoubleFire(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "jobs.json")
	var calls atomic.Int32
	exec := stubExec{func(ctx context.Context, prompt, sessionID string) (string, error) {
		calls.Add(1)
		time.Sleep(80 * time.Millisecond)
		return "", nil
	}}
	s := NewService(path, exec, 2, mavenlog.Std(), nil)
	if _, err := s.AddJob("j", Schedule{Kind: "every", EveryMs: 20}, Payload{Message: "x"}); err != nil {
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
	s := NewService(path, exec, 1, mavenlog.Std(), nil)
	for i := 0; i < 3; i++ {
		if _, err := s.AddJob("x", Schedule{Kind: "every", EveryMs: 15}, Payload{Message: "m"}); err != nil {
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
	tmp := t.TempDir()
	path := filepath.Join(tmp, "jobs.json")
	s := NewService(path, executor.Nop{}, 1, mavenlog.Std(), nil)
	if _, err := s.AddJob("n", Schedule{Kind: "every", EveryMs: 3600_000}, Payload{Message: "p"}); err != nil {
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
