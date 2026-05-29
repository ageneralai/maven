package audio

import (
	"context"
	"errors"
	"strings"
	"testing"
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

func TestEchoCancel_LoadsWhenAbsent(t *testing.T) {
	r := &fakeRunner{}
	e := NewEchoCancel().WithRunner(r.run)
	r.fn = func(call string) ([]byte, error) {
		if strings.Contains(call, "list short sources") {
			return []byte("0\tdefault\tauto_null.monitor\n"), nil
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
		t.Fatalf("expected aec_method=webrtc in load call: %q", r.calls[1])
	}
}

func TestEchoCancel_SkipsWhenPresent(t *testing.T) {
	r := &fakeRunner{}
	e := NewEchoCancel().WithRunner(r.run)
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

func TestEchoCancel_UnloadsOnlyOwned(t *testing.T) {
	r := &fakeRunner{}
	e := NewEchoCancel().WithRunner(r.run)
	r.fn = func(call string) ([]byte, error) {
		if strings.Contains(call, "list short sources") {
			return []byte("0\tdefault\tauto_null.monitor\n"), nil
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

func TestEchoCancel_SkipPresentNoUnload(t *testing.T) {
	r := &fakeRunner{}
	e := NewEchoCancel().WithRunner(r.run)
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

func TestEchoCancel_PactlFailure(t *testing.T) {
	r := &fakeRunner{}
	e := NewEchoCancel().WithRunner(r.run)
	r.fn = func(call string) ([]byte, error) {
		return nil, errors.New("pactl: connection refused")
	}
	err := e.Ensure(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "pulseaudio required") {
		t.Fatalf("expected pulseaudio message, got: %v", err)
	}
	if !strings.Contains(msg, "connection refused") {
		t.Fatalf("expected wrapped cause, got: %v", err)
	}
}
