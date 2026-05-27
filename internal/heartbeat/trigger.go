package heartbeat

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"log/slog"
)

// Trigger decides whether a heartbeat tick should run and supplies the agent prompt.
type Trigger interface {
	Prompt() string
}

type fileTrigger struct {
	workspace string
	log       *slog.Logger
}

func (t *fileTrigger) Prompt() string {
	hbPath := filepath.Join(t.workspace, "HEARTBEAT.md")
	// #nosec G304 -- path is workspace/HEARTBEAT.md under app-configured workspace
	data, err := os.ReadFile(hbPath)
	if err != nil {
		if !os.IsNotExist(err) {
			t.log.Error("heartbeat read error", "err", err)
		}
		return ""
	}
	return strings.TrimSpace(string(data))
}

// FileTrigger reads HEARTBEAT.md from workspace on each tick.
func FileTrigger(workspace string, log *slog.Logger) Trigger {
	return &fileTrigger{workspace: workspace, log: log}
}

// WithTrigger replaces the default file-based trigger (for tests or extensions).
func WithTrigger(tr Trigger) Option {
	return func(s *Service) {
		if tr == nil {
			panic("heartbeat: nil Trigger")
		}
		s.trigger = tr
	}
}

type staticTrigger struct {
	prompt string
}

func (t staticTrigger) Prompt() string {
	return t.prompt
}

// StaticTrigger returns a fixed prompt; empty prompt skips the turn.
func StaticTrigger(prompt string) Trigger {
	return staticTrigger{prompt: strings.TrimSpace(prompt)}
}

func triggerOrDefault(workspace string, log *slog.Logger, tr Trigger) Trigger {
	if tr != nil {
		return tr
	}
	if log == nil {
		panic(fmt.Errorf("heartbeat: logger is required for default trigger"))
	}
	return FileTrigger(workspace, log)
}
