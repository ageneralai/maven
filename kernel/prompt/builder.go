package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Build assembles the system prompt from workspace markdown and optional memory context.
func Build(workspace string, memoryContext string) (string, error) {
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
	if memoryContext != "" {
		sb.WriteString(memoryContext)
	}
	return sb.String(), nil
}
