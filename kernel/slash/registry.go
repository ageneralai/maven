package slash

import (
	"fmt"
	"strings"
)

// Registry maps normalized command names to handlers.
type Registry struct {
	handlers map[string]Handler
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]Handler)}
}

// Register adds one command. Name is normalized to lower case.
func (r *Registry) Register(def Definition, h Handler) error {
	if r == nil {
		return fmt.Errorf("slash: nil registry")
	}
	name := strings.ToLower(strings.TrimSpace(def.Name))
	if name == "" {
		return fmt.Errorf("slash: empty command name")
	}
	if !validName(name) {
		return fmt.Errorf("slash: invalid command name %q", def.Name)
	}
	if h == nil {
		return fmt.Errorf("slash: nil handler for %q", name)
	}
	if _, exists := r.handlers[name]; exists {
		return fmt.Errorf("slash: duplicate command %q", name)
	}
	r.handlers[name] = h
	return nil
}

// Lookup returns the handler for name, or nil.
func (r *Registry) Lookup(name string) Handler {
	if r == nil {
		return nil
	}
	return r.handlers[strings.ToLower(strings.TrimSpace(name))]
}
