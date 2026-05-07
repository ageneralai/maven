package cronschedule

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/stellarlinkco/maven/internal/cron"
	"github.com/stellarlinkco/maven/internal/inboundctx"
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
}

func Add(svc *cron.Service, p AddParams, now time.Time) (*cron.CronJob, error) {
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
	var sch cron.Schedule
	switch {
	case pp.Expr != "":
		sch = cron.Schedule{Kind: "cron", Expr: pp.Expr}
	case pp.In != "":
		d, err := time.ParseDuration(pp.In)
		if err != nil {
			return nil, fmt.Errorf("in: %w", err)
		}
		sch = cron.Schedule{Kind: "at", AtMs: now.UnixMilli() + d.Round(time.Millisecond).Milliseconds()}
	default:
		sch = cron.Schedule{Kind: "at", AtMs: pp.AtMs}
	}
	if pp.Deliver && (pp.Channel == "" || pp.To == "") {
		return nil, fmt.Errorf("deliver requires channel and to, or use deliver_to_incoming_chat in a gateway chat session")
	}
	return svc.AddJob(pp.Name, sch, cron.Payload{
		Message: pp.Message,
		Deliver: pp.Deliver,
		Channel: pp.Channel,
		To:      pp.To,
	})
}

func AddFromToolMap(svc *cron.Service, ctx context.Context, m map[string]interface{}, now time.Time) (*cron.CronJob, error) {
	if err := validateCronDelivery(ctx, m); err != nil {
		return nil, err
	}
	p := AddParams{
		Name:    stringFrom(m, "name"),
		Message: stringFrom(m, "message"),
		Expr:    stringFrom(m, "expr"),
		In:      stringFrom(m, "in"),
	}
	if v, ok := m["at_ms"]; ok && v != nil {
		x, err := numberToInt64(v)
		if err != nil {
			return nil, fmt.Errorf("at_ms: %w", err)
		}
		p.AtMs = x
		p.HasAtMs = true
	}
	if truthy(m["deliver_to_incoming_chat"]) {
		p.DeliverToIncomingChat = true
	}
	if truthy(m["deliver"]) {
		p.Deliver = true
	}
	p.Channel = stringFrom(m, "channel")
	p.To = stringFrom(m, "to")
	if p.DeliverToIncomingChat {
		ch, _ := inboundctx.Channel(ctx)
		id, _ := inboundctx.ChatID(ctx)
		p.Channel = ch
		p.To = id
	}
	return Add(svc, p, now)
}

func stringFrom(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func truthy(vi interface{}) bool {
	if vi == nil {
		return false
	}
	v, ok := vi.(bool)
	if ok {
		return v
	}
	s := strings.ToLower(strings.TrimSpace(fmt.Sprint(vi)))
	return s == "true" || s == "1" || s == "yes"
}

func numberToInt64(v interface{}) (int64, error) {
	switch t := v.(type) {
	case int64:
		return t, nil
	case int:
		return int64(t), nil
	case float64:
		return int64(t), nil
	case json.Number:
		return t.Int64()
	default:
		s := strings.TrimSpace(fmt.Sprint(t))
		return strconv.ParseInt(s, 10, 64)
	}
}
