package plugin

import (
	"context"
	"errors"
	"testing"

	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/voice"
)

type stubToolPlugin struct {
	name     string
	startErr error
	stopErr  error
	tools    []tool.Tool
	started  bool
	stopped  bool
}

func (s *stubToolPlugin) Name() string { return s.name }

func (s *stubToolPlugin) Tools(*config.Config) []tool.Tool { return s.tools }

func (s *stubToolPlugin) Start(context.Context) error {
	s.started = true
	return s.startErr
}

func (s *stubToolPlugin) Stop() error {
	s.stopped = true
	return s.stopErr
}

type stubTTSPlugin struct {
	name string
	tts  voice.TTSProvider
}

func (s stubTTSPlugin) Name() string { return s.name }

func (s stubTTSPlugin) Start(context.Context) error { return nil }

func (s stubTTSPlugin) Stop() error { return nil }

func (s stubTTSPlugin) TTSProvider(*config.Config) voice.TTSProvider { return s.tts }

type stubSTTPlugin struct {
	name string
	stt  voice.STTProvider
}

func (s stubSTTPlugin) Name() string { return s.name }

func (s stubSTTPlugin) Start(context.Context) error { return nil }

func (s stubSTTPlugin) Stop() error { return nil }

func (s stubSTTPlugin) STTProvider(*config.Config) voice.STTProvider { return s.stt }

func TestRegistry_Start_FailFast(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	first := &stubToolPlugin{name: "ok"}
	second := &stubToolPlugin{name: "bad", startErr: boom}
	third := &stubToolPlugin{name: "after"}
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
	t.Parallel()
	var r *Registry
	if got := r.Tools(&config.Config{}); got != nil {
		t.Fatalf("Tools = %v, want nil", got)
	}
}

func TestRegistry_Tools_NilCfg(t *testing.T) {
	t.Parallel()
	r := NewRegistry(&stubToolPlugin{name: "x"})
	if got := r.Tools(nil); got != nil {
		t.Fatalf("Tools(nil) = %v, want nil", got)
	}
}

func TestRegistry_TTSProvider_AllNil(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	r := NewRegistry(stubTTSPlugin{name: "a"}, stubTTSPlugin{name: "b"})
	if got := r.TTSProvider(cfg); got != nil {
		t.Fatalf("TTSProvider = %v, want nil", got)
	}
}

func TestRegistry_STTProvider_AllNil(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	r := NewRegistry(stubSTTPlugin{name: "a"}, stubSTTPlugin{name: "b"})
	if got := r.STTProvider(cfg); got != nil {
		t.Fatalf("STTProvider = %v, want nil", got)
	}
}
