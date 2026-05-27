package heartbeat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ageneralai/maven/internal/health"
	"github.com/ageneralai/maven/internal/testutil"
	mavenlog "github.com/ageneralai/maven/pkg/log"
)

var testLG = mavenlog.Std()

func mustNewHeartbeat(t *testing.T, workspace string, exec stubExec, interval time.Duration, opts ...Option) *Service {
	t.Helper()
	s, err := New(workspace, exec, interval, testLG, opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func writeHeartbeatPromptFile(t *testing.T, dir string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "HEARTBEAT.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

type stubExec struct {
	fn func(ctx context.Context, prompt, sessionID string) (string, error)
}

func (s stubExec) RunTurn(ctx context.Context, prompt, sessionID string) (string, error) {
	if s.fn != nil {
		return s.fn(ctx, prompt, sessionID)
	}
	return "", nil
}

func TestNew(t *testing.T) {
	s := mustNewHeartbeat(t, "/tmp/ws", stubExec{}, 0)
	if s == nil {
		t.Fatal("New returned nil")
	}
	if s.interval != 30*time.Minute {
		t.Errorf("default interval = %v, want 30m", s.interval)
	}
}

func TestNew_CustomInterval(t *testing.T) {
	s := mustNewHeartbeat(t, "/tmp/ws", stubExec{}, 5*time.Minute)
	if s.interval != 5*time.Minute {
		t.Errorf("interval = %v, want 5m", s.interval)
	}
}

func TestTick_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	var called atomic.Int32
	s := mustNewHeartbeat(t, tmpDir, stubExec{func(ctx context.Context, prompt, sessionID string) (string, error) {
		called.Add(1)
		return "ok", nil
	}}, time.Second)
	s.tick(context.Background())
	if called.Load() != 0 {
		t.Error("executor should not be called when HEARTBEAT.md doesn't exist")
	}
}

func TestTick_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	writeHeartbeatPromptFile(t, tmpDir, "")
	var called atomic.Int32
	s := mustNewHeartbeat(t, tmpDir, stubExec{func(ctx context.Context, prompt, sessionID string) (string, error) {
		called.Add(1)
		return "ok", nil
	}}, time.Second)
	s.tick(context.Background())
	time.Sleep(20 * time.Millisecond)
	if called.Load() != 0 {
		t.Error("executor should not be called for empty HEARTBEAT.md")
	}
}

func TestTick_WithContent(t *testing.T) {
	tmpDir := t.TempDir()
	writeHeartbeatPromptFile(t, tmpDir, "Check tasks")
	var receivedPrompt string
	s := mustNewHeartbeat(t, tmpDir, stubExec{func(ctx context.Context, prompt, sessionID string) (string, error) {
		receivedPrompt = prompt
		return "done", nil
	}}, time.Second)
	s.tick(context.Background())
	time.Sleep(50 * time.Millisecond)
	if receivedPrompt != "Check tasks" {
		t.Errorf("prompt = %q, want 'Check tasks'", receivedPrompt)
	}
}

func TestStart_ContextCancel(t *testing.T) {
	tmpDir := t.TempDir()
	s := mustNewHeartbeat(t, tmpDir, stubExec{}, 100*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- s.Start(ctx)
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Start returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not exit after context cancel")
	}
}

func TestStart_TickerFires(t *testing.T) {
	tmpDir := t.TempDir()
	writeHeartbeatPromptFile(t, tmpDir, "tick")
	var tickCount atomic.Int32
	s := mustNewHeartbeat(t, tmpDir, stubExec{func(ctx context.Context, prompt, sessionID string) (string, error) {
		tickCount.Add(1)
		return "ok", nil
	}}, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- s.Start(ctx)
	}()
	time.Sleep(150 * time.Millisecond)
	cancel()
	<-done
	if tickCount.Load() == 0 {
		t.Error("expected at least one tick")
	}
}

func TestTick_HandlerError(t *testing.T) {
	tmpDir := t.TempDir()
	writeHeartbeatPromptFile(t, tmpDir, "Check tasks")
	s := mustNewHeartbeat(t, tmpDir, stubExec{func(ctx context.Context, prompt, sessionID string) (string, error) {
		return "", fmt.Errorf("handler error")
	}}, time.Second)
	s.tick(context.Background())
	time.Sleep(30 * time.Millisecond)
}

func TestTick_HeartbeatOK(t *testing.T) {
	tmpDir := t.TempDir()
	writeHeartbeatPromptFile(t, tmpDir, "Check tasks")
	var called bool
	s := mustNewHeartbeat(t, tmpDir, stubExec{func(ctx context.Context, prompt, sessionID string) (string, error) {
		called = true
		return "HEARTBEAT_OK - nothing to do", nil
	}}, time.Second)
	s.tick(context.Background())
	time.Sleep(30 * time.Millisecond)
	if !called {
		t.Error("executor should be called")
	}
}

func TestHeartbeatSkipsIfBusy(t *testing.T) {
	tmpDir := t.TempDir()
	writeHeartbeatPromptFile(t, tmpDir, "hb")
	var runs atomic.Int32
	block := make(chan struct{})
	defer close(block)
	s := mustNewHeartbeat(t, tmpDir, stubExec{func(ctx context.Context, prompt, sessionID string) (string, error) {
		runs.Add(1)
		<-block
		return "", nil
	}}, 20*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = s.Start(ctx) }()
	time.Sleep(5 * time.Millisecond)
	s.tick(ctx)
	time.Sleep(40 * time.Millisecond)
	cancel()
	if runs.Load() != 1 {
		t.Fatalf("runs=%d want 1 (second tick skipped)", runs.Load())
	}
}

func TestHeartbeatSkipsEmptyPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	var runs atomic.Int32
	s := mustNewHeartbeat(t, tmpDir, stubExec{func(ctx context.Context, prompt, sessionID string) (string, error) {
		runs.Add(1)
		return "", nil
	}}, time.Second)
	s.tick(context.Background())
	time.Sleep(20 * time.Millisecond)
	if runs.Load() != 0 {
		t.Fatal("RunTurn should not run without HEARTBEAT.md")
	}
}

func TestHeartbeatFreshSessionPerTick(t *testing.T) {
	tmpDir := t.TempDir()
	writeHeartbeatPromptFile(t, tmpDir, "x")
	var mu sync.Mutex
	var ids []string
	s := mustNewHeartbeat(t, tmpDir, stubExec{func(ctx context.Context, prompt, sessionID string) (string, error) {
		mu.Lock()
		ids = append(ids, sessionID)
		mu.Unlock()
		return "ok", nil
	}}, time.Second)
	s.tick(context.Background())
	time.Sleep(40 * time.Millisecond)
	s.tick(context.Background())
	time.Sleep(40 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if len(ids) != 2 {
		t.Fatalf("sessions=%d want 2", len(ids))
	}
	if ids[0] == ids[1] {
		t.Fatal("expected distinct session ids")
	}
	if !MatchesSession(ids[0]) || !MatchesSession(ids[1]) {
		t.Fatalf("not heartbeat sessions: %v", ids)
	}
}

func TestStart_PulsesHeartbeatHealthSignal(t *testing.T) {
	tmpDir := t.TempDir()
	writeHeartbeatPromptFile(t, tmpDir, "noop")
	var rec testutil.PulseRecorder
	s := mustNewHeartbeat(t, tmpDir, stubExec{func(context.Context, string, string) (string, error) {
		return "HEARTBEAT_OK", nil
	}}, 25*time.Millisecond, WithHealthReporter(&rec))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- s.Start(ctx)
	}()
	time.Sleep(60 * time.Millisecond)
	cancel()
	<-done
	if rec.Count(health.SignalHeartbeatTick) == 0 {
		t.Fatal("want at least one health.SignalHeartbeatTick")
	}
}
