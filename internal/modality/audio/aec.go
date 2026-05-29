package audio

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ageneralai/maven/internal/kernel/config"
)

// PulseAudio module-echo-cancel identifiers owned by maven. webrtc is the same
// AEC algorithm the browser enables by default for getUserMedia; these are
// internal device names, not user configuration.
const (
	aecSourceName = "maven_echocancel_source"
	aecSinkName   = "maven_echocancel_sink"
	aecMethod     = "webrtc"
)

// PulseUnavailableError means pactl is missing or cannot reach a PulseAudio server.
type PulseUnavailableError struct {
	Err error
}

func (e *PulseUnavailableError) Error() string {
	return fmt.Sprintf("audio: pulseaudio unavailable for voice mode: %v", e.Err)
}

func (e *PulseUnavailableError) Unwrap() error { return e.Err }

// EchoCancelUnavailableError means PulseAudio is reachable but module-echo-cancel failed to load.
type EchoCancelUnavailableError struct {
	Err    error
	Method string
}

func (e *EchoCancelUnavailableError) Error() string {
	return fmt.Sprintf("audio: echo-cancel module unavailable (aec_method=%s): %v", e.Method, e.Err)
}

func (e *EchoCancelUnavailableError) Unwrap() error { return e.Err }

// Runner execs a command and returns stdout.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

func defaultRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	return runCommand(ctx, name, args...)
}

func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if msg := commandOutputDiagnostics(stdout.String(), stderr.String()); msg != "" {
			return stdout.Bytes(), fmt.Errorf("%w: %s", err, msg)
		}
		return stdout.Bytes(), err
	}
	return stdout.Bytes(), nil
}

func commandOutputDiagnostics(stdout, stderr string) string {
	var parts []string
	if s := strings.TrimSpace(stdout); s != "" {
		parts = append(parts, s)
	}
	if s := strings.TrimSpace(stderr); s != "" {
		parts = append(parts, s)
	}
	return strings.Join(parts, "\n")
}

// EchoCancel applies PulseAudio module-echo-cancel and routes capture/playback through it.
type EchoCancel struct {
	run          Runner
	loadedModule int
}

// NewEchoCancel builds an AEC provider.
func NewEchoCancel() *EchoCancel {
	return &EchoCancel{run: defaultRunner, loadedModule: -1}
}

// WithRunner injects a command runner (tests).
func (e *EchoCancel) WithRunner(run Runner) *EchoCancel {
	if run != nil {
		e.run = run
	}
	return e
}

// Ensure loads module-echo-cancel when maven's source is absent.
func (e *EchoCancel) Ensure(ctx context.Context) error {
	if e == nil {
		return errors.New("audio: echo cancel provider is nil")
	}
	out, err := e.run(ctx, "pactl", "list", "short", "sources")
	if err != nil {
		return &PulseUnavailableError{Err: err}
	}
	if sourcePresent(out, aecSourceName) {
		return nil
	}
	loadOut, err := e.run(ctx, "pactl",
		"load-module", "module-echo-cancel",
		"source_name="+aecSourceName,
		"sink_name="+aecSinkName,
		"aec_method="+aecMethod,
	)
	if err != nil {
		return &EchoCancelUnavailableError{Err: err, Method: aecMethod}
	}
	idx, err := parseModuleIndex(loadOut)
	if err != nil {
		return fmt.Errorf("audio: parse echo-cancel module index: %w", err)
	}
	e.loadedModule = idx
	return nil
}

// Teardown unloads the module loaded by Ensure; no-op when Ensure reused an existing source.
func (e *EchoCancel) Teardown(ctx context.Context) error {
	if e == nil || e.loadedModule < 0 {
		return nil
	}
	_, err := e.run(ctx, "pactl", "unload-module", strconv.Itoa(e.loadedModule))
	e.loadedModule = -1
	return err
}

// Capture returns mic capture from the echo-cancelled source.
func (e *EchoCancel) Capture(speech config.SpeechConfig) Capture {
	cmd, args := speech.CaptureCommand()
	args = appendDeviceArg(args, aecSourceName)
	return &ExecCapture{Command: cmd, Args: args}
}

// Playback returns speaker playback to the echo-cancelled sink (AEC reference path).
func (e *EchoCancel) Playback(speech config.SpeechConfig) Playback {
	cmd, args := speech.PlaybackCommand()
	args = appendDeviceArg(args, aecSinkName)
	return &ExecPlayback{Command: cmd, Args: args}
}

func sourcePresent(listOut []byte, name string) bool {
	want := strings.TrimSpace(name)
	for _, line := range strings.Split(string(listOut), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == want {
			return true
		}
	}
	return false
}

func parseModuleIndex(out []byte) (int, error) {
	s := strings.TrimSpace(string(out))
	if s == "" {
		return 0, errors.New("empty module index")
	}
	return strconv.Atoi(s)
}

func appendDeviceArg(args []string, device string) []string {
	out := append([]string(nil), args...)
	out = append(out, "--device="+device)
	return out
}
