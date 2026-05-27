package log

import (
	"log/slog"
	"os"

	"golang.org/x/term"
)

// Std returns a *slog.Logger with a text handler for TTY or JSON handler otherwise.
func Std() *slog.Logger {
	var handler slog.Handler
	if term.IsTerminal(int(os.Stderr.Fd())) {
		handler = slog.NewTextHandler(os.Stderr, nil)
	} else {
		handler = slog.NewJSONHandler(os.Stderr, nil)
	}
	return slog.New(handler)
}
