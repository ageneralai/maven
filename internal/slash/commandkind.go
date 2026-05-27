package slash

import "github.com/ageneralai/maven/internal/slashkind"

type CommandKind = slashkind.CommandKind

const (
	CommandKindLocal    = slashkind.CommandKindLocal
	CommandKindAgent    = slashkind.CommandKindAgent
	CommandKindPipeline = slashkind.CommandKindPipeline
)
