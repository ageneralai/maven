package voice

import (
	"context"
	"errors"
	"strings"
	"sync"

	pkgvoice "github.com/ageneralai/maven/pkg/voice"
)

// Session coordinates mic-scoped STT and agent-scoped TTS without transport.
type Session struct {
	stt pkgvoice.STT
	tts pkgvoice.TTS

	micCtx    context.Context
	micCancel context.CancelFunc

	mu          sync.Mutex
	agentCancel context.CancelFunc
}

func NewSession(ctx context.Context, stt pkgvoice.STT, tts pkgvoice.TTS) *Session {
	micCtx, micCancel := context.WithCancel(ctx)
	return &Session{
		stt:       stt,
		tts:       tts,
		micCtx:    micCtx,
		micCancel: micCancel,
	}
}

func (s *Session) Interrupt() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.agentCancel != nil {
		s.agentCancel()
		s.agentCancel = nil
	}
}

func (s *Session) Close() {
	s.Interrupt()
	s.micCancel()
}

func (s *Session) RunSTT(audio <-chan []byte, onTranscript func(string)) error {
	txtCh, err := s.stt.Transcribe(s.micCtx, audio)
	if err != nil {
		return err
	}
	for {
		select {
		case <-s.micCtx.Done():
			return nil
		case t, ok := <-txtCh:
			if !ok {
				return nil
			}
			t = strings.TrimSpace(t)
			if t != "" {
				onTranscript(t)
			}
		}
	}
}

func drainTTSChunks(ctx context.Context, chunks <-chan []byte, writeAudio func([]byte) error) error {
	var pending []byte
	for {
		select {
		case <-ctx.Done():
			return nil
		case chunk, ok := <-chunks:
			if !ok {
				return nil
			}
			if len(chunk) == 0 {
				continue
			}
			if len(pending) > 0 {
				chunk = append(pending, chunk...)
				pending = nil
			}
			if len(chunk)%2 != 0 {
				pending = append(pending, chunk[len(chunk)-1])
				chunk = chunk[:len(chunk)-1]
			}
			if len(chunk) == 0 {
				continue
			}
			if err := writeAudio(chunk); err != nil {
				return err
			}
		}
	}
}

func (s *Session) RunTTS(ctx context.Context, textCh <-chan string, writeAudio func([]byte) error) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case text, ok := <-textCh:
			if !ok {
				return nil
			}
			text = strings.TrimSpace(text)
			if text == "" {
				continue
			}
			chunks, err := s.tts.Synthesize(ctx, text)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				return err
			}
			if err := drainTTSChunks(ctx, chunks, writeAudio); err != nil {
				return err
			}
		}
	}
}

func (s *Session) NewAgentCtx() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.agentCancel != nil {
		s.agentCancel()
	}
	ctx, cancel := context.WithCancel(s.micCtx)
	s.agentCancel = cancel
	return ctx
}
