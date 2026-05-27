package bus

import (
	"github.com/ageneralai/maven/internal/kernel/session"
)

// RoutingHints are gateway-controlled flags for builtin commands, slash routing,
// streaming policy, and reaction correlation. Channels set these explicitly.
type RoutingHints struct {
	SessionMode    session.SessionMode
	ForceSync      bool
	BuiltinCommand string
	SlashCommand   string
	SlashType      string
	SlashArgs      string
	MessageID      int
}
