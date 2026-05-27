package slash

import (
	"context"
	"strings"
)

// Definition names a /slash command for documentation and registration.
type Definition struct {
	Name        string
	Description string
}

// Result is a handler outcome. Non-empty trimmed Output skips the model; Metadata is merged into api.Request.
type Result struct {
	Command    string
	Output     string
	Metadata   map[string]any
	PostAction PostAction
}

// Invocation is one parsed /command from user text (aligned with former agentsdk-go/pkg/runtime/commands).
type Invocation struct {
	Name     string
	Args     []string
	Flags    map[string]string
	Raw      string
	Position int
}

// Flag returns a flag value; name is matched case-insensitively.
func (i Invocation) Flag(name string) (string, bool) {
	if i.Flags == nil {
		return "", false
	}
	v, ok := i.Flags[strings.ToLower(name)]
	return v, ok
}

// Handler runs when a registered slash command matches.
type Handler interface {
	Handle(ctx context.Context, inv Invocation) (Result, error)
}

// HandlerFunc adapts a function to Handler.
type HandlerFunc func(ctx context.Context, inv Invocation) (Result, error)

// Handle implements Handler.
func (f HandlerFunc) Handle(ctx context.Context, inv Invocation) (Result, error) {
	return f(ctx, inv)
}

// Execution records one handler result for post-model processing (e.g. compact).
type Execution struct {
	Result Result
}

// Input is inbound data for PreTurn; no dependency on transport packages.
type Input struct {
	Text string
	// ExpectedSlashName, if non-empty, must equal the parsed command name or PreTurn falls through to the model.
	ExpectedSlashName string
}

// Outcome is the single result of the pre-model slash phase.
type Outcome struct {
	ContinueToModel bool
	DirectReply     string
	RequestMetadata map[string]any
	Trail           []Execution
}
