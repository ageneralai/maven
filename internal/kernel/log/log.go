package log

import (
	"log/slog"
	"os"

	"golang.org/x/term"
)

var stdLevel slog.LevelVar

// New returns a *slog.Logger backed by the shared process-level LevelVar.
// Calling New sets the initial threshold; SetLevel adjusts it at any time (e.g. on config reload).
func New(level slog.Level) *slog.Logger {
	stdLevel.Set(level)
	opts := &slog.HandlerOptions{Level: &stdLevel}
	var handler slog.Handler
	if term.IsTerminal(int(os.Stderr.Fd())) {
		handler = slog.NewTextHandler(os.Stderr, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	}
	return slog.New(handler)
}

// SetLevel updates the shared process log threshold without recreating handlers.
func SetLevel(level slog.Level) {
	stdLevel.Set(level)
}
