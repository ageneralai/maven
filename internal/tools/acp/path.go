package acp

import (
	"fmt"
	"path/filepath"
	"strings"
)

// resolveWorkspacePath returns an absolute, cleaned path and ensures it stays inside workspace when restrict is true.
func resolveWorkspacePath(workspace string, restrict bool, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is empty")
	}
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	if !restrict {
		return abs, nil
	}
	ws, err := filepath.Abs(filepath.Clean(workspace))
	if err != nil {
		return "", fmt.Errorf("resolve workspace: %w", err)
	}
	wsResolved, err := filepath.EvalSymlinks(ws)
	if err != nil {
		wsResolved = ws
	}
	pathResolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		pathResolved = abs
	}
	rel, err := filepath.Rel(wsResolved, pathResolved)
	if err != nil {
		return "", fmt.Errorf("path outside workspace: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes workspace")
	}
	return abs, nil
}
