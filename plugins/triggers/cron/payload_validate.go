package cron

import (
	"fmt"
	"strings"
)

var reservedRecipients = []string{
	"deliver_to_incoming_chat",
}

func IsReservedRecipient(to string) bool {
	s := strings.ToLower(strings.TrimSpace(to))
	for _, r := range reservedRecipients {
		if s == strings.ToLower(r) {
			return true
		}
	}
	return false
}

func (p Payload) Validate() error {
	ch := strings.TrimSpace(p.Channel)
	to := strings.TrimSpace(p.To)
	if !p.Deliver {
		if ch != "" || to != "" {
			return fmt.Errorf("cron: deliver=false requires empty channel and to")
		}
		return nil
	}
	if ch == "" || to == "" {
		return fmt.Errorf("cron: deliver=true requires non-empty channel and to")
	}
	if IsReservedRecipient(to) {
		return fmt.Errorf("cron: invalid recipient placeholder in to")
	}
	return nil
}
