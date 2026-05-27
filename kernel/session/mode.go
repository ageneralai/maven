package session

import (
	"fmt"
	"strings"
)

type SessionMode uint8

const (
	SessionModeCurrent SessionMode = iota
	SessionModeIsolated
)

func (m SessionMode) String() string {
	switch m {
	case SessionModeCurrent:
		return "current"
	case SessionModeIsolated:
		return "isolated"
	default:
		return fmt.Sprintf("SessionMode(%d)", m)
	}
}

func (m *SessionMode) UnmarshalText(text []byte) error {
	switch strings.ToLower(strings.TrimSpace(string(text))) {
	case "", "current":
		*m = SessionModeCurrent
	case "isolated":
		*m = SessionModeIsolated
	default:
		return fmt.Errorf("session: unsupported mode %q", text)
	}
	return nil
}
