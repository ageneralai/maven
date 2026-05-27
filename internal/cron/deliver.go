package cron

import (
	"context"

	"log/slog"

	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/channel/manager"
)

// Deliver performs proactive cron output delivery to the message bus when a job
// completes successfully. Nil or partial fields are ignored.
type Deliver struct {
	Bus      *bus.MessageBus
	Channels *manager.ChannelManager
	Log      *slog.Logger
}

// AfterSuccessfulRun enqueues outbound text for jobs with deliver=true, unless the
// target channel is ReactiveOnly (reply-path only).
func (d *Deliver) AfterSuccessfulRun(ctx context.Context, job CronJob, output string) {
	if d == nil || d.Bus == nil || d.Channels == nil {
		return
	}
	if !job.Payload.Deliver {
		return
	}
	ch := d.Channels.GetChannel(job.Payload.Channel)
	if ch != nil && ch.Capabilities().ReactiveOnly {
		if d.Log != nil {
			d.Log.Info("cron deliver skipped: channel is reactive-only", "channel", job.Payload.Channel)
		}
		return
	}
	_ = d.Bus.PublishOutbound(ctx, bus.OutboundMessage{
		Channel: job.Payload.Channel,
		ChatID:  job.Payload.To,
		Content: output,
	})
}
