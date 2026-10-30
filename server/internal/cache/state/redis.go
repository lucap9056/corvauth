package state

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lucap9056/corvauth/server/internal/cache"
	"github.com/redis/rueidis"
)

const redisKeyPrefix = "oauth2:state:"

type redisCache struct {
	client *cache.RedisClient
	ttl    time.Duration
}

func newRedisCache(client *cache.RedisClient, ttl time.Duration) *redisCache {
	return &redisCache{client: client, ttl: ttl}
}

func (r *redisCache) Take(ctx context.Context, state string) (Session, error) {
	val, err := r.client.GetDel(ctx, redisKeyPrefix+state).ToString()
	if rueidis.IsRedisNil(err) {
		return Session{}, nil
	}
	if err != nil {
		return Session{}, fmt.Errorf("redis getdel failed: %w", err)
	}
	return decodeSession(val)
}

func (r *redisCache) Set(ctx context.Context, state string, session Session) error {
	encoded, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("encode state session: %w", err)
	}
	return r.client.Set(ctx, redisKeyPrefix+state, string(encoded), r.ttl)
}

func decodeSession(val string) (Session, error) {
	if !strings.HasPrefix(val, "{") {
		return Session{Verifier: val}, nil
	}
	var session Session
	if err := json.Unmarshal([]byte(val), &session); err != nil {
		return Session{}, fmt.Errorf("decode state session: %w", err)
	}
	return session, nil
}
