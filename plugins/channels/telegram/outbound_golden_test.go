package telegram

import (
	"strings"
	"testing"

	"github.com/ageneralai/maven/internal/testutil"
)

func TestGolden_ToTelegramHTML(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	cases := []struct {
		in  string
		out string
	}{
		{"hello", "hello"},
		{"**bold**", "<b>bold</b>"},
		{"`code`", "<code>code</code>"},
		{"a & b", "a &amp; b"},
		{"<tag>", "&lt;tag&gt;"},
		{"```go\nfunc main() {}\n```", "<pre>func main() {}\n</pre>"},
	}
	for _, c := range cases {
		b.WriteString("IN: ")
		b.WriteString(c.in)
		b.WriteString("\nOUT: ")
		b.WriteString(ToTelegramHTML(c.in))
		b.WriteString("\n---\n")
	}
	testutil.AssertGoldenFile(t, "telegram_html_escape.txt.golden", []byte(b.String()))
}
