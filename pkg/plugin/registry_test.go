package plugin

import (
	"context"
	"errors"
	"testing"

	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/config"
	"github.com/ageneralai/maven/pkg/voice"
)

type stubPlugin struct {
	name     string
	startErr error
	stopErr  error
	enabled  bool
	tools    []tool.Tool
	tts      voice.TTSProvider
	stt      voice.STTProvider
	started  bool
	stopped  bool
}

func (s *stubPlugin) Name() string { return s.name }

func (s *stubPlugin) Enabled(*config.Config) bool { return s.enabled }

func (s *stubPlugin) Tools(*config.Config) []tool.Tool { return s.tools }

func (s *stubPlugin) TTSProvider(*config.Config) voice.TTSProvider { return s.tts }

func (s *stubPlugin) STTProvider(*config.Config) voice.STTProvider { return s.stt }

func (s *stubPlugin) Start(context.Context) error {
	s.started = true
	return s.startErr
}

func (s *stubPlugin) Stop() error {
	s.stopped = true
	return s.stopErr
}

func TestRegistry_Start_FailFast(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	var r *Registry
	if got := r.Tools(&config.Config{}); got != nil {
		t.Fatalf("Tools = %v, want nil", got)
	}
}

func TestRegistry_Tools_NilCfg(t *testing.T) {
	t.Parallel()
	r := NewRegistry(&stubPlugin{name: "x", enabled: true})
	if got := r.Tools(nil); got != nil {
		t.Fatalf("Tools(nil) = %v, want nil", got)
	}
}

func TestRegistry_TTSProvider_AllNil(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	r := NewRegistry(
		&stubPlugin{name: "a", enabled: true, tts: nil},
		&stubPlugin{name: "b", enabled: true, tts: nil},
	)
	if got := r.TTSProvider(cfg); got != nil {
		t.Fatalf("TTSProvider = %v, want nil", got)
	}
}

func TestRegistry_TTSProvider_NilRegistry(t *testing.T) {
	t.Parallel()
	var r *Registry
	if got := r.TTSProvider(&config.Config{}); got != nil {
		t.Fatalf("TTSProvider = %v, want nil", got)
	}
}

func TestRegistry_TTSProvider_NilCfg(t *testing.T) {
	t.Parallel()
	r := NewRegistry(&stubPlugin{name: "x", enabled: true})
	if got := r.TTSProvider(nil); got != nil {
		t.Fatalf("TTSProvider(nil) = %v, want nil", got)
	}
}

func TestRegistry_STTProvider_AllNil(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	r := NewRegistry(
		&stubPlugin{name: "a", enabled: true, stt: nil},
		&stubPlugin{name: "b", enabled: true, stt: nil},
	)
	if got := r.STTProvider(cfg); got != nil {
		t.Fatalf("STTProvider = %v, want nil", got)
	}
}

func TestRegistry_STTProvider_NilRegistry(t *testing.T) {
	t.Parallel()
	var r *Registry
	if got := r.STTProvider(&config.Config{}); got != nil {
		t.Fatalf("STTProvider = %v, want nil", got)
	}
}

func TestRegistry_STTProvider_NilCfg(t *testing.T) {
	t.Parallel()
	r := NewRegistry(&stubPlugin{name: "x", enabled: true})
	if got := r.STTProvider(nil); got != nil {
		t.Fatalf("STTProvider(nil) = %v, want nil", got)
	}
}
