package state

import (
	"context"
	"time"

	"github.com/lucap9056/corvauth/server/internal/cache"
)

type Session struct {
	Verifier    string `json:"verifier"`
	RedirectURL string `json:"redirect_url,omitempty"`
}

type Cache interface {
	Set(ctx context.Context, state string, session Session) error
	Take(ctx context.Context, state string) (Session, error)
}

type config struct {
	TTL         time.Duration
	MaximumSize int
}

type Option func(*config)

func WithTTL(ttl time.Duration) Option {
	return func(c *config) {
		c.TTL = ttl
	}
}

func WithMaximumSize(size int) Option {
	return func(c *config) {
		c.MaximumSize = size
	}
}

func NewCache(client *cache.RedisClient, opts ...Option) (Cache, error) {
	cfg := &config{TTL: 10 * time.Minute, MaximumSize: 100_000}
	for _, opt := range opts {
		opt(cfg)
	}

	if client == nil {
		return newMemoryCache(cfg.MaximumSize, cfg.TTL)
	}
	return newRedisCache(client, cfg.TTL), nil
}
