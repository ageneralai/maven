package heartbeat

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"log/slog"

	"github.com/ageneralai/maven/kernel/health"
	"github.com/ageneralai/maven/kernel/scheduling"
	"github.com/ageneralai/maven/kernel/sessionid"
	"github.com/ageneralai/maven/kernel/executor"
	"github.com/ageneralai/maven/kernel/stringutil"
)

// Option configures Service after required fields are set.
type Option func(*Service)

// WithHealthReporter wires liveness pulses (ticker-driven). Nil uses the default NoOp.
func WithHealthReporter(r health.HealthReporter) Option {
	return func(s *Service) {
		s.rep = health.OrHealthReporter(r)
	}
}

type Service struct {
	workspace string
	exec      executor.TurnExecutor
	lane      *scheduling.Lane
	trigger   Trigger
	interval  time.Duration
	log       *slog.Logger
	rep       health.HealthReporter
	loopWg    sync.WaitGroup
	fireWg    sync.WaitGroup
}

func New(workspace string, exec executor.TurnExecutor, interval time.Duration, log *slog.Logger, opts ...Option) (*Service, error) {
	if exec == nil {
		return nil, fmt.Errorf("heartbeat: TurnExecutor is required")
	}
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	s := &Service{
		workspace: workspace,
		exec:      exec,
		lane:      scheduling.NewLane(1),
		interval:  interval,
		log:       log,
		rep:       health.NoOp{},
	}
	for _, o := range opts {
		o(s)
	}
	s.trigger = triggerOrDefault(workspace, log, s.trigger)
	return s, nil
}

func (s *Service) Start(ctx context.Context) error {
	s.loopWg.Add(1)
	defer s.loopWg.Done()
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	s.log.Info("heartbeat started", "interval", s.interval)
	for {
		select {
		case <-ticker.C:
			s.rep.Pulse(health.SignalHeartbeatTick)
			s.tick(ctx)
		case <-ctx.Done():
			s.log.Info("heartbeat stopped")
			return nil
		}
	}
}

func (s *Service) Stop() {
	s.loopWg.Wait()
	s.fireWg.Wait()
}

func (s *Service) tick(ctx context.Context) {
	if !s.lane.TryAcquire() {
		s.log.Debug("heartbeat skipped: previous tick still running")
		return
	}
	s.fireWg.Add(1)
	go func() {
		defer s.lane.Release()
		defer s.fireWg.Done()
		s.execute(ctx)
	}()
}

func (s *Service) execute(ctx context.Context) {
	prompt := s.trigger.Prompt()
	if prompt == "" {
		return
	}
	s.log.Debug("heartbeat triggering", "prompt_len", len(prompt))
	sessionID := sessionid.New(sessionid.KindHeartbeat, "").String()
	result, err := s.exec.RunTurn(ctx, prompt, sessionID)
	if err != nil {
		s.log.Error("heartbeat error", "err", err)
		return
	}
	if strings.Contains(result, "HEARTBEAT_OK") {
		s.log.Debug("heartbeat nothing to do")
	} else {
		s.log.Info("heartbeat result", "output", stringutil.Truncate(result, 200))
	}
}
