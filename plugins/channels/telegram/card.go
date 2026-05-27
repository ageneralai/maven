package telegram

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ageneralai/maven/kernel/stringutil"
)

type toolStatus int

const (
	toolRunning toolStatus = iota
	toolDone
	toolError
)

type toolEntry struct {
	name    string
	summary string
	status  toolStatus
	output  string
}

const maxToolOutputRunes = 1200

type StatusCard struct {
	started   time.Time
	iteration int
	tools     []toolEntry
	toolIndex map[string]int
}

func NewStatusCard() *StatusCard {
	return &StatusCard{
		started:   time.Now(),
		toolIndex: make(map[string]int),
	}
}

func (c *StatusCard) AddTool(toolUseID, name, summary string) {
	c.toolIndex[toolUseID] = len(c.tools)
	c.tools = append(c.tools, toolEntry{name: name, summary: summary, status: toolRunning})
}

func (c *StatusCard) FinishTool(toolUseID string, failed bool) {
	if idx, ok := c.toolIndex[toolUseID]; ok {
		if failed {
			c.tools[idx].status = toolError
		} else {
			c.tools[idx].status = toolDone
		}
	}
}

// AppendToolOutput appends streamed subprocess output for the tool row identified by toolUseID (e.g. DelegateTask).
func (c *StatusCard) AppendToolOutput(toolUseID, text string) {
	if c == nil || toolUseID == "" || text == "" {
		return
	}
	idx, ok := c.toolIndex[toolUseID]
	if !ok {
		return
	}
	c.tools[idx].output += text
	if r := utf8.RuneCountInString(c.tools[idx].output); r > maxToolOutputRunes {
		c.tools[idx].output = stringutil.TruncateRunesTail(c.tools[idx].output, maxToolOutputRunes, "… ")
	}
}

func (c *StatusCard) SetIteration(n int) {
	c.iteration = n
}

func (c *StatusCard) Render() string {
	var b strings.Builder
	b.WriteString("🤖 <b>Working...</b>\n")
	if c.iteration > 0 {
		fmt.Fprintf(&b, "\n🔄 Iteration %d\n", c.iteration)
	}
	if len(c.tools) > 0 {
		b.WriteString("\n")
		for _, t := range c.tools {
			var icon string
			switch t.status {
			case toolRunning:
				icon = "⏳"
			case toolDone:
				icon = "✅"
			case toolError:
				icon = "❌"
			}
			if t.summary != "" {
				fmt.Fprintf(&b, "%s <code>%s</code>(%s)\n", icon, EscapeHTML(t.name), EscapeHTML(t.summary))
			} else {
				fmt.Fprintf(&b, "%s <code>%s</code>\n", icon, EscapeHTML(t.name))
			}
			if t.output != "" {
				fmt.Fprintf(&b, "<pre>%s</pre>\n", EscapeHTML(t.output))
			}
		}
	}
	elapsed := time.Since(c.started).Truncate(time.Second)
	fmt.Fprintf(&b, "\n⏱ %s", elapsed)
	return b.String()
}
