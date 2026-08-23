package service

import (
	"context"
	"sync"
	"time"

	ratelimitConfig "github.com/hecc-blot/ratelimit/config"
	ratelimit "github.com/hecc-blot/ratelimit/contract"
	algorithm "github.com/hecc-blot/ratelimit/enum/algorithm"

	"golang.org/x/time/rate"
)

// NewMemoryLimiter 创建内存限流器，按 cfg.Algorithm 选择滑动窗口或令牌桶实现。
// 供单实例场景使用；集群需跨实例统一计数时改用 Redis 后端。
func NewMemoryLimiter(cfg ratelimitConfig.Config) ratelimit.RateLimiter {
	cfg = ratelimitConfig.Normalize(cfg)
	switch cfg.Algorithm {
	case algorithm.TokenBucket:
		return &tokenBucketLimiter{
			limit:   cfg.Limit,
			window:  time.Duration(cfg.Window) * time.Second,
			buckets: make(map[string]*tokenBucket),
		}
	default:
		return &slidingWindowLimiter{
			limit:  cfg.Limit,
			window: time.Duration(cfg.Window) * time.Second,
			hits:   make(map[string][]time.Time),
		}
	}
}

// slidingWindowLimiter 滑动窗口限流器：每 key 维护时间戳队列，窗口内计数，边界平滑无突发。
type slidingWindowLimiter struct {
	mu        sync.Mutex
	limit     int
	window    time.Duration
	hits      map[string][]time.Time
	lastSweep time.Time
}

func (l *slidingWindowLimiter) Allow(_ context.Context, key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	// 惰性清理：每隔一个窗口清扫一次过期 key，避免 map 无限增长
	if now.Sub(l.lastSweep) >= l.window {
		l.sweep(now)
		l.lastSweep = now
	}

	cutoff := now.Add(-l.window)
	times := l.hits[key]

	// 裁剪窗口外的旧时间戳（时间戳按追加顺序单调递增，只需从头部裁）
	i := 0
	for i < len(times) && times[i].Before(cutoff) {
		i++
	}
	times = times[i:]

	if len(times) >= l.limit {
		l.hits[key] = times
		return false
	}

	l.hits[key] = append(times, now)
	return true
}

func (l *slidingWindowLimiter) sweep(now time.Time) {
	cutoff := now.Add(-l.window)
	for key, times := range l.hits {
		i := 0
		for i < len(times) && times[i].Before(cutoff) {
			i++
		}
		times = times[i:]
		if len(times) == 0 {
			delete(l.hits, key)
		} else {
			l.hits[key] = times
		}
	}
}

// tokenBucketLimiter 令牌桶限流器：每 key 一个 rate.Limiter，恒定速率放行、允许短时突发。
type tokenBucketLimiter struct {
	mu        sync.Mutex
	limit     int
	window    time.Duration
	buckets   map[string]*tokenBucket
	lastSweep time.Time
}

type tokenBucket struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

func (l *tokenBucketLimiter) Allow(_ context.Context, key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.lastSweep) >= l.window {
		l.sweep(now)
		l.lastSweep = now
	}

	b, ok := l.buckets[key]
	if !ok {
		// 速率 = limit/window（每秒），容量 = limit（允许窗口内极限突发）
		b = &tokenBucket{
			lim: rate.NewLimiter(rate.Limit(float64(l.limit)/l.window.Seconds()), l.limit),
		}
		l.buckets[key] = b
	}
	b.lastSeen = now
	return b.lim.Allow()
}

func (l *tokenBucketLimiter) sweep(now time.Time) {
	cutoff := now.Add(-l.window)
	for key, b := range l.buckets {
		if b.lastSeen.Before(cutoff) {
			delete(l.buckets, key)
		}
	}
}
