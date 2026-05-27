package slash

import (
	"context"
	"fmt"

	"github.com/ageneralai/maven/internal/kernel/plugin"
)

// RegisterPluginCommands merges plugin slash commands into r.
func RegisterPluginCommands(r *Registry, cmds []plugin.SlashCommand) error {
	for _, c := range cmds {
		h := pluginSlashHandler{c.Handler}
		if err := r.Register(Definition{Name: c.Definition.Name, Description: c.Definition.Description}, h); err != nil {
			return fmt.Errorf("slash.RegisterPluginCommands: %s: %w", c.Definition.Name, err)
		}
	}
	return nil
}

type pluginSlashHandler struct {
	h plugin.SlashHandler
}

func (a pluginSlashHandler) Handle(ctx context.Context, inv Invocation) (Result, error) {
	out, err := a.h.Handle(ctx, plugin.SlashInvocation{
		Name: inv.Name, Args: inv.Args, Flags: inv.Flags, Raw: inv.Raw, Position: inv.Position,
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Command: out.Command, Output: out.Output, Metadata: out.Metadata}, nil
}
