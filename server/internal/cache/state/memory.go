package state

import (
	"context"
	"time"

	"github.com/lucap9056/corvauth/server/internal/cache"
	"github.com/maypok86/otter/v2"
)

type memoryCache struct {
	sessions *otter.Cache[string, Session]
}

func newMemoryCache(maximumSize int, ttl time.Duration) (*memoryCache, error) {
	sessions, err := cache.NewMemoryCache[string](maximumSize, func(Session) time.Duration {
		return ttl
	})
	if err != nil {
		return nil, err
	}
	return &memoryCache{sessions: sessions}, nil
}

func (m *memoryCache) Take(_ context.Context, key string) (Session, error) {
	var session Session
	m.sessions.Compute(key, func(oldValue Session, found bool) (Session, otter.ComputeOp) {
		session = oldValue
		return Session{}, otter.InvalidateOp
	})
	return session, nil
}

func (m *memoryCache) Set(_ context.Context, key string, session Session) error {
	m.sessions.Set(key, session)
	return nil
}
