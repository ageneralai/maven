package slash

import (
	"fmt"
	"sort"
	"strings"
)

type registryEntry struct {
	def Definition
	h   Handler
}

// Registry maps normalized command names to handlers.
type Registry struct {
	entries map[string]registryEntry
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{entries: make(map[string]registryEntry)}
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
	if _, exists := r.entries[name]; exists {
		return fmt.Errorf("slash: duplicate command %q", name)
	}
	r.entries[name] = registryEntry{
		def: Definition{Name: name, Description: def.Description},
		h:   h,
	}
	return nil
}

// Definitions returns registered command metadata sorted by name.
func (r *Registry) Definitions() []Definition {
	if r == nil || len(r.entries) == 0 {
		return nil
	}
	names := make([]string, 0, len(r.entries))
	for name := range r.entries {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]Definition, 0, len(names))
	for _, name := range names {
		out = append(out, r.entries[name].def)
	}
	return out
}

// Lookup returns the handler for name, or nil.
func (r *Registry) Lookup(name string) Handler {
	if r == nil {
		return nil
	}
	entry, ok := r.entries[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return nil
	}
	return entry.h
}
