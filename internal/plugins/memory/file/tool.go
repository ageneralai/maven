package memory

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/plugin"
)

type rememberTool struct {
	plug *Plugin
	cfg  *config.Config
}

type memorySearchTool struct {
	cfg *config.Config
}

type memoryGetTool struct {
	cfg *config.Config
}

var rememberSchema = &tool.JSONSchema{
	Type: "object",
	Properties: map[string]any{
		"content": map[string]any{"type": "string", "description": "The information to store."},
		"kind": map[string]any{
			"type":        "string",
			"enum":        []string{"fact", "event", "preference"},
			"description": `"fact" replaces MEMORY.md (long-term), "event" appends to today's journal, "preference" appends a preference note.`,
		},
	},
	Required: []string{"content", "kind"},
}

var memorySearchSchema = &tool.JSONSchema{
	Type: "object",
	Properties: map[string]any{
		"query": map[string]any{"type": "string", "description": "Keyword or topic to search for."},
		"limit": map[string]any{"type": "integer", "description": "Maximum number of matching entries to return (default 5)."},
	},
	Required: []string{"query"},
}

var memoryGetSchema = &tool.JSONSchema{
	Type: "object",
	Properties: map[string]any{
		"date": map[string]any{"type": "string", "description": `Journal date: "today", "yesterday", or "YYYY-MM-DD".`},
	},
	Required: []string{"date"},
}

func (p *Plugin) Tools(cfg *config.Config) []tool.Tool {
	return []tool.Tool{
		&rememberTool{plug: p, cfg: cfg},
		&memorySearchTool{cfg: cfg},
		&memoryGetTool{cfg: cfg},
	}
}

func (t *rememberTool) Name() string { return "remember" }

func (t *rememberTool) Description() string {
	return "Store information to long-term memory. Use kind='fact' for persistent facts about the user or context, 'event' for things that happened today, 'preference' for user preferences."
}

func (t *rememberTool) Schema() *tool.JSONSchema { return rememberSchema }

func (t *rememberTool) Execute(ctx context.Context, params map[string]any) (*tool.ToolResult, error) {
	content := strings.TrimSpace(stringParam(params, "content"))
	kindStr := strings.TrimSpace(stringParam(params, "kind"))
	if content == "" {
		return nil, fmt.Errorf("remember: content is required")
	}
	kind := plugin.MemoryKind(kindStr)
	if err := t.plug.Write(ctx, t.cfg, plugin.MemoryEntry{
		Content:   content,
		Kind:      kind,
		Timestamp: time.Now(),
		Source:    "agent",
	}); err != nil {
		return &tool.ToolResult{Success: false, Output: err.Error()}, err
	}
	if t.plug.refreshFn != nil && kind != plugin.MemoryKindEvent {
		if err := t.plug.refreshFn(ctx); err != nil {
			_ = err
		}
	}
	return &tool.ToolResult{Success: true, Output: fmt.Sprintf("Stored (%s).", kind)}, nil
}

func (t *memorySearchTool) Name() string { return "memory_search" }

func (t *memorySearchTool) Description() string {
	return "Search long-term memory journal files for a keyword or topic. Returns matching daily entries with their dates. Use before answering questions about past conversations, decisions, or events."
}

func (t *memorySearchTool) Schema() *tool.JSONSchema { return memorySearchSchema }

func (t *memorySearchTool) Execute(_ context.Context, params map[string]any) (*tool.ToolResult, error) {
	query := strings.TrimSpace(stringParam(params, "query"))
	if query == "" {
		return nil, fmt.Errorf("memory_search: query is required")
	}
	limit := intParam(params, "limit", 5)
	if limit <= 0 {
		limit = 5
	}
	dir := memoryDir(t.cfg)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &tool.ToolResult{Success: true, Output: "No memory journal files found."}, nil
		}
		return &tool.ToolResult{Success: false, Output: err.Error()}, err
	}
	type match struct {
		date    string
		snippet string
	}
	var matches []match
	queryLower := strings.ToLower(query)
	for _, e := range entries {
		name := e.Name()
		if !isDailyJournalFile(name) {
			continue
		}
		date := dailyJournalDate(name)
		if date == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		content := string(data)
		if !strings.Contains(strings.ToLower(content), queryLower) {
			continue
		}
		matches = append(matches, match{date: date, snippet: truncateSnippet(content, 500)})
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].date > matches[j].date
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	if len(matches) == 0 {
		return &tool.ToolResult{Success: true, Output: fmt.Sprintf("No journal entries matching %q.", query)}, nil
	}
	var sb strings.Builder
	for _, m := range matches {
		fmt.Fprintf(&sb, "## %s\n%s\n\n", m.date, m.snippet)
	}
	return &tool.ToolResult{Success: true, Output: strings.TrimSpace(sb.String())}, nil
}

func (t *memoryGetTool) Name() string { return "memory_get" }

func (t *memoryGetTool) Description() string {
	return "Read a specific memory journal entry. Use date='today', date='yesterday', or a date like '2026-05-27'."
}

func (t *memoryGetTool) Schema() *tool.JSONSchema { return memoryGetSchema }

func (t *memoryGetTool) Execute(_ context.Context, params map[string]any) (*tool.ToolResult, error) {
	dateRaw := strings.TrimSpace(stringParam(params, "date"))
	if dateRaw == "" {
		return nil, fmt.Errorf("memory_get: date is required")
	}
	date, err := resolveJournalDate(dateRaw)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(memoryDir(t.cfg), date+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &tool.ToolResult{Success: true, Output: fmt.Sprintf("No journal entry for %s.", date)}, nil
		}
		return &tool.ToolResult{Success: false, Output: err.Error()}, err
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return &tool.ToolResult{Success: true, Output: fmt.Sprintf("No journal entry for %s.", date)}, nil
	}
	return &tool.ToolResult{Success: true, Output: fmt.Sprintf("## %s\n%s", date, content)}, nil
}

func stringParam(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return strings.TrimSpace(fmt.Sprint(v))
	}
	return strings.TrimSpace(s)
}

func intParam(m map[string]any, key string, defaultVal int) int {
	v, ok := m[key]
	if !ok || v == nil {
		return defaultVal
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return defaultVal
	}
}

func isDailyJournalFile(name string) bool {
	if !strings.HasSuffix(name, ".md") || name == "MEMORY.md" {
		return false
	}
	base := strings.TrimSuffix(name, ".md")
	if len(base) < 10 {
		return false
	}
	_, err := time.Parse("2006-01-02", base[:10])
	return err == nil
}

func dailyJournalDate(name string) string {
	base := strings.TrimSuffix(name, ".md")
	if len(base) < 10 {
		return ""
	}
	date := base[:10]
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return ""
	}
	return date
}

func resolveJournalDate(raw string) (string, error) {
	switch strings.ToLower(raw) {
	case "today":
		return time.Now().Format("2006-01-02"), nil
	case "yesterday":
		return time.Now().AddDate(0, 0, -1).Format("2006-01-02"), nil
	default:
		if _, err := time.Parse("2006-01-02", raw); err != nil {
			return "", fmt.Errorf("memory_get: invalid date %q (use today, yesterday, or YYYY-MM-DD)", raw)
		}
		return raw, nil
	}
}

func truncateSnippet(s string, maxChars int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= maxChars {
		return s
	}
	return string([]rune(s)[:maxChars]) + "…"
}
