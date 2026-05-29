package adapter

import (
	"context"
	"log/slog"
)

func logVoiceTurnInterrupted(lg *slog.Logger, session string, ctx context.Context) {
	if lg == nil || ctx.Err() == nil {
		return
	}
	lg.Debug("voice turn interrupted", "session", session, "err", ctx.Err())
}
