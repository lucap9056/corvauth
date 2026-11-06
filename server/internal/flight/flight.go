package flight

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"fmt"
	"time"

	"github.com/lucap9056/corvauth/server/internal/cache"
	"github.com/lucap9056/corvauth/server/internal/events"
	"github.com/lucap9056/corvauth/server/internal/logs"
	"github.com/maypok86/otter/v2"
	"github.com/redis/rueidis"
	"github.com/redis/rueidis/rueidislock"
	"golang.org/x/sync/singleflight"
)

const (
	lockKeyPrefix   = "flight:lock"
	resultKeyPrefix = "flight:result:"
	redisOpTimeout  = time.Second
	defaultTTL      = 30 * time.Second
	memoryMaxSize   = 100_000
)

type config struct {
	RedisURL  string
	ResultTTL time.Duration
}

type Option func(*config)

func WithRedis(url string, resultTTL time.Duration) Option {
	return func(c *config) {
		c.RedisURL = url
		c.ResultTTL = resultTTL
	}
}

func newLocker(url string) (rueidislock.Locker, error) {
	clientOption, err := rueidis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("invalid redis url: %w", err)
	}
	clientOption.Dialer.Timeout = 5 * time.Second

	locker, err := rueidislock.NewLocker(rueidislock.LockerOption{
		ClientOption: clientOption,
		KeyPrefix:    lockKeyPrefix,
		KeyMajority:  1,
	})
	if err != nil {
		return nil, fmt.Errorf("redis locker creation failed: %w", err)
	}
	return locker, nil
}

type redisStore struct {
	locker    rueidislock.Locker
	client    *cache.RedisClient
	resultTTL time.Duration
}

func (r *redisStore) close() {
	r.locker.Close()
}

func (r *redisStore) lock(ctx context.Context, key string) (context.Context, context.CancelFunc, error) {
	return r.locker.WithContext(ctx, key)
}

func (r *redisStore) load(ctx context.Context, key string) ([]byte, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, redisOpTimeout)
	defer cancel()

	raw, err := r.client.Get(ctx, resultKeyPrefix+key).AsBytes()
	if rueidis.IsRedisNil(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("redis get result failed: %w", err)
	}
	return raw, true, nil
}

func (r *redisStore) store(ctx context.Context, key string, raw []byte) error {
	ctx, cancel := context.WithTimeout(ctx, redisOpTimeout)
	defer cancel()

	if err := r.client.Set(ctx, resultKeyPrefix+key, rueidis.BinaryString(raw), r.resultTTL); err != nil {
		return fmt.Errorf("redis set result failed: %w", err)
	}
	return nil
}

type Group struct {
	group  singleflight.Group
	redis  *redisStore
	memory *otter.Cache[string, any]
}

func New(opts ...Option) (*Group, error) {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}

	resultTTL := cfg.ResultTTL
	if resultTTL <= 0 {
		resultTTL = defaultTTL
	}

	g := &Group{}
	if cfg.RedisURL == "" {
		memory, err := cache.NewMemoryCache[string](memoryMaxSize, func(any) time.Duration {
			return resultTTL
		})
		if err != nil {
			return nil, err
		}
		g.memory = memory
		return g, nil
	}

	locker, err := newLocker(cfg.RedisURL)
	if err != nil {
		return nil, err
	}

	g.redis = &redisStore{
		locker:    locker,
		client:    &cache.RedisClient{Client: locker.Client()},
		resultTTL: resultTTL,
	}
	return g, nil
}

func (g *Group) Close() {
	if g.redis != nil {
		g.redis.close()
	}
	if g.memory != nil {
		g.memory.StopAllGoroutines()
	}
}

type callConfig struct {
	inFlightOnly bool
}

type CallOption func(*callConfig)

func InFlightOnly() CallOption {
	return func(c *callConfig) {
		c.inFlightOnly = true
	}
}

func Do[T any](ctx context.Context, g *Group, key string, fn func(ctx context.Context) (T, error), opts ...CallOption) (T, error) {
	call := &callConfig{}
	for _, opt := range opts {
		opt(call)
	}

	v, err, _ := g.group.Do(key, func() (any, error) {
		if g.redis != nil {
			return doDistributed(ctx, g.redis, key, call.inFlightOnly, fn)
		}
		if call.inFlightOnly {
			return fn(ctx)
		}
		return doLocal(ctx, g.memory, key, fn)
	})

	result, _ := v.(T)
	return result, err
}

func doLocal[T any](ctx context.Context, memory *otter.Cache[string, any], key string, fn func(ctx context.Context) (T, error)) (T, error) {
	if cached, found := memory.GetIfPresent(key); found {
		if result, ok := cached.(T); ok {
			return result, nil
		}
	}

	result, err := fn(ctx)
	if err != nil {
		return result, err
	}

	memory.Set(key, result)
	return result, nil
}

func doDistributed[T any](ctx context.Context, r *redisStore, key string, inFlightOnly bool, fn func(ctx context.Context) (T, error)) (T, error) {
	hashedKey := hashKey(key)

	result, found, err := loadResult[T](ctx, r, hashedKey)
	if err != nil {
		logs.Degraded(ctx, events.ReasonFlightRedisUnavailable, "flight result lookup failed, running locally", err)
		return fn(ctx)
	}
	if found && !inFlightOnly {
		return result, nil
	}
	// A result that already existed before this call started belongs to an earlier, finished call, so an
	// in-flight-only caller is a replay and must neither reuse it nor overwrite it with its own outcome.
	completedBeforeStart := found

	lockCtx, unlock, err := r.lock(ctx, hashedKey)
	if err != nil {
		logs.Degraded(ctx, events.ReasonFlightLockUnavailable, "flight distributed lock unavailable, running locally", err)
		return fn(ctx)
	}
	defer unlock()

	if !completedBeforeStart {
		if result, found, _ := loadResult[T](lockCtx, r, hashedKey); found {
			return result, nil
		}
	}

	result, err = fn(lockCtx)
	if err != nil {
		return result, err
	}

	if !completedBeforeStart {
		storeResult(lockCtx, r, hashedKey, result)
	}
	return result, nil
}

func hashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func loadResult[T any](ctx context.Context, r *redisStore, hashedKey string) (T, bool, error) {
	var result T

	raw, found, err := r.load(ctx, hashedKey)
	if err != nil || !found {
		return result, false, err
	}

	if err := json.Unmarshal(raw, &result); err != nil {
		logs.Degraded(ctx, events.ReasonFlightResultCorrupt, "flight result unmarshal failed", err)
		return result, false, nil
	}
	return result, true, nil
}

func storeResult[T any](ctx context.Context, r *redisStore, hashedKey string, result T) {
	raw, err := json.Marshal(result)
	if err != nil {
		logs.Degraded(ctx, events.ReasonFlightResultCorrupt, "flight result marshal failed", err)
		return
	}

	if err := r.store(ctx, hashedKey, raw); err != nil {
		logs.Degraded(ctx, events.ReasonFlightRedisUnavailable, "flight result store failed", err)
	}
}
