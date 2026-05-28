package events

import (
	"context"
	"sync"
)

// Subscriber handles one event. Invoked in a goroutine by Fanout.Publish.
type Subscriber func(context.Context, Event)

// Fanout dispatches events to type-specific subscribers and an optional sink.
type Fanout struct {
	mu     sync.RWMutex
	subs   map[string]map[int]Subscriber
	nextID int
	sink   EventPublisher
}

func NewFanout(sink EventPublisher) *Fanout {
	return &Fanout{subs: make(map[string]map[int]Subscriber), sink: OrPublisher(sink)}
}

func (f *Fanout) SetSink(sink EventPublisher) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sink = OrPublisher(sink)
}

func (f *Fanout) Subscribe(typ string, fn Subscriber) func() {
	f.mu.Lock()
	id := f.nextID
	f.nextID++
	if f.subs[typ] == nil {
		f.subs[typ] = make(map[int]Subscriber)
	}
	f.subs[typ][id] = fn
	f.mu.Unlock()
	return func() {
		f.mu.Lock()
		if m := f.subs[typ]; m != nil {
			delete(m, id)
		}
		f.mu.Unlock()
	}
}

func (f *Fanout) Publish(ctx context.Context, e Event) {
	f.mu.RLock()
	sink := f.sink
	var handlers []Subscriber
	if m := f.subs[e.Type]; m != nil {
		handlers = make([]Subscriber, 0, len(m))
		for _, h := range m {
			handlers = append(handlers, h)
		}
	}
	f.mu.RUnlock()
	sink.Publish(ctx, e)
	bg := context.WithoutCancel(ctx)
	for _, h := range handlers {
		go h(bg, e)
	}
}
