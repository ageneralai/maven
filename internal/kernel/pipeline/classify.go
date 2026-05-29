package pipeline

import (
	"github.com/ageneralai/maven/internal/kernel/bus"
	"github.com/ageneralai/maven/internal/kernel/channels"
	"github.com/ageneralai/maven/internal/kernel/session"
)

type turnPlan struct {
	useStream   bool
	slashName   string
	sessionMode session.SessionMode
}

func classifyTurn(msg bus.InboundMessage, ch channels.Channel) turnPlan {
	plan := turnPlan{
		slashName:   msg.Hints.SlashCommand,
		sessionMode: msg.Hints.SessionMode,
	}
	if ch != nil && !msg.Hints.ForceSync {
		if _, ok := ch.(channels.StreamChannel); ok {
			plan.useStream = true
		}
	}
	return plan
}
