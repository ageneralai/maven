package memory

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	sdkapi "github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/events"
	"github.com/ageneralai/maven/internal/kernel/plugin"
)

type record struct {
	Level slog.Level
	Msg   string
	Attrs map[string]any
}

type captureHandler struct {
	mu      sync.Mutex
	records []record
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	rec := record{Level: r.Level, Msg: r.Message, Attrs: map[string]any{}}
	r.Attrs(func(a slog.Attr) bool {
		rec.Attrs[a.Key] = a.Value.Any()
		return true
	})
	h.mu.Lock()
	h.records = append(h.records, rec)
	h.mu.Unlock()
	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(string) slog.Handler      { return h }

func (h *captureHandler) infos() []record {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []record
	for _, r := range h.records {
		if r.Level == slog.LevelInfo {
			out = append(out, r)
		}
	}
	return out
}

func (h *captureHandler) debugs() []record {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []record
	for _, r := range h.records {
		if r.Level == slog.LevelDebug {
			out = append(out, r)
		}
	}
	return out
}

func newCaptureLogger() (*slog.Logger, *captureHandler) {
	h := &captureHandler{}
	return slog.New(h), h
}

type fakeRuntime struct {
	runFn   func(ctx context.Context, req sdkapi.Request) (*sdkapi.Response, error)
	closed  chan struct{}
	closeMu sync.Mutex
	runMu   sync.Mutex
	runN    int
}

func newFakeRuntime(run func(ctx context.Context, req sdkapi.Request) (*sdkapi.Response, error)) *fakeRuntime {
	return &fakeRuntime{runFn: run, closed: make(chan struct{})}
}

func (f *fakeRuntime) Run(ctx context.Context, req sdkapi.Request) (*sdkapi.Response, error) {
	f.runMu.Lock()
	f.runN++
	f.runMu.Unlock()
	return f.runFn(ctx, req)
}

func (f *fakeRuntime) runCount() int {
	f.runMu.Lock()
	defer f.runMu.Unlock()
	return f.runN
}

func (f *fakeRuntime) Close() error {
	f.closeMu.Lock()
	defer f.closeMu.Unlock()
	select {
	case <-f.closed:
	default:
		close(f.closed)
	}
	return nil
}

func newTestPlugin(lg *slog.Logger, factory shadowRuntimeFactory) *Plugin {
	return &Plugin{log: lg, newShadow: factory}
}

func factoryFor(rt shadowRuntime) shadowRuntimeFactory {
	return func(_ *config.Config, _ string, _ []tool.Tool) (shadowRuntime, error) {
		return rt, nil
	}
}

func cfgWithShadowEnabled(workspace string) *config.Config {
	return &config.Config{
		Agent:         config.AgentConfig{Workspace: workspace},
		ShadowJournal: config.ShadowJournalConfig{Enabled: true},
	}
}

func waitClosed(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("runtime not closed within deadline")
	}
}

func publishTurnCompleted(f *events.Fanout, userMsg, assistantMsg string) {
	f.Publish(context.Background(), events.Event{
		Type: events.EventTurnCompleted,
		Payload: events.TurnCompleted{
			UserMsg:      userMsg,
			AssistantMsg: assistantMsg,
		},
	})
}

func TestConfigureTurnJournal_disabledClearsRuntime(t *testing.T) {
	log, _ := newCaptureLogger()
	cfg := cfgWithShadowEnabled(t.TempDir())
	fake := newFakeRuntime(func(context.Context, sdkapi.Request) (*sdkapi.Response, error) {
		return &sdkapi.Response{}, nil
	})
	p := newTestPlugin(log, factoryFor(fake))
	p.ConfigureTurnJournal(cfg)
	p.ConfigureTurnJournal(&config.Config{Agent: cfg.Agent})
	if err := p.Stop(); err != nil {
		t.Fatal(err)
	}
	waitClosed(t, fake.closed)
}

func TestConfigureTurnJournal_returnsNilOnFactoryError(t *testing.T) {
	log, _ := newCaptureLogger()
	p := newTestPlugin(log, func(_ *config.Config, _ string, _ []tool.Tool) (shadowRuntime, error) {
		return nil, errors.New("init failed")
	})
	p.ConfigureTurnJournal(cfgWithShadowEnabled("/tmp/ws"))
	p.mu.Lock()
	rt := p.rt
	p.mu.Unlock()
	if rt != nil {
		t.Fatal("expected p.rt unset on factory error")
	}
}

func TestConfigureTurnJournal_swapsAndAsyncClosesPrevious(t *testing.T) {
	log, _ := newCaptureLogger()
	cfg := cfgWithShadowEnabled(t.TempDir())
	fakeA := newFakeRuntime(func(context.Context, sdkapi.Request) (*sdkapi.Response, error) {
		return &sdkapi.Response{}, nil
	})
	fakeB := newFakeRuntime(func(context.Context, sdkapi.Request) (*sdkapi.Response, error) {
		return &sdkapi.Response{}, nil
	})
	var current shadowRuntime
	p := newTestPlugin(log, func(_ *config.Config, _ string, _ []tool.Tool) (shadowRuntime, error) {
		return current, nil
	})
	current = fakeA
	p.ConfigureTurnJournal(cfg)
	current = fakeB
	p.ConfigureTurnJournal(cfg)
	waitClosed(t, fakeA.closed)
	p.mu.Lock()
	got := p.rt
	p.mu.Unlock()
	if got != fakeB {
		t.Fatalf("p.rt = %v, want fakeB", got)
	}
}

func TestOnTurnCompleted_usesCurrentRuntime(t *testing.T) {
	log, _ := newCaptureLogger()
	cfg := cfgWithShadowEnabled(t.TempDir())
	var fakeACalled, fakeBCalled bool
	fakeA := newFakeRuntime(func(context.Context, sdkapi.Request) (*sdkapi.Response, error) {
		fakeACalled = true
		return &sdkapi.Response{}, nil
	})
	fakeB := newFakeRuntime(func(context.Context, sdkapi.Request) (*sdkapi.Response, error) {
		fakeBCalled = true
		return &sdkapi.Response{}, nil
	})
	var current shadowRuntime
	p := newTestPlugin(log, func(_ *config.Config, _ string, _ []tool.Tool) (shadowRuntime, error) {
		return current, nil
	})
	f := events.NewFanout(nil)
	p.SetEventBus(f)
	if err := p.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Stop() }()
	current = fakeA
	p.ConfigureTurnJournal(cfg)
	current = fakeB
	p.ConfigureTurnJournal(cfg)
	publishTurnCompleted(f, "hi", "hello")
	time.Sleep(100 * time.Millisecond)
	if !fakeBCalled {
		t.Fatal("expected fakeB.Run called with current runtime")
	}
	if fakeACalled {
		t.Fatal("fakeA.Run must not be called after runtime swap")
	}
}

func TestOnTurnCompleted_skipsEmptyExchange(t *testing.T) {
	tests := []struct {
		name    string
		user    string
		asst    string
		wantRun bool
	}{
		{name: "empty", wantRun: false},
		{name: "user_only", user: "hi", wantRun: true},
		{name: "assistant_only", asst: "hello", wantRun: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log, _ := newCaptureLogger()
			cfg := cfgWithShadowEnabled(t.TempDir())
			fake := newFakeRuntime(func(context.Context, sdkapi.Request) (*sdkapi.Response, error) {
				return &sdkapi.Response{}, nil
			})
			p := newTestPlugin(log, factoryFor(fake))
			f := events.NewFanout(nil)
			p.SetEventBus(f)
			if err := p.Start(context.Background()); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = p.Stop() }()
			p.ConfigureTurnJournal(cfg)
			publishTurnCompleted(f, tt.user, tt.asst)
			time.Sleep(100 * time.Millisecond)
			got := fake.runCount()
			if tt.wantRun && got != 1 {
				t.Fatalf("Run count = %d, want 1", got)
			}
			if !tt.wantRun && got != 0 {
				t.Fatalf("Run count = %d, want 0", got)
			}
		})
	}
}

func TestOnTurnCompleted_appliesTimeout(t *testing.T) {
	log, _ := newCaptureLogger()
	cfg := cfgWithShadowEnabled(t.TempDir())
	var sawDeadline bool
	fake := newFakeRuntime(func(ctx context.Context, _ sdkapi.Request) (*sdkapi.Response, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Error("expected ctx deadline")
			return &sdkapi.Response{}, nil
		}
		until := time.Until(deadline)
		if until <= 0 || until > shadowTurnTimeout+500*time.Millisecond {
			t.Errorf("deadline until = %v, want (0, %v]", until, shadowTurnTimeout+500*time.Millisecond)
		}
		if ctx.Err() != nil {
			t.Errorf("ctx must not be cancelled, err=%v", ctx.Err())
		}
		sawDeadline = true
		return &sdkapi.Response{}, nil
	})
	p := newTestPlugin(log, factoryFor(fake))
	f := events.NewFanout(nil)
	p.SetEventBus(f)
	if err := p.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Stop() }()
	p.ConfigureTurnJournal(cfg)
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	f.Publish(parent, events.Event{
		Type:    events.EventTurnCompleted,
		Payload: events.TurnCompleted{UserMsg: "x", AssistantMsg: "y"},
	})
	time.Sleep(100 * time.Millisecond)
	if !sawDeadline {
		t.Fatal("Run was not invoked or deadline not checked")
	}
}

func TestStop_closesCurrentRuntime(t *testing.T) {
	log, _ := newCaptureLogger()
	cfg := cfgWithShadowEnabled(t.TempDir())
	fake := newFakeRuntime(func(context.Context, sdkapi.Request) (*sdkapi.Response, error) {
		return &sdkapi.Response{}, nil
	})
	p := newTestPlugin(log, factoryFor(fake))
	p.ConfigureTurnJournal(cfg)
	if err := p.Stop(); err != nil {
		t.Fatal(err)
	}
	waitClosed(t, fake.closed)
	if err := p.Stop(); err != nil {
		t.Fatal("second Stop must be no-op")
	}
}

func TestPlugin_satisfiesTurnJournalPluginInterface(t *testing.T) {
	var _ plugin.TurnJournalPlugin = (*Plugin)(nil)
	var _ plugin.EventAwarePlugin = (*Plugin)(nil)
}
