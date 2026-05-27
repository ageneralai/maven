package task

import (
	"context"
	"strings"
	"testing"

	"github.com/ageneralai/ageneral-agents-go/pkg/middleware"
	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/config"
)

func TestTools_DisabledReturnsNil(t *testing.T) {
	if got := Tools(config.TaskToolConfig{}, &RuntimeHolder{}); got != nil {
		t.Fatalf("Tools(enabled=false) = %v, want nil", got)
	}
}

func TestNew_NilHolderReturnsNil(t *testing.T) {
	if got := New(nil); got != nil {
		t.Fatalf("New(nil) = %v, want nil", got)
	}
}

func TestSchema_RequiredFields(t *testing.T) {
	tt := New(&RuntimeHolder{})
	s := tt.Schema()
	if len(s.Required) != 2 {
		t.Fatalf("len(Required) = %d, want 2", len(s.Required))
	}
}

func TestInterface_Compliance(t *testing.T) {
	var _ tool.Tool = (*taskTool)(nil)
}

func TestExecute_UnknownSubagent(t *testing.T) {
	tt := &taskTool{holder: &RuntimeHolder{}}
	_, err := tt.Execute(context.Background(), map[string]any{
		"name": "shell",
		"goal": "inspect repo",
	})
	if err == nil || !strings.Contains(err.Error(), "unknown subagent") {
		t.Fatalf("Execute() err = %v, want unknown subagent", err)
	}
}

func TestExecute_NestedTaskRejected(t *testing.T) {
	tt := &taskTool{holder: &RuntimeHolder{}}
	ctx := context.WithValue(context.Background(), middleware.TraceSessionIDContextKey, "task-550e8400-e29b-41d4-a716-446655440000")
	_, err := tt.Execute(ctx, map[string]any{
		"name": "explore",
		"goal": "find main",
	})
	if err == nil || !strings.Contains(err.Error(), "nested Task") {
		t.Fatalf("Execute() err = %v, want nested Task rejection", err)
	}
}

func TestChildSessionID_Prefix(t *testing.T) {
	got := childSessionID("telegram-12345")
	if !strings.HasPrefix(got, "task-") {
		t.Fatalf("childSessionID = %q, want task- prefix", got)
	}
}

func TestParseModelTier(t *testing.T) {
	cases := map[string]string{
		"haiku":  "low",
		"sonnet": "mid",
		"opus":   "high",
		"":       "",
		"bogus":  "",
	}
	for in, want := range cases {
		got := parseModelTier(in)
		if string(got) != want {
			t.Fatalf("parseModelTier(%q) = %q, want %q", in, got, want)
		}
	}
}
