package webhook

import (
	"strings"
	"sync"
	"time"
)

const (
	DefaultMsgCacheTTL        = 5 * time.Minute
	DefaultReplyCacheTTL      = 1 * time.Hour
	DefaultCacheScanInterval  = 1 * time.Minute
)

type MsgIDCache struct {
	mu     sync.Mutex
	items  map[string]time.Time
	ttl    time.Duration
	lastGC time.Time
}

func NewMsgIDCache(ttl time.Duration) *MsgIDCache {
	if ttl <= 0 {
		ttl = DefaultMsgCacheTTL
	}
	return &MsgIDCache{
		items: make(map[string]time.Time),
		ttl:   ttl,
	}
}

func (c *MsgIDCache) Seen(key string) bool {
	if key == "" {
		return false
	}
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	if exp, ok := c.items[key]; ok {
		if now.Before(exp) {
			return true
		}
		delete(c.items, key)
	}
	c.items[key] = now.Add(c.ttl)
	c.gcLocked(now)
	return false
}

func (c *MsgIDCache) gcLocked(now time.Time) {
	if c.lastGC.IsZero() || now.Sub(c.lastGC) >= DefaultCacheScanInterval {
		for messageID, exp := range c.items {
			if now.After(exp) {
				delete(c.items, messageID)
			}
		}
		c.lastGC = now
	}
}

type replyTarget struct {
	url       string
	expiresAt time.Time
}

type ReplyURLCache struct {
	mu     sync.Mutex
	items  map[string]replyTarget
	ttl    time.Duration
	lastGC time.Time
}

func NewReplyURLCache(ttl time.Duration) *ReplyURLCache {
	if ttl <= 0 {
		ttl = DefaultReplyCacheTTL
	}
	return &ReplyURLCache{
		items: make(map[string]replyTarget),
		ttl:   ttl,
	}
}

func (c *ReplyURLCache) Set(chatID, replyURL string) {
	chatID = strings.TrimSpace(chatID)
	replyURL = strings.TrimSpace(replyURL)
	if chatID == "" || replyURL == "" {
		return
	}
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[chatID] = replyTarget{
		url:       replyURL,
		expiresAt: now.Add(c.ttl),
	}
	c.gcLocked(now)
}

func (c *ReplyURLCache) Get(chatID string) (string, bool) {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return "", false
	}
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	target, ok := c.items[chatID]
	if !ok {
		return "", false
	}
	if now.After(target.expiresAt) {
		delete(c.items, chatID)
		return "", false
	}
	c.gcLocked(now)
	return target.url, true
}

func (c *ReplyURLCache) gcLocked(now time.Time) {
	if c.lastGC.IsZero() || now.Sub(c.lastGC) >= DefaultCacheScanInterval {
		for chatID, target := range c.items {
			if now.After(target.expiresAt) {
				delete(c.items, chatID)
			}
		}
		c.lastGC = now
	}
}
