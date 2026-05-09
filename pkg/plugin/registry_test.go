package plugin

import (
	"context"
	"errors"
	"testing"

	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/config"
)

type stubPlugin struct {
	name     string
	startErr error
	stopErr  error
	enabled  bool
	tools    []tool.Tool
	started  bool
	stopped  bool
}

func (s *stubPlugin) Name() string { return s.name }

func (s *stubPlugin) Enabled(*config.Config) bool { return s.enabled }

func (s *stubPlugin) Tools(*config.Config) []tool.Tool { return s.tools }

func (s *stubPlugin) Start(context.Context) error {
	s.started = true
	return s.startErr
}

func (s *stubPlugin) Stop() error {
	s.stopped = true
	return s.stopErr
}

func TestRegistry_Start_FailFast(t *testing.T) {
	boom := errors.New("boom")
	first := &stubPlugin{name: "ok", enabled: true}
	second := &stubPlugin{name: "bad", enabled: true, startErr: boom}
	third := &stubPlugin{name: "after", enabled: true}
	r := NewRegistry(first, second, third)
	err := r.Start(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("Start error = %v, want %v", err, boom)
	}
	if !first.started {
		t.Fatal("first plugin should have started")
	}
	if third.started {
		t.Fatal("third plugin must not start after failure")
	}
}

func TestRegistry_Tools_NilRegistry(t *testing.T) {
	var r *Registry
	if got := r.Tools(&config.Config{}); got != nil {
		t.Fatalf("Tools = %v, want nil", got)
	}
}

func TestRegistry_Tools_NilCfg(t *testing.T) {
	r := NewRegistry(&stubPlugin{name: "x", enabled: true})
	if got := r.Tools(nil); got != nil {
		t.Fatalf("Tools(nil) = %v, want nil", got)
	}
}
