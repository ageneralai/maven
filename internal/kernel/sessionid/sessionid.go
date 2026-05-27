package sessionid

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const WebChannelName = "web"

// ChatSessionID returns the default agentsdk session id for a channel+peer pair.
func ChatSessionID(channel, peer string) string {
	channel = strings.TrimSpace(channel)
	peer = strings.TrimSpace(peer)
	if channel == "" {
		return peer
	}
	prefix := channel + "-"
	if strings.HasPrefix(peer, prefix) {
		return peer
	}
	return prefix + peer
}

// FromRouteKey derives the chat session id from a StableRouteKey (channel:peer).
func FromRouteKey(routeKey string) string {
	routeKey = strings.TrimSpace(routeKey)
	if i := strings.Index(routeKey, ":"); i > 0 && i < len(routeKey)-1 {
		return ChatSessionID(routeKey[:i], routeKey[i+1:])
	}
	return routeKey
}

type Kind string

const (
	KindChat      Kind = "chat"
	KindCron      Kind = "cron"
	KindHeartbeat Kind = "heartbeat"
	KindIsolated  Kind = "isolated"
	KindRotated   Kind = "rotated"
	KindTask      Kind = "task"
)

type SessionID struct {
	Kind  Kind
	Owner string
	Token string
}

func New(kind Kind, seed string) SessionID {
	seed = strings.TrimSpace(seed)
	switch kind {
	case KindCron:
		return SessionID{Kind: KindCron, Owner: seed, Token: uuid.NewString()}
	case KindHeartbeat:
		return SessionID{Kind: KindHeartbeat, Token: uuid.NewString()}
	case KindIsolated:
		if seed == "" {
			seed = "session"
		}
		return SessionID{Kind: KindIsolated, Owner: seed, Token: strconv.FormatInt(time.Now().UnixNano(), 10)}
	case KindRotated:
		if seed == "" {
			seed = "session"
		}
		return SessionID{Kind: KindRotated, Owner: seed, Token: strconv.FormatInt(time.Now().UnixNano(), 10)}
	case KindTask:
		return SessionID{Kind: KindTask, Token: uuid.NewString()}
	default:
		return SessionID{Kind: KindChat, Owner: seed}
	}
}

func (id SessionID) String() string {
	switch id.Kind {
	case KindCron:
		return fmt.Sprintf("cron:%s:%s", id.Owner, id.Token)
	case KindHeartbeat:
		return "heartbeat:" + id.Token
	case KindTask:
		return "task:" + id.Token
	case KindIsolated:
		return fmt.Sprintf("%s:isolated:%s", id.Owner, id.Token)
	case KindRotated:
		return fmt.Sprintf("%s:rotated:%s", id.Owner, id.Token)
	default:
		return id.Owner
	}
}

func MatchesTask(sessionID string) bool {
	id, err := Parse(sessionID)
	return err == nil && id.Kind == KindTask
}

func Parse(s string) (SessionID, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return SessionID{}, fmt.Errorf("session id is empty")
	}
	if strings.HasPrefix(s, "cron:") {
		rest := strings.TrimPrefix(s, "cron:")
		jobID, token, ok := strings.Cut(rest, ":")
		if !ok || jobID == "" {
			return SessionID{}, fmt.Errorf("invalid cron session id %q", s)
		}
		if _, err := uuid.Parse(token); err != nil {
			return SessionID{}, fmt.Errorf("invalid cron session id %q", s)
		}
		return SessionID{Kind: KindCron, Owner: jobID, Token: token}, nil
	}
	if strings.HasPrefix(s, "heartbeat:") {
		token := strings.TrimPrefix(s, "heartbeat:")
		if _, err := uuid.Parse(token); err != nil {
			return SessionID{}, fmt.Errorf("invalid heartbeat session id %q", s)
		}
		return SessionID{Kind: KindHeartbeat, Token: token}, nil
	}
	if strings.HasPrefix(s, "task:") {
		token := strings.TrimPrefix(s, "task:")
		if _, err := uuid.Parse(token); err != nil {
			return SessionID{}, fmt.Errorf("invalid task session id %q", s)
		}
		return SessionID{Kind: KindTask, Token: token}, nil
	}
	if base, suffix, ok := strings.Cut(s, ":isolated:"); ok {
		if _, err := strconv.ParseInt(suffix, 10, 64); err != nil {
			return SessionID{}, fmt.Errorf("invalid isolated session id %q", s)
		}
		return SessionID{Kind: KindIsolated, Owner: base, Token: suffix}, nil
	}
	if base, suffix, ok := strings.Cut(s, ":rotated:"); ok {
		if _, err := strconv.ParseInt(suffix, 10, 64); err != nil {
			return SessionID{}, fmt.Errorf("invalid rotated session id %q", s)
		}
		return SessionID{Kind: KindRotated, Owner: base, Token: suffix}, nil
	}
	return SessionID{Kind: KindChat, Owner: s}, nil
}
