package converse

import (
	"context"
	"sync"
)

// Converse runs a serialized turn loop: fan-in sources, barge-in on speech start
// or new utterances, tee each reply stream to every sink in its own goroutine.
func Converse(ctx context.Context, sources []Source, sinks []Sink, agent Agent) error {
	events := fanIn(ctx, sources)
	var (
		turnMu     sync.Mutex
		turnCancel context.CancelFunc
		turnWg     sync.WaitGroup
	)
	cancelTurn := func() {
		turnMu.Lock()
		defer turnMu.Unlock()
		if turnCancel != nil {
			turnCancel()
			turnCancel = nil
		}
		turnWg.Wait()
	}
	defer cancelTurn()
	for {
		select {
		case <-ctx.Done():
			cancelTurn()
			return ctx.Err()
		case ev, ok := <-events:
			if !ok {
				turnMu.Lock()
				turnWg.Wait()
				turnMu.Unlock()
				return nil
			}
			cancelTurn()
			switch e := ev.(type) {
			case SpeechStart:
				continue
			case Utterance:
				turnCtx, cancel := context.WithCancel(ctx)
				turnMu.Lock()
				turnCancel = cancel
				turnMu.Unlock()
				reply := agent.Stream(turnCtx, e.Text)
				tees := tee(turnCtx, reply, len(sinks))
				for i, sink := range sinks {
					turnWg.Add(1)
					go func(s Sink, ch <-chan string) {
						defer turnWg.Done()
						_ = s.Render(turnCtx, ch)
					}(sink, tees[i])
				}
			}
		}
	}
}

func fanIn(ctx context.Context, sources []Source) <-chan Event {
	out := make(chan Event, len(sources))
	var wg sync.WaitGroup
	for _, src := range sources {
		wg.Add(1)
		go func(s Source) {
			defer wg.Done()
			for ev := range s.Listen(ctx) {
				select {
				case <-ctx.Done():
					return
				case out <- ev:
				}
			}
		}(src)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func tee(ctx context.Context, in <-chan string, n int) []<-chan string {
	outs := make([]chan string, n)
	for i := range outs {
		outs[i] = make(chan string, 64)
	}
	go func() {
		defer func() {
			for _, ch := range outs {
				close(ch)
			}
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case s, ok := <-in:
				if !ok {
					return
				}
				for _, ch := range outs {
					select {
					case <-ctx.Done():
						return
					case ch <- s:
					}
				}
			}
		}
	}()
	readOnly := make([]<-chan string, n)
	for i, ch := range outs {
		readOnly[i] = ch
	}
	return readOnly
}
