package slash

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ageneralai/maven/internal/cron"
	tcron "github.com/ageneralai/maven/internal/tools/cron"
)

func cronHandlers(svc *cron.Service) []handlerEntry {
	if svc == nil {
		return nil
	}
	return []handlerEntry{
		{
			def: Definition{
				Name: "cron-add",
				Description: "Schedule a persisted cron job. Exactly one of --expr (cron with seconds, six fields), " +
					"--in (duration from now, e.g. 1m), or --at-ms (Unix ms). When the job runs, the agent executes with --message. " +
					"Use --deliver true with --channel and --to to send the agent reply (e.g. telegram + chat id).",
			},
			h: HandlerFunc(func(ctx context.Context, inv Invocation) (Result, error) {
				return handleCronAdd(ctx, svc, inv)
			}),
		},
		{
			def: Definition{
				Name:        "cron-list",
				Description: "List persisted cron jobs (id, name, schedule, delivery).",
			},
			h: HandlerFunc(func(ctx context.Context, inv Invocation) (Result, error) {
				return handleCronList(ctx, svc, inv)
			}),
		},
		{
			def: Definition{
				Name:        "cron-remove",
				Description: "Remove a persisted cron job by id (see /cron-list).",
			},
			h: HandlerFunc(func(ctx context.Context, inv Invocation) (Result, error) {
				return handleCronRemove(ctx, svc, inv)
			}),
		},
	}
}

type handlerEntry struct {
	def Definition
	h   Handler
}

func handleCronAdd(_ context.Context, svc *cron.Service, inv Invocation) (Result, error) {
	name, ok := inv.Flag("name")
	if !ok || strings.TrimSpace(name) == "" {
		return Result{}, fmt.Errorf("cron-add: --name is required")
	}
	msg, ok := inv.Flag("message")
	if !ok || strings.TrimSpace(msg) == "" {
		return Result{}, fmt.Errorf("cron-add: --message is required")
	}
	expr, hasExpr := inv.Flag("expr")
	inStr, hasIn := inv.Flag("in")
	atMsStr, hasAtMs := inv.Flag("at-ms")
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
		return Result{}, fmt.Errorf("cron-add: exactly one of --expr, --in, or --at-ms is required")
	}
	deliver := false
	if v, ok := inv.Flag("deliver"); ok && (v == "true" || v == "1" || strings.EqualFold(v, "yes")) {
		deliver = true
	}
	chv, _ := inv.Flag("channel")
	to, _ := inv.Flag("to")
	chv = strings.TrimSpace(chv)
	to = strings.TrimSpace(to)
	if deliver && (chv == "" || to == "") {
		return Result{}, fmt.Errorf("cron-add: --deliver requires non-empty --channel and --to")
	}
	p := tcron.AddParams{
		Name:    strings.TrimSpace(name),
		Message: strings.TrimSpace(msg),
		Expr:    expr,
		In:      inStr,
		Deliver: deliver,
		Channel: chv,
		To:      to,
	}
	if hasAtMs && atMsStr != "" {
		atMs, err := strconv.ParseInt(atMsStr, 10, 64)
		if err != nil {
			return Result{}, fmt.Errorf("cron-add: --at-ms: %w", err)
		}
		p.AtMs = atMs
		p.HasAtMs = true
	}
	job, err := tcron.Add(svc, p, time.Now())
	if err != nil {
		return Result{}, err
	}
	out := tcron.FormatJobAdded(job)
	return Result{Command: "cron-add", Output: out}, nil
}

func handleCronList(_ context.Context, svc *cron.Service, _ Invocation) (Result, error) {
	out := tcron.FormatList(svc.ListJobs())
	return Result{Command: "cron-list", Output: out}, nil
}

func handleCronRemove(_ context.Context, svc *cron.Service, inv Invocation) (Result, error) {
	id, ok := inv.Flag("id")
	id = strings.TrimSpace(id)
	if !ok || id == "" {
		return Result{}, fmt.Errorf("cron-remove: --id is required")
	}
	if !svc.RemoveJob(id) {
		return Result{}, fmt.Errorf("cron-remove: no job with id %q", id)
	}
	return Result{Command: "cron-remove", Output: fmt.Sprintf("Removed job %q.", id)}, nil
}
