package pipeline

import (
	"github.com/ageneralai/maven/internal/bus"
	"github.com/ageneralai/maven/internal/channel"
	"github.com/ageneralai/maven/internal/session"
)

type turnPlan struct {
	useStream   bool
	slashName   string
	sessionMode session.SessionMode
}

func classifyTurn(msg bus.InboundMessage, ch channel.Channel) turnPlan {
	plan := turnPlan{
		slashName:   msg.Hints.SlashCommand,
		sessionMode: msg.Hints.SessionMode,
	}
	if ch != nil && !msg.Hints.ForceSync {
		if _, ok := ch.(channel.StreamChannel); ok {
			plan.useStream = true
		}
	}
	return plan
}
