package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	rcron "github.com/robfig/cron/v3"
	mavenlog "github.com/ageneralai/maven/internal/log"
)

type Service struct {
	storePath string
	log       mavenlog.PrintLogger
	mu        sync.Mutex
	jobs      []CronJob
	OnJob     func(job CronJob) (string, error)
	cron      *rcron.Cron
	entryMap  map[string]rcron.EntryID
	wakeChan  chan struct{}
}

func NewService(storePath string, lg mavenlog.PrintLogger) *Service {
	return &Service{
		storePath: storePath,
		log:       lg,
		entryMap:  make(map[string]rcron.EntryID),
		wakeChan:  make(chan struct{}, 1),
	}
}

func (s *Service) notify() {
	select {
	case s.wakeChan <- struct{}{}:
	default:
	}
}

func (s *Service) Start(ctx context.Context) error {
	if err := s.load(); err != nil {
		s.log.Printf("[cron] warning: failed to load jobs: %v", err)
	}
	s.initNextRun()
	s.cron = rcron.New(rcron.WithSeconds())
	s.mu.Lock()
	for i := range s.jobs {
		if s.jobs[i].Enabled && s.jobs[i].Schedule.Kind == "cron" {
			s.registerJob(&s.jobs[i])
		}
	}
	s.mu.Unlock()
	s.cron.Start()
	s.log.Printf("[cron] started with %d jobs", len(s.jobs))
	go s.tickLoop(ctx)
	go func() {
		<-ctx.Done()
		s.Stop()
	}()
	return nil
}

func (s *Service) initNextRun() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UnixMilli()
	for i := range s.jobs {
		j := &s.jobs[i]
		if !j.Enabled || j.Schedule.Kind != "every" || j.Schedule.EveryMs <= 0 {
			continue
		}
		if j.State.LastRunAtMs == 0 && j.State.NextRunAtMs == 0 {
			j.State.NextRunAtMs = now + j.Schedule.EveryMs
		}
	}
}

func (s *Service) registerJob(job *CronJob) {
	if s.cron == nil {
		return
	}
	if id, ok := s.entryMap[job.ID]; ok {
		s.cron.Remove(id)
		delete(s.entryMap, job.ID)
	}
	jobCopy := *job
	entryID, err := s.cron.AddFunc(job.Schedule.Expr, func() {
		s.executeJob(jobCopy)
	})
	if err != nil {
		s.log.Printf("[cron] failed to register job %s (%s): %v", job.Name, job.Schedule.Expr, err)
		return
	}
	s.entryMap[job.ID] = entryID
}

func (s *Service) executeJob(job CronJob) {
	s.log.Printf("[cron] executing job %s (%s)", job.Name, job.ID)
	if s.OnJob == nil {
		s.log.Printf("[cron] no OnJob handler set")
		return
	}
	result, err := s.OnJob(job)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.jobs {
		if s.jobs[i].ID != job.ID {
			continue
		}
		s.jobs[i].State.LastRunAtMs = time.Now().UnixMilli()
		if s.jobs[i].Schedule.Kind == "every" {
			s.jobs[i].State.NextRunAtMs = 0
		}
		if err != nil {
			s.jobs[i].State.LastStatus = "error"
			s.jobs[i].State.LastError = err.Error()
			s.log.Printf("[cron] job %s error: %v", job.Name, err)
		} else {
			s.jobs[i].State.LastStatus = "ok"
			s.jobs[i].State.LastError = ""
			s.log.Printf("[cron] job %s result: %s", job.Name, truncate(result, 100))
		}
		if s.jobs[i].DeleteAfterRun {
			s.jobs = append(s.jobs[:i], s.jobs[i+1:]...)
		}
		break
	}
	_ = s.save()
}

func (s *Service) computeMinDelay() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UnixMilli()
	var minRemaining int64 = -1
	for i := range s.jobs {
		job := &s.jobs[i]
		if !job.Enabled {
			continue
		}
		switch job.Schedule.Kind {
		case "every":
			if job.Schedule.EveryMs <= 0 {
				continue
			}
			var rem int64
			if job.State.LastRunAtMs == 0 {
				if job.State.NextRunAtMs > 0 {
					rem = job.State.NextRunAtMs - now
				} else {
					rem = job.Schedule.EveryMs
				}
			} else {
				rem = job.State.LastRunAtMs + job.Schedule.EveryMs - now
			}
			if rem <= 0 {
				rem = 1
			}
			if minRemaining < 0 || rem < minRemaining {
				minRemaining = rem
			}
		case "at":
			if job.Schedule.AtMs <= 0 {
				continue
			}
			rem := job.Schedule.AtMs - now
			if rem <= 0 {
				rem = 1
			}
			if minRemaining < 0 || rem < minRemaining {
				minRemaining = rem
			}
		}
	}
	if minRemaining < 0 {
		return time.Hour
	}
	return time.Duration(minRemaining) * time.Millisecond
}

func (s *Service) checkDueJobs(now int64) {
	s.mu.Lock()
	var toRun []CronJob
	for i := range s.jobs {
		job := &s.jobs[i]
		if !job.Enabled {
			continue
		}
		switch job.Schedule.Kind {
		case "every":
			var due bool
			if job.State.LastRunAtMs == 0 {
				due = job.State.NextRunAtMs > 0 && now >= job.State.NextRunAtMs
			} else {
				due = now >= job.State.LastRunAtMs+job.Schedule.EveryMs
			}
			if due {
				toRun = append(toRun, *job)
			}
		case "at":
			if job.Schedule.AtMs > 0 && now >= job.Schedule.AtMs {
				job.Enabled = false
				job.State.NextRunAtMs = 0
				toRun = append(toRun, *job)
			}
		}
	}
	if len(toRun) > 0 {
		_ = s.save()
	}
	s.mu.Unlock()
	for _, job := range toRun {
		s.executeJob(job)
	}
}

func (s *Service) tickLoop(ctx context.Context) {
	timer := time.NewTimer(time.Hour)
	defer func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()
	resetTimer := func() {
		d := s.computeMinDelay()
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(d)
	}
	resetTimer()
	for {
		select {
		case <-timer.C:
			s.checkDueJobs(time.Now().UnixMilli())
			resetTimer()
		case <-s.wakeChan:
			resetTimer()
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		}
	}
}

func (s *Service) Stop() {
	if s.cron != nil {
		s.cron.Stop()
	}
	s.log.Printf("[cron] stopped")
}

func (s *Service) AddJob(name string, schedule Schedule, payload Payload) (*CronJob, error) {
	s.mu.Lock()
	job := NewCronJob(name, schedule, payload)
	if job.Schedule.Kind == "every" && job.Schedule.EveryMs > 0 && job.State.LastRunAtMs == 0 {
		job.State.NextRunAtMs = time.Now().UnixMilli() + job.Schedule.EveryMs
	}
	s.jobs = append(s.jobs, job)
	if job.Schedule.Kind == "cron" && s.cron != nil {
		s.registerJob(&s.jobs[len(s.jobs)-1])
	}
	err := s.save()
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
		if entryID, ok := s.entryMap[id]; ok && s.cron != nil {
			s.cron.Remove(entryID)
			delete(s.entryMap, id)
		}
		s.jobs = append(s.jobs[:i], s.jobs[i+1:]...)
		_ = s.save()
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
	s.mu.Lock()
	defer s.mu.Unlock()
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
		if s.jobs[i].Schedule.Kind == "cron" && s.cron != nil {
			if enabled {
				s.registerJob(&s.jobs[i])
			} else if entryID, ok := s.entryMap[id]; ok {
				s.cron.Remove(entryID)
				delete(s.entryMap, id)
			}
		}
		_ = s.save()
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

func (s *Service) save() error {
	dir := filepath.Dir(s.storePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.jobs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.storePath, data, 0644)
}

func truncate(str string, n int) string {
	if len(str) <= n {
		return str
	}
	return str[:n] + "..."
}
