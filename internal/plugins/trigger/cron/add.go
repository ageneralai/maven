package cron

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type AddParams struct {
	Name                  string
	Message               string
	Expr                  string
	In                    string
	AtMs                  int64
	HasAtMs               bool
	Deliver               bool
	Channel               string
	To                    string
	DeliverToIncomingChat bool
	MessageID             int
}

func Add(s *Service, p AddParams, now time.Time) (*CronJob, error) {
	p.Name = strings.TrimSpace(p.Name)
	p.Message = strings.TrimSpace(p.Message)
	p.Expr = strings.TrimSpace(p.Expr)
	p.In = strings.TrimSpace(p.In)
	p.Channel = strings.TrimSpace(p.Channel)
	p.To = strings.TrimSpace(p.To)
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if p.Message == "" {
		return nil, fmt.Errorf("message is required")
	}
	pp := p
	if pp.DeliverToIncomingChat {
		pp.Deliver = true
	}
	n := 0
	if pp.Expr != "" {
		n++
	}
	if pp.In != "" {
		n++
	}
	if pp.HasAtMs {
		n++
	}
	if n != 1 {
		return nil, fmt.Errorf("exactly one of expr, in, or at_ms is required")
	}
	var sch Schedule
	switch {
	case pp.Expr != "":
		sch = CronSchedule{Expr: pp.Expr}
	case pp.In != "":
		d, err := time.ParseDuration(pp.In)
		if err != nil {
			return nil, fmt.Errorf("in: %w", err)
		}
		sch = AtSchedule{At: now.Add(d.Round(time.Millisecond))}
	default:
		sch = AtSchedule{At: time.UnixMilli(pp.AtMs)}
	}
	if pp.Deliver && (pp.Channel == "" || pp.To == "") {
		return nil, fmt.Errorf("deliver requires channel and to, or use deliver_to_incoming_chat in the current chat session")
	}
	payload := Payload{
		Message: pp.Message,
		Deliver: pp.Deliver,
		Channel: pp.Channel,
		To:      pp.To,
	}
	if err := payload.Validate(); err != nil {
		return nil, err
	}
	return s.AddJob(pp.Name, sch, payload)
}

func AddFromToolMap(s *Service, ctx context.Context, m map[string]any, now time.Time) (*CronJob, error) {
	in, err := ParseCronToolInput(m)
	if err != nil {
		return nil, err
	}
	in.ApplyGatewayDeliveryDefaults(ctx)
	if err := in.ValidateDeliveryPolicy(ctx); err != nil {
		return nil, err
	}
	return Add(s, in.ToAddParams(ctx), now)
}
