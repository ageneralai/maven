package voice

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxSpeechFragmentRunes is the empirical TTS latency sweet spot;
// shorter increases round trips, longer delays the first word.
const maxSpeechFragmentRunes = 800

// TakeCompleteSentences moves leading sentence-sized fragments from buf (ending at . ! ? before whitespace or EOF).
func TakeCompleteSentences(buf *string) []string {
	s := *buf
	var out []string
	for {
		end := findSentenceEndIndex(s)
		if end < 0 {
			break
		}
		sent := strings.TrimSpace(s[:end])
		s = s[end:]
		if sent != "" {
			out = append(out, sent)
		}
	}
	if utf8.RuneCountInString(s) >= maxSpeechFragmentRunes {
		cut := truncateToMaxRunes(s, maxSpeechFragmentRunes)
		sent := strings.TrimSpace(s[:cut])
		s = strings.TrimLeft(s[cut:], " \t\n\r")
		if sent != "" {
			out = append(out, sent)
		}
	}
	*buf = s
	return out
}

// FlushRemainder returns trimmed trailing text and clears buf.
func FlushRemainder(buf *string) string {
	s := strings.TrimSpace(*buf)
	*buf = ""
	return s
}

func findSentenceEndIndex(s string) int {
	i := 0
	for i < len(s) {
		ru, w := utf8.DecodeRuneInString(s[i:])
		if ru == '.' || ru == '!' || ru == '?' {
			after := i + w
			if after >= len(s) {
				return after
			}
			ru2, _ := utf8.DecodeRuneInString(s[after:])
			if unicode.IsSpace(ru2) {
				return after
			}
		}
		i += w
	}
	return -1
}

func truncateToMaxRunes(s string, max int) int {
	i := 0
	n := 0
	for n < max && i < len(s) {
		_, w := utf8.DecodeRuneInString(s[i:])
		i += w
		n++
	}
	for i < len(s) {
		ru, w := utf8.DecodeRuneInString(s[i:])
		if unicode.IsSpace(ru) {
			return i + w
		}
		i += w
	}
	return len(s)
}
