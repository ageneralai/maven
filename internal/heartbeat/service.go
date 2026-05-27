package heartbeat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"log/slog"

	"github.com/ageneralai/maven/internal/health"
	"github.com/ageneralai/maven/internal/sessionid"
	"github.com/ageneralai/maven/pkg/executor"
	"github.com/ageneralai/maven/pkg/stringutil"
	"golang.org/x/sync/semaphore"
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
	sem       *semaphore.Weighted
	interval  time.Duration
	log       *slog.Logger
	rep       health.HealthReporter
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
		sem:       semaphore.NewWeighted(1),
		interval:  interval,
		log:       log,
		rep:       health.NoOp{},
	}
	for _, o := range opts {
		o(s)
	}
	return s, nil
}

func (s *Service) buildPrompt() string {
	hbPath := filepath.Join(s.workspace, "HEARTBEAT.md")
	data, err := os.ReadFile(hbPath)
	if err != nil {
		if !os.IsNotExist(err) {
			s.log.Error("heartbeat read error", "err", err)
		}
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (s *Service) Start(ctx context.Context) error {
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

func (s *Service) tick(ctx context.Context) {
	if !s.sem.TryAcquire(1) {
		s.log.Debug("heartbeat skipped: previous tick still running")
		return
	}
	go func() {
		defer s.sem.Release(1)
		s.execute(ctx)
	}()
}

func (s *Service) execute(ctx context.Context) {
	prompt := s.buildPrompt()
	if prompt == "" {
		return
	}
	s.log.Debug("heartbeat triggering", "prompt_len", len(prompt))
	sessionID := sessionid.New(sessionid.KindHeartbeat, "")
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
