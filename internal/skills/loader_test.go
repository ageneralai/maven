package skills

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	runtimeskills "github.com/ageneralai/ageneral-agents-go/pkg/runtime/skills"

	mavenlog "github.com/ageneralai/maven/pkg/log"
)

var testLG = mavenlog.Std()

func TestLoadSkills_LoadSingleSkill(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	skillPath := filepath.Join(root, "writer", skillFileName)
	content := "---\nname: writer\ndescription: writing helper\nkeywords: [write, draft]\n---\n# Writer\nUse this skill for writing tasks.\n"
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write skill file: %v", err)
	}

	loaded, err := LoadSkills(root, testLG)
	if err != nil {
		t.Fatalf("load skills: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("skill count = %d, want 1", len(loaded))
	}

	reg := loaded[0]
	def := reg.Definition
	if def.Name != "writer" {
		t.Fatalf("name = %q, want writer", def.Name)
	}
	if def.Description != "writing helper" {
		t.Fatalf("description = %q, want writing helper", def.Description)
	}
	kw := skillKeywords(def)
	if len(kw) != 2 {
		t.Fatalf("keywords count = %d, want 2", len(kw))
	}
	if kw[0] != "draft" || kw[1] != "write" {
		t.Fatalf("keywords = %v, want [draft write]", kw)
	}
	wantBody := "# Writer\nUse this skill for writing tasks."
	out, meta := skillExec(t, reg)
	if out != wantBody {
		t.Fatalf("handler output = %q, want %q", out, wantBody)
	}
	if meta["source_path"] != skillPath {
		t.Fatalf("source path = %v, want %q", meta["source_path"], skillPath)
	}
}

func TestLoadSkills_DirNotFound(t *testing.T) {
	t.Parallel()

	notFoundDir := filepath.Join(t.TempDir(), "missing")
	loaded, err := LoadSkills(notFoundDir, testLG)
	if err != nil {
		t.Fatalf("load skills from missing dir: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("skill count = %d, want 0", len(loaded))
	}
}

func TestLoadSkills_MissingFrontmatter(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	skillPath := filepath.Join(root, "broken", skillFileName)
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte("# No frontmatter"), 0o600); err != nil {
		t.Fatalf("write skill file: %v", err)
	}

	_, err := LoadSkills(root, testLG)
	if err == nil {
		t.Fatalf("expected error for invalid frontmatter")
	}
}

func TestLoadSkills_DuplicateSkillName(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	firstPath := filepath.Join(root, "one", skillFileName)
	secondPath := filepath.Join(root, "two", skillFileName)
	firstContent := "---\nname: shared\ndescription: first\nkeywords: [a]\n---\nfirst body\n"
	secondContent := "---\nname: shared\ndescription: second\nkeywords: [b]\n---\nsecond body\n"

	if err := os.MkdirAll(filepath.Dir(firstPath), 0o755); err != nil {
		t.Fatalf("mkdir first skill dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(secondPath), 0o755); err != nil {
		t.Fatalf("mkdir second skill dir: %v", err)
	}
	if err := os.WriteFile(firstPath, []byte(firstContent), 0o600); err != nil {
		t.Fatalf("write first skill file: %v", err)
	}
	if err := os.WriteFile(secondPath, []byte(secondContent), 0o600); err != nil {
		t.Fatalf("write second skill file: %v", err)
	}

	_, err := LoadSkills(root, testLG)
	if err == nil {
		t.Fatalf("expected duplicate name error")
	}
}

func TestLoadSkills_MultipleSkills(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestSkillFile(t, root, "alpha", "---\nname: alpha\ndescription: alpha helper\nkeywords: [alpha]\n---\nalpha body\n")
	writeTestSkillFile(t, root, "beta", "---\nname: beta\ndescription: beta helper\nkeywords: [beta]\n---\nbeta body\n")
	writeTestSkillFile(t, root, "gamma", "---\nname: gamma\ndescription: gamma helper\nkeywords: [gamma]\n---\ngamma body\n")

	loaded, err := LoadSkills(root, testLG)
	if err != nil {
		t.Fatalf("load skills: %v", err)
	}
	if len(loaded) != 3 {
		t.Fatalf("skill count = %d, want 3", len(loaded))
	}

	wantNames := []string{"alpha", "beta", "gamma"}
	for i, wantName := range wantNames {
		if loaded[i].Definition.Name != wantName {
			t.Fatalf("skill[%d].name = %q, want %q", i, loaded[i].Definition.Name, wantName)
		}
	}
}

func TestLoadSkills_KeywordSanitize(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestSkillFile(t, root, "web-search", "---\nname: web-search\ndescription: Search the web\nkeywords:\n  - \" Search \"\n  - WEB\n  - web\n  - find online\n  - \"  \"\n---\n# Web Search\n")

	loaded, err := LoadSkills(root, testLG)
	if err != nil {
		t.Fatalf("load skills: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("skill count = %d, want 1", len(loaded))
	}

	wantKeywords := []string{"find online", "search", "web"}
	gotKW := skillKeywords(loaded[0].Definition)
	if len(gotKW) != len(wantKeywords) {
		t.Fatalf("keyword count = %d, want %d", len(gotKW), len(wantKeywords))
	}
	for i, wantKeyword := range wantKeywords {
		if gotKW[i] != wantKeyword {
			t.Fatalf("keyword[%d] = %q, want %q", i, gotKW[i], wantKeyword)
		}
	}
}

func TestLoadSkills_EmptyKeywords(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestSkillFile(t, root, "empty-keywords", "---\nname: empty-keywords\ndescription: no keywords\n---\n# Empty Keywords\nStill valid skill body.\n")

	loaded, err := LoadSkills(root, testLG)
	if err != nil {
		t.Fatalf("load skills: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("skill count = %d, want 1", len(loaded))
	}

	if loaded[0].Definition.Name != "empty-keywords" {
		t.Fatalf("name = %q, want empty-keywords", loaded[0].Definition.Name)
	}
	if len(skillKeywords(loaded[0].Definition)) != 0 {
		t.Fatalf("keywords count = %d, want 0", len(skillKeywords(loaded[0].Definition)))
	}
}

func TestLoadSkills_InvalidYAML(t *testing.T) {
	root := t.TempDir()
	invalidSkillPath := writeTestSkillFile(t, root, "broken", "---\nname: broken\ndescription: invalid yaml\nkeywords: [search, web\n---\n# Broken\n")
	writeTestSkillFile(t, root, "ok", "---\nname: ok\ndescription: valid\nkeywords: [ok]\n---\n# OK\n")

	var logBuf bytes.Buffer
	lg := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	loaded, err := LoadSkills(root, lg)
	if err != nil {
		t.Fatalf("load skills: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("skill count = %d, want 1", len(loaded))
	}
	if loaded[0].Definition.Name != "ok" {
		t.Fatalf("name = %q, want ok", loaded[0].Definition.Name)
	}

	output := logBuf.String()
	if !strings.Contains(output, "skills skipping invalid YAML skill") {
		t.Fatalf("expected warning log, got: %q", output)
	}
	if !strings.Contains(output, invalidSkillPath) {
		t.Fatalf("expected warning log to include invalid skill path %q, got: %q", invalidSkillPath, output)
	}
}

func skillKeywords(def runtimeskills.Definition) []string {
	for _, m := range def.Matchers {
		if km, ok := m.(runtimeskills.KeywordMatcher); ok {
			return append([]string(nil), km.Any...)
		}
	}
	return nil
}

func skillExec(t *testing.T, reg api.SkillRegistration) (string, map[string]any) {
	res, err := reg.Handler.Execute(context.Background(), runtimeskills.ActivationContext{})
	if err != nil {
		t.Fatalf("handler execute: %v", err)
	}
	out, _ := res.Output.(string)
	return out, res.Metadata
}

func writeTestSkillFile(t *testing.T, root, dirName, content string) string {
	t.Helper()

	skillPath := filepath.Join(root, dirName, skillFileName)
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write skill file: %v", err)
	}
	return skillPath
}
