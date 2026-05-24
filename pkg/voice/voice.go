// Package voice defines provider-agnostic speech primitives (STT/TTS, audio I/O).
package voice

import "context"

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

// TTSProvider is the plugin-facing name for TTS (same method set).
type TTSProvider = TTS

// STTProvider is the plugin-facing name for STT (same method set).
type STTProvider = STT
