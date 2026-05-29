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

// Route prepares the platform's audio path for a voice session and yields the
// mic capture and speaker playback bound to it. The desktop route manages a
// PulseAudio echo-cancel module; the direct route runs the configured I/O
// commands verbatim for platforms (Android/Termux) without WebRTC AEC.
type Route interface {
	Ensure(ctx context.Context) error
	Teardown(ctx context.Context) error
	Capture() Capture
	Playback() Playback
}

// NewVoiceRoute selects the voice route from speech config. echoCancel "off"
// runs capture/playback as configured with no PulseAudio management; otherwise
// Maven loads module-echo-cancel and routes through its devices.
func NewVoiceRoute(speech config.SpeechConfig) Route {
	if speech.EchoCancelDisabled() {
		return newDirectRoute(speech)
	}
	return newEchoRoute(speech)
}

// PulseUnavailableError means pactl is missing or cannot reach a PulseAudio server.
type PulseUnavailableError struct {
	Err error
}

func (e *PulseUnavailableError) Error() string {
	return fmt.Sprintf("audio: pulseaudio unavailable for voice mode: %v", e.Err)
}

func (e *PulseUnavailableError) Unwrap() error { return e.Err }

// EchoCancelUnavailableError means PulseAudio is reachable but module-echo-cancel
// failed to load with every attempted AEC method.
type EchoCancelUnavailableError struct {
	Err     error
	Methods []string
}

func (e *EchoCancelUnavailableError) Error() string {
	return fmt.Sprintf("audio: echo-cancel module unavailable (tried aec_method %s) — set speech.echoCancel \"off\" for direct capture: %v",
		strings.Join(e.Methods, ", "), e.Err)
}

func (e *EchoCancelUnavailableError) Unwrap() error { return e.Err }

// Runner execs a command and returns stdout; on failure it surfaces stdout+stderr.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

func defaultRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
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
	return append(out, "--device="+device)
}
