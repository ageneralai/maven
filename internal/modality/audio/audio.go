package audio

import "context"

// Capture streams mic PCM s16le 16 kHz mono from an external process.
type Capture interface {
	Capture(ctx context.Context) (<-chan []byte, error)
}

// Playback plays PCM s16le 24 kHz mono via an external process.
type Playback interface {
	Play(ctx context.Context, pcm <-chan []byte) error
}
