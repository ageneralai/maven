package session

import (
	"github.com/ageneralai/ageneral-agents-go/pkg/api"
	"github.com/ageneralai/ageneral-agents-go/pkg/message"
	"github.com/ageneralai/maven/internal/kernel/sessionid"
)

// NoIsolatedStore wraps a Store and skips persistence for KindIsolated sessions.
// All other session kinds (chat, rotated, cron, heartbeat, task) persist normally.
type NoIsolatedStore struct{ inner *Store }

// NewNoIsolatedStore returns a NoIsolatedStore backed by inner.
func NewNoIsolatedStore(inner *Store) *NoIsolatedStore { return &NoIsolatedStore{inner: inner} }

var _ api.SessionStore = (*NoIsolatedStore)(nil)

func (s *NoIsolatedStore) Load(id string) ([]message.Message, error) {
	if isIsolated(id) {
		return nil, nil
	}
	return s.inner.Load(id)
}

func (s *NoIsolatedStore) Save(id string, msgs []message.Message) error {
	if isIsolated(id) {
		return nil
	}
	return s.inner.Save(id, msgs)
}

func isIsolated(id string) bool {
	sid, err := sessionid.Parse(id)
	return err == nil && sid.Kind == sessionid.KindIsolated
}
