package voice

import (
	"context"
	"errors"
	"log/slog"

	kvoice "github.com/ageneralai/maven/internal/kernel/voice"
	"github.com/ageneralai/maven/internal/plugins/voice/audio"
)

// Sink segments reply text, synthesizes speech, and plays PCM.
type Sink struct {
	TTS      kvoice.TTS
	Playback audio.Playback
	Log      *slog.Logger
	Session  string
}

func (s *Sink) Render(ctx context.Context, reply <-chan string) error {
	if s == nil || s.TTS == nil || s.Playback == nil {
		return nil
	}
	sentences := kvoice.Sentencize(ctx, reply)
	pcm, synthRes := kvoice.Synthesize(ctx, s.TTS, sentences)
	playErr := s.Playback.Play(ctx, pcm)
	synthRes.Wait()
	s.logRenderDone(ctx, playErr, synthRes.Err)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if synthRes.Err != nil && !errors.Is(synthRes.Err, context.Canceled) {
		return synthRes.Err
	}
	return playErr
}

func (s *Sink) logRenderDone(ctx context.Context, playErr, synthErr error) {
	if s == nil || s.Log == nil {
		return
	}
	args := []any{"playErr", playErr, "synthErr", synthErr, "ctxErr", ctx.Err()}
	if s.Session != "" {
		args = append([]any{"session", s.Session}, args...)
	}
	s.Log.Debug("voice render done", args...)
}
