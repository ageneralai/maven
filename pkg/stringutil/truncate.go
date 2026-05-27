package stringutil

import (
	"strings"
	"unicode/utf8"
)

// Truncate returns s if len(s) <= n, otherwise s[:n]+"...". It measures byte length
// (same as the previous per-call-site helpers); not safe for arbitrary UTF-8 display width.
func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// TruncateBytes returns s if its UTF-8 byte length is <= maxBytes; otherwise the longest
// prefix whose encoded length is <= maxBytes without splitting a code point.
func TruncateBytes(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	runes := []rune(s)
	bytesCount := 0
	for i, r := range runes {
		runeBytes := utf8.RuneLen(r)
		if runeBytes < 0 {
			runeBytes = 1
		}
		if bytesCount+runeBytes > maxBytes {
			return string(runes[:i])
		}
		bytesCount += runeBytes
	}
	return s
}

// TruncateRunes returns s if it has <= maxRunes code points; otherwise the prefix of
// the first maxRunes runes.
func TruncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return s
	}
	n := 0
	for i := range s {
		if n == maxRunes {
			return s[:i]
		}
		n++
	}
	return s
}

// TruncateRunesTail returns s when it has <= maxRunes code points; otherwise marker plus
// the suffix needed so the result has at most maxRunes runes.
func TruncateRunesTail(s string, maxRunes int, marker string) string {
	if maxRunes <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	markerRunes := len([]rune(marker))
	if markerRunes >= maxRunes {
		return string(runes[len(runes)-maxRunes:])
	}
	tail := string(runes[len(runes)-maxRunes+markerRunes:])
	return marker + tail
}

// ChunkRunes splits s into chunks of at most maxRunes bytes, preferring breaks at the
// last newline within each chunk and otherwise cutting at a UTF-8 rune boundary.
func ChunkRunes(s string, maxRunes int) []string {
	if maxRunes <= 0 || len(s) <= maxRunes {
		return []string{s}
	}
	var chunks []string
	for len(s) > maxRunes {
		split := strings.LastIndex(s[:maxRunes], "\n")
		if split > 0 {
			chunks = append(chunks, s[:split])
			s = s[split:]
			continue
		}
		split = RuneAlignedCut(s, maxRunes)
		chunks = append(chunks, s[:split])
		s = s[split:]
	}
	if s != "" {
		chunks = append(chunks, s)
	}
	return chunks
}

// RuneAlignedCut returns the largest byte offset <= maxBytes that starts a valid UTF-8
// rune, or maxBytes when no earlier rune boundary exists.
func RuneAlignedCut(s string, maxBytes int) int {
	if maxBytes >= len(s) {
		return len(s)
	}
	for i := maxBytes; i > 0; i-- {
		if utf8.RuneStart(s[i]) {
			return i
		}
	}
	return maxBytes
}
