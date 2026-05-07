package heartbeat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	mavenlog "github.com/ageneralai/maven/internal/log"
)

type Service struct {
	workspace   string
	onHeartbeat func(prompt string) (string, error)
	interval    time.Duration
	log         mavenlog.PrintLogger
}

func New(workspace string, onHB func(string) (string, error), interval time.Duration, log mavenlog.PrintLogger) *Service {
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	return &Service{
		workspace:   workspace,
		onHeartbeat: onHB,
		interval:    interval,
		log:         log,
	}
}

func (s *Service) Start(ctx context.Context) error {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.log.Printf("[heartbeat] started, interval=%s", s.interval)

	for {
		select {
		case <-ticker.C:
			s.tick()
		case <-ctx.Done():
			s.log.Printf("[heartbeat] stopped")
			return nil
		}
	}
}

func (s *Service) tick() {
	hbPath := filepath.Join(s.workspace, "HEARTBEAT.md")
	data, err := os.ReadFile(hbPath)
	if err != nil {
		if !os.IsNotExist(err) {
			s.log.Printf("[heartbeat] read error: %v", err)
		}
		return
	}

	content := strings.TrimSpace(string(data))
	if content == "" {
		return
	}

	s.log.Printf("[heartbeat] triggering with prompt (%d chars)", len(content))

	if s.onHeartbeat == nil {
		s.log.Printf("[heartbeat] no handler set")
		return
	}

	result, err := s.onHeartbeat(content)
	if err != nil {
		s.log.Printf("[heartbeat] error: %v", err)
		return
	}

	if strings.Contains(result, "HEARTBEAT_OK") {
		s.log.Printf("[heartbeat] nothing to do")
	} else {
		s.log.Printf("[heartbeat] result: %s", truncate(result, 200))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
