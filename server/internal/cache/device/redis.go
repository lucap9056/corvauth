package device

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/lucap9056/corvauth/server/internal/cache"
	"github.com/lucap9056/corvauth/server/internal/events"
	"github.com/lucap9056/corvauth/server/internal/logs"
	"github.com/redis/rueidis"
)

const (
	secretKeyPrefix = "oauth2:device:secret:"
	opTimeout       = time.Second
)

var setNewerSecretScript = rueidis.NewLuaScript(`
local current = redis.call('GET', KEYS[1])
if current then
	if current == '' then
		return 0
	end
	local generation = tonumber(string.match(current, '^(%d+):'))
	if generation and generation >= tonumber(ARGV[2]) then
		return 0
	end
end
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[3])
return 1
`)

type redisSecretCache struct {
	client *cache.RedisClient
	ttl    time.Duration
}

func newRedisSecretCache(client *cache.RedisClient) *redisSecretCache {
	return &redisSecretCache{client: client, ttl: secretTTL}
}

func (r *redisSecretCache) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), opTimeout)
}

func encodeSecret(secret Secret) string {
	return strconv.Itoa(secret.Generation) + ":" + secret.Value
}

func decodeSecret(raw string) (Secret, bool) {
	generation, value, found := strings.Cut(raw, ":")
	if !found || value == "" {
		return Secret{}, false
	}
	n, err := strconv.Atoi(generation)
	if err != nil {
		return Secret{}, false
	}
	return Secret{Value: value, Generation: n}, true
}

func (r *redisSecretCache) GetSecret(deviceID string) (Secret, bool) {
	ctx, cancel := r.ctx()
	defer cancel()

	raw, err := r.client.Get(ctx, secretKeyPrefix+deviceID).ToString()
	if err != nil {
		if !rueidis.IsRedisNil(err) {
			logs.Degraded(ctx, events.ReasonCacheRedisFailed, "device secret lookup in redis failed", err)
		}
		return Secret{}, false
	}
	return decodeSecret(raw)
}

func (r *redisSecretCache) SetSecret(deviceID string, secret Secret) {
	ctx, cancel := r.ctx()
	defer cancel()

	err := setNewerSecretScript.Exec(ctx, r.client.Client,
		[]string{secretKeyPrefix + deviceID},
		[]string{encodeSecret(secret), strconv.Itoa(secret.Generation), strconv.FormatInt(r.ttl.Milliseconds(), 10)},
	).Error()
	if err != nil {
		logs.Degraded(ctx, events.ReasonCacheRedisFailed, "device secret store in redis failed", err)
	}
}

func (r *redisSecretCache) DeleteSecret(deviceID string) {
	ctx, cancel := r.ctx()
	defer cancel()
	r.client.Set(ctx, secretKeyPrefix+deviceID, "", r.ttl+deletedSecretExtraTTL)
}

func (r *redisSecretCache) Close() error {
	return nil
}
