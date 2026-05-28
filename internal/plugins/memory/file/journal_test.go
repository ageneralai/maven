package memory

import (
	"context"
	"errors"
	"testing"

	sdkapi "github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/ageneral-agents-go/pkg/model"
	"github.com/ageneralai/ageneral-agents-go/pkg/tool"
	"github.com/ageneralai/maven/internal/kernel/hook"
)

func TestRunShadowTurn_logsRememberToolCalls(t *testing.T) {
	t.Parallel()
	log, cap := newCaptureLogger()
	fake := newFakeRuntime(func(context.Context, sdkapi.Request) (*sdkapi.Response, error) {
		return &sdkapi.Response{
			Result: &sdkapi.Result{
				ToolCalls: []model.ToolCall{
					{Name: "memory_search", Arguments: map[string]any{"query": "x"}},
					{Name: "remember", Arguments: map[string]any{"content": "hello"}},
					{Name: "remember", Arguments: map[string]any{"content": "world"}},
				},
			},
		}, nil
	})
	runShadowTurn(context.Background(), fake, log, hook.PostTurnEvent{UserMsg: "u", AssistantMsg: "a"})
	infos := cap.infos()
	if len(infos) != 2 {
		t.Fatalf("info records = %d, want 2", len(infos))
	}
	want := []string{"hello", "world"}
	for i, rec := range infos {
		if rec.Msg != "memory-file: remembered" {
			t.Fatalf("info[%d].Msg = %q", i, rec.Msg)
		}
		if rec.Attrs["content"] != want[i] {
			t.Fatalf("info[%d].content = %v, want %q", i, rec.Attrs["content"], want[i])
		}
	}
}

func TestRunShadowTurn_logsNothingToJournalWhenNoRemember(t *testing.T) {
	t.Parallel()
	log, cap := newCaptureLogger()
	fake := newFakeRuntime(func(context.Context, sdkapi.Request) (*sdkapi.Response, error) {
		return &sdkapi.Response{
			Result: &sdkapi.Result{
				ToolCalls: []model.ToolCall{
					{Name: "memory_search", Arguments: map[string]any{"query": "x"}},
				},
			},
		}, nil
	})
	runShadowTurn(context.Background(), fake, log, hook.PostTurnEvent{UserMsg: "u", AssistantMsg: "a"})
	debugs := cap.debugs()
	if len(debugs) != 1 || debugs[0].Msg != "memory-file: nothing to journal" {
		t.Fatalf("debugs = %+v, want one nothing-to-journal", debugs)
	}
	if len(cap.infos()) != 0 {
		t.Fatalf("infos = %+v, want none", cap.infos())
	}
}

func TestRunShadowTurn_logsNothingToJournalWhenResultNil(t *testing.T) {
	t.Parallel()
	log, cap := newCaptureLogger()
	fake := newFakeRuntime(func(context.Context, sdkapi.Request) (*sdkapi.Response, error) {
		return &sdkapi.Response{Result: nil}, nil
	})
	runShadowTurn(context.Background(), fake, log, hook.PostTurnEvent{UserMsg: "u", AssistantMsg: "a"})
	debugs := cap.debugs()
	if len(debugs) != 1 || debugs[0].Msg != "memory-file: nothing to journal" {
		t.Fatalf("debugs = %+v, want one nothing-to-journal", debugs)
	}
}

func TestRunShadowTurn_logsNothingToJournalWhenResponseNil(t *testing.T) {
	t.Parallel()
	log, cap := newCaptureLogger()
	fake := newFakeRuntime(func(context.Context, sdkapi.Request) (*sdkapi.Response, error) {
		return nil, nil //nolint:nilnil // exercises production nil-response branch
	})
	runShadowTurn(context.Background(), fake, log, hook.PostTurnEvent{UserMsg: "u", AssistantMsg: "a"})
	debugs := cap.debugs()
	if len(debugs) != 1 || debugs[0].Msg != "memory-file: nothing to journal" {
		t.Fatalf("debugs = %+v, want one nothing-to-journal", debugs)
	}
}

func TestRunShadowTurn_logsDebugOnError(t *testing.T) {
	t.Parallel()
	log, cap := newCaptureLogger()
	fake := newFakeRuntime(func(context.Context, sdkapi.Request) (*sdkapi.Response, error) {
		return nil, errors.New("boom")
	})
	runShadowTurn(context.Background(), fake, log, hook.PostTurnEvent{UserMsg: "u", AssistantMsg: "a"})
	debugs := cap.debugs()
	if len(debugs) != 1 || debugs[0].Msg != "memory-file: shadow turn failed" {
		t.Fatalf("debugs = %+v, want shadow turn failed", debugs)
	}
	errVal, ok := debugs[0].Attrs["err"]
	if !ok {
		t.Fatal("expected err attr")
	}
	if errVal.(error).Error() != "boom" {
		t.Fatalf("err attr = %v, want boom", errVal)
	}
}

type nameTool struct{ name string }

func (f nameTool) Name() string        { return f.name }
func (nameTool) Description() string   { return "" }
func (nameTool) Schema() *tool.JSONSchema { return nil }
func (nameTool) Execute(context.Context, map[string]any) (*tool.ToolResult, error) {
	return nil, nil //nolint:nilnil // stub tool for shadowTools filter test
}

func TestShadowTools_filtersToRememberAndMemorySearch(t *testing.T) {
	t.Parallel()
	all := []tool.Tool{
		nameTool{name: "remember"},
		nameTool{name: "memory_search"},
		nameTool{name: "memory_get"},
		nameTool{name: "unrelated"},
	}
	got := shadowTools(all)
	if len(got) != 2 {
		t.Fatalf("shadowTools len = %d, want 2", len(got))
	}
	if got[0].Name() != "remember" || got[1].Name() != "memory_search" {
		t.Fatalf("names = %q, %q; want remember, memory_search", got[0].Name(), got[1].Name())
	}
}
