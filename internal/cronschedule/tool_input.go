package cronschedule

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ageneralai/maven/internal/cron"
	"github.com/ageneralai/maven/internal/inboundctx"
)

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

func ParseCronToolInput(m map[string]interface{}) (CronToolInput, error) {
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
	if _, ok := inboundctx.Channel(ctx); !ok {
		return
	}
	if _, ok := inboundctx.ChatID(ctx); !ok {
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
		return fmt.Errorf("cronschedule: use either deliver_to_incoming_chat or deliver with channel and to, not both")
	}
	if deliverIncoming {
		if ch != "" || to != "" {
			return fmt.Errorf("cronschedule: with deliver_to_incoming_chat omit channel and to (they come from the current gateway chat)")
		}
		_, okCh := inboundctx.Channel(ctx)
		_, okID := inboundctx.ChatID(ctx)
		if !okCh || !okID {
			return fmt.Errorf("cronschedule: deliver_to_incoming_chat needs an active gateway conversation (missing inbound channel or chat id)")
		}
		return nil
	}
	if deliver {
		if ch == "" || to == "" {
			return fmt.Errorf("cronschedule: deliver=true requires non-empty channel and to")
		}
		if cron.IsReservedRecipient(to) {
			return fmt.Errorf("cronschedule: invalid to=%q — use boolean deliver_to_incoming_chat for same-chat delivery, not a magic string in to", to)
		}
		return nil
	}
	if ch != "" || to != "" {
		return fmt.Errorf("cronschedule: omit channel and to unless deliver=true or deliver_to_incoming_chat=true (got channel=%q to=%q)", ch, to)
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
		ch, _ := inboundctx.Channel(ctx)
		id, _ := inboundctx.ChatID(ctx)
		p.Channel = ch
		p.To = id
	}
	return p
}

func stringFromMap(m map[string]interface{}, key string) string {
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

func truthyMap(vi interface{}) bool {
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
