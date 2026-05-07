package cronschedule

import (
	"context"
	"fmt"
	"strings"

	"github.com/stellarlinkco/maven/internal/inboundctx"
)

var reservedRecipientTokens = []string{
	"deliver_to_incoming_chat",
}

func reservedRecipient(to string) bool {
	s := strings.ToLower(strings.TrimSpace(to))
	for _, r := range reservedRecipientTokens {
		if s == strings.ToLower(r) {
			return true
		}
	}
	return false
}

// validateCronDelivery enforces exactly one delivery policy for CronSchedule tool input:
// none (no deliver flags), explicit (deliver + channel + to), or incoming_chat (deliver_to_incoming_chat + inbound ctx).
func validateCronDelivery(ctx context.Context, m map[string]interface{}) error {
	deliverIncoming := truthy(m["deliver_to_incoming_chat"])
	deliver := truthy(m["deliver"])
	ch := stringFrom(m, "channel")
	to := stringFrom(m, "to")
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
		if reservedRecipient(to) {
			return fmt.Errorf("cronschedule: invalid to=%q — use boolean deliver_to_incoming_chat for same-chat delivery, not a magic string in to", to)
		}
		return nil
	}
	if ch != "" || to != "" {
		return fmt.Errorf("cronschedule: omit channel and to unless deliver=true or deliver_to_incoming_chat=true (got channel=%q to=%q)", ch, to)
	}
	return nil
}
