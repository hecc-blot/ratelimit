package ratelimit

import "context"

// RateLimiter 限流器：判断 key 本次请求是否放行。
//
// 内存实现按 key（如客户端 IP）本地计数；Redis 实现跨实例统一计数，
// 供集群场景使用。实现方应保证 Allow 并发安全。
type RateLimiter interface {
	Allow(ctx context.Context, key string) bool
}
