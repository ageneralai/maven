package audio

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestEnsureAndroidPulse_ProbeOnlyWhenHealthy(t *testing.T) {
	r := &fakeRunner{}
	r.fn = func(call string) ([]byte, error) {
		switch {
		case strings.Contains(call, "pactl info"):
			return []byte("Server String: test\n"), nil
		case strings.Contains(call, "list short sources"):
			return []byte("2\tOpenSL_ES_source\tmodule-sles-source.c\n"), nil
		default:
			return nil, errors.New("unexpected: " + call)
		}
	}
	if err := ensureAndroidPulse(context.Background(), r.run); err != nil {
		t.Fatalf("ensureAndroidPulse() = %v", err)
	}
	for _, bad := range []string{"pulseaudio -k", "killall", "load-module"} {
		for _, call := range r.calls {
			if strings.Contains(call, bad) {
				t.Fatalf("healthy pulse should not run %q; calls=%v", bad, r.calls)
			}
		}
	}
}

func TestEnsureAndroidPulse_RestartsAndLoadsMic(t *testing.T) {
	r := &fakeRunner{}
	started := false
	r.fn = func(call string) ([]byte, error) {
		switch {
		case strings.Contains(call, "pactl info"):
			if !started {
				return nil, errors.New("connection refused")
			}
			return []byte("Server String: test\n"), nil
		case strings.Contains(call, "pulseaudio -k"):
			return nil, nil
		case strings.Contains(call, "killall pulseaudio"):
			return nil, nil
		case strings.Contains(call, "pulseaudio --start"):
			started = true
			return nil, nil
		case strings.Contains(call, "list short sources"):
			return []byte("1\tAAudio_sink.monitor\tmodule-aaudio-sink.c\n"), nil
		case strings.Contains(call, "load-module module-sles-source"):
			return []byte("17\n"), nil
		default:
			return nil, errors.New("unexpected: " + call)
		}
	}
	if err := ensureAndroidPulse(context.Background(), r.run); err != nil {
		t.Fatalf("ensureAndroidPulse() = %v", err)
	}
	for _, w := range []string{
		"pactl info",
		"pulseaudio -k",
		"killall pulseaudio",
		"pulseaudio --start",
		"pactl list short sources",
		"pactl load-module module-sles-source",
	} {
		found := false
		for _, call := range r.calls {
			if strings.Contains(call, w) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing call containing %q; calls=%v", w, r.calls)
		}
	}
}

func TestEnsureAndroidPulse_LoadsMicWhenMissing(t *testing.T) {
	r := &fakeRunner{}
	r.fn = func(call string) ([]byte, error) {
		switch {
		case strings.Contains(call, "pactl info"):
			return []byte("Server String: test\n"), nil
		case strings.Contains(call, "list short sources"):
			return []byte("1\tAAudio_sink.monitor\tmodule-aaudio-sink.c\n"), nil
		case strings.Contains(call, "load-module module-sles-source"):
			return []byte("17\n"), nil
		default:
			return nil, errors.New("unexpected: " + call)
		}
	}
	if err := ensureAndroidPulse(context.Background(), r.run); err != nil {
		t.Fatalf("ensureAndroidPulse() = %v", err)
	}
	if len(r.calls) != 3 {
		t.Fatalf("calls = %v, want 3", r.calls)
	}
}
