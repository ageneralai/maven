package cron

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ageneralai/maven/internal/kernel/plugin"
)

func slashCommands(svc *Service) []plugin.SlashCommand {
	if svc == nil {
		return nil
	}
	return []plugin.SlashCommand{
		{
			Definition: plugin.SlashDefinition{
				Name: "cron-add",
				Description: "Schedule a persisted cron job. Exactly one of --expr (cron with seconds, six fields), " +
					"--in (duration from now, e.g. 1m), or --at-ms (Unix ms). When the job runs, the agent executes with --message. " +
					"Use --deliver true with --channel and --to to send the agent reply (e.g. telegram + chat id).",
			},
			Handler: slashHandlerFunc(func(ctx context.Context, inv plugin.SlashInvocation) (plugin.SlashResult, error) {
				return slashCronAdd(ctx, svc, inv)
			}),
		},
		{
			Definition: plugin.SlashDefinition{Name: "cron-list", Description: "List persisted cron jobs (id, name, schedule, delivery)."},
			Handler: slashHandlerFunc(func(ctx context.Context, inv plugin.SlashInvocation) (plugin.SlashResult, error) {
				return slashCronList(ctx, svc, inv)
			}),
		},
		{
			Definition: plugin.SlashDefinition{
				Name:        "jobs",
				Description: "List persisted cron jobs (alias for /cron-list).",
			},
			Handler: slashHandlerFunc(func(ctx context.Context, inv plugin.SlashInvocation) (plugin.SlashResult, error) {
				return slashCronList(ctx, svc, inv)
			}),
		},
		{
			Definition: plugin.SlashDefinition{Name: "cron-remove", Description: "Remove a persisted cron job by id (see /cron-list)."},
			Handler: slashHandlerFunc(func(ctx context.Context, inv plugin.SlashInvocation) (plugin.SlashResult, error) {
				return slashCronRemove(ctx, svc, inv)
			}),
		},
	}
}

type slashHandlerFunc func(context.Context, plugin.SlashInvocation) (plugin.SlashResult, error)

func (f slashHandlerFunc) Handle(ctx context.Context, inv plugin.SlashInvocation) (plugin.SlashResult, error) {
	return f(ctx, inv)
}

func slashCronAdd(_ context.Context, svc *Service, inv plugin.SlashInvocation) (plugin.SlashResult, error) {
	name, ok := slashFlag(inv, "name")
	if !ok || strings.TrimSpace(name) == "" {
		return plugin.SlashResult{}, fmt.Errorf("cron-add: --name is required")
	}
	msg, ok := slashFlag(inv, "message")
	if !ok || strings.TrimSpace(msg) == "" {
		return plugin.SlashResult{}, fmt.Errorf("cron-add: --message is required")
	}
	expr, hasExpr := slashFlag(inv, "expr")
	inStr, hasIn := slashFlag(inv, "in")
	atMsStr, hasAtMs := slashFlag(inv, "at-ms")
	expr = strings.TrimSpace(expr)
	inStr = strings.TrimSpace(inStr)
	atMsStr = strings.TrimSpace(atMsStr)
	nSched := 0
	if hasExpr && expr != "" {
		nSched++
	}
	if hasIn && inStr != "" {
		nSched++
	}
	if hasAtMs && atMsStr != "" {
		nSched++
	}
	if nSched != 1 {
		return plugin.SlashResult{}, fmt.Errorf("cron-add: exactly one of --expr, --in, or --at-ms is required")
	}
	deliver := false
	if v, ok := slashFlag(inv, "deliver"); ok && (v == "true" || v == "1" || strings.EqualFold(v, "yes")) {
		deliver = true
	}
	chv, _ := slashFlag(inv, "channel")
	to, _ := slashFlag(inv, "to")
	chv = strings.TrimSpace(chv)
	to = strings.TrimSpace(to)
	if deliver && (chv == "" || to == "") {
		return plugin.SlashResult{}, fmt.Errorf("cron-add: --deliver requires non-empty --channel and --to")
	}
	p := AddParams{
		Name: strings.TrimSpace(name), Message: strings.TrimSpace(msg), Expr: expr, In: inStr,
		Deliver: deliver, Channel: chv, To: to,
	}
	if hasAtMs && atMsStr != "" {
		atMs, err := strconv.ParseInt(atMsStr, 10, 64)
		if err != nil {
			return plugin.SlashResult{}, fmt.Errorf("cron-add: --at-ms: %w", err)
		}
		p.AtMs = atMs
		p.HasAtMs = true
	}
	job, err := Add(svc, p, time.Now())
	if err != nil {
		return plugin.SlashResult{}, err
	}
	return plugin.SlashResult{Command: "cron-add", Output: FormatJobAdded(job)}, nil
}

func slashCronList(_ context.Context, svc *Service, _ plugin.SlashInvocation) (plugin.SlashResult, error) {
	return plugin.SlashResult{Command: "cron-list", Output: FormatList(svc.ListJobs())}, nil
}

func slashCronRemove(_ context.Context, svc *Service, inv plugin.SlashInvocation) (plugin.SlashResult, error) {
	id, ok := slashFlag(inv, "id")
	id = strings.TrimSpace(id)
	if !ok || id == "" {
		return plugin.SlashResult{}, fmt.Errorf("cron-remove: --id is required")
	}
	if !svc.RemoveJob(id) {
		return plugin.SlashResult{}, fmt.Errorf("cron-remove: no job with id %q", id)
	}
	return plugin.SlashResult{Command: "cron-remove", Output: fmt.Sprintf("Removed job %q.", id)}, nil
}

func slashFlag(inv plugin.SlashInvocation, name string) (string, bool) {
	if inv.Flags == nil {
		return "", false
	}
	v, ok := inv.Flags[strings.ToLower(name)]
	return v, ok
}
