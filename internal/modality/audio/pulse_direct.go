package audio

import (
	"context"

	"github.com/ageneralai/maven/internal/kernel/config"
)

// directRoute runs the configured capture/playback commands verbatim with no
// echo-cancel module and no forced device. Used on Android/Termux where
// PulseAudio lacks a working module-echo-cancel; the platform's mic/speaker
// access is the operator's responsibility via speech.capture / speech.playback.
type directRoute struct {
	speech config.SpeechConfig
}

func newDirectRoute(speech config.SpeechConfig) *directRoute {
	return &directRoute{speech: speech}
}

func (r *directRoute) Ensure(context.Context) error { return nil }

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
