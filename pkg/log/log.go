package log

import (
	"log/slog"
	"os"
	"sync"

	"golang.org/x/term"
)

var (
	stdOnce   sync.Once
	stdLogger *slog.Logger
)

// Std returns a *slog.Logger with a text handler for TTY or JSON handler otherwise.
func Std() *slog.Logger {
	stdOnce.Do(func() {
		var handler slog.Handler
		if term.IsTerminal(int(os.Stderr.Fd())) {
			handler = slog.NewTextHandler(os.Stderr, nil)
		} else {
			handler = slog.NewJSONHandler(os.Stderr, nil)
		}
		stdLogger = slog.New(handler)
	})
	return stdLogger
}
