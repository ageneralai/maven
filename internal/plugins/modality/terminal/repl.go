package terminal

import (
	"bufio"
	"context"
	"fmt"
	"io"

	"github.com/ageneralai/maven/internal/kernel/converse"
)

// Session is the single CLI REPL transcript.
// A long-lived goroutine owns stdin; keyboard and voice share the same lines channel.
type Session struct {
	Transcript *Transcript
	lines      <-chan string
}

func NewSession(stdout io.Writer, stdin io.Reader) *Session {
	tx := &Transcript{Out: stdout}
	return &Session{
		Transcript: tx,
		lines:      scanLines(stdin),
	}
}

func (s *Session) PrintYouPrompt() {
	_, _ = fmt.Fprint(s.Transcript, "\n"+userLabel)
}

func (s *Session) Keyboard() converse.Source {
	return &KeyboardSource{lines: s.lines}
}

func (s *Session) Screen() converse.Sink {
	return &TerminalSink{
		session: s,
		screen:  &ScreenSink{Out: s.Transcript, Label: MavenLabel},
	}
}

func (s *Session) Voice(src converse.Source) converse.Source {
	return &voiceLine{source: src, session: s}
}

// TerminalSink wraps ScreenSink and prints "you ▸" only on natural completion.
type TerminalSink struct {
	session *Session
	screen  *ScreenSink
}

func (t *TerminalSink) Render(ctx context.Context, reply <-chan string) error {
	err := t.screen.Render(ctx, reply)
	if err == nil {
		_, _ = fmt.Fprint(t.session.Transcript, "\n"+userLabel)
	}
	return err
}

// scanLines pumps lines from r into a channel for the session lifetime.
func scanLines(r io.Reader) <-chan string {
	ch := make(chan string, 4)
	go func() {
		defer close(ch)
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			ch <- sc.Text()
		}
	}()
	return ch
}
