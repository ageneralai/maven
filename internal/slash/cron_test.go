package slash

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ageneralai/maven/internal/cron"
	mavenlog "github.com/ageneralai/maven/internal/log"
)

var testCronLog = mavenlog.Std()

func TestHandleCronAdd_atDuration(t *testing.T) {
	dir := t.TempDir()
	svc := cron.NewService(filepath.Join(dir, "jobs.json"), testCronLog)
	h := handleCronAddBody(svc)
	res, err := h(context.Background(), mustParseCron(t, `/cron-add --name n1 --in 2m --message "ping"`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(anyStr(res.Output), "Added job") {
		t.Fatalf("output: %v", res.Output)
	}
	jobs := svc.ListJobs()
	if len(jobs) != 1 {
		t.Fatalf("jobs len=%d", len(jobs))
	}
	if jobs[0].Name != "n1" || jobs[0].Schedule.Kind != "at" {
		t.Fatalf("%+v", jobs[0])
	}
	if jobs[0].Payload.Message != "ping" {
		t.Fatalf("message %q", jobs[0].Payload.Message)
	}
}

func TestHandleCronAdd_deliverRequiresChannel(t *testing.T) {
	svc := cron.NewService(filepath.Join(t.TempDir(), "jobs.json"), testCronLog)
	h := handleCronAddBody(svc)
	_, err := h(context.Background(), mustParseCron(t, `/cron-add --name x --in 1s --message hi --deliver true`))
	if err == nil || !strings.Contains(err.Error(), "--channel") {
		t.Fatalf("want channel error, got %v", err)
	}
}

func TestHandleCronRemove(t *testing.T) {
	dir := t.TempDir()
	svc := cron.NewService(filepath.Join(dir, "jobs.json"), testCronLog)
	add := handleCronAddBody(svc)
	if _, err := add(context.Background(), mustParseCron(t, `/cron-add --name z --in 1h --message m`)); err != nil {
		t.Fatal(err)
	}
	id := svc.ListJobs()[0].ID
	rem := handleCronRemoveBody(svc)
	if _, err := rem(context.Background(), mustParseCron(t, `/cron-remove --id `+id)); err != nil {
		t.Fatal(err)
	}
	if len(svc.ListJobs()) != 0 {
		t.Fatal("expected no jobs")
	}
}

func mustParseCron(t *testing.T, line string) Invocation {
	t.Helper()
	inv, err := Parse(line)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv) != 1 {
		t.Fatalf("got %d invocations", len(inv))
	}
	return inv[0]
}

func anyStr(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func handleCronAddBody(svc *cron.Service) func(context.Context, Invocation) (Result, error) {
	return func(ctx context.Context, inv Invocation) (Result, error) {
		return handleCronAdd(ctx, svc, inv)
	}
}

func handleCronRemoveBody(svc *cron.Service) func(context.Context, Invocation) (Result, error) {
	return func(ctx context.Context, inv Invocation) (Result, error) {
		return handleCronRemove(ctx, svc, inv)
	}
}
