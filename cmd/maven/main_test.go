package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"log/slog"

	"github.com/ageneralai/maven/internal/agent"
	"github.com/ageneralai/maven/internal/config"
	"github.com/ageneralai/maven/pkg/memory"
	"github.com/ageneralai/maven/pkg/prompt"
	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	runtimeskills "github.com/ageneralai/ageneral-agents-go/pkg/runtime/skills"
	"github.com/spf13/cobra"
)

var testApp = cmdContext{log: slog.New(slog.DiscardHandler)}

func setupTestHome(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
}

func writeTestConfig(t *testing.T, mutate func(*config.Config)) {
	t.Helper()
	cfg := config.DefaultConfig()
	if mutate != nil {
		mutate(cfg)
	}
	if err := config.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
}

func TestWriteIfNotExists_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")

	writeIfNotExists(path, "test content")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(data) != "test content" {
		t.Errorf("content = %q, want 'test content'", string(data))
	}
}

func TestWriteIfNotExists_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")

	// Create existing file
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}

	writeIfNotExists(path, "new content")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	// Should not overwrite
	if string(data) != "original" {
		t.Errorf("content = %q, want 'original'", string(data))
	}
}

func captureRunOutput(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = w

	runErr := fn()

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, cerr := io.Copy(&buf, r); cerr != nil {
		t.Fatalf("io.Copy: %v", cerr)
	}
	return buf.String(), runErr
}

func writeSkillFile(t *testing.T, workspaceDir, skillName, description string) string {
	t.Helper()
	skillDir := filepath.Join(workspaceDir, "skills", skillName)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	content := fmt.Sprintf(`---
name: %s
description: %s
keywords: [write, draft]
---
# %s
Use this skill for writing tasks.
`, skillName, description, skillName)
	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(content), 0644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}
	return skillPath
}

func buildJSONCommand() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("json", false, "")
	_ = cmd.Flags().Set("json", "true")
	return cmd
}

func TestBuildSystemPrompt(t *testing.T) {
	tmpDir := t.TempDir()

	// Create workspace files
	if err := os.WriteFile(filepath.Join(tmpDir, "AGENTS.md"), []byte("# Agent\nYou help."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "SOUL.md"), []byte("# Soul\nBe nice."), 0644); err != nil {
		t.Fatal(err)
	}

	mem := memory.NewMemoryStore(tmpDir)

	prompt, err := prompt.Build(tmpDir, mem.GetMemoryContext())
	if err != nil {
		t.Fatalf("prompt.Build: %v", err)
	}

	if !strings.Contains(prompt, "# Agent") {
		t.Error("missing AGENTS.md content")
	}
	if !strings.Contains(prompt, "# Soul") {
		t.Error("missing SOUL.md content")
	}
}

func TestBuildSystemPrompt_WithMemory(t *testing.T) {
	tmpDir := t.TempDir()

	mem := memory.NewMemoryStore(tmpDir)
	if err := mem.WriteLongTerm("Important info"); err != nil {
		t.Fatal(err)
	}

	prompt, err := prompt.Build(tmpDir, mem.GetMemoryContext())
	if err != nil {
		t.Fatalf("prompt.Build: %v", err)
	}

	if !strings.Contains(prompt, "Important info") {
		t.Error("missing memory content")
	}
}

func TestBuildSystemPrompt_NoFiles(t *testing.T) {
	tmpDir := t.TempDir()

	mem := memory.NewMemoryStore(tmpDir)

	prompt, err := prompt.Build(tmpDir, mem.GetMemoryContext())
	if err != nil {
		t.Fatalf("prompt.Build: %v", err)
	}

	if prompt != "" {
		t.Errorf("expected empty prompt, got %q", prompt)
	}
}

func TestDefaultConstants(t *testing.T) {
	// Verify default constants are exported in embedded strings
	if !strings.Contains(defaultAgentsMD, "Maven") {
		t.Error("defaultAgentsMD should mention Maven")
	}
	if !strings.Contains(defaultSoulMD, "assistant") {
		t.Error("defaultSoulMD should mention assistant")
	}
}

func TestRunOnboard(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := testApp.runOnboard(&cobra.Command{}, []string{})

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil { t.Fatalf("io.Copy: %v", err) }
	output := buf.String()

	if err != nil {
		t.Errorf("testApp.runOnboard error: %v", err)
	}

	// Check config was created
	cfgPath := filepath.Join(tmpDir, ".maven", "config.json")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Error("config file was not created")
	}

	// Check workspace was created
	wsPath := filepath.Join(tmpDir, ".maven", "workspace")
	if _, err := os.Stat(wsPath); os.IsNotExist(err) {
		t.Error("workspace was not created")
	}
	skillsPath := filepath.Join(wsPath, "skills")
	if _, err := os.Stat(skillsPath); os.IsNotExist(err) {
		t.Error("skills directory was not created")
	}

	// Check output contains expected text
	if !strings.Contains(output, "Created config") && !strings.Contains(output, "Config already exists") {
		t.Errorf("unexpected output: %s", output)
	}
}

func TestRunOnboard_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create existing config
	cfgDir := filepath.Join(tmpDir, ".maven")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := testApp.runOnboard(&cobra.Command{}, []string{})

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil { t.Fatalf("io.Copy: %v", err) }
	output := buf.String()

	if err != nil {
		t.Errorf("testApp.runOnboard error: %v", err)
	}

	// Should say config already exists
	if !strings.Contains(output, "Config already exists") {
		t.Errorf("expected 'Config already exists', got: %s", output)
	}
}

func TestRunStatus(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := testApp.runStatus(&cobra.Command{}, []string{})

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil { t.Fatalf("io.Copy: %v", err) }
	output := buf.String()

	if err != nil {
		t.Errorf("testApp.runStatus error: %v", err)
	}

	if !strings.Contains(output, "Build:") {
		t.Errorf("missing Build in output: %s", output)
	}
	if !strings.Contains(output, "maven  dev") {
		t.Errorf("missing maven build version in output: %s", output)
	}
	// Should contain config info
	if !strings.Contains(output, "Config:") {
		t.Errorf("missing Config in output: %s", output)
	}
	if !strings.Contains(output, "API Key: not set") {
		t.Errorf("missing API Key info in output: %s", output)
	}
	if !strings.Contains(output, "Telegram: enabled=") {
		t.Errorf("missing Telegram status in output: %s", output)
	}
	if !strings.Contains(output, "Feishu: enabled=") {
		t.Errorf("missing Feishu status in output: %s", output)
	}
	if !strings.Contains(output, "WeCom: enabled=") {
		t.Errorf("missing WeCom status in output: %s", output)
	}
	if !strings.Contains(output, "Skills: enabled=") {
		t.Errorf("missing Skills status in output: %s", output)
	}
}

func TestRunStatus_WithAPIKey(t *testing.T) {
	setupTestHome(t)
	writeTestConfig(t, func(cfg *config.Config) {
		cfg.Provider.APIKey = "sk-ant-test-key-12345678"
	})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := testApp.runStatus(&cobra.Command{}, []string{})

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil { t.Fatalf("io.Copy: %v", err) }
	output := buf.String()

	if err != nil {
		t.Errorf("testApp.runStatus error: %v", err)
	}

	// Should show masked API key
	if !strings.Contains(output, "sk-a...") {
		t.Errorf("API key should be masked in output: %s", output)
	}
}

func TestRunStatus_WithShortAPIKey(t *testing.T) {
	setupTestHome(t)
	writeTestConfig(t, func(cfg *config.Config) {
		cfg.Provider.APIKey = "short"
	})

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := testApp.runStatus(&cobra.Command{}, []string{})

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil { t.Fatalf("io.Copy: %v", err) }
	output := buf.String()

	if err != nil {
		t.Errorf("testApp.runStatus error: %v", err)
	}

	// Should show "set" for short key
	if !strings.Contains(output, "API Key: set") {
		t.Errorf("short API key should show 'set': %s", output)
	}
}

func TestRunStatus_WithWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create workspace with memory
	wsDir := filepath.Join(tmpDir, ".maven", "workspace", "memory")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, "MEMORY.md"), []byte("test memory content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := testApp.runStatus(&cobra.Command{}, []string{})

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil { t.Fatalf("io.Copy: %v", err) }
	output := buf.String()

	if err != nil {
		t.Errorf("testApp.runStatus error: %v", err)
	}

	// Should show memory bytes
	if !strings.Contains(output, "Memory:") {
		t.Errorf("missing Memory in output: %s", output)
	}
}

func TestRunStatus_WorkspaceNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create config with non-existent workspace
	cfgDir := filepath.Join(tmpDir, ".maven")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(`{"agent":{"workspace":"/nonexistent"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := testApp.runStatus(&cobra.Command{}, []string{})

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil { t.Fatalf("io.Copy: %v", err) }
	output := buf.String()

	if err != nil {
		t.Errorf("testApp.runStatus error: %v", err)
	}

	// Should say workspace not found
	if !strings.Contains(output, "not found") {
		t.Errorf("expected 'not found' in output: %s", output)
	}
}

func TestRunSkillsList(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	if err := testApp.runOnboard(&cobra.Command{}, []string{}); err != nil {
		t.Fatalf("testApp.runOnboard error: %v", err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	writeSkillFile(t, cfg.Agent.Workspace, "writer", "writing helper")

	output, runErr := captureRunOutput(t, func() error {
		return testApp.runSkillsList(&cobra.Command{}, []string{})
	})
	if runErr != nil {
		t.Fatalf("testApp.runSkillsList error: %v", runErr)
	}
	if !strings.Contains(output, "Loaded skills: 1") {
		t.Errorf("expected loaded skills count in output: %s", output)
	}
	if !strings.Contains(output, "- writer: writing helper") {
		t.Errorf("expected writer skill in output: %s", output)
	}
}

func TestRunSkillsList_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	if err := testApp.runOnboard(&cobra.Command{}, []string{}); err != nil {
		t.Fatalf("testApp.runOnboard error: %v", err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	writeSkillFile(t, cfg.Agent.Workspace, "writer", "writing helper")

	output, runErr := captureRunOutput(t, func() error {
		return testApp.runSkillsList(buildJSONCommand(), []string{})
	})
	if runErr != nil {
		t.Fatalf("testApp.runSkillsList json error: %v", runErr)
	}

	var payload struct {
		SchemaVersion int    `json:"schemaVersion"`
		Command       string `json:"command"`
		OK            bool   `json:"ok"`
		Enabled       bool   `json:"enabled"`
		Loaded        int    `json:"loaded"`
		Skills        []struct {
			Name string `json:"name"`
		} `json:"skills"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("unmarshal json: %v; output=%s", err, output)
	}
	if payload.SchemaVersion != skillsJSONSchemaVersion {
		t.Errorf("expected schemaVersion=%d, got %d", skillsJSONSchemaVersion, payload.SchemaVersion)
	}
	if payload.Command != "skills.list" {
		t.Errorf("expected command skills.list, got %s", payload.Command)
	}
	if !payload.OK {
		t.Errorf("expected ok=true, got false")
	}
	if !payload.Enabled {
		t.Errorf("expected enabled=true, got false")
	}
	if payload.Loaded != 1 {
		t.Errorf("expected loaded=1, got %d", payload.Loaded)
	}
	if len(payload.Skills) != 1 || payload.Skills[0].Name != "writer" {
		t.Errorf("unexpected skills payload: %+v", payload.Skills)
	}
}

func TestRunSkillsInfo(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	if err := testApp.runOnboard(&cobra.Command{}, []string{}); err != nil {
		t.Fatalf("testApp.runOnboard error: %v", err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	skillPath := writeSkillFile(t, cfg.Agent.Workspace, "writer", "writing helper")

	output, runErr := captureRunOutput(t, func() error {
		return testApp.runSkillsInfo(&cobra.Command{}, []string{"writer"})
	})
	if runErr != nil {
		t.Fatalf("testApp.runSkillsInfo error: %v", runErr)
	}
	if !strings.Contains(output, "Name: writer") {
		t.Errorf("expected name in output: %s", output)
	}
	if !strings.Contains(output, "Description: writing helper") {
		t.Errorf("expected description in output: %s", output)
	}
	if !strings.Contains(output, "Source: "+skillPath) {
		t.Errorf("expected source path in output: %s", output)
	}
}

func TestRunSkillsInfo_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	if err := testApp.runOnboard(&cobra.Command{}, []string{}); err != nil {
		t.Fatalf("testApp.runOnboard error: %v", err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	skillPath := writeSkillFile(t, cfg.Agent.Workspace, "writer", "writing helper")

	output, runErr := captureRunOutput(t, func() error {
		return testApp.runSkillsInfo(buildJSONCommand(), []string{"writer"})
	})
	if runErr != nil {
		t.Fatalf("testApp.runSkillsInfo json error: %v", runErr)
	}

	var payload struct {
		SchemaVersion int      `json:"schemaVersion"`
		Command       string   `json:"command"`
		OK            bool     `json:"ok"`
		Name          string   `json:"name"`
		Description   string   `json:"description"`
		Source        string   `json:"source"`
		Keywords      []string `json:"keywords"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("unmarshal json: %v; output=%s", err, output)
	}
	if payload.SchemaVersion != skillsJSONSchemaVersion {
		t.Errorf("expected schemaVersion=%d, got %d", skillsJSONSchemaVersion, payload.SchemaVersion)
	}
	if payload.Command != "skills.info" {
		t.Errorf("expected command skills.info, got %s", payload.Command)
	}
	if !payload.OK {
		t.Errorf("expected ok=true, got false")
	}
	if payload.Name != "writer" {
		t.Errorf("expected name writer, got %s", payload.Name)
	}
	if payload.Description != "writing helper" {
		t.Errorf("expected description writing helper, got %s", payload.Description)
	}
	if payload.Source != skillPath {
		t.Errorf("expected source %s, got %s", skillPath, payload.Source)
	}
	if len(payload.Keywords) == 0 {
		t.Errorf("expected keywords in payload")
	}
}

func TestRunSkillsCheck(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	if err := testApp.runOnboard(&cobra.Command{}, []string{}); err != nil {
		t.Fatalf("testApp.runOnboard error: %v", err)
	}

	output, runErr := captureRunOutput(t, func() error {
		return testApp.runSkillsCheck(&cobra.Command{}, []string{})
	})
	if runErr != nil {
		t.Fatalf("testApp.runSkillsCheck error: %v", runErr)
	}
	if !strings.Contains(output, "Skill folders: 0") {
		t.Errorf("expected folder count in output: %s", output)
	}
	if !strings.Contains(output, "Loaded skills: 0") {
		t.Errorf("expected loaded count in output: %s", output)
	}
	if !strings.Contains(output, "Result: ok") {
		t.Errorf("expected ok result in output: %s", output)
	}
}

func TestRunSkillsCheck_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	if err := testApp.runOnboard(&cobra.Command{}, []string{}); err != nil {
		t.Fatalf("testApp.runOnboard error: %v", err)
	}

	output, runErr := captureRunOutput(t, func() error {
		return testApp.runSkillsCheck(buildJSONCommand(), []string{})
	})
	if runErr != nil {
		t.Fatalf("testApp.runSkillsCheck json error: %v", runErr)
	}

	var payload struct {
		SchemaVersion int    `json:"schemaVersion"`
		Command       string `json:"command"`
		OK            bool   `json:"ok"`
		Result        string `json:"result"`
		SkillFolder   int    `json:"skillFolders"`
		Loaded        int    `json:"loaded"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("unmarshal json: %v; output=%s", err, output)
	}
	if payload.SchemaVersion != skillsJSONSchemaVersion {
		t.Errorf("expected schemaVersion=%d, got %d", skillsJSONSchemaVersion, payload.SchemaVersion)
	}
	if payload.Command != "skills.check" {
		t.Errorf("expected command skills.check, got %s", payload.Command)
	}
	if !payload.OK {
		t.Errorf("expected ok=true, got false")
	}
	if payload.Result != "ok" {
		t.Errorf("expected result ok, got %s", payload.Result)
	}
	if payload.SkillFolder != 0 {
		t.Errorf("expected skillFolders=0, got %d", payload.SkillFolder)
	}
	if payload.Loaded != 0 {
		t.Errorf("expected loaded=0, got %d", payload.Loaded)
	}
}

func TestRunSkillsCheck_MissingSkillFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	if err := testApp.runOnboard(&cobra.Command{}, []string{}); err != nil {
		t.Fatalf("testApp.runOnboard error: %v", err)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	brokenDir := filepath.Join(cfg.Agent.Workspace, "skills", "broken")
	if err := os.MkdirAll(brokenDir, 0755); err != nil {
		t.Fatalf("mkdir broken skill dir: %v", err)
	}

	output, runErr := captureRunOutput(t, func() error {
		return testApp.runSkillsCheck(&cobra.Command{}, []string{})
	})
	if runErr != nil {
		t.Fatalf("testApp.runSkillsCheck error: %v", runErr)
	}
	if !strings.Contains(output, "Missing SKILL.md: broken") {
		t.Errorf("expected missing SKILL.md warning, got: %s", output)
	}
	if !strings.Contains(output, "Loaded skills: 0") {
		t.Errorf("expected loaded skills 0, got: %s", output)
	}
}

func TestLoadRuntimeSkills_Disabled(t *testing.T) {
	cfg := &config.Config{
		Agent: config.AgentConfig{
			Workspace: t.TempDir(),
		},
		Skills: config.SkillsConfig{
			Enabled: false,
		},
	}

	got := testApp.loadRuntimeSkills(cfg)
	if len(got) != 0 {
		t.Fatalf("expected no skills when disabled, got %d", len(got))
	}
}

func TestLoadRuntimeSkills_LoadsAndWorks(t *testing.T) {
	workspaceDir := t.TempDir()
	writeSkillFile(t, workspaceDir, "writer", "writing helper")

	cfg := &config.Config{
		Agent: config.AgentConfig{
			Workspace: workspaceDir,
		},
		Skills: config.SkillsConfig{
			Enabled: true,
		},
	}

	got := testApp.loadRuntimeSkills(cfg)
	if len(got) != 1 {
		t.Fatalf("expected one loaded skill, got %d", len(got))
	}
	if got[0].Definition.Name != "writer" {
		t.Fatalf("expected loaded skill writer, got %s", got[0].Definition.Name)
	}
	result, err := got[0].Handler.Execute(context.Background(), runtimeskills.ActivationContext{
		Prompt: "please draft",
	})
	if err != nil {
		t.Fatalf("execute loaded skill handler: %v", err)
	}
	output, ok := result.Output.(string)
	if !ok {
		t.Fatalf("expected string output, got %T", result.Output)
	}
	if !strings.Contains(output, "Use this skill for writing tasks.") {
		t.Fatalf("unexpected skill output: %s", output)
	}
}

func TestLoadRuntimeSkills_InvalidDirReturnsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	notDirPath := filepath.Join(tmpDir, "not-dir")
	if err := os.WriteFile(notDirPath, []byte("x"), 0644); err != nil {
		t.Fatalf("write not-dir file: %v", err)
	}

	cfg := &config.Config{
		Agent: config.AgentConfig{
			Workspace: tmpDir,
		},
		Skills: config.SkillsConfig{
			Enabled: true,
			Dir:     notDirPath,
		},
	}

	got := testApp.loadRuntimeSkills(cfg)
	if len(got) != 0 {
		t.Fatalf("expected no skills on invalid dir, got %d", len(got))
	}
}

func TestInit(t *testing.T) {
	// Verify init() sets up commands correctly
	if rootCmd == nil {
		t.Error("rootCmd should not be nil")
	}
	if agentCmd == nil {
		t.Error("agentCmd should not be nil")
	}
	if gatewayCmd == nil {
		t.Error("gatewayCmd should not be nil")
	}
	if onboardCmd == nil {
		t.Error("onboardCmd should not be nil")
	}
	if statusCmd == nil {
		t.Error("statusCmd should not be nil")
	}
	if skillsCmd == nil {
		t.Error("skillsCmd should not be nil")
	}
	if skillsListCmd == nil {
		t.Error("skillsListCmd should not be nil")
	}
	if skillsInfoCmd == nil {
		t.Error("skillsInfoCmd should not be nil")
	}
	if skillsCheckCmd == nil {
		t.Error("skillsCheckCmd should not be nil")
	}

	// Check message flag exists
	flag := agentCmd.Flags().Lookup("message")
	if flag == nil {
		t.Error("message flag should exist")
	}
}

func TestRunAgent_NoAPIKey(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Clear API key env vars
	err := testApp.runAgent(&cobra.Command{}, []string{})
	if err == nil {
		t.Fatal("expected error when API key is not set")
	}
	low := strings.ToLower(err.Error())
	if !strings.Contains(low, "apikey") && !strings.Contains(low, "api key") {
		t.Errorf("error should mention API key: %v", err)
	}
}

func TestRunGateway_NoAPIKey(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Clear API key env vars
	err := testApp.runGateway(&cobra.Command{}, []string{})
	if err == nil {
		t.Fatal("expected error when API key is not set")
	}
	low := strings.ToLower(err.Error())
	if !strings.Contains(low, "apikey") && !strings.Contains(low, "api key") {
		t.Errorf("error should mention API key: %v", err)
	}
}

func TestRunStatus_EmptyMemory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create workspace with empty memory
	wsDir := filepath.Join(tmpDir, ".maven", "workspace", "memory")
	if err := os.MkdirAll(wsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, "MEMORY.md"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := testApp.runStatus(&cobra.Command{}, []string{})

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil { t.Fatalf("io.Copy: %v", err) }
	output := buf.String()

	if err != nil {
		t.Errorf("testApp.runStatus error: %v", err)
	}

	// Should show "Memory: empty" for empty file
	if !strings.Contains(output, "Memory: empty") {
		t.Errorf("expected 'Memory: empty', got: %s", output)
	}
}

// mockRuntime implements agent.Runtime for testing.
type mockRuntime struct {
	response *api.Response
	err      error
	closed   bool
}

func (m *mockRuntime) Run(ctx context.Context, req api.Request) (*api.Response, error) {
	return m.response, m.err
}

func (m *mockRuntime) RunStream(ctx context.Context, req api.Request) (<-chan api.StreamEvent, error) {
	ch := make(chan api.StreamEvent)
	close(ch)
	return ch, nil
}

func (m *mockRuntime) Close() {
	m.closed = true
}

func mockRuntimeFactory(rt agent.Runtime) func(cfg *config.Config) (agent.Runtime, error) {
	return func(cfg *config.Config) (agent.Runtime, error) {
		return rt, nil
	}
}

func TestRunAgentWithOptions_SingleMessage(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Clear API key env vars
	mockRt := &mockRuntime{
		response: &api.Response{
			Result: &api.Result{Output: "Hello from mock!"},
		},
	}

	var stdout bytes.Buffer

	// Set messageFlag for single message mode
	oldFlag := messageFlag
	messageFlag = "test message"
	defer func() { messageFlag = oldFlag }()

	err := testApp.runAgentWithOptions(AgentOptions{
		RuntimeFactory: mockRuntimeFactory(mockRt),
		Stdout:         &stdout,
	})

	if err != nil {
		t.Errorf("runAgentWithOptions error: %v", err)
	}

	if !strings.Contains(stdout.String(), "Hello from mock!") {
		t.Errorf("expected 'Hello from mock!' in output, got: %s", stdout.String())
	}

	if !mockRt.closed {
		t.Error("runtime should be closed")
	}
}

func TestRunAgentWithOptions_REPLMode(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Clear API key env vars
	mockRt := &mockRuntime{
		response: &api.Response{
			Result: &api.Result{Output: "REPL response"},
		},
	}

	// Simulate REPL input: one message then exit
	stdin := strings.NewReader("hello\nexit\n")
	var stdout, stderr bytes.Buffer

	// Clear messageFlag for REPL mode
	oldFlag := messageFlag
	messageFlag = ""
	defer func() { messageFlag = oldFlag }()

	err := testApp.runAgentWithOptions(AgentOptions{
		RuntimeFactory: mockRuntimeFactory(mockRt),
		Stdin:          stdin,
		Stdout:         &stdout,
		Stderr:         &stderr,
	})

	if err != nil {
		t.Errorf("runAgentWithOptions error: %v", err)
	}

	if !strings.Contains(stdout.String(), "maven agent") {
		t.Errorf("expected REPL welcome message, got: %s", stdout.String())
	}

	if !strings.Contains(stdout.String(), "REPL response") {
		t.Errorf("expected 'REPL response' in output, got: %s", stdout.String())
	}
}

func TestRunAgentWithOptions_REPLMode_EmptyInput(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	mockRt := &mockRuntime{
		response: &api.Response{
			Result: &api.Result{Output: "response"},
		},
	}

	// Empty lines should be skipped
	stdin := strings.NewReader("\n\nhello\nquit\n")
	var stdout bytes.Buffer

	oldFlag := messageFlag
	messageFlag = ""
	defer func() { messageFlag = oldFlag }()

	err := testApp.runAgentWithOptions(AgentOptions{
		RuntimeFactory: mockRuntimeFactory(mockRt),
		Stdin:          stdin,
		Stdout:         &stdout,
	})

	if err != nil {
		t.Errorf("error: %v", err)
	}
}

func TestRunAgentWithOptions_REPLMode_Error(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	mockRt := &mockRuntime{
		err: context.DeadlineExceeded,
	}

	stdin := strings.NewReader("hello\nexit\n")
	var stdout, stderr bytes.Buffer

	oldFlag := messageFlag
	messageFlag = ""
	defer func() { messageFlag = oldFlag }()

	err := testApp.runAgentWithOptions(AgentOptions{
		RuntimeFactory: mockRuntimeFactory(mockRt),
		Stdin:          stdin,
		Stdout:         &stdout,
		Stderr:         &stderr,
	})

	if err != nil {
		t.Errorf("error: %v", err)
	}

	// Error should be written to stderr
	if !strings.Contains(stderr.String(), "Error:") {
		t.Errorf("expected error in stderr, got: %s", stderr.String())
	}
}

func TestRunAgentWithOptions_SingleMessage_Error(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	mockRt := &mockRuntime{
		err: context.DeadlineExceeded,
	}

	oldFlag := messageFlag
	messageFlag = "test"
	defer func() { messageFlag = oldFlag }()

	err := testApp.runAgentWithOptions(AgentOptions{
		RuntimeFactory: mockRuntimeFactory(mockRt),
	})

	if err == nil {
		t.Error("expected error")
	}
	if !strings.Contains(err.Error(), "agent error") {
		t.Errorf("expected 'agent error', got: %v", err)
	}
}

func TestRunAgentWithOptions_NilResult(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	mockRt := &mockRuntime{
		response: &api.Response{Result: nil},
	}

	var stdout bytes.Buffer

	oldFlag := messageFlag
	messageFlag = "test"
	defer func() { messageFlag = oldFlag }()

	err := testApp.runAgentWithOptions(AgentOptions{
		RuntimeFactory: mockRuntimeFactory(mockRt),
		Stdout:         &stdout,
	})

	if err != nil {
		t.Errorf("error: %v", err)
	}
}

func TestDefaultRuntimeFactory_NoAPIKey(t *testing.T) {
	cfg := &config.Config{
		Provider: config.ProviderConfig{
			APIKey: "",
		},
	}

	_, err := testApp.defaultAgentRuntime(cfg)
	if err == nil {
		t.Fatal("expected error when API key is not set")
	}
	low := strings.ToLower(err.Error())
	if !strings.Contains(low, "apikey") && !strings.Contains(low, "api key") {
		t.Errorf("error should mention API key: %v", err)
	}
}
