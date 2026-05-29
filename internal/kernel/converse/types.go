package converse

import "context"

type Event interface{ event() }

type Utterance struct{ Text string }

type SpeechStart struct{}

func (Utterance) event()   {}
func (SpeechStart) event() {}

type Source interface {
	Listen(ctx context.Context) <-chan Event
}

type Sink interface {
	Render(ctx context.Context, reply <-chan string) error
}

type Agent interface {
	Stream(ctx context.Context, prompt string) <-chan string
}
