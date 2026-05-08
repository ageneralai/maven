package log

import (
	stdlog "log"
)

// PrintLogger is printf-style logging only — a stepping stone until structured
// logging (Priority 3). Std() implements it via the standard library log package.
type PrintLogger interface {
	Printf(format string, v ...any)
}

type stdLogger struct{}

func (stdLogger) Printf(format string, v ...any) { stdlog.Printf(format, v...) }

// Std returns a PrintLogger that delegates to the standard library log package.
func Std() PrintLogger { return stdLogger{} }
