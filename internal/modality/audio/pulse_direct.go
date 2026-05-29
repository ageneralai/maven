package audio

import (
	"context"
	"runtime"

	"github.com/ageneralai/maven/internal/kernel/config"
)

// directRoute runs the configured capture/playback commands verbatim with no
// echo-cancel module and no forced device. Used on Android where PulseAudio
// lacks a working module-echo-cancel; Ensure reconciles PulseAudio (single
// daemon + mic source) before capture starts.
type directRoute struct {
	speech config.SpeechConfig
	run    Runner
}

func newDirectRoute(speech config.SpeechConfig) *directRoute {
	return &directRoute{speech: speech, run: defaultRunner}
}

func (r *directRoute) WithRunner(run Runner) *directRoute {
	if run != nil {
		r.run = run
	}
	return r
}

func (r *directRoute) Ensure(ctx context.Context) error {
	if runtime.GOOS != "android" {
		return nil
	}
	return ensureAndroidPulse(ctx, r.run)
}

func (r *directRoute) Teardown(context.Context) error { return nil }

func (r *directRoute) Capture() Capture {
	cmd, args := r.speech.CaptureCommand()
	return &ExecCapture{Command: cmd, Args: args}
}

func (r *directRoute) Playback() Playback {
	cmd, args := r.speech.PlaybackCommand()
	return &ExecPlayback{Command: cmd, Args: args}
}

var _ Route = (*directRoute)(nil)
