package responses

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteSSE_FramingIsolation(t *testing.T) {
	rec := httptest.NewRecorder()
	payload := map[string]string{"delta": "hello\nworld\r\n"}
	if err := writeSSE(rec, rec, "response.output_text.delta", payload); err != nil {
		t.Fatalf("writeSSE: %v", err)
	}
	body := rec.Body.String()
	if !strings.HasPrefix(body, "event: response.output_text.delta\n") {
		t.Fatalf("missing event line: %q", body)
	}
	if !strings.Contains(body, "data: ") {
		t.Fatalf("missing data line: %q", body)
	}
	lines := strings.Split(body, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected event, data, blank lines: %q", body)
	}
	if lines[2] != "" {
		t.Fatalf("expected blank line after event/data pair, got %q", body)
	}
	if strings.Contains(lines[1], "\n") {
		t.Fatal("data line must not contain embedded newlines")
	}
	if !strings.Contains(lines[1], `\n`) {
		t.Fatalf("JSON should escape embedded newlines: %q", lines[1])
	}
}
