package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ageneralai/ageneral-agents-go/pkg/message"
)

// Store persists conversation history as JSONL under dir.
// One file per session: {dir}/{sessionID}.jsonl
type Store struct {
	dir string
}

// NewStore creates a Store backed by dir (created if absent).
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("session store mkdir: %w", err)
	}
	return &Store{dir: dir}, nil
}

// Load reads all messages for sessionID from disk. Returns nil, nil if not found.
func (s *Store) Load(sessionID string) ([]message.Message, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, nil
	}
	path := s.path(sessionID)
	// #nosec G304 -- session path is derived from sanitized sessionID under store dir
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("session load open: %w", err)
	}
	defer func() { _ = f.Close() }()
	var msgs []message.Message
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var m message.Message
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue // skip malformed lines
		}
		msgs = append(msgs, m)
	}
	return msgs, sc.Err()
}

// Save writes the full history for sessionID to disk, replacing any prior content.
func (s *Store) Save(sessionID string, msgs []message.Message) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || len(msgs) == 0 {
		return nil
	}
	path := s.path(sessionID)
	// #nosec G304 -- session path is derived from sanitized sessionID under store dir
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("session save create: %w", err)
	}
	defer func() { _ = f.Close() }()
	w := bufio.NewWriter(f)
	for _, m := range msgs {
		b, err := json.Marshal(m)
		if err != nil {
			continue
		}
		if _, err := fmt.Fprintf(w, "%s\n", b); err != nil {
			return fmt.Errorf("session save write: %w", err)
		}
	}
	return w.Flush()
}

func (s *Store) path(sessionID string) string {
	safe := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "..", "_").Replace(sessionID)
	return filepath.Join(s.dir, safe+".jsonl")
}
