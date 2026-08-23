package service

import (
	"context"
	"strconv"
	"time"

	ratelimitConfig "github.com/hecc-blot/ratelimit/config"
	ratelimit "github.com/hecc-blot/ratelimit/contract"

	"github.com/redis/go-redis/v9"
)

// redisSlidingWindowScript 滑动窗口限流 Lua 脚本。
// KEYS[1]：限流 key；ARGV[1]：当前时间(ms)；ARGV[2]：窗口时长(ms)；
// ARGV[3]：阈值；ARGV[4]：唯一成员 ID。
// 用 ZSET 记录每次放行的时间戳，原子地清理窗口外记录、计数、放行/拒绝，
// 保证集群多实例下计数一致。
var redisSlidingWindowScript = redis.NewScript(`
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
redis.call('ZREMRANGEBYSCORE', KEYS[1], 0, now - window)
local count = redis.call('ZCARD', KEYS[1])
if count >= limit then
    return 0
end
redis.call('ZADD', KEYS[1], now, ARGV[4])
redis.call('PEXPIRE', KEYS[1], window)
return 1
`)

// redisRateLimitSvc Redis 滑动窗口限流器，集群场景下跨实例统一计数。
type redisRateLimitSvc struct {
	client redis.UniversalClient
	prefix string
	limit  int
	window time.Duration
}

// NewRedisLimiter 创建 Redis 后端限流器。client 由调用方注入（如复用缓存模块的 redis client），
// 不重复建连。Redis 异常时 fail-open（放行），保证限流组件不拖垮业务。
func NewRedisLimiter(client redis.UniversalClient, cfg ratelimitConfig.Config) ratelimit.RateLimiter {
	if client == nil {
		panic("ratelimit: redis 客户端不能为空")
	}
	cfg = ratelimitConfig.Normalize(cfg)
	return &redisRateLimitSvc{
		client: client,
		prefix: "hecc:ratelimit:",
		limit:  cfg.Limit,
		window: time.Duration(cfg.Window) * time.Second,
	}
}

func (r *redisRateLimitSvc) Allow(ctx context.Context, key string) bool {
	now := time.Now()
	member := strconv.FormatInt(now.UnixNano(), 10)

	res, err := redisSlidingWindowScript.Run(
		ctx,
		r.client,
		[]string{r.prefix + key},
		now.UnixMilli(), r.window.Milliseconds(), r.limit, member,
	).Int()
	if err != nil {
		// fail-open：Redis 异常时放行，可用性优先
		return true
	}
	return res == 1
}
