package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BuildTemplate assembles the static system prompt from workspace markdown (AGENTS.md, SOUL.md).
// Memory context is injected separately at Apply time via memory.Registry.Context.
func BuildTemplate(workspace string) (string, error) {
	var sb strings.Builder
	agentsPath := filepath.Join(workspace, "AGENTS.md")
	if data, err := os.ReadFile(agentsPath); err == nil {
		sb.Write(data)
		sb.WriteString("\n\n")
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read %s: %w", agentsPath, err)
	}
	soulPath := filepath.Join(workspace, "SOUL.md")
	if data, err := os.ReadFile(soulPath); err == nil {
		sb.Write(data)
		sb.WriteString("\n\n")
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read %s: %w", soulPath, err)
	}
	return sb.String(), nil
}
