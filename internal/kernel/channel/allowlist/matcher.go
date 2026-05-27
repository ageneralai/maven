package allowlist

import "strings"

type Matcher map[string]struct{}

func NewMatcher(ids []string) Matcher {
	m := make(Matcher, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		m[id] = struct{}{}
	}
	return m
}

func (m Matcher) Allow(id string) bool {
	if len(m) == 0 {
		return true
	}
	_, ok := m[id]
	return ok
}
