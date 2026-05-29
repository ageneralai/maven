package adapter

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
)

type streamConfig struct {
	session string
	log     *slog.Logger
	errOut  io.Writer
}

func streamDeltas(ctx context.Context, cfg streamConfig, open func() (<-chan api.StreamEvent, error)) <-chan string {
	out := make(chan string, 64)
	go func() {
		defer close(out)
		if open == nil {
			return
		}
		events, err := open()
		if err != nil {
			reportStreamError(cfg, err)
			return
		}
		if events == nil {
			return
		}
		for delta := range Deltas(ctx, events) {
			select {
			case <-ctx.Done():
				logVoiceTurnInterrupted(ctx, cfg.log, cfg.session)
				return
			case out <- delta:
			}
		}
	}()
	return out
}

func reportStreamError(cfg streamConfig, err error) {
	if cfg.log != nil {
		cfg.log.Error("voice agent stream", "session", cfg.session, "err", err)
	}
	if cfg.errOut != nil {
		_, _ = fmt.Fprintf(cfg.errOut, "Error: %v\n", err)
	}
}

func logVoiceTurnInterrupted(ctx context.Context, lg *slog.Logger, session string) {
	if lg == nil || ctx.Err() == nil {
		return
	}
	lg.Debug("voice turn interrupted", "session", session, "err", ctx.Err())
}
