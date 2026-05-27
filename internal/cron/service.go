package cron

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"log/slog"

	"github.com/adhocore/gronx"
	"github.com/ageneralai/maven/pkg/executor"
	"github.com/ageneralai/maven/pkg/stringutil"
	"golang.org/x/sync/semaphore"
)

type Service struct {
	storePath string
	exec      executor.TurnExecutor
	deliver   *Deliver
	sem       *semaphore.Weighted
	log       *slog.Logger
	mu        sync.RWMutex
	jobs      []CronJob
	runCtx    context.Context
	runCancel context.CancelFunc
	stopChan  chan struct{}
	stopOnce  sync.Once
	wakeChan  chan struct{}
}

func NewService(storePath string, exec executor.TurnExecutor, maxConcurrent int, lg *slog.Logger, deliver *Deliver) *Service {
	if exec == nil {
		panic("cron: TurnExecutor is required")
	}
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &Service{
		storePath: storePath,
		exec:      exec,
		deliver:   deliver,
		sem:       semaphore.NewWeighted(int64(maxConcurrent)),
		log:       lg,
		stopChan:  make(chan struct{}),
		wakeChan:  make(chan struct{}, 1),
	}
}

func (s *Service) notify() {
	select {
	case s.wakeChan <- struct{}{}:
	default:
	}
}

func (s *Service) JobByID(id string) (CronJob, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range s.jobs {
		if s.jobs[i].ID == id {
			return s.jobs[i], true
		}
	}
	return CronJob{}, false
}

func (s *Service) findJobIndex(id string) int {
	for i := range s.jobs {
		if s.jobs[i].ID == id {
			return i
		}
	}
	return -1
}

func (s *Service) ensureNextRunLocked(now int64) {
	for i := range s.jobs {
		j := &s.jobs[i]
		if !j.Enabled || j.State.NextRunAtMs > 0 {
			continue
		}
		switch j.Schedule.Kind {
		case "every":
			if j.Schedule.EveryMs <= 0 {
				continue
			}
			if j.State.LastRunAtMs == 0 {
				j.State.NextRunAtMs = now + j.Schedule.EveryMs
			} else {
				j.State.NextRunAtMs = j.State.LastRunAtMs + j.Schedule.EveryMs
			}
		case "at":
			j.State.NextRunAtMs = j.Schedule.AtMs
		case "cron":
			next, err := gronx.NextTickAfter(j.Schedule.Expr, time.UnixMilli(now), false)
			if err != nil {
				s.log.Error("cron job invalid expr", "job", j.Name, "err", err)
				continue
			}
			j.State.NextRunAtMs = next.UnixMilli()
		}
	}
}

func computeNextScheduleRun(sch Schedule, fromMs int64) int64 {
	switch sch.Kind {
	case "at":
		if sch.AtMs > fromMs {
			return sch.AtMs
		}
		return 0
	case "every":
		if sch.EveryMs <= 0 {
			return 0
		}
		return fromMs + sch.EveryMs
	case "cron":
		next, err := gronx.NextTickAfter(sch.Expr, time.UnixMilli(fromMs), false)
		if err != nil {
			return 0
		}
		return next.UnixMilli()
	}
	return 0
}

func (s *Service) nextDelayLocked() time.Duration {
	now := time.Now().UnixMilli()
	var minRem int64 = -1
	for i := range s.jobs {
		j := &s.jobs[i]
		if !j.Enabled || j.State.NextRunAtMs <= 0 {
			continue
		}
		rem := j.State.NextRunAtMs - now
		if rem < 0 {
			rem = 0
		}
		if minRem < 0 || rem < minRem {
			minRem = rem
		}
	}
	if minRem < 0 {
		return time.Hour
	}
	if minRem == 0 {
		return time.Millisecond
	}
	return time.Duration(minRem) * time.Millisecond
}

func drainTimer(t *time.Timer) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
}

func resetTimer(t *time.Timer, d time.Duration) {
	drainTimer(t)
	t.Reset(d)
}

func (s *Service) runLoop() {
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	for {
		s.mu.RLock()
		delay := s.nextDelayLocked()
		s.mu.RUnlock()
		resetTimer(timer, delay)
		select {
		case <-s.stopChan:
			return
		case <-s.wakeChan:
			drainTimer(timer)
			continue
		case <-timer.C:
			s.checkAndFire()
		}
	}
}

func (s *Service) isDue(j *CronJob, now int64) bool {
	if !j.Enabled || j.State.NextRunAtMs <= 0 {
		return false
	}
	return now >= j.State.NextRunAtMs
}

func (s *Service) clearNextRun(j *CronJob) {
	j.State.NextRunAtMs = 0
}

func (s *Service) checkAndFire() {
	s.mu.Lock()
	now := time.Now().UnixMilli()
	var due []CronJob
	for i := range s.jobs {
		j := &s.jobs[i]
		if !s.isDue(j, now) {
			continue
		}
		if j.Schedule.Kind == "at" {
			j.Enabled = false
		}
		due = append(due, *j)
		s.clearNextRun(j)
	}
	_ = s.saveAtomicLocked()
	s.mu.Unlock()
	for _, job := range due {
		job := job
		go s.fire(job)
	}
}

func (s *Service) fire(job CronJob) {
	if err := job.Payload.Validate(); err != nil {
		s.mu.Lock()
		s.applyJobValidationFailure(job.ID, err)
		_ = s.saveAtomicLocked()
		s.mu.Unlock()
		return
	}
	runCtx := s.runCtx
	if err := s.sem.Acquire(runCtx, 1); err != nil {
		return
	}
	defer s.sem.Release(1)
	sessionID := SessionKey(job.ID)
	out, err := s.exec.RunTurn(runCtx, job.Payload.Message, sessionID)
	doneMs := time.Now().UnixMilli()
	s.mu.Lock()
	idx := s.findJobIndex(job.ID)
	if idx < 0 {
		_ = s.saveAtomicLocked()
		s.mu.Unlock()
		return
	}
	j := &s.jobs[idx]
	j.State.LastRunAtMs = doneMs
	if err != nil {
		j.State.LastStatus = "error"
		j.State.LastError = err.Error()
		s.log.Error("cron job error", "job", j.Name, "err", err)
	} else {
		j.State.LastStatus = "ok"
		j.State.LastError = ""
		s.log.Info("cron job result", "job", j.Name, "output", stringutil.Truncate(out, 100))
	}
	if j.Schedule.Kind == "at" {
		j.Enabled = false
		j.State.NextRunAtMs = 0
	}
	if j.DeleteAfterRun {
		s.jobs = append(s.jobs[:idx], s.jobs[idx+1:]...)
	} else if j.Schedule.Kind != "at" {
		j.State.NextRunAtMs = computeNextScheduleRun(j.Schedule, j.State.LastRunAtMs)
	}
	_ = s.saveAtomicLocked()
	s.mu.Unlock()
	if err == nil && s.deliver != nil {
		s.deliver.AfterSuccessfulRun(runCtx, job, out)
	}
}

func (s *Service) applyJobValidationFailure(jobID string, validateErr error) {
	idx := s.findJobIndex(jobID)
	if idx < 0 {
		return
	}
	j := &s.jobs[idx]
	j.State.LastStatus = "error"
	j.State.LastError = validateErr.Error()
	j.State.NextRunAtMs = computeNextScheduleRun(j.Schedule, time.Now().UnixMilli())
}

func (s *Service) Start(ctx context.Context) error {
	s.runCtx, s.runCancel = context.WithCancel(ctx)
	if err := s.load(); err != nil {
		s.log.Warn("cron failed to load jobs", "err", err)
	}
	s.mu.Lock()
	now := time.Now().UnixMilli()
	s.ensureNextRunLocked(now)
	_ = s.saveAtomicLocked()
	n := len(s.jobs)
	s.mu.Unlock()
	s.log.Info("cron started", "jobs", n)
	go s.runLoop()
	go func() {
		<-ctx.Done()
		s.Stop()
	}()
	return nil
}

func (s *Service) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopChan)
		if s.runCancel != nil {
			s.runCancel()
		}
	})
	s.log.Info("cron stopped")
}

func (s *Service) AddJob(name string, schedule Schedule, payload Payload) (*CronJob, error) {
	s.mu.Lock()
	job := NewCronJob(name, schedule, payload)
	switch job.Schedule.Kind {
	case "every":
		if job.Schedule.EveryMs > 0 && job.State.LastRunAtMs == 0 {
			job.State.NextRunAtMs = time.Now().UnixMilli() + job.Schedule.EveryMs
		}
	case "at":
		job.State.NextRunAtMs = job.Schedule.AtMs
	case "cron":
		next, err := gronx.NextTickAfter(job.Schedule.Expr, time.Now(), false)
		if err != nil {
			s.mu.Unlock()
			return nil, fmt.Errorf("cron expr: %w", err)
		}
		job.State.NextRunAtMs = next.UnixMilli()
	}
	s.jobs = append(s.jobs, job)
	err := s.saveAtomicLocked()
	out := s.jobs[len(s.jobs)-1]
	s.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("save jobs: %w", err)
	}
	s.notify()
	return &out, nil
}

func (s *Service) RemoveJob(id string) bool {
	s.mu.Lock()
	removed := false
	for i, job := range s.jobs {
		if job.ID != id {
			continue
		}
		s.jobs = append(s.jobs[:i], s.jobs[i+1:]...)
		_ = s.saveAtomicLocked()
		removed = true
		break
	}
	s.mu.Unlock()
	if removed {
		s.notify()
	}
	return removed
}

func (s *Service) ListJobs() []CronJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]CronJob, len(s.jobs))
	copy(result, s.jobs)
	return result
}

func (s *Service) EnableJob(id string, enabled bool) (*CronJob, error) {
	s.mu.Lock()
	var out *CronJob
	for i := range s.jobs {
		if s.jobs[i].ID != id {
			continue
		}
		s.jobs[i].Enabled = enabled
		if enabled {
			if s.jobs[i].State.NextRunAtMs == 0 {
				now := time.Now().UnixMilli()
				switch s.jobs[i].Schedule.Kind {
				case "every":
					if s.jobs[i].Schedule.EveryMs > 0 {
						if s.jobs[i].State.LastRunAtMs == 0 {
							s.jobs[i].State.NextRunAtMs = now + s.jobs[i].Schedule.EveryMs
						} else {
							s.jobs[i].State.NextRunAtMs = s.jobs[i].State.LastRunAtMs + s.jobs[i].Schedule.EveryMs
						}
					}
				case "at":
					s.jobs[i].State.NextRunAtMs = s.jobs[i].Schedule.AtMs
				case "cron":
					next, err := gronx.NextTickAfter(s.jobs[i].Schedule.Expr, time.UnixMilli(now), false)
					if err == nil {
						s.jobs[i].State.NextRunAtMs = next.UnixMilli()
					}
				}
			}
		} else {
			s.jobs[i].State.NextRunAtMs = 0
		}
		_ = s.saveAtomicLocked()
		job := s.jobs[i]
		out = &job
		break
	}
	s.mu.Unlock()
	if out == nil {
		return nil, fmt.Errorf("job %s not found", id)
	}
	s.notify()
	return out, nil
}

func (s *Service) load() error {
	data, err := os.ReadFile(s.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &s.jobs)
}

func (s *Service) saveAtomicLocked() error {
	data, err := json.MarshalIndent(s.jobs, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(s.storePath, data, 0o600)
}

func writeAtomic(path string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".cron-jobs-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	_, werr := tmp.Write(data)
	serr := tmp.Sync()
	cerr := tmp.Close()
	if werr != nil || serr != nil || cerr != nil {
		os.Remove(tmpName)
		return errors.Join(werr, serr, cerr)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	if perm != 0 {
		_ = os.Chmod(path, perm)
	}
	return nil
}
