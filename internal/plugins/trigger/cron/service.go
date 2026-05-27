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

	"github.com/ageneralai/maven/internal/kernel/sessionid"
	"github.com/ageneralai/maven/internal/kernel/scheduling"
	"github.com/ageneralai/maven/internal/kernel/executor"
	"github.com/ageneralai/maven/internal/kernel/stringutil"
)

type Service struct {
	storePath string
	exec      executor.TurnExecutor
	deliver   *Deliver
	lane      *scheduling.Lane
	log       *slog.Logger
	mu        sync.RWMutex
	jobs      []CronJob
	fireWg    sync.WaitGroup
	stopChan  chan struct{}
	stopOnce  sync.Once
	wakeChan  chan struct{}
}

func NewService(storePath string, exec executor.TurnExecutor, maxConcurrent int, lg *slog.Logger, deliver *Deliver) (*Service, error) {
	if exec == nil {
		return nil, fmt.Errorf("cron: TurnExecutor is required")
	}
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &Service{
		storePath: storePath,
		exec:      exec,
		deliver:   deliver,
		lane:      scheduling.NewLane(int64(maxConcurrent)),
		log:       lg,
		stopChan:  make(chan struct{}),
		wakeChan:  make(chan struct{}, 1),
	}, nil
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
		fromMs := j.State.LastRunAtMs
		if fromMs == 0 {
			fromMs = now
		}
		next, err := computeNextScheduleRun(j.Schedule, fromMs)
		if err != nil {
			s.log.Error("cron job invalid expr", "job", j.Name, "err", err)
			continue
		}
		j.State.NextRunAtMs = next
	}
}

func computeNextScheduleRun(sch Schedule, fromMs int64) (int64, error) {
	next, err := sch.Next(time.UnixMilli(fromMs))
	if err != nil {
		return 0, err
	}
	if next.IsZero() {
		return 0, nil
	}
	return next.UnixMilli(), nil
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

func (s *Service) runLoop(ctx context.Context) {
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
		case <-ctx.Done():
			return
		case <-s.wakeChan:
			drainTimer(timer)
			continue
		case <-timer.C:
			s.checkAndFire(ctx)
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

func (s *Service) checkAndFire(ctx context.Context) {
	s.mu.Lock()
	now := time.Now().UnixMilli()
	var due []CronJob
	for i := range s.jobs {
		j := &s.jobs[i]
		if !s.isDue(j, now) {
			continue
		}
		if IsAtSchedule(j.Schedule) {
			j.Enabled = false
		}
		due = append(due, *j)
		s.clearNextRun(j)
	}
	if err := s.saveAtomicLocked(); err != nil {
		s.log.Error("cron save after firing", "err", err)
	}
	s.fireWg.Add(len(due))
	s.mu.Unlock()
	for _, job := range due {
		job := job // capture by value so the goroutine does not hold s.mu during execution
		go func() {
			defer s.fireWg.Done()
			s.fire(ctx, job)
		}()
	}
}

func (s *Service) fire(ctx context.Context, job CronJob) {
	if err := job.Payload.Validate(); err != nil {
		s.mu.Lock()
		s.applyJobValidationFailure(job.ID, err)
		if serr := s.saveAtomicLocked(); serr != nil {
			s.log.Error("cron save after validation failure", "err", serr)
		}
		s.mu.Unlock()
		return
	}
	if err := s.lane.Acquire(ctx); err != nil {
		return
	}
	defer s.lane.Release()
	sessionID := sessionid.New(sessionid.KindCron, job.ID).String()
	out, err := s.exec.RunTurn(ctx, job.Payload.Message, sessionID)
	doneMs := time.Now().UnixMilli()
	s.mu.Lock()
	idx := s.findJobIndex(job.ID)
	if idx < 0 {
		if serr := s.saveAtomicLocked(); serr != nil {
			s.log.Error("cron save after job removed", "err", serr)
		}
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
	if IsAtSchedule(j.Schedule) {
		j.Enabled = false
		j.State.NextRunAtMs = 0
	}
	if j.DeleteAfterRun {
		s.jobs = append(s.jobs[:idx], s.jobs[idx+1:]...)
	} else if !IsAtSchedule(j.Schedule) {
		next, err := computeNextScheduleRun(j.Schedule, j.State.LastRunAtMs)
		if err != nil {
			s.log.Error("cron job invalid expr after run", "job", j.Name, "err", err)
		} else {
			j.State.NextRunAtMs = next
		}
	}
	if serr := s.saveAtomicLocked(); serr != nil {
		s.log.Error("cron save after run", "job", j.Name, "err", serr)
	}
	s.mu.Unlock()
	if err == nil && s.deliver != nil {
		s.deliver.AfterSuccessfulRun(ctx, job, out)
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
	next, err := computeNextScheduleRun(j.Schedule, time.Now().UnixMilli())
	if err != nil {
		s.log.Error("cron job invalid expr after validation failure", "job", j.Name, "err", err)
	} else {
		j.State.NextRunAtMs = next
	}
}

func (s *Service) Start(ctx context.Context) error {
	if err := s.load(); err != nil {
		s.log.Warn("cron failed to load jobs", "err", err)
	}
	s.mu.Lock()
	now := time.Now().UnixMilli()
	s.ensureNextRunLocked(now)
	if err := s.saveAtomicLocked(); err != nil {
		s.log.Error("cron save on start", "err", err)
	}
	n := len(s.jobs)
	s.mu.Unlock()
	s.log.Info("cron started", "jobs", n)
	go s.runLoop(ctx)
	return nil
}

func (s *Service) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopChan)
	})
	s.fireWg.Wait()
	s.log.Info("cron stopped")
}

func (s *Service) AddJob(name string, schedule Schedule, payload Payload) (*CronJob, error) {
	if err := schedule.Validate(); err != nil {
		return nil, fmt.Errorf("cron schedule: %w", err)
	}
	s.mu.Lock()
	job := NewCronJob(name, schedule, payload)
	now := time.Now().UnixMilli()
	fromMs := job.State.LastRunAtMs
	if fromMs == 0 {
		fromMs = now
	}
	next, err := computeNextScheduleRun(job.Schedule, fromMs)
	if err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("cron expr: %w", err)
	}
	job.State.NextRunAtMs = next
	s.jobs = append(s.jobs, job)
	err = s.saveAtomicLocked()
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
		if err := s.saveAtomicLocked(); err != nil {
			s.log.Error("cron save after remove", "id", id, "err", err)
		}
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
				fromMs := s.jobs[i].State.LastRunAtMs
				if fromMs == 0 {
					fromMs = now
				}
				next, err := computeNextScheduleRun(s.jobs[i].Schedule, fromMs)
				if err != nil {
					s.log.Error("cron job invalid expr on enable", "job", s.jobs[i].Name, "err", err)
				} else {
					s.jobs[i].State.NextRunAtMs = next
				}
			}
		} else {
			s.jobs[i].State.NextRunAtMs = 0
		}
		if err := s.saveAtomicLocked(); err != nil {
			s.log.Error("cron save after enable/disable", "id", id, "err", err)
		}
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
	if err := os.MkdirAll(dir, 0o750); err != nil {
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
		return errors.Join(werr, serr, cerr, os.Remove(tmpName))
	}
	if err := os.Rename(tmpName, path); err != nil {
		return errors.Join(err, os.Remove(tmpName))
	}
	if perm != 0 {
		_ = os.Chmod(path, perm)
	}
	return nil
}
