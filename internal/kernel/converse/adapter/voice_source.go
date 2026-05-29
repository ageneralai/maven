package adapter

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ageneralai/maven/internal/kernel/converse"
	"github.com/ageneralai/maven/internal/kernel/voice"
)

// PCMOpener opens a raw PCM stream (s16le mono) for the lifetime of ctx.
type PCMOpener func(ctx context.Context) (<-chan []byte, error)

// VoiceSourceConfig wires mic PCM + STT into converse events.
type VoiceSourceConfig struct {
	Open    PCMOpener
	STT     voice.STT
	Log     *slog.Logger
	Session string
}

type voiceSource struct {
	open    PCMOpener
	stt     voice.STT
	log     *slog.Logger
	session string
}

// NewVoiceSource turns a PCM stream + STT into converse events:
// VAD onset → SpeechStart; final transcript → Utterance.
func NewVoiceSource(cfg VoiceSourceConfig) converse.Source {
	return &voiceSource{
		open:    cfg.Open,
		stt:     cfg.STT,
		log:     cfg.Log,
		session: cfg.Session,
	}
}

func (s *voiceSource) Listen(ctx context.Context) <-chan converse.Event {
	out := make(chan converse.Event, 4)
	go func() {
		var wg sync.WaitGroup
		defer func() {
			wg.Wait()
			close(out)
		}()
		if s == nil || s.open == nil || s.stt == nil {
			return
		}
		pcm, err := s.open(ctx)
		if err != nil {
			s.logError("voice pcm open", "err", err)
			return
		}
		tees := converse.Tee(ctx, pcm, 2)
		pcmVAD, pcmSTT := tees[0], tees[1]
		vad := &voice.VAD{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for chunk := range pcmVAD {
				if vad.Push(chunk) == voice.SpeechStart {
					s.logDebug("voice vad interrupt", "rms", voice.RMS(chunk))
					select {
					case <-ctx.Done():
						return
					case out <- converse.SpeechStart{}:
					}
				}
			}
		}()
		txtCh, err := s.stt.Transcribe(ctx, pcmSTT)
		if err != nil {
			s.logError("voice stt", "err", err)
			return
		}
		var transcriptN atomic.Int64
		for {
			select {
			case <-ctx.Done():
				return
			case t, ok := <-txtCh:
				if !ok {
					return
				}
				t = strings.TrimSpace(t)
				if t == "" {
					continue
				}
				s.logDebug("voice transcript", "n", transcriptN.Add(1), "text", t)
				select {
				case <-ctx.Done():
					return
				case out <- converse.Utterance{Text: t}:
				}
			}
		}
	}()
	return out
}

func (s *voiceSource) logDebug(msg string, args ...any) {
	if s == nil || s.log == nil {
		return
	}
	s.log.Debug(msg, s.logArgs(args)...)
}

func (s *voiceSource) logError(msg string, args ...any) {
	if s == nil || s.log == nil {
		return
	}
	s.log.Error(msg, s.logArgs(args)...)
}

func (s *voiceSource) logArgs(args []any) []any {
	if s.session == "" {
		return args
	}
	out := make([]any, 0, len(args)+2)
	out = append(out, "session", s.session)
	return append(out, args...)
}

var _ converse.Source = (*voiceSource)(nil)
