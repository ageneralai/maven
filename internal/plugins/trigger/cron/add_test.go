package cron

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	turnctx "github.com/ageneralai/maven/internal/kernel/turnctx"
	"github.com/ageneralai/maven/internal/kernel/executor"
	"log/slog"
)

var toolTestLog = slog.New(slog.DiscardHandler)

func TestAddFromToolMap_incomingChat(t *testing.T) {
	t.Parallel()
	ctx := turnctx.WithInbound(context.Background(), "telegram", "4242")
	svc, err := NewService(filepath.Join(t.TempDir(), "j.json"), executor.Nop{}, 1, toolTestLog, nil)
	if err != nil {
		t.Fatal(err)
	}
	job, err := AddFromToolMap(svc, ctx, map[string]any{
		"name":                     "n",
		"message":                  "m",
		"in":                       "2m",
		"deliver_to_incoming_chat": true,
	}, time.Unix(1000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if !job.Payload.Deliver || job.Payload.Channel != "telegram" || job.Payload.To != "4242" {
		t.Fatalf("%+v", job.Payload)
	}
}

func TestAddFromToolMap_inferIncomingFromGateway(t *testing.T) {
	t.Parallel()
	ctx := turnctx.WithInbound(context.Background(), "telegram", "999")
	svc, err := NewService(filepath.Join(t.TempDir(), "j.json"), executor.Nop{}, 1, toolTestLog, nil)
	if err != nil {
		t.Fatal(err)
	}
	job, err := AddFromToolMap(svc, ctx, map[string]any{
		"name": "n", "message": "m", "in": "2m",
	}, time.Unix(1000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if !job.Payload.Deliver || job.Payload.Channel != "telegram" || job.Payload.To != "999" {
		t.Fatalf("%+v", job.Payload)
	}
}

func TestAddFromToolMap_explicitDeliverFalseNoInfer(t *testing.T) {
	t.Parallel()
	ctx := turnctx.WithInbound(context.Background(), "telegram", "999")
	svc, err := NewService(filepath.Join(t.TempDir(), "j.json"), executor.Nop{}, 1, toolTestLog, nil)
	if err != nil {
		t.Fatal(err)
	}
	job, err := AddFromToolMap(svc, ctx, map[string]any{
		"name": "n", "message": "m", "in": "2m",
		"deliver": false,
	}, time.Unix(1000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if job.Payload.Deliver || job.Payload.Channel != "" || job.Payload.To != "" {
		t.Fatalf("%+v", job.Payload)
	}
}

func TestAddFromToolMap_incomingChatMissingContext(t *testing.T) {
	t.Parallel()
	svc, err := NewService(filepath.Join(t.TempDir(), "j.json"), executor.Nop{}, 1, toolTestLog, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, addErr := AddFromToolMap(svc, context.Background(), map[string]any{
		"name":                     "n",
		"message":                  "m",
		"in":                       "1s",
		"deliver_to_incoming_chat": true,
	}, time.Now())
	if addErr == nil {
		t.Fatal("expected error")
	}
}

func TestScheduleToolExecute(t *testing.T) {
	t.Parallel()
	svc, err := NewService(filepath.Join(t.TempDir(), "j.json"), executor.Nop{}, 1, toolTestLog, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := &Plugin{log: toolTestLog}
	p.svc = svc
	tools := Tools(p, toolTestLog)
	if len(tools) != 3 {
		t.Fatalf("tools=%d", len(tools))
	}
	ctx := turnctx.WithInbound(context.Background(), "telegram", "1")
	res, err := tools[0].Execute(ctx, map[string]any{
		"name":                     "a",
		"message":                  "b",
		"in":                       "500ms",
		"deliver_to_incoming_chat": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success || !strings.Contains(res.Output, "Added job") {
		t.Fatalf("%+v", res)
	}
}

func TestAdd_duplicateScheduleKinds(t *testing.T) {
	t.Parallel()
	svc, err := NewService(filepath.Join(t.TempDir(), "j.json"), executor.Nop{}, 1, toolTestLog, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, addErr := Add(svc, AddParams{Name: "x", Message: "y", Expr: "0 0 * * * *", In: "1m"}, time.Now())
	if addErr == nil {
		t.Fatal("expected error")
	}
}
