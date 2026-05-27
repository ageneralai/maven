package wsession

import (
	"strings"
	"sync"
	"time"

	"github.com/ageneralai/maven/internal/sessionid"
)

const (
	responseSessionTTL        = 30 * time.Minute
	maxResponseSessionEntries = 500
)

type responseSessionEntry struct {
	sessionID string
	ts        time.Time
}

type ResponseSessions struct {
	mu      sync.Mutex
	entries map[string]responseSessionEntry
}

func NewResponseSessions() *ResponseSessions {
	return &ResponseSessions{entries: map[string]responseSessionEntry{}}
}

func IsMavenResponseID(id string) bool {
	id = strings.TrimSpace(id)
	return strings.HasPrefix(id, "resp_") && len(id) > len("resp_")
}

func (s *ResponseSessions) StoreMavenResponseSession(responseID, sessionID string) {
	responseID = strings.TrimSpace(responseID)
	sessionID = sessionid.ChatSessionID(sessionid.WebChannelName, sessionID)
	if responseID == "" || sessionID == "" {
		return
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(now)
	delete(s.entries, responseID)
	s.entries[responseID] = responseSessionEntry{sessionID: sessionID, ts: now}
	s.evictLocked()
}

func (s *ResponseSessions) lookupMavenResponseSession(responseID string) (string, bool) {
	responseID = strings.TrimSpace(responseID)
	if responseID == "" {
		return "", false
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[responseID]
	if !ok {
		return "", false
	}
	if now.Sub(entry.ts) > responseSessionTTL {
		delete(s.entries, responseID)
		return "", false
	}
	delete(s.entries, responseID)
	s.entries[responseID] = responseSessionEntry{sessionID: entry.sessionID, ts: now}
	return entry.sessionID, true
}

func (s *ResponseSessions) pruneLocked(now time.Time) {
	for id, entry := range s.entries {
		if now.Sub(entry.ts) > responseSessionTTL {
			delete(s.entries, id)
		}
	}
}

func (s *ResponseSessions) evictLocked() {
	for len(s.entries) > maxResponseSessionEntries {
		var oldestID string
		var oldest time.Time
		for id, entry := range s.entries {
			if oldestID == "" || entry.ts.Before(oldest) {
				oldestID = id
				oldest = entry.ts
			}
		}
		if oldestID == "" {
			return
		}
		delete(s.entries, oldestID)
	}
}
