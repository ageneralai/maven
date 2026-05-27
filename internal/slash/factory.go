package slash

import (
	"fmt"

	"github.com/ageneralai/maven/internal/cron"
)

// BuiltIns returns the default slash registry (compact + optional cron commands).
func BuiltIns(cronSvc *cron.Service) (*Registry, error) {
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
	for _, e := range cronHandlers(cronSvc) {
		if err := r.Register(e.def, e.h); err != nil {
			return nil, fmt.Errorf("slash.BuiltIns: %s: %w", e.def.Name, err)
		}
	}
	return r, nil
}
