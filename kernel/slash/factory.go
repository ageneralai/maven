package slash

import "fmt"

// BuiltIns returns the default slash registry (compact).
func BuiltIns() (*Registry, error) {
	r := NewRegistry()
	if err := r.Register(
		Definition{
			Name:        "compact",
			Description: "Compress the current conversation into a fresh continuation context.",
		},
		HandlerFunc(handleCompact),
	); err != nil {
		return nil, fmt.Errorf("slash.BuiltIns: compact: %w", err)
	}
	return r, nil
}
