package audio

import (
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

// Runner execs a command and returns combined stdout.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

func defaultRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
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
		return pulseRequiredErr(err)
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
		return pulseRequiredErr(err)
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

func pulseRequiredErr(cause error) error {
	return fmt.Errorf("audio: pulseaudio required for voice mode — install pulseaudio: %w", cause)
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
