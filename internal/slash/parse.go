package slash

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

var (
	ErrNoCommand      = errors.New("slash: no slash command found")
	ErrInvalidCommand = errors.New("slash: invalid command")
)

// Parse extracts slash commands from input. Each line beginning with '/' is one command.
// Lexer matches former agentsdk-go/pkg/runtime/commands (quotes, --flag=value, escapes).
func Parse(input string) ([]Invocation, error) {
	lines := strings.Split(input, "\n")
	var invocations []Invocation
	for idx, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || !strings.HasPrefix(trimmed, "/") {
			continue
		}
		inv, err := parseLine(trimmed)
		if err != nil {
			return nil, fmt.Errorf("slash: line %d: %w", idx+1, err)
		}
		inv.Position = idx + 1
		inv.Raw = trimmed
		invocations = append(invocations, inv)
	}
	if len(invocations) == 0 {
		return nil, ErrNoCommand
	}
	return invocations, nil
}

func parseLine(line string) (Invocation, error) {
	tokens, err := lex(line)
	if err != nil {
		return Invocation{}, err
	}
	if len(tokens) == 0 {
		return Invocation{}, ErrInvalidCommand
	}
	name := tokens[0]
	if !strings.HasPrefix(name, "/") {
		return Invocation{}, ErrInvalidCommand
	}
	normalized := strings.ToLower(strings.TrimPrefix(name, "/"))
	if normalized == "" || !validName(normalized) {
		return Invocation{}, fmt.Errorf("slash: invalid name %q", name)
	}
	inv := Invocation{Name: normalized, Flags: map[string]string{}}
	for i := 1; i < len(tokens); i++ {
		token := tokens[i]
		if strings.HasPrefix(token, "--") {
			key, value, consumed := parseFlag(token)
			key = strings.ToLower(key)
			if key == "" {
				return Invocation{}, fmt.Errorf("slash: invalid flag %q", token)
			}
			if !consumed && i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "-") {
				value = tokens[i+1]
				i++
			}
			if value == "" {
				value = "true"
			}
			inv.Flags[key] = value
			continue
		}
		inv.Args = append(inv.Args, token)
	}
	if len(inv.Flags) == 0 {
		inv.Flags = nil
	}
	return inv, nil
}

func parseFlag(token string) (key, value string, hasValue bool) {
	trimmed := strings.TrimPrefix(token, "--")
	if key, value, ok := strings.Cut(trimmed, "="); ok {
		return strings.TrimSpace(key), value, true
	}
	return strings.TrimSpace(trimmed), "", false
}

func lex(line string) ([]string, error) {
	var tokens []string
	var buf strings.Builder
	var quote rune
	escaped := false
	emit := func() {
		if buf.Len() > 0 {
			tokens = append(tokens, buf.String())
			buf.Reset()
		}
	}
	for _, r := range line {
		switch {
		case escaped:
			buf.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
				continue
			}
			buf.WriteRune(r)
		case r == '\'' || r == '"':
			quote = r
		case unicode.IsSpace(r):
			emit()
		default:
			buf.WriteRune(r)
		}
	}
	if escaped {
		return nil, errors.New("slash: dangling escape")
	}
	if quote != 0 {
		return nil, errors.New("slash: unclosed quote")
	}
	emit()
	return tokens, nil
}

func validName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r != '-' && r != '_' && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}
