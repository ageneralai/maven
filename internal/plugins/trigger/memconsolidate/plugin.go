package memconsolidate

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"log/slog"

	"github.com/ageneralai/maven/internal/kernel/config"
	"github.com/ageneralai/maven/internal/kernel/executor"
	"github.com/ageneralai/maven/internal/kernel/plugin"
	"github.com/ageneralai/maven/internal/kernel/scheduling"
	"github.com/ageneralai/maven/internal/kernel/sessionid"
)

const defaultInterval = 24 * time.Hour
const journalDaysToReview = 7
const maxJournalCharsPerFile = 4000

// Plugin runs a nightly memory consolidation pass: reads recent daily journal files,
// asks the agent to promote worth-keeping facts to MEMORY.md.
type Plugin struct {
	interval time.Duration
	log      *slog.Logger
	mu       sync.Mutex
	runCtx   context.CancelFunc
}

func NewPlugin(intervalHours int, lg *slog.Logger) *Plugin {
	iv := time.Duration(intervalHours) * time.Hour
	if iv <= 0 {
		iv = defaultInterval
	}
	return &Plugin{interval: iv, log: lg}
}

func (p *Plugin) Name() string                { return "mem-consolidate" }
func (p *Plugin) Start(context.Context) error { return nil }
func (p *Plugin) Stop() error                 { return nil }

func (p *Plugin) Triggers(cfg *config.Config) []plugin.Trigger {
	if cfg == nil || !cfg.MemConsolidate.Enabled {
		return nil
	}
	return []plugin.Trigger{&runner{plugin: p, workspace: cfg.Agent.Workspace}}
}

type runner struct {
	plugin    *Plugin
	workspace string
}

func (r *runner) Name() string { return "mem-consolidate" }

func (r *runner) Start(ctx context.Context, exec executor.TurnExecutor, _ plugin.OutboundPublisher) error {
	r.plugin.mu.Lock()
	if r.plugin.runCtx != nil {
		r.plugin.runCtx()
	}
	runCtx, cancel := context.WithCancel(ctx)
	r.plugin.runCtx = cancel
	r.plugin.mu.Unlock()
	lane := scheduling.NewLane(1)
	go func() {
		ticker := time.NewTicker(r.plugin.interval)
		defer ticker.Stop()
		r.plugin.log.Info("mem-consolidate started", "interval", r.plugin.interval)
		for {
			select {
			case <-ticker.C:
				if !lane.TryAcquire() {
					r.plugin.log.Debug("mem-consolidate skipped: previous pass still running")
					continue
				}
				go func() {
					defer lane.Release()
					r.fire(runCtx, exec)
				}()
			case <-runCtx.Done():
				r.plugin.log.Info("mem-consolidate stopped")
				return
			}
		}
	}()
	return nil
}

func (r *runner) Stop() error {
	r.plugin.mu.Lock()
	defer r.plugin.mu.Unlock()
	if r.plugin.runCtx != nil {
		r.plugin.runCtx()
		r.plugin.runCtx = nil
	}
	return nil
}

func (r *runner) fire(ctx context.Context, exec executor.TurnExecutor) {
	prompt, err := buildPrompt(r.workspace)
	if err != nil || prompt == "" {
		r.plugin.log.Debug("mem-consolidate skipped: no journal content", "err", err)
		return
	}
	sessionID := sessionid.New(sessionid.KindIsolated, "mem-consolidate").String()
	result, err := exec.RunTurn(ctx, prompt, sessionID)
	if err != nil {
		r.plugin.log.Error("mem-consolidate error", "err", err)
		return
	}
	r.plugin.log.Info("mem-consolidate complete", "output", truncate(result, 120))
}

func buildPrompt(workspace string) (string, error) {
	memDir := filepath.Join(workspace, "memory")
	entries, err := os.ReadDir(memDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	cutoff := time.Now().AddDate(0, 0, -journalDaysToReview)
	var files []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".md") || name == "MEMORY.md" {
			continue
		}
		base := strings.TrimSuffix(name, ".md")
		if len(base) < 10 {
			continue
		}
		t, err := time.Parse("2006-01-02", base[:10])
		if err != nil || t.Before(cutoff) {
			continue
		}
		files = append(files, name)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	if len(files) == 0 {
		return "", nil
	}
	var sb strings.Builder
	for _, name := range files {
		// #nosec G304 -- name is from ReadDir under workspace/memory, validated as YYYY-MM-DD.md
		data, err := os.ReadFile(filepath.Join(memDir, name))
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		date := strings.TrimSuffix(name, ".md")
		fmt.Fprintf(&sb, "### %s\n%s\n\n", date, truncate(content, maxJournalCharsPerFile))
	}
	journal := strings.TrimSpace(sb.String())
	if journal == "" {
		return "", nil
	}
	return fmt.Sprintf(`You are performing a memory consolidation pass.

Your current long-term memory (MEMORY.md) is already in your context above.

Recent journal entries:
%s

Review these entries. Identify facts, preferences, decisions, or recurring context that belong in long-term memory and are not already captured in MEMORY.md.

Update memory/MEMORY.md to add or refine long-term facts. Rewrite the whole file. Be conservative — only include what clearly should persist indefinitely. If nothing new warrants promotion, leave the file unchanged.

Do not reply with any text.`, journal), nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
