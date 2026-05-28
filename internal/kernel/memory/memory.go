package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type MemoryStore struct {
	workspace string
}

func NewMemoryStore(workspace string) *MemoryStore {
	return &MemoryStore{workspace: workspace}
}

func (m *MemoryStore) memoryDir() string {
	return filepath.Join(m.workspace, "memory")
}

func (m *MemoryStore) ensureDir() error {
	return os.MkdirAll(m.memoryDir(), 0755)
}

// Long-term memory

func (m *MemoryStore) ReadLongTerm() (string, error) {
	data, err := os.ReadFile(filepath.Join(m.memoryDir(), "MEMORY.md"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

func (m *MemoryStore) WriteLongTerm(content string) error {
	if err := m.ensureDir(); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(m.memoryDir(), "MEMORY.md"), []byte(content), 0644)
}

// Daily journal


// GetRecentMemories returns journal sections from dated .md files in memory/, newest first.
// limit is the maximum number of daily files to include; 0 means no limit.
func (m *MemoryStore) GetRecentMemories(limit int) (string, error) {
	dir := m.memoryDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	// Collect date-named files
	var dateFiles []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".md") && name != "MEMORY.md" {
			dateFiles = append(dateFiles, name)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dateFiles)))

	if limit > 0 && len(dateFiles) > limit {
		dateFiles = dateFiles[:limit]
	}

	var sb strings.Builder
	for _, name := range dateFiles {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		date := strings.TrimSuffix(name, ".md")
		fmt.Fprintf(&sb, "## %s\n%s\n\n", date, content)
	}
	return sb.String(), nil
}

// Context assembly for LLM system prompt

func (m *MemoryStore) GetMemoryContext() string {
	var sb strings.Builder

	longTerm, err := m.ReadLongTerm()
	if err == nil && strings.TrimSpace(longTerm) != "" {
		sb.WriteString("# Long-term Memory\n")
		sb.WriteString(longTerm)
		sb.WriteString("\n\n")
	}

	recent, err := m.GetRecentMemories(7)
	if err == nil && strings.TrimSpace(recent) != "" {
		sb.WriteString("# Recent Journal\n")
		sb.WriteString(recent)
		sb.WriteString("\n")
	}

	return sb.String()
}
