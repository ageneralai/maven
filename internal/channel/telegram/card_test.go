package telegram

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestStatusCardAppendToolOutput_truncates(t *testing.T) {
	c := NewStatusCard()
	c.AddTool("tid", "DelegateTask", "{}")
	long := strings.Repeat("x", 5000)
	c.AppendToolOutput("tid", long)
	idx := c.toolIndex["tid"]
	out := c.tools[idx].output
	if n := utf8.RuneCountInString(out); n > maxToolOutputRunes {
		t.Fatalf("output runes = %d, want <= %d", n, maxToolOutputRunes)
	}
	if !strings.HasPrefix(out, "… ") {
		prefixLen := 8
		if len(out) < prefixLen {
			prefixLen = len(out)
		}
		t.Fatalf("expected truncated tail marker, got prefix %q", out[:prefixLen])
	}
}
