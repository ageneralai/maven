package cron

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ageneralai/maven/pkg/executor"
	mavenlog "github.com/ageneralai/maven/pkg/log"
)

var testLG = mavenlog.Std()

func newTestService(t *testing.T) *Service {
	t.Helper()
	return NewService(filepath.Join(t.TempDir(), "jobs.json"), executor.Nop{}, 1, testLG, nil)
}

func TestNewCronJob(t *testing.T) {
	job := NewCronJob("test", Schedule{Kind: "cron", Expr: "0 * * * *"}, Payload{Message: "hello"})
	if job.ID == "" {
		t.Error("job ID should not be empty")
	}
	if job.Name != "test" {
		t.Errorf("name = %q, want test", job.Name)
	}
	if !job.Enabled {
		t.Error("job should be enabled by default")
	}
	if job.Payload.Message != "hello" {
		t.Errorf("message = %q, want hello", job.Payload.Message)
	}
}

func TestEnableJobCron(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewService(filepath.Join(tmpDir, "jobs.json"), executor.Nop{}, 1, testLG, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	j, err := s.AddJob("c", Schedule{Kind: "cron", Expr: "0 0 * * * *"}, Payload{Message: "x"})
	if err != nil {
		t.Fatalf("AddJob: %v", err)
	}
	if j.State.NextRunAtMs == 0 {
		t.Fatal("cron job should have next run scheduled")
	}
	if _, err := s.EnableJob(j.ID, false); err != nil {
		t.Fatalf("EnableJob disable: %v", err)
	}
	disabled := s.ListJobs()[0]
	if disabled.Enabled || disabled.State.NextRunAtMs != 0 {
		t.Fatalf("disabled job: %+v", disabled)
	}
	if _, err := s.EnableJob(j.ID, true); err != nil {
		t.Fatalf("EnableJob enable: %v", err)
	}
	enabled := s.ListJobs()[0]
	if !enabled.Enabled || enabled.State.NextRunAtMs == 0 {
		t.Fatalf("enabled job missing schedule: %+v", enabled)
	}
	s.Stop()
}

func TestAddJobEveryAt(t *testing.T) {
	s := newTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := s.AddJob("e", Schedule{Kind: "every", EveryMs: 1000}, Payload{}); err != nil {
		t.Fatalf("AddJob every: %v", err)
	}
	if _, err := s.AddJob("a", Schedule{Kind: "at", AtMs: time.Now().UnixMilli() + 60000}, Payload{}); err != nil {
		t.Fatalf("AddJob at: %v", err)
	}
	jobs := s.ListJobs()
	if len(jobs) != 2 {
		t.Fatalf("jobs=%d want 2", len(jobs))
	}
	s.Stop()
}

func TestAtJobPersistsDisabledBeforeFire(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "jobs.json")
	var sawDisabled atomic.Bool
	exec := stubExec{func(ctx context.Context, prompt, sessionID string) (string, error) {
		data, err := os.ReadFile(storePath)
		if err != nil {
			t.Fatalf("read store: %v", err)
		}
		var stored []CronJob
		if err := json.Unmarshal(data, &stored); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(stored) != 1 {
			t.Fatalf("len=%d", len(stored))
		}
		if stored[0].Enabled {
			t.Fatal("expected job disabled in store before handler runs")
		}
		sawDisabled.Store(true)
		return "ok", nil
	}}
	s := NewService(storePath, exec, 1, testLG, nil)
	at := time.Now().UnixMilli()
	if _, err := s.AddJob("one", Schedule{Kind: "at", AtMs: at}, Payload{Message: "m"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for !sawDisabled.Load() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	s.Stop()
	if !sawDisabled.Load() {
		t.Fatal("executor not run")
	}
}

func TestService_AddAndListJobs(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "jobs.json")
	s := NewService(storePath, executor.Nop{}, 1, testLG, nil)
	job, err := s.AddJob("job1", Schedule{Kind: "every", EveryMs: 60000}, Payload{Message: "tick"})
	if err != nil {
		t.Fatalf("AddJob error: %v", err)
	}
	if job.Name != "job1" {
		t.Errorf("name = %q, want job1", job.Name)
	}
	jobs := s.ListJobs()
	if len(jobs) != 1 {
		t.Fatalf("len(jobs) = %d, want 1", len(jobs))
	}
	data, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatalf("read store: %v", err)
	}
	var stored []CronJob
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(stored) != 1 {
		t.Errorf("stored jobs = %d, want 1", len(stored))
	}
}

func TestService_RemoveJob(t *testing.T) {
	s := newTestService(t)
	job, _ := s.AddJob(" rm-test", Schedule{Kind: "every", EveryMs: 1000}, Payload{Message: "x"})
	if !s.RemoveJob(job.ID) {
		t.Error("RemoveJob returned false")
	}
	if len(s.ListJobs()) != 0 {
		t.Error("job not removed")
	}
	if s.RemoveJob("nonexistent") {
		t.Error("RemoveJob should return false for nonexistent")
	}
}

func TestService_EnableJob(t *testing.T) {
	s := newTestService(t)
	job, _ := s.AddJob("toggle", Schedule{Kind: "every", EveryMs: 1000}, Payload{Message: "x"})
	updated, err := s.EnableJob(job.ID, false)
	if err != nil {
		t.Fatalf("EnableJob error: %v", err)
	}
	if updated.Enabled {
		t.Error("job should be disabled")
	}
	updated, err = s.EnableJob(job.ID, true)
	if err != nil {
		t.Fatalf("EnableJob error: %v", err)
	}
	if !updated.Enabled {
		t.Error("job should be enabled")
	}
	if _, err = s.EnableJob("nonexistent", true); err == nil {
		t.Error("expected error for nonexistent job")
	}
}

func TestService_StartStop(t *testing.T) {
	s := newTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	cancel()
	s.Stop()
}

func TestService_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "jobs.json")
	s1 := NewService(storePath, executor.Nop{}, 1, testLG, nil)
	if _, err := s1.AddJob("persist1", Schedule{Kind: "every", EveryMs: 1000}, Payload{Message: "p1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s1.AddJob("persist2", Schedule{Kind: "every", EveryMs: 2000}, Payload{Message: "p2"}); err != nil {
		t.Fatal(err)
	}
	s2 := NewService(storePath, executor.Nop{}, 1, testLG, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s2.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	jobs := s2.ListJobs()
	if len(jobs) != 2 {
		t.Fatalf("expected 2 persisted jobs, got %d", len(jobs))
	}
	s2.Stop()
}

func TestService_ExecutePath_Error(t *testing.T) {
	exec := stubExec{func(ctx context.Context, prompt, sessionID string) (string, error) {
		return "", context.Canceled
	}}
	s := NewService(filepath.Join(t.TempDir(), "jobs.json"), exec, 1, testLG, nil)
	job, _ := s.AddJob("err", Schedule{Kind: "every", EveryMs: 500}, Payload{Message: "x"})
	ctx, cancel := context.WithCancel(context.Background())
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		jobs := s.ListJobs()
		if len(jobs) == 1 && jobs[0].State.LastStatus == "error" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	s.Stop()
	jobs := s.ListJobs()
	if len(jobs) != 1 {
		t.Fatalf("jobs=%v", jobs)
	}
	if jobs[0].ID != job.ID {
		t.Fatal("job id mismatch")
	}
	if jobs[0].State.LastStatus != "error" {
		t.Fatalf("status=%q", jobs[0].State.LastStatus)
	}
}

func TestService_RemoveCronJob(t *testing.T) {
	s := newTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	job, err := s.AddJob("rc", Schedule{Kind: "cron", Expr: "0 0 * * * *"}, Payload{Message: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if !s.RemoveJob(job.ID) {
		t.Fatal("RemoveJob")
	}
	if len(s.ListJobs()) != 0 {
		t.Fatal("expected no jobs")
	}
	s.Stop()
}

func TestService_CronJobWithInvalidExpr(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "jobs.json")
	jobs := []CronJob{{
		ID: "bad-cron", Name: "invalid-cron", Enabled: true,
		Schedule: Schedule{Kind: "cron", Expr: "invalid"},
		Payload:  Payload{Message: "x"},
	}}
	data, _ := json.MarshalIndent(jobs, "", "  ")
	if err := os.WriteFile(storePath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewService(storePath, executor.Nop{}, 1, testLG, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Errorf("Start: %v", err)
	}
	s.Stop()
}
