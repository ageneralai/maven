package stringutil

// Truncate returns s if len(s) <= n, otherwise s[:n]+"...". It measures byte length
// (same as the previous per-call-site helpers); not safe for arbitrary UTF-8 display width.
func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
