package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	mavenlog "github.com/ageneralai/maven/internal/log"
)

var testLG = mavenlog.Std()

func TestNewService(t *testing.T) {
	s := NewService(filepath.Join(t.TempDir(), "jobs.json"), testLG)
	if s.wakeChan == nil {
		t.Fatal("wakeChan should be initialized")
	}
	if s.entryMap == nil {
		t.Fatal("entryMap should be initialized")
	}
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
	s := NewService(filepath.Join(tmpDir, "jobs.json"), testLG)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	j, err := s.AddJob("c", Schedule{Kind: "cron", Expr: "0 0 * * * *"}, Payload{Message: "x"})
	if err != nil {
		t.Fatalf("AddJob: %v", err)
	}
	if len(s.entryMap) != 1 {
		t.Fatalf("entryMap after add = %d, want 1", len(s.entryMap))
	}
	if _, err := s.EnableJob(j.ID, false); err != nil {
		t.Fatalf("EnableJob disable: %v", err)
	}
	if len(s.entryMap) != 0 {
		t.Fatalf("entryMap after disable = %d, want 0", len(s.entryMap))
	}
	if _, err := s.EnableJob(j.ID, true); err != nil {
		t.Fatalf("EnableJob enable: %v", err)
	}
	if len(s.entryMap) != 1 {
		t.Fatalf("entryMap after re-enable = %d, want 1", len(s.entryMap))
	}
	s.Stop()
}

func TestAddJobEveryAt(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewService(filepath.Join(tmpDir, "jobs.json"), testLG)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	before := len(s.entryMap)
	if _, err := s.AddJob("e", Schedule{Kind: "every", EveryMs: 1000}, Payload{}); err != nil {
		t.Fatalf("AddJob every: %v", err)
	}
	if _, err := s.AddJob("a", Schedule{Kind: "at", AtMs: time.Now().UnixMilli() + 60000}, Payload{}); err != nil {
		t.Fatalf("AddJob at: %v", err)
	}
	if len(s.entryMap) != before {
		t.Fatalf("entryMap changed for non-cron jobs: before=%d after=%d", before, len(s.entryMap))
	}
	s.Stop()
}

func TestComputeMinDelayNeverRunEvery(t *testing.T) {
	s := NewService(filepath.Join(t.TempDir(), "jobs.json"), testLG)
	s.jobs = []CronJob{{
		ID: "e", Name: "e", Enabled: true,
		Schedule: Schedule{Kind: "every", EveryMs: 60000},
		State:    JobState{LastRunAtMs: 0},
	}}
	got := s.computeMinDelay()
	want := 60 * time.Second
	const slop = 10 * time.Millisecond
	if got < want-slop || got > want+slop {
		t.Fatalf("computeMinDelay = %v, want %v ± %v", got, want, slop)
	}
}

func TestComputeMinDelayIdle(t *testing.T) {
	s := NewService(filepath.Join(t.TempDir(), "jobs.json"), testLG)
	if d := s.computeMinDelay(); d != time.Hour {
		t.Fatalf("computeMinDelay = %v, want %v", d, time.Hour)
	}
}

func TestComputeMinDelayMixed(t *testing.T) {
	now := time.Now().UnixMilli()
	s := NewService(filepath.Join(t.TempDir(), "jobs.json"), testLG)
	s.jobs = []CronJob{
		{ID: "a", Name: "a", Enabled: true, Schedule: Schedule{Kind: "at", AtMs: now + 3600_000}},
		{ID: "b", Name: "b", Enabled: true, Schedule: Schedule{Kind: "at", AtMs: now + 500}},
	}
	got := s.computeMinDelay()
	want := 500 * time.Millisecond
	const slop = 50 * time.Millisecond
	if got < want-slop || got > want+slop {
		t.Fatalf("computeMinDelay = %v, want ~%v", got, want)
	}
}

func TestCheckDueJobsAtDisable(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "jobs.json")
	s := NewService(storePath, testLG)
	at := time.Now().UnixMilli()
	job := NewCronJob("one", Schedule{Kind: "at", AtMs: at}, Payload{Message: "m"})
	s.jobs = []CronJob{job}
	_ = s.save()
	var sawPersisted bool
	s.OnJob = func(j CronJob) (string, error) {
		data, err := os.ReadFile(storePath)
		if err != nil {
			t.Fatalf("read store in handler: %v", err)
		}
		var stored []CronJob
		if err := json.Unmarshal(data, &stored); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(stored) != 1 {
			t.Fatalf("stored len = %d", len(stored))
		}
		if stored[0].Enabled {
			t.Fatal("expected job disabled in store before handler runs")
		}
		if stored[0].State.NextRunAtMs != 0 {
			t.Fatalf("NextRunAtMs = %d, want 0", stored[0].State.NextRunAtMs)
		}
		if stored[0].ID != j.ID {
			t.Fatal("stored job id mismatch")
		}
		sawPersisted = true
		return "ok", nil
	}
	s.checkDueJobs(at + 1)
	if !sawPersisted {
		t.Fatal("OnJob not invoked")
	}
}

func TestService_AddAndListJobs(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "jobs.json")
	s := NewService(storePath, testLG)

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
	if jobs[0].Name != "job1" {
		t.Errorf("jobs[0].name = %q, want job1", jobs[0].Name)
	}

	// Verify persistence
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
	tmpDir := t.TempDir()
	s := NewService(filepath.Join(tmpDir, "jobs.json"), testLG)

	job, _ := s.AddJob("rm-test", Schedule{Kind: "every", EveryMs: 1000}, Payload{Message: "x"})

	if !s.RemoveJob(job.ID) {
		t.Error("RemoveJob returned false")
	}
	if len(s.ListJobs()) != 0 {
		t.Error("job not removed")
	}

	// Remove nonexistent
	if s.RemoveJob("nonexistent") {
		t.Error("RemoveJob should return false for nonexistent")
	}
}

func TestService_EnableJob(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewService(filepath.Join(tmpDir, "jobs.json"), testLG)

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

	// Nonexistent job
	_, err = s.EnableJob("nonexistent", true)
	if err == nil {
		t.Error("expected error for nonexistent job")
	}
}

func TestService_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewService(filepath.Join(tmpDir, "jobs.json"), testLG)

	ctx, cancel := context.WithCancel(context.Background())

	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	cancel()
	s.Stop()
}

func TestService_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "jobs.json")

	// Add jobs with first service
	s1 := NewService(storePath, testLG)
	s1.AddJob("persist1", Schedule{Kind: "every", EveryMs: 1000}, Payload{Message: "p1"})
	s1.AddJob("persist2", Schedule{Kind: "every", EveryMs: 2000}, Payload{Message: "p2"})

	// Load with second service
	s2 := NewService(storePath, testLG)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s2.Start(ctx)

	jobs := s2.ListJobs()
	if len(jobs) != 2 {
		t.Fatalf("expected 2 persisted jobs, got %d", len(jobs))
	}
	s2.Stop()
}

func TestService_ExecuteJob_WithHandler(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewService(filepath.Join(tmpDir, "jobs.json"), testLG)

	var executed bool
	var receivedJob CronJob
	s.OnJob = func(job CronJob) (string, error) {
		executed = true
		receivedJob = job
		return "success", nil
	}

	job, _ := s.AddJob("exec-test", Schedule{Kind: "every", EveryMs: 1000}, Payload{Message: "test msg"})

	// Directly call executeJob
	s.executeJob(*job)

	if !executed {
		t.Error("OnJob handler was not called")
	}
	if receivedJob.Name != "exec-test" {
		t.Errorf("job name = %q, want exec-test", receivedJob.Name)
	}

	// Check state was updated
	jobs := s.ListJobs()
	if len(jobs) == 0 {
		t.Fatal("no jobs found")
	}
	if jobs[0].State.LastStatus != "ok" {
		t.Errorf("lastStatus = %q, want ok", jobs[0].State.LastStatus)
	}
}

func TestService_ExecuteJob_NoHandler(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewService(filepath.Join(tmpDir, "jobs.json"), testLG)

	job, _ := s.AddJob("no-handler", Schedule{Kind: "every", EveryMs: 1000}, Payload{Message: "x"})

	// Should not panic when OnJob is nil
	s.executeJob(*job)
}

func TestService_ExecuteJob_HandlerError(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewService(filepath.Join(tmpDir, "jobs.json"), testLG)

	s.OnJob = func(job CronJob) (string, error) {
		return "", fmt.Errorf("handler error")
	}

	job, _ := s.AddJob("error-test", Schedule{Kind: "every", EveryMs: 1000}, Payload{Message: "x"})
	s.executeJob(*job)

	jobs := s.ListJobs()
	if jobs[0].State.LastStatus != "error" {
		t.Errorf("lastStatus = %q, want error", jobs[0].State.LastStatus)
	}
	if jobs[0].State.LastError != "handler error" {
		t.Errorf("lastError = %q, want 'handler error'", jobs[0].State.LastError)
	}
}

func TestService_ExecuteJob_DeleteAfterRun(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewService(filepath.Join(tmpDir, "jobs.json"), testLG)

	s.OnJob = func(job CronJob) (string, error) {
		return "done", nil
	}

	// Add job with DeleteAfterRun set
	job := NewCronJob("delete-me", Schedule{Kind: "at", AtMs: time.Now().UnixMilli()}, Payload{Message: "x"})
	job.DeleteAfterRun = true
	s.jobs = append(s.jobs, job)
	_ = s.save()

	s.executeJob(job)

	jobs := s.ListJobs()
	if len(jobs) != 0 {
		t.Errorf("job should be deleted after run, got %d jobs", len(jobs))
	}
}

func TestService_TickLoop_EverySchedule(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewService(filepath.Join(tmpDir, "jobs.json"), testLG)

	executeCount := 0
	s.OnJob = func(job CronJob) (string, error) {
		executeCount++
		return "tick", nil
	}

	// Add job with 100ms interval, with LastRunAtMs in the past
	job := NewCronJob("fast-tick", Schedule{Kind: "every", EveryMs: 100}, Payload{Message: "tick"})
	job.State.LastRunAtMs = time.Now().UnixMilli() - 200 // Already due
	s.jobs = append(s.jobs, job)

	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)

	// Wait for tickLoop to execute the job
	time.Sleep(1500 * time.Millisecond)

	cancel()
	s.Stop()

	if executeCount == 0 {
		t.Error("expected at least one execution from tickLoop")
	}
}

func TestService_TickLoop_AtSchedule(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewService(filepath.Join(tmpDir, "jobs.json"), testLG)

	executed := false
	s.OnJob = func(job CronJob) (string, error) {
		executed = true
		return "at-job", nil
	}

	// Add "at" job scheduled for now
	job := NewCronJob("at-job", Schedule{Kind: "at", AtMs: time.Now().UnixMilli()}, Payload{Message: "at"})
	s.jobs = append(s.jobs, job)

	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)

	// Wait for tickLoop
	time.Sleep(1500 * time.Millisecond)

	cancel()
	s.Stop()

	if !executed {
		t.Error("at-scheduled job was not executed")
	}
}

func TestService_RegisterCronJob(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewService(filepath.Join(tmpDir, "jobs.json"), testLG)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)

	// Add a cron job - should be registered with the cron scheduler
	_, err := s.AddJob("cron-job", Schedule{Kind: "cron", Expr: "*/1 * * * * *"}, Payload{Message: "cron"})
	if err != nil {
		t.Fatalf("AddJob error: %v", err)
	}

	// Check jobs were added
	jobs := s.ListJobs()
	if len(jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(jobs))
	}

	s.Stop()
}

func TestService_CronJobWithInvalidExpr(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "jobs.json")

	// Create a job file with invalid cron expression
	jobs := []CronJob{{
		ID:       "bad-cron",
		Name:     "invalid-cron",
		Enabled:  true,
		Schedule: Schedule{Kind: "cron", Expr: "invalid"},
		Payload:  Payload{Message: "x"},
	}}
	data, _ := json.MarshalIndent(jobs, "", "  ")
	os.WriteFile(storePath, data, 0644)

	s := NewService(storePath, testLG)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start should handle invalid cron expression gracefully
	err := s.Start(ctx)
	if err != nil {
		t.Errorf("Start should not error on invalid cron: %v", err)
	}

	s.Stop()
}

func TestService_RegisterCronJob_Success(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "jobs.json")

	// Create a job file with valid cron expression
	jobs := []CronJob{{
		ID:       "valid-cron",
		Name:     "valid-cron-job",
		Enabled:  true,
		Schedule: Schedule{Kind: "cron", Expr: "0 0 * * * *"}, // Every hour
		Payload:  Payload{Message: "hourly"},
	}}
	data, _ := json.MarshalIndent(jobs, "", "  ")
	os.WriteFile(storePath, data, 0644)

	s := NewService(storePath, testLG)
	s.OnJob = func(job CronJob) (string, error) {
		return "done", nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := s.Start(ctx)
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}

	// Verify the job was registered in entryMap
	if len(s.entryMap) != 1 {
		t.Errorf("expected 1 entry in entryMap, got %d", len(s.entryMap))
	}

	s.Stop()
}

func TestService_RemoveJob_WithCron(t *testing.T) {
	tmpDir := t.TempDir()
	s := NewService(filepath.Join(tmpDir, "jobs.json"), testLG)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)

	// Add a cron job
	job, _ := s.AddJob("remove-cron", Schedule{Kind: "cron", Expr: "0 0 * * * *"}, Payload{Message: "x"})

	// Verify it's in entryMap
	if len(s.entryMap) != 1 {
		t.Errorf("expected 1 entry in entryMap, got %d", len(s.entryMap))
	}

	// Remove it
	if !s.RemoveJob(job.ID) {
		t.Error("RemoveJob returned false")
	}

	// Verify it's removed from entryMap
	if len(s.entryMap) != 0 {
		t.Errorf("expected 0 entries in entryMap, got %d", len(s.entryMap))
	}

	s.Stop()
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		n     int
		want  string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is longer than ten", 10, "this is lo..."},
		{"", 5, ""},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.n)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
		}
	}
}
