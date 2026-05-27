package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	turnctx "github.com/ageneralai/maven/internal/kernel/turnctx"
)

// CronToolInput is parsed JSON for cron-schedule / cron-add tool flows (inbound-chat defaults applied separately).
type CronToolInput struct {
	Name                  string
	Message               string
	Expr                  string
	In                    string
	AtMs                  int64
	HasAtMs               bool
	Deliver               bool
	DeliverKeyPresent     bool
	DeliverToIncomingChat bool
	Channel               string
	To                    string
}

func ParseCronToolInput(m map[string]any) (CronToolInput, error) {
	var in CronToolInput
	in.Name = stringFromMap(m, "name")
	in.Message = stringFromMap(m, "message")
	in.Expr = stringFromMap(m, "expr")
	in.In = stringFromMap(m, "in")
	if v, ok := m["at_ms"]; ok && v != nil {
		x, err := numberToInt64(v)
		if err != nil {
			return in, fmt.Errorf("at_ms: %w", err)
		}
		in.AtMs = x
		in.HasAtMs = true
	}
	if _, ok := m["deliver"]; ok {
		in.DeliverKeyPresent = true
	}
	in.Deliver = truthyMap(m["deliver"])
	in.DeliverToIncomingChat = truthyMap(m["deliver_to_incoming_chat"])
	in.Channel = stringFromMap(m, "channel")
	in.To = stringFromMap(m, "to")
	return in, nil
}

func (in *CronToolInput) ApplyGatewayDeliveryDefaults(ctx context.Context) {
	if in.DeliverKeyPresent {
		return
	}
	if in.DeliverToIncomingChat {
		return
	}
	if in.Channel != "" || in.To != "" {
		return
	}
	if _, ok := turnctx.From(ctx); !ok {
		return
	}
	in.DeliverToIncomingChat = true
}

func (in *CronToolInput) ValidateDeliveryPolicy(ctx context.Context) error {
	deliverIncoming := in.DeliverToIncomingChat
	deliver := in.Deliver
	ch := strings.TrimSpace(in.Channel)
	to := strings.TrimSpace(in.To)
	if deliverIncoming && deliver {
		return fmt.Errorf("tool: use either deliver_to_incoming_chat or deliver with channel and to — not both")
	}
	if deliverIncoming {
		if ch != "" || to != "" {
			return fmt.Errorf("tool: with deliver_to_incoming_chat omit channel and to (they come from the current chat)")
		}
		if _, ok := turnctx.From(ctx); !ok {
			return fmt.Errorf("tool: deliver_to_incoming_chat needs an active conversation (missing inbound channel or chat id)")
		}
		return nil
	}
	if deliver {
		if ch == "" || to == "" {
			return fmt.Errorf("tool: deliver=true requires non-empty channel and to")
		}
		if IsReservedRecipient(to) {
			return fmt.Errorf("tool: invalid to=%q — use boolean deliver_to_incoming_chat for same-chat delivery, not a magic string in to", to)
		}
		return nil
	}
	if ch != "" || to != "" {
		return fmt.Errorf("tool: omit channel and to unless deliver=true or deliver_to_incoming_chat=true (got channel=%q to=%q)", ch, to)
	}
	return nil
}

func (in CronToolInput) ToAddParams(ctx context.Context) AddParams {
	p := AddParams{
		Name:                  in.Name,
		Message:               in.Message,
		Expr:                  in.Expr,
		In:                    in.In,
		AtMs:                  in.AtMs,
		HasAtMs:               in.HasAtMs,
		Deliver:               in.Deliver,
		Channel:               in.Channel,
		To:                    in.To,
		DeliverToIncomingChat: in.DeliverToIncomingChat,
	}
	if in.DeliverToIncomingChat {
		tc, ok := turnctx.From(ctx)
		if !ok {
			return p
		}
		p.Channel = tc.Channel
		p.To = tc.ChatID
		if mid, has := tc.Metadata["message_id"]; has {
			p.MessageID = turnctx.IntFromAny(mid)
		}
	}
	return p
}

func stringFromMap(m map[string]any, key string) string {
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

func truthyMap(vi any) bool {
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

func numberToInt64(v any) (int64, error) {
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
