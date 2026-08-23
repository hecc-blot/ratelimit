package service

import (
	"context"
	"testing"

	ratelimitConfig "github.com/hecc-blot/ratelimit/config"
	algorithm "github.com/hecc-blot/ratelimit/enum/algorithm"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// TestRedisLimiter 验证 Redis 滑动窗口限流（依赖真实 Redis，与 cache 测试一致）。
func TestRedisLimiter(t *testing.T) {
	ctx := context.Background()
	key := "hcc-ratelimit-test"

	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	defer client.Close()
	// 清理残留 key，保证测试确定性
	_ = client.Del(ctx, "hecc:ratelimit:"+key)

	limiter := NewRedisLimiter(client, ratelimitConfig.Config{Algorithm: algorithm.SlidingWindow, Limit: 2, Window: 60})

	assert.True(t, limiter.Allow(ctx, key))
	assert.True(t, limiter.Allow(ctx, key))
	assert.False(t, limiter.Allow(ctx, key))
}
