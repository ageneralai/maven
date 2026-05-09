package prompt

import (
	"os"
	"path/filepath"
	"strings"
)

// Build assembles the system prompt from workspace markdown and optional memory context.
func Build(workspace string, memoryContext string) string {
	var sb strings.Builder
	if data, err := os.ReadFile(filepath.Join(workspace, "AGENTS.md")); err == nil {
		sb.Write(data)
		sb.WriteString("\n\n")
	}
	if data, err := os.ReadFile(filepath.Join(workspace, "SOUL.md")); err == nil {
		sb.Write(data)
		sb.WriteString("\n\n")
	}
	if memoryContext != "" {
		sb.WriteString(memoryContext)
	}
	return sb.String()
}
