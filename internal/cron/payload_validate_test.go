package cron_test

import (
	"strings"
	"testing"

	"github.com/ageneralai/maven/internal/cron"
)

func TestPayload_Validate(t *testing.T) {
	t.Run("silent_ok", func(t *testing.T) {
		p := cron.Payload{Message: "x", Deliver: false, Channel: "", To: ""}
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("silent_stray_channel_rejected", func(t *testing.T) {
		p := cron.Payload{Deliver: false, Channel: "telegram", To: ""}
		err := p.Validate()
		if err == nil || !strings.Contains(err.Error(), "deliver=false") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("deliver_ok", func(t *testing.T) {
		p := cron.Payload{Deliver: true, Channel: "telegram", To: "1"}
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("deliver_missing_to_rejected", func(t *testing.T) {
		p := cron.Payload{Deliver: true, Channel: "telegram", To: ""}
		err := p.Validate()
		if err == nil || !strings.Contains(err.Error(), "deliver=true requires") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("deliver_reserved_to_rejected", func(t *testing.T) {
		p := cron.Payload{Deliver: true, Channel: "telegram", To: "deliver_to_incoming_chat"}
		err := p.Validate()
		if err == nil || !strings.Contains(err.Error(), "invalid recipient") {
			t.Fatalf("got %v", err)
		}
	})
}

func TestIsReservedRecipient(t *testing.T) {
	if !cron.IsReservedRecipient("DELIVER_TO_INCOMING_CHAT") {
		t.Fatal("expected reserved")
	}
	if cron.IsReservedRecipient("12345") {
		t.Fatal("unexpected reserved")
	}
}
