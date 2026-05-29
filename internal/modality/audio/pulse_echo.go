package audio

import (
	"context"
	"fmt"
	"runtime"
	"strconv"

	"github.com/ageneralai/maven/internal/kernel/config"
)

// PulseAudio module-echo-cancel identifiers owned by maven; internal device
// names, not user configuration. webrtc is the same AEC algorithm the browser
// enables for getUserMedia; speex is the fallback on builds without WebRTC.
const (
	aecSourceName = "maven_echocancel_source"
	aecSinkName   = "maven_echocancel_sink"
)

var aecMethods = []string{"webrtc", "speex"}

// echoRoute loads PulseAudio module-echo-cancel and routes capture/playback
// through its devices so the mic never hears the agent's own TTS.
type echoRoute struct {
	speech       config.SpeechConfig
	run          Runner
	loadedModule int
}

func newEchoRoute(speech config.SpeechConfig) *echoRoute {
	return &echoRoute{speech: speech, run: defaultRunner, loadedModule: -1}
}

// WithRunner injects a command runner (tests).
func (r *echoRoute) WithRunner(run Runner) *echoRoute {
	if run != nil {
		r.run = run
	}
	return r
}

// Ensure loads module-echo-cancel when maven's source is absent, trying each
// AEC method until one initializes.
func (r *echoRoute) Ensure(ctx context.Context) error {
	if runtime.GOOS == "android" {
		if err := ensureAndroidPulse(ctx, r.run); err != nil {
			return err
		}
	}
	out, err := r.run(ctx, "pactl", "list", "short", "sources")
	if err != nil {
		return &PulseUnavailableError{Err: err}
	}
	if sourcePresent(out, aecSourceName) {
		return nil
	}
	var lastErr error
	for _, method := range aecMethods {
		loadOut, loadErr := r.run(ctx, "pactl",
			"load-module", "module-echo-cancel",
			"source_name="+aecSourceName,
			"sink_name="+aecSinkName,
			"aec_method="+method,
		)
		if loadErr != nil {
			lastErr = loadErr
			continue
		}
		idx, perr := parseModuleIndex(loadOut)
		if perr != nil {
			return fmt.Errorf("audio: parse echo-cancel module index: %w", perr)
		}
		r.loadedModule = idx
		return nil
	}
	return &EchoCancelUnavailableError{Err: lastErr, Methods: aecMethods}
}

// Teardown unloads the module loaded by Ensure; no-op when Ensure reused an existing source.
func (r *echoRoute) Teardown(ctx context.Context) error {
	if r.loadedModule < 0 {
		return nil
	}
	_, err := r.run(ctx, "pactl", "unload-module", strconv.Itoa(r.loadedModule))
	r.loadedModule = -1
	return err
}

func (r *echoRoute) Capture() Capture {
	cmd, args := r.speech.CaptureCommand()
	return &ExecCapture{Command: cmd, Args: appendDeviceArg(args, aecSourceName)}
}

func (r *echoRoute) Playback() Playback {
	cmd, args := r.speech.PlaybackCommand()
	return &ExecPlayback{Command: cmd, Args: appendDeviceArg(args, aecSinkName)}
}

var _ Route = (*echoRoute)(nil)
