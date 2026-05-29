package voice

import (
	"context"
	"log/slog"
	"strings"
	"time"
	"unicode"

	"github.com/ageneralai/maven/internal/kernel/converse"
)

// DefaultWakeWindow is a defensive fallback when an invalid window is supplied;
// the authoritative default lives in config (DefaultWakeTimeoutMs).
const DefaultWakeWindow = 8 * time.Second

// NewWakeGate wraps src so voice turns require a spoken wake phrase. The phrase
// opens a conversation window: utterances pass through until the window idles
// out with no speech, then the gate re-arms to dormant. The idle timer runs
// only between turns — it pauses while Maven is generating or playing a reply.
// An empty (or punctuation-only) phrase returns src unchanged, preserving stock
// always-on voice.
func NewWakeGate(src converse.Source, phrase string, window time.Duration, replyDone <-chan struct{}, log *slog.Logger) converse.Source {
	normalized := normalizeWake(phrase)
	if normalized == "" {
		return src
	}
	if window <= 0 {
		window = DefaultWakeWindow
	}
	return &wakeGate{source: src, phrase: normalized, window: window, replyDone: replyDone, log: log}
}

type wakeGate struct {
	source    converse.Source
	phrase    string
	window    time.Duration
	replyDone <-chan struct{}
	log       *slog.Logger
}

func (g *wakeGate) Listen(ctx context.Context) <-chan converse.Event {
	inner := g.source.Listen(ctx)
	out := make(chan converse.Event, 4)
	go func() {
		defer close(out)
		active := false
		awaitingReply := false
		timer := time.NewTimer(g.window)
		stopTimer(timer)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				active = false
				awaitingReply = false
				g.logDebug("wake window closed")
			case <-g.replyDone:
				if active {
					awaitingReply = false
					resetTimer(timer, g.window)
					g.logDebug("wake window extended", "reason", "reply finished")
				}
			case ev, ok := <-inner:
				if !ok {
					return
				}
				if !g.step(ctx, out, ev, &active, &awaitingReply, timer) {
					return
				}
			}
		}
	}()
	return out
}

func (g *wakeGate) step(ctx context.Context, out chan<- converse.Event, ev converse.Event, active, awaitingReply *bool, timer *time.Timer) bool {
	switch e := ev.(type) {
	case converse.SpeechStart:
		if !*active {
			return true
		}
		return g.forward(ctx, out, ev)
	case converse.Utterance:
		if *active {
			*awaitingReply = true
			stopTimer(timer)
			return g.forward(ctx, out, ev)
		}
		remainder, matched := matchWake(e.Text, g.phrase)
		if !matched {
			return true
		}
		*active = true
		*awaitingReply = true
		stopTimer(timer)
		g.logDebug("wake activated", "phrase", g.phrase, "command", remainder)
		prompt := remainder
		if prompt == "" {
			prompt = strings.TrimSpace(e.Text)
		}
		return g.forward(ctx, out, converse.Utterance{Text: prompt})
	default:
		return true
	}
}

func (g *wakeGate) logDebug(msg string, args ...any) {
	if g.log == nil {
		return
	}
	g.log.Debug(msg, args...)
}

func (g *wakeGate) forward(ctx context.Context, out chan<- converse.Event, ev converse.Event) bool {
	select {
	case <-ctx.Done():
		return false
	case out <- ev:
		return true
	}
}

func stopTimer(t *time.Timer) {
	if !t.Stop() {
		drainTimer(t)
	}
}

func resetTimer(t *time.Timer, d time.Duration) {
	stopTimer(t)
	t.Reset(d)
}

func drainTimer(t *time.Timer) {
	select {
	case <-t.C:
	default:
	}
}

// normalizeWake lowercases, drops punctuation, and collapses whitespace to single
// spaces so STT artifacts ("Hey, Maven.") match a configured phrase ("hey maven").
func normalizeWake(s string) string {
	tokens := strings.Fields(s)
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if n := normalizeToken(t); n != "" {
			out = append(out, n)
		}
	}
	return strings.Join(out, " ")
}

func normalizeToken(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// matchWake reports whether utterance begins with the normalized phrase. It matches
// on normalized leading tokens but returns the remainder sliced from the ORIGINAL
// tokens, preserving command casing ("Hey Maven, email Bob" -> "email Bob").
func matchWake(utterance, phrase string) (string, bool) {
	phraseTokens := strings.Fields(phrase)
	if len(phraseTokens) == 0 {
		return "", false
	}
	origTokens := strings.Fields(utterance)
	if len(origTokens) < len(phraseTokens) {
		return "", false
	}
	for i, pt := range phraseTokens {
		if normalizeToken(origTokens[i]) != pt {
			return "", false
		}
	}
	return strings.TrimSpace(strings.Join(origTokens[len(phraseTokens):], " ")), true
}

var _ converse.Source = (*wakeGate)(nil)
