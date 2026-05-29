package audio

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ageneralai/maven/internal/kernel/config"
)

type fakeRunner struct {
	calls []string
	fn    func(call string) ([]byte, error)
}

func (f *fakeRunner) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	call := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, call)
	if f.fn != nil {
		return f.fn(call)
	}
	return nil, errors.New("unexpected: " + call)
}

func TestNewVoiceRoute_SelectsByConfig(t *testing.T) {
	if _, ok := NewVoiceRoute(config.SpeechConfig{}).(*echoRoute); !ok {
		t.Fatal("default speech config should select echoRoute")
	}
	if _, ok := NewVoiceRoute(config.SpeechConfig{EchoCancel: "pulse"}).(*echoRoute); !ok {
		t.Fatal("echoCancel=pulse should select echoRoute")
	}
	if _, ok := NewVoiceRoute(config.SpeechConfig{EchoCancel: "off"}).(*directRoute); !ok {
		t.Fatal("echoCancel=off should select directRoute")
	}
	if _, ok := NewVoiceRoute(config.SpeechConfig{EchoCancel: "OFF"}).(*directRoute); !ok {
		t.Fatal("echoCancel is case-insensitive")
	}
}

func TestEchoRoute_LoadsWhenAbsent(t *testing.T) {
	r := &fakeRunner{}
	e := newEchoRoute(config.SpeechConfig{}).WithRunner(r.run)
	r.fn = func(call string) ([]byte, error) {
		if strings.Contains(call, "list short sources") {
			return []byte("0\tauto_null.monitor\tmodule-null-sink.c\n"), nil
		}
		if strings.Contains(call, "load-module") {
			return []byte("42\n"), nil
		}
		return nil, errors.New("unexpected: " + call)
	}
	if err := e.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if e.loadedModule != 42 {
		t.Fatalf("loadedModule = %d, want 42", e.loadedModule)
	}
	if len(r.calls) != 2 {
		t.Fatalf("calls = %v, want 2", r.calls)
	}
	if !strings.Contains(r.calls[1], "load-module module-echo-cancel") {
		t.Fatalf("expected load-module call, got %q", r.calls[1])
	}
	if !strings.Contains(r.calls[1], "source_name=maven_echocancel_source") {
		t.Fatalf("expected source_name in load call: %q", r.calls[1])
	}
	if !strings.Contains(r.calls[1], "aec_method=webrtc") {
		t.Fatalf("expected first attempt aec_method=webrtc: %q", r.calls[1])
	}
}

func TestEchoRoute_FallsBackToSpeex(t *testing.T) {
	r := &fakeRunner{}
	e := newEchoRoute(config.SpeechConfig{}).WithRunner(r.run)
	r.fn = func(call string) ([]byte, error) {
		if strings.Contains(call, "list short sources") {
			return []byte("0\tauto_null.monitor\tmodule-null-sink.c\n"), nil
		}
		if strings.Contains(call, "aec_method=webrtc") {
			return nil, errors.New("Failure: Module initialization failed")
		}
		if strings.Contains(call, "aec_method=speex") {
			return []byte("7\n"), nil
		}
		return nil, errors.New("unexpected: " + call)
	}
	if err := e.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if e.loadedModule != 7 {
		t.Fatalf("loadedModule = %d, want 7 (speex fallback)", e.loadedModule)
	}
}

func TestEchoRoute_SkipsWhenPresent(t *testing.T) {
	r := &fakeRunner{}
	e := newEchoRoute(config.SpeechConfig{}).WithRunner(r.run)
	r.fn = func(call string) ([]byte, error) {
		if strings.Contains(call, "list short sources") {
			return []byte("1\tmaven_echocancel_source\tmodule-echo-cancel.c\n"), nil
		}
		return nil, errors.New("unexpected: " + call)
	}
	if err := e.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if e.loadedModule != -1 {
		t.Fatalf("loadedModule = %d, want -1", e.loadedModule)
	}
	if len(r.calls) != 1 {
		t.Fatalf("calls = %v, want 1 (no load)", r.calls)
	}
}

func TestEchoRoute_UnloadsOnlyOwned(t *testing.T) {
	r := &fakeRunner{}
	e := newEchoRoute(config.SpeechConfig{}).WithRunner(r.run)
	r.fn = func(call string) ([]byte, error) {
		if strings.Contains(call, "list short sources") {
			return []byte("0\tauto_null.monitor\tmodule-null-sink.c\n"), nil
		}
		if strings.Contains(call, "load-module") {
			return []byte("7\n"), nil
		}
		if strings.Contains(call, "unload-module 7") {
			return nil, nil
		}
		return nil, errors.New("unexpected: " + call)
	}
	if err := e.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if err := e.Teardown(context.Background()); err != nil {
		t.Fatalf("Teardown: %v", err)
	}
	if e.loadedModule != -1 {
		t.Fatalf("loadedModule after Teardown = %d, want -1", e.loadedModule)
	}
	unloadCalls := 0
	for _, c := range r.calls {
		if strings.Contains(c, "unload-module") {
			unloadCalls++
		}
	}
	if unloadCalls != 1 {
		t.Fatalf("unload calls = %d, want 1", unloadCalls)
	}
}

func TestEchoRoute_SkipPresentNoUnload(t *testing.T) {
	r := &fakeRunner{}
	e := newEchoRoute(config.SpeechConfig{}).WithRunner(r.run)
	r.fn = func(call string) ([]byte, error) {
		if strings.Contains(call, "list short sources") {
			return []byte("1\tmaven_echocancel_source\tmodule-echo-cancel.c\n"), nil
		}
		return nil, errors.New("unexpected: " + call)
	}
	if err := e.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if err := e.Teardown(context.Background()); err != nil {
		t.Fatalf("Teardown: %v", err)
	}
	for _, c := range r.calls {
		if strings.Contains(c, "unload-module") {
			t.Fatalf("unexpected unload when module was pre-existing: %q", c)
		}
	}
}

func TestEchoRoute_PulseUnavailable(t *testing.T) {
	r := &fakeRunner{}
	e := newEchoRoute(config.SpeechConfig{}).WithRunner(r.run)
	r.fn = func(call string) ([]byte, error) {
		return nil, errors.New("pactl: connection refused")
	}
	err := e.Ensure(context.Background())
	var pulseErr *PulseUnavailableError
	if !errors.As(err, &pulseErr) {
		t.Fatalf("expected PulseUnavailableError, got: %T (%v)", err, err)
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("expected wrapped cause, got: %v", err)
	}
}

func TestEchoRoute_ModuleLoadFailure(t *testing.T) {
	r := &fakeRunner{}
	e := newEchoRoute(config.SpeechConfig{}).WithRunner(r.run)
	r.fn = func(call string) ([]byte, error) {
		if strings.Contains(call, "list short sources") {
			return []byte("0\tauto_null.monitor\tmodule-null-sink.c\n"), nil
		}
		if strings.Contains(call, "load-module") {
			return nil, errors.New("exit status 1: Failure: Module initialization failed")
		}
		return nil, errors.New("unexpected: " + call)
	}
	err := e.Ensure(context.Background())
	var aecErr *EchoCancelUnavailableError
	if !errors.As(err, &aecErr) {
		t.Fatalf("expected EchoCancelUnavailableError, got: %T (%v)", err, err)
	}
	msg := err.Error()
	for _, want := range []string{"echo-cancel module unavailable", "webrtc, speex", "speech.echoCancel", "Module initialization failed"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error missing %q: %v", want, err)
		}
	}
}

func TestEchoRoute_CapturePlaybackUseDevices(t *testing.T) {
	e := newEchoRoute(config.SpeechConfig{})
	cap, ok := e.Capture().(*ExecCapture)
	if !ok {
		t.Fatal("Capture is not *ExecCapture")
	}
	if !hasArg(cap.Args, "--device=maven_echocancel_source") {
		t.Fatalf("capture missing AEC source device: %v", cap.Args)
	}
	play, ok := e.Playback().(*ExecPlayback)
	if !ok {
		t.Fatal("Playback is not *ExecPlayback")
	}
	if !hasArg(play.Args, "--device=maven_echocancel_sink") {
		t.Fatalf("playback missing AEC sink device: %v", play.Args)
	}
}

func TestDirectRoute_NoManagementNoDevice(t *testing.T) {
	d := newDirectRoute(config.SpeechConfig{})
	if err := d.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure should be no-op: %v", err)
	}
	if err := d.Teardown(context.Background()); err != nil {
		t.Fatalf("Teardown should be no-op: %v", err)
	}
	cap := d.Capture().(*ExecCapture)
	for _, a := range cap.Args {
		if strings.HasPrefix(a, "--device=") {
			t.Fatalf("direct capture must not force a device: %v", cap.Args)
		}
	}
	play := d.Playback().(*ExecPlayback)
	for _, a := range play.Args {
		if strings.HasPrefix(a, "--device=") {
			t.Fatalf("direct playback must not force a device: %v", play.Args)
		}
	}
}

func TestDirectRoute_UsesConfiguredCommands(t *testing.T) {
	d := newDirectRoute(config.SpeechConfig{
		Capture:  config.SpeechExecConfig{Command: "termux-mic", Args: []string{"--raw"}},
		Playback: config.SpeechExecConfig{Command: "termux-spk", Args: []string{"--raw"}},
	})
	cap := d.Capture().(*ExecCapture)
	if cap.Command != "termux-mic" || !hasArg(cap.Args, "--raw") {
		t.Fatalf("capture did not honor config: %s %v", cap.Command, cap.Args)
	}
	play := d.Playback().(*ExecPlayback)
	if play.Command != "termux-spk" || !hasArg(play.Args, "--raw") {
		t.Fatalf("playback did not honor config: %s %v", play.Command, play.Args)
	}
}

func TestCommandOutputDiagnostics(t *testing.T) {
	if got := commandOutputDiagnostics("", ""); got != "" {
		t.Fatalf("empty = %q, want empty", got)
	}
	if got := commandOutputDiagnostics("  out\n", ""); got != "out" {
		t.Fatalf("stdout only = %q, want out", got)
	}
	if got := commandOutputDiagnostics("", " err "); got != "err" {
		t.Fatalf("stderr only = %q, want err", got)
	}
	if got := commandOutputDiagnostics("out", "err"); got != "out\nerr" {
		t.Fatalf("both = %q, want out\\nerr", got)
	}
}

func TestDefaultRunner_SurfacesStderr(t *testing.T) {
	_, err := defaultRunner(context.Background(), "sh", "-c", "echo Module initialization failed >&2; exit 1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Module initialization failed") {
		t.Fatalf("expected stderr in error, got: %v", err)
	}
}

func TestDefaultRunner_SurfacesStdout(t *testing.T) {
	_, err := defaultRunner(context.Background(), "sh", "-c", "echo partial diagnostic; exit 1")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "partial diagnostic") {
		t.Fatalf("expected stdout in error, got: %v", err)
	}
}

func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}
