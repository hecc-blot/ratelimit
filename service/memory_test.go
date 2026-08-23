package service

import (
	"context"
	"sync"
	"testing"

	ratelimitConfig "github.com/hecc-blot/ratelimit/config"
	algorithm "github.com/hecc-blot/ratelimit/enum/algorithm"

	"github.com/stretchr/testify/assert"
)

func TestMemoryLimiterSlidingWindow(t *testing.T) {
	limiter := NewMemoryLimiter(ratelimitConfig.Config{Algorithm: algorithm.SlidingWindow, Limit: 2, Window: 60})

	ctx := context.Background()
	assert.True(t, limiter.Allow(ctx, "ip1"))
	assert.True(t, limiter.Allow(ctx, "ip1"))
	assert.False(t, limiter.Allow(ctx, "ip1"))
	// 不同 key 互不影响
	assert.True(t, limiter.Allow(ctx, "ip2"))
}

func TestMemoryLimiterTokenBucket(t *testing.T) {
	limiter := NewMemoryLimiter(ratelimitConfig.Config{Algorithm: algorithm.TokenBucket, Limit: 2, Window: 60})

	ctx := context.Background()
	assert.True(t, limiter.Allow(ctx, "ip1"))
	assert.True(t, limiter.Allow(ctx, "ip1"))
	assert.False(t, limiter.Allow(ctx, "ip1"))
	assert.True(t, limiter.Allow(ctx, "ip2"))
}

// TestMemoryLimiterConcurrent 验证并发调用下无数据竞争、不 panic。
func TestMemoryLimiterConcurrent(t *testing.T) {
	limiter := NewMemoryLimiter(ratelimitConfig.Config{Algorithm: algorithm.SlidingWindow, Limit: 100, Window: 60})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_ = limiter.Allow(context.Background(), "ip")
			}
		}()
	}
	wg.Wait()
}
