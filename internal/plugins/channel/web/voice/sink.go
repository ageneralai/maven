package voice

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/ageneralai/maven/internal/kernel/converse"
	"github.com/ageneralai/maven/internal/kernel/voice"
	"github.com/ageneralai/maven/internal/plugins/channel/web/wsmsg"
	"log/slog"
)

type frameWriter interface {
	writeBinary(ctx context.Context, data []byte) error
	writeText(ctx context.Context, data []byte) error
}

type wsVoiceSink struct {
	w       frameWriter
	tts     voice.TTS
	log     *slog.Logger
	session string
}

func (s *wsVoiceSink) Render(ctx context.Context, reply <-chan string) error {
	if s == nil || s.w == nil || s.tts == nil {
		return nil
	}
	sentences := voice.Sentencize(ctx, reply)
	pcm, synthRes := voice.Synthesize(ctx, s.tts, sentences)
	var interrupted bool
pcmLoop:
	for {
		select {
		case <-ctx.Done():
			interrupted = true
			break pcmLoop
		case b, ok := <-pcm:
			if !ok {
				synthRes.Wait()
				if synthRes.Err != nil && !errors.Is(synthRes.Err, context.Canceled) && !errors.Is(synthRes.Err, context.DeadlineExceeded) {
					if s.log != nil {
						s.log.Error("web voice tts", "err", synthRes.Err)
					}
				}
				doneMsg, err := json.Marshal(wsmsg.Message{Type: "stream_done"})
				if err != nil {
					return err
				}
				s.logRenderDone(ctx, synthRes.Err, false)
				return s.w.writeText(ctx, doneMsg)
			}
			if err := s.w.writeBinary(ctx, b); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					interrupted = true
					break pcmLoop
				}
				if s.log != nil {
					s.log.Error("web voice pcm write", "err", err)
				}
				return err
			}
		}
	}
	if interrupted {
		writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.w.writeBinary(writeCtx, []byte{voiceClearSentinel}); err != nil && s.log != nil {
			s.log.Debug("web voice clear sentinel", "session", s.session, "err", err)
		}
		synthRes.Wait()
		s.logRenderDone(ctx, synthRes.Err, true)
		return ctx.Err()
	}
	return nil
}

func (s *wsVoiceSink) logRenderDone(ctx context.Context, synthErr error, interrupted bool) {
	if s == nil || s.log == nil {
		return
	}
	s.log.Debug("web voice render done",
		"session", s.session,
		"interrupted", interrupted,
		"synthErr", synthErr,
		"ctxErr", ctx.Err())
}

var _ converse.Sink = (*wsVoiceSink)(nil)
