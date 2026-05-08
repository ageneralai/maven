package slash

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
)

// PreTurn parses Input.Text and runs at most one registered slash command.
// One agent turn supports a single slash invocation; multiple lines with / each yield an error.
// If routing ExpectedSlashName is set and does not match the parsed name, PreTurn is a no-op (model runs).
func PreTurn(ctx context.Context, reg *Registry, in Input) (Outcome, error) {
	out := Outcome{ContinueToModel: true}
	if reg == nil {
		return out, nil
	}
	text := strings.TrimSpace(in.Text)
	if text == "" {
		return out, nil
	}
	invocations, err := Parse(text)
	if errors.Is(err, ErrNoCommand) {
		return out, nil
	}
	if err != nil {
		return Outcome{}, err
	}
	if len(invocations) == 0 {
		return out, nil
	}
	if len(invocations) > 1 {
		return Outcome{}, fmt.Errorf("slash: multiple commands in one message are not supported")
	}
	inv := invocations[0]
	if exp := strings.TrimSpace(in.ExpectedSlashName); exp != "" && !strings.EqualFold(exp, inv.Name) {
		return out, nil
	}
	h := reg.Lookup(inv.Name)
	if h == nil {
		return out, nil
	}
	res, err := h.Handle(ctx, inv)
	if err != nil {
		return Outcome{}, err
	}
	trail := []Execution{{Result: res}}
	if direct := directString(res); direct != "" {
		return Outcome{ContinueToModel: false, DirectReply: direct, Trail: trail}, nil
	}
	out.ContinueToModel = true
	out.Trail = trail
	if len(res.Metadata) > 0 {
		out.RequestMetadata = maps.Clone(res.Metadata)
	}
	return out, nil
}

func directString(res Result) string {
	if res.Output == nil {
		return ""
	}
	if s, ok := res.Output.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(res.Output))
}
