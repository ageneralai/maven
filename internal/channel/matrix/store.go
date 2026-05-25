package matrix

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
	"maunium.net/go/mautrix/id"
)

const matrixStateFile = "state.json"

type persistedState struct {
	NextBatch string `json:"nextBatch"`
	FilterID  string `json:"filterId"`
	DeviceID  string `json:"deviceId"`
}

type fileSyncStore struct {
	mu     sync.Mutex
	path   string
	userID id.UserID
	state  persistedState
}

func matrixStatePath(workspace string) string {
	return filepath.Join(workspace, ".matrix", matrixStateFile)
}

func openFileSyncStore(workspace string, userID id.UserID, configuredDeviceID string) (*fileSyncStore, error) {
	path := matrixStatePath(workspace)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create matrix state dir: %w", err)
	}
	s := &fileSyncStore{
		path:   path,
		userID: userID,
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	if s.state.DeviceID == "" {
		if configuredDeviceID != "" {
			s.state.DeviceID = configuredDeviceID
		} else {
			s.state.DeviceID = "MAVEN" + uuid.NewString()[:8]
		}
		if err := s.persist(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *fileSyncStore) DeviceID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state.DeviceID
}

func (s *fileSyncStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read matrix state: %w", err)
	}
	if len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, &s.state); err != nil {
		return fmt.Errorf("parse matrix state: %w", err)
	}
	return nil
}

func (s *fileSyncStore) persist() error {
	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal matrix state: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write matrix state: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("commit matrix state: %w", err)
	}
	return nil
}

func (s *fileSyncStore) SaveFilterID(ctx context.Context, userID id.UserID, filterID string) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	if userID != s.userID {
		return fmt.Errorf("matrix store: unexpected user %s", userID)
	}
	if filterID == s.state.FilterID {
		return nil
	}
	s.state.FilterID = filterID
	return s.persist()
}

func (s *fileSyncStore) LoadFilterID(ctx context.Context, userID id.UserID) (string, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	if userID != s.userID {
		return "", fmt.Errorf("matrix store: unexpected user %s", userID)
	}
	return s.state.FilterID, nil
}

func (s *fileSyncStore) SaveNextBatch(ctx context.Context, userID id.UserID, nextBatchToken string) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	if userID != s.userID {
		return fmt.Errorf("matrix store: unexpected user %s", userID)
	}
	if nextBatchToken == s.state.NextBatch {
		return nil
	}
	s.state.NextBatch = nextBatchToken
	return s.persist()
}

func (s *fileSyncStore) LoadNextBatch(ctx context.Context, userID id.UserID) (string, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	if userID != s.userID {
		return "", fmt.Errorf("matrix store: unexpected user %s", userID)
	}
	return s.state.NextBatch, nil
}
