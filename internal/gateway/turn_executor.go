package gateway

import (
	"context"
	"fmt"
	"strings"

	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/channel"
	"github.com/ageneralai/maven/internal/cron"
	mavenlog "github.com/ageneralai/maven/pkg/log"
	"github.com/ageneralai/maven/internal/pipeline"
	"github.com/google/uuid"
)

const cronSessionPrefix = "maven:cron-"

// gatewayTurnExecutor runs the pipeline and performs cron outbound delivery when the session id is a cron run.
type gatewayTurnExecutor struct {
	pipeFn   func() *pipeline.Pipeline
	bus      *bus.MessageBus
	channels *channel.ChannelManager
	log      mavenlog.PrintLogger
	cron     *cron.Service
}

func (e *gatewayTurnExecutor) RunTurn(ctx context.Context, prompt, sessionID string) (string, error) {
	p := e.pipeFn()
	if p == nil {
		return "", fmt.Errorf("gateway: pipeline not initialized")
	}
	if jobID, ok := parseCronJobSessionID(sessionID); ok {
		if job, ok := e.cron.JobByID(jobID); ok {
			if err := job.Payload.Validate(); err != nil {
				return "", err
			}
		}
	}
	out, err := p.RunText(ctx, prompt, sessionID, nil)
	if err != nil {
		return out, err
	}
	jobID, ok := parseCronJobSessionID(sessionID)
	if !ok {
		return out, nil
	}
	job, ok := e.cron.JobByID(jobID)
	if !ok || !job.Payload.Deliver {
		return out, nil
	}
	ch := e.channels.GetChannel(job.Payload.Channel)
	if ch != nil && ch.Capabilities().ReactiveOnly {
		e.log.Printf("[gateway] cron deliver skipped: channel %q is reactive-only", job.Payload.Channel)
		return out, nil
	}
	e.bus.Outbound <- bus.OutboundMessage{
		Channel: job.Payload.Channel,
		ChatID:  job.Payload.To,
		Content: out,
	}
	return out, nil
}

func parseCronJobSessionID(sessionID string) (jobID string, ok bool) {
	if !strings.HasPrefix(sessionID, cronSessionPrefix) {
		return "", false
	}
	rest := strings.TrimPrefix(sessionID, cronSessionPrefix)
	if len(rest) < 38 {
		return "", false
	}
	uuidPart := rest[len(rest)-36:]
	if _, err := uuid.Parse(uuidPart); err != nil {
		return "", false
	}
	prefix := rest[:len(rest)-36]
	if len(prefix) == 0 || prefix[len(prefix)-1] != '-' {
		return "", false
	}
	return prefix[:len(prefix)-1], true
}
