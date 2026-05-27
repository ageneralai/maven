package cron

import (
	"context"

	"log/slog"

	"github.com/ageneralai/maven/internal/kernel/channel"
	"github.com/ageneralai/maven/internal/kernel/plugin"
)

// ChannelLookup resolves a channel for delivery policy checks.
type ChannelLookup interface {
	GetChannel(name string) channels.Channel
}

// Deliver performs proactive cron output delivery when a job completes successfully.
type Deliver struct {
	Pub      plugin.OutboundPublisher
	Channels ChannelLookup
	Log      *slog.Logger
}

func (d *Deliver) AfterSuccessfulRun(ctx context.Context, job CronJob, output string) {
	if d == nil || d.Pub == nil {
		return
	}
	if !job.Payload.Deliver {
		return
	}
	if d.Channels != nil {
		if ch := d.Channels.GetChannel(job.Payload.Channel); ch != nil && ch.Capabilities().ReactiveOnly {
			if d.Log != nil {
				d.Log.Info("cron deliver skipped: channel is reactive-only", "channel", job.Payload.Channel)
			}
			return
		}
	}
	if err := d.Pub.PublishOutbound(ctx, job.Payload.Channel, job.Payload.To, output); err != nil && d.Log != nil {
		d.Log.Error("cron deliver publish failed", "channel", job.Payload.Channel, "job", job.Name, "err", err)
	}
}
