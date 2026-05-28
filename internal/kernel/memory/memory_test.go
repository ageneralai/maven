package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewMemoryStore(t *testing.T) {
	t.Parallel()
	ms := NewMemoryStore("/tmp/test-workspace")
	if ms == nil {
		t.Fatal("NewMemoryStore returned nil")
	}
	if ms.workspace != "/tmp/test-workspace" {
		t.Errorf("workspace = %q, want /tmp/test-workspace", ms.workspace)
	}
}

func TestLongTermMemory(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	ms := NewMemoryStore(tmpDir)

	// Read when no file exists
	content, err := ms.ReadLongTerm()
	if err != nil {
		t.Fatalf("ReadLongTerm error: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty, got %q", content)
	}

	// Write
	if err := ms.WriteLongTerm("test memory content"); err != nil {
		t.Fatalf("WriteLongTerm error: %v", err)
	}

	// Read back
	content, err = ms.ReadLongTerm()
	if err != nil {
		t.Fatalf("ReadLongTerm error: %v", err)
	}
	if content != "test memory content" {
		t.Errorf("content = %q, want 'test memory content'", content)
	}
}


func TestGetRecentMemories(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	ms := NewMemoryStore(tmpDir)

	// Create memory dir and some date files
	memDir := filepath.Join(tmpDir, "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatal(err)
	}

	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	if err := os.WriteFile(filepath.Join(memDir, today+".md"), []byte("today's notes"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memDir, yesterday+".md"), []byte("yesterday's notes"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := ms.GetRecentMemories(7)
	if err != nil {
		t.Fatalf("GetRecentMemories error: %v", err)
	}
	if !strings.Contains(result, "today's notes") {
		t.Error("missing today's notes")
	}
	if !strings.Contains(result, "yesterday's notes") {
		t.Error("missing yesterday's notes")
	}
}

func TestGetRecentMemories_Limit(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	ms := NewMemoryStore(tmpDir)

	memDir := filepath.Join(tmpDir, "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create 3 days of files
	for i := 0; i < 3; i++ {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		if err := os.WriteFile(filepath.Join(memDir, date+".md"), []byte("day "+date), 0644); err != nil {
			t.Fatal(err)
		}
	}

	result, err := ms.GetRecentMemories(1)
	if err != nil {
		t.Fatalf("GetRecentMemories error: %v", err)
	}
	// Should only have 1 day
	sections := strings.Count(result, "## ")
	if sections != 1 {
		t.Errorf("expected 1 section, got %d", sections)
	}
}

func TestGetMemoryContext(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	ms := NewMemoryStore(tmpDir)

	// Empty context when no memory exists
	ctx := ms.GetMemoryContext()
	if ctx != "" {
		t.Errorf("expected empty context, got %q", ctx)
	}

	// Write long-term memory
	if err := ms.WriteLongTerm("important fact"); err != nil {
		t.Fatal(err)
	}

	ctx = ms.GetMemoryContext()
	if !strings.Contains(ctx, "Long-term Memory") {
		t.Error("missing Long-term Memory header")
	}
	if !strings.Contains(ctx, "important fact") {
		t.Error("missing memory content")
	}
}

func TestGetMemoryContext_WithRecentMemories(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	ms := NewMemoryStore(tmpDir)

	// Write long-term memory
	if err := ms.WriteLongTerm("long-term fact"); err != nil {
		t.Fatal(err)
	}

	// Write today's journal
	memDir := filepath.Join(tmpDir, "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatal(err)
	}
	today := time.Now().Format("2006-01-02")
	if err := os.WriteFile(filepath.Join(memDir, today+".md"), []byte("today's entry\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := ms.GetMemoryContext()
	if !strings.Contains(ctx, "Long-term Memory") {
		t.Error("missing Long-term Memory header")
	}
	if !strings.Contains(ctx, "Recent Journal") {
		t.Error("missing Recent Journal header")
	}
	if !strings.Contains(ctx, "long-term fact") {
		t.Error("missing long-term content")
	}
	if !strings.Contains(ctx, "today's entry") {
		t.Error("missing today's entry")
	}
}

func TestGetRecentMemories_EmptyFiles(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	ms := NewMemoryStore(tmpDir)

	memDir := filepath.Join(tmpDir, "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create an empty date file
	today := time.Now().Format("2006-01-02")
	if err := os.WriteFile(filepath.Join(memDir, today+".md"), []byte("   \n\n  "), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := ms.GetRecentMemories(7)
	if err != nil {
		t.Fatalf("GetRecentMemories error: %v", err)
	}
	// Empty/whitespace-only files should be skipped
	if strings.Contains(result, "## ") {
		t.Error("empty file should not produce a section")
	}
}

func TestGetRecentMemories_NoLimit(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	ms := NewMemoryStore(tmpDir)

	memDir := filepath.Join(tmpDir, "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create 5 days of files
	for i := 0; i < 5; i++ {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		if err := os.WriteFile(filepath.Join(memDir, date+".md"), []byte("content "+date), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// limit=0 means no limit
	result, err := ms.GetRecentMemories(0)
	if err != nil {
		t.Fatalf("GetRecentMemories error: %v", err)
	}
	sections := strings.Count(result, "## ")
	if sections != 5 {
		t.Errorf("expected 5 sections, got %d", sections)
	}
}

func TestMemoryDir(t *testing.T) {
	t.Parallel()
	ms := NewMemoryStore("/test/workspace")
	expected := "/test/workspace/memory"
	if ms.memoryDir() != expected {
		t.Errorf("memoryDir = %q, want %q", ms.memoryDir(), expected)
	}
}

