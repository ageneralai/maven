package voice

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	pkgvoice "github.com/ageneralai/maven/pkg/voice"
)

// Session owns one voice route’s STT/TTS lifecycle and streaming TTS coordination.
type Session struct {
	stt       pkgvoice.STT
	tts       pkgvoice.TTS
	mu        sync.Mutex
	ttsCancel context.CancelFunc
	speaking  atomic.Bool
}

// NewSession wires provider implementations created once per transport session.
func NewSession(stt pkgvoice.STT, tts pkgvoice.TTS) *Session {
	return &Session{stt: stt, tts: tts}
}

// Speaking is true for the duration of one assistant StreamEventsToTTS response (all phrases).
func (s *Session) Speaking() bool {
	return s.speaking.Load()
}

// InterruptPlayback cancels in-flight phrase synthesis and optionally sends a transport-level clear (e.g. sentinel bytes).
func (s *Session) InterruptPlayback(ctx context.Context, sendClear func(context.Context) error) error {
	s.mu.Lock()
	if s.ttsCancel != nil {
		s.ttsCancel()
		s.ttsCancel = nil
	}
	s.mu.Unlock()
	if sendClear != nil {
		return sendClear(ctx)
	}
	return nil
}

// ConsumeTranscripts runs STT until audio input ends or ctx is done. onFinal receives trimmed non-empty transcripts.
func (s *Session) ConsumeTranscripts(ctx context.Context, audio <-chan []byte, onFinal func(context.Context, string) error) error {
	txtCh, err := s.stt.Transcribe(ctx, audio)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case t, ok := <-txtCh:
			if !ok {
				return nil
			}
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			if err := onFinal(ctx, t); err != nil {
				return err
			}
		}
	}
}

// StreamEventsToTTS maps streamed model deltas to TTS binary chunks via sentence-sized phrases.
func (s *Session) StreamEventsToTTS(ctx context.Context, events <-chan api.StreamEvent, writeBinary func(context.Context, []byte) error) error {
	s.speaking.Store(true)
	defer s.speaking.Store(false)
	sentenceBuf := ""
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-events:
			if !ok {
				tail := pkgvoice.FlushRemainder(&sentenceBuf)
				if err := s.synthPhrase(ctx, tail, writeBinary); err != nil {
					return err
				}
				return nil
			}
			if ev.Type == api.EventError {
				return streamEventError(ev)
			}
			if ev.Type == api.EventContentBlockDelta && ev.Delta != nil && ev.Delta.Text != "" {
				sentenceBuf += ev.Delta.Text
				for _, sent := range pkgvoice.TakeCompleteSentences(&sentenceBuf) {
					if err := s.synthPhrase(ctx, sent, writeBinary); err != nil {
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

func (s *Session) synthPhrase(ctx context.Context, text string, writeBinary func(context.Context, []byte) error) error {
	t := strings.TrimSpace(text)
	if t == "" {
		return nil
	}
	subCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	if s.ttsCancel != nil {
		s.ttsCancel()
	}
	s.ttsCancel = cancel
	s.mu.Unlock()
	defer cancel()
	chunks, err := s.tts.Synthesize(subCtx, t)
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
