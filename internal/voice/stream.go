package voice

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	pkgvoice "github.com/ageneralai/maven/pkg/voice"
)

// StreamEventsToTTS maps streamed model deltas to streaming TTS audio using sentence-sized phrases.
func StreamEventsToTTS(ctx context.Context, tts pkgvoice.TTS, events <-chan api.StreamEvent, writeBinary func(context.Context, []byte) error, speaking *atomic.Bool, storePhraseCancel func(context.CancelFunc)) error {
	sentenceBuf := ""
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-events:
			if !ok {
				tail := pkgvoice.FlushRemainder(&sentenceBuf)
				if err := synthPhrase(ctx, tts, tail, writeBinary, speaking, storePhraseCancel); err != nil {
					return err
				}
				return nil
			}
			if ev.Type == api.EventError {
				return streamEventError(ev)
			}
			if ev.Type == api.EventContentBlockDelta && ev.Delta != nil && ev.Delta.Text != "" {
				sentenceBuf += ev.Delta.Text
				sents := pkgvoice.TakeCompleteSentences(&sentenceBuf)
				for _, sent := range sents {
					if err := synthPhrase(ctx, tts, sent, writeBinary, speaking, storePhraseCancel); err != nil {
						return err
					}
				}
			}
		}
	}
}

func streamEventError(ev api.StreamEvent) error {
	msg := strings.TrimSpace(fmt.Sprintf("%v", ev.Output))
	if msg == "" {
		msg = "stream error"
	}
	return fmt.Errorf("%s", msg)
}

func synthPhrase(ctx context.Context, tts pkgvoice.TTS, text string, writeBinary func(context.Context, []byte) error, speaking *atomic.Bool, storePhraseCancel func(context.CancelFunc)) error {
	t := strings.TrimSpace(text)
	if t == "" {
		return nil
	}
	subCtx, cancel := context.WithCancel(ctx)
	if storePhraseCancel != nil {
		storePhraseCancel(cancel)
	}
	if speaking != nil {
		speaking.Store(true)
	}
	defer func() {
		cancel()
		if speaking != nil {
			speaking.Store(false)
		}
	}()
	chunks, err := tts.Synthesize(subCtx, t)
	if err != nil {
		return err
	}
	for chunk := range chunks {
		select {
		case <-subCtx.Done():
			return nil
		default:
		}
		if len(chunk) == 0 {
			continue
		}
		if err := writeBinary(subCtx, chunk); err != nil {
			return err
		}
	}
	return nil
}
