// Package voice defines provider-agnostic speech primitives (STT/TTS, audio I/O).
package voice

import (
	"context"
)

// AudioSource yields inbound audio chunks from a transport (e.g. WebSocket).
type AudioSource interface {
	ReadAudio(ctx context.Context) ([]byte, error)
}

// AudioSink receives outbound audio chunks toward a transport (e.g. WebSocket).
type AudioSink interface {
	WriteAudio(ctx context.Context, chunk []byte) error
}

// STT streams audio chunks in and emits final transcripts only.
type STT interface {
	Transcribe(ctx context.Context, audio <-chan []byte) (<-chan string, error)
}

// TTS converts one text segment to streaming PCM chunks (int16 LE mono, 24 kHz).
// Chunks are raw samples with no container header — providers must be configured
// accordingly (e.g. Deepgram container=none, ElevenLabs output_format=pcm_24000).
type TTS interface {
	Synthesize(ctx context.Context, text string) (<-chan []byte, error)
}
