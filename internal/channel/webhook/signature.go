package webhook

import (
	"crypto/sha1"
	"fmt"
	"sort"
	"strings"
)

// Signature computes the sorted SHA1 hex digest used by WeCom-style webhook verification.
func Signature(token, timestamp, nonce, body string) string {
	parts := []string{token, timestamp, nonce, body}
	sort.Strings(parts)
	sum := sha1.Sum([]byte(strings.Join(parts, "")))
	return fmt.Sprintf("%x", sum)
}
