package slashkind

import (
	"fmt"
	"strings"
)

type CommandKind uint8

const (
	CommandKindLocal CommandKind = iota + 1
	CommandKindAgent
	CommandKindPipeline
)

func (k CommandKind) String() string {
	switch k {
	case CommandKindLocal:
		return "local"
	case CommandKindAgent:
		return "agent"
	case CommandKindPipeline:
		return "pipeline"
	default:
		return fmt.Sprintf("CommandKind(%d)", k)
	}
}

func (k *CommandKind) UnmarshalText(text []byte) error {
	switch strings.ToLower(strings.TrimSpace(string(text))) {
	case "local":
		*k = CommandKindLocal
	case "agent", "":
		*k = CommandKindAgent
	case "pipeline":
		*k = CommandKindPipeline
	default:
		return fmt.Errorf("slash: unsupported command kind %q", text)
	}
	return nil
}
