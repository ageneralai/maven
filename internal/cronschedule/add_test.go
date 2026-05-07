package cronschedule

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stellarlinkco/myclaw/internal/cron"
	"github.com/stellarlinkco/myclaw/internal/inboundctx"
)

func TestAddFromToolMap_incomingChat(t *testing.T) {
	ctx := inboundctx.With(context.Background(), "telegram", "4242")
	svc := cron.NewService(filepath.Join(t.TempDir(), "j.json"))
	job, err := AddFromToolMap(svc, ctx, map[string]interface{}{
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

func TestAddFromToolMap_incomingChatMissingContext(t *testing.T) {
	svc := cron.NewService(filepath.Join(t.TempDir(), "j.json"))
	_, err := AddFromToolMap(svc, context.Background(), map[string]interface{}{
		"name":                     "n",
		"message":                  "m",
		"in":                       "1s",
		"deliver_to_incoming_chat": true,
	}, time.Now())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCronScheduleTool_Execute(t *testing.T) {
	svc := cron.NewService(filepath.Join(t.TempDir(), "j.json"))
	tools := Tools(svc)
	if len(tools) != 3 {
		t.Fatalf("tools=%d", len(tools))
	}
	ctx := inboundctx.With(context.Background(), "telegram", "1")
	res, err := tools[0].Execute(ctx, map[string]interface{}{
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
	svc := cron.NewService(filepath.Join(t.TempDir(), "j.json"))
	_, err := Add(svc, AddParams{Name: "x", Message: "y", Expr: "0 0 * * * *", In: "1m"}, time.Now())
	if err == nil {
		t.Fatal("expected error")
	}
}
