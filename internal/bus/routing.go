package bus

// SessionMode selects how the SDK session ID is derived for this inbound message.
type SessionMode string

const (
	SessionModeCurrent  SessionMode = ""
	SessionModeIsolated SessionMode = "isolated"
)

// RoutingHints are gateway-controlled flags for builtin commands, slash routing,
// streaming policy, and reaction correlation. Channels set these explicitly.
type RoutingHints struct {
	SessionMode    SessionMode
	ForceSync      bool
	BuiltinCommand string
	SlashCommand   string
	SlashType      string
	SlashArgs      string
	MessageID      int
}
