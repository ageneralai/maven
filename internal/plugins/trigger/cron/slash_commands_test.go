package cron

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ageneralai/maven/internal/kernel/executor"
	"github.com/ageneralai/maven/internal/kernel/plugin"
	"github.com/ageneralai/maven/internal/kernel/slash"
	"log/slog"
)

var testCronLog = slog.New(slog.DiscardHandler)

func TestSlashCronAdd_atDuration(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewService(filepath.Join(dir, "jobs.json"), executor.Nop{}, 1, testCronLog, nil)
	if err != nil {
		t.Fatal(err)
	}
	cmds := slashCommands(svc)
	h := cmds[0].Handler
	inv := mustParseSlash(t, `/cron-add --name n1 --in 2m --message "ping"`)
	res, err := h.Handle(context.Background(), toPluginInv(inv))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Output, "Added job") {
		t.Fatalf("output: %q", res.Output)
	}
	jobs := svc.ListJobs()
	if len(jobs) != 1 {
		t.Fatalf("jobs len=%d", len(jobs))
	}
	if jobs[0].Name != "n1" || !IsAtSchedule(jobs[0].Schedule) {
		t.Fatalf("%+v", jobs[0])
	}
	if jobs[0].Payload.Message != "ping" {
		t.Fatalf("message %q", jobs[0].Payload.Message)
	}
}

func TestSlashCronAdd_deliverRequiresChannel(t *testing.T) {
	svc, serr := NewService(filepath.Join(t.TempDir(), "jobs.json"), executor.Nop{}, 1, testCronLog, nil)
	if serr != nil {
		t.Fatal(serr)
	}
	cmds := slashCommands(svc)
	h := cmds[0].Handler
	inv := mustParseSlash(t, `/cron-add --name x --in 1s --message hi --deliver true`)
	_, err := h.Handle(context.Background(), toPluginInv(inv))
	if err == nil || !strings.Contains(err.Error(), "--channel") {
		t.Fatalf("want channel error, got %v", err)
	}
}

func TestSlashCronRemove(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewService(filepath.Join(dir, "jobs.json"), executor.Nop{}, 1, testCronLog, nil)
	if err != nil {
		t.Fatal(err)
	}
	cmds := slashCommands(svc)
	addH := cmds[0].Handler
	inv := mustParseSlash(t, `/cron-add --name z --in 1h --message m`)
	if _, err := addH.Handle(context.Background(), toPluginInv(inv)); err != nil {
		t.Fatal(err)
	}
	id := svc.ListJobs()[0].ID
	remH := cmds[3].Handler
	remInv := mustParseSlash(t, `/cron-remove --id `+id)
	if _, err := remH.Handle(context.Background(), toPluginInv(remInv)); err != nil {
		t.Fatal(err)
	}
	if len(svc.ListJobs()) != 0 {
		t.Fatal("expected no jobs")
	}
}

func mustParseSlash(t *testing.T, line string) slash.Invocation {
	t.Helper()
	inv, err := slash.Parse(line)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv) != 1 {
		t.Fatalf("got %d invocations", len(inv))
	}
	return inv[0]
}

func toPluginInv(inv slash.Invocation) plugin.SlashInvocation {
	return plugin.SlashInvocation{Name: inv.Name, Args: inv.Args, Flags: inv.Flags, Raw: inv.Raw, Position: inv.Position}
}
