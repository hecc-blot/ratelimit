# hecc-blot-ratelimit

请求频率限流：内存 + Redis 两种后端，滑动窗口 / 令牌桶两种算法。只提供接口契约与实现，限流中间件由业务方自行组装并注册。

## 安装

```bash
go get github.com/hecc-blot/ratelimit
```

## 接口定义

```go
import ratelimitContract "github.com/hecc-blot/ratelimit/contract"

// RateLimiter 限流器：判断 key 本次请求是否放行。
type RateLimiter interface {
    Allow(ctx context.Context, key string) bool
}
```

- 内存实现按 key（如客户端 IP）本地计数；
- Redis 实现跨实例统一计数，供集群场景使用；
- 实现方应保证 `Allow` 并发安全。

## 后端与算法

限流器通过 `RateLimiter` 接口抽象，两种后端：

| 后端 | 构造方式 | 适用场景 |
|------|---------|---------|
| 内存 | `ratelimitSvc.NewMemoryLimiter(cfg)` | 单实例 |
| Redis | `ratelimitSvc.NewRedisLimiter(client, cfg)` | 集群（跨实例统一计数，Lua 原子） |

两种后端均支持两种算法（由 `cfg.Algorithm` 决定）：

| 算法 | 值 | 说明 |
|------|-----|------|
| 滑动窗口 | `sliding_window`（默认） | 窗口内计数，边界平滑无突发 |
| 令牌桶 | `token_bucket` | 恒定速率，允许短时突发 |

> Redis 后端当前为滑动窗口实现；Redis 异常时 fail-open（放行），保证限流组件不拖垮业务。

## 使用

### 1. 构造限流器

```go
import (
    ratelimitConfig "github.com/hecc-blot/ratelimit/config"
    ratelimitContract "github.com/hecc-blot/ratelimit/contract"
    algorithm "github.com/hecc-blot/ratelimit/enum/algorithm"
    ratelimitSvc "github.com/hecc-blot/ratelimit/service"
)

// 内存后端（单实例）
limiter := ratelimitSvc.NewMemoryLimiter(ratelimitConfig.Config{
    Algorithm: algorithm.SlidingWindow,
    Limit:     100, // 窗口内最大请求数
    Window:    60,  // 窗口时长（秒）
})

// Redis 后端（集群，独立 Redis 连接，统一计数）
client := redis.NewClient(&redis.Options{Addr: config.Cache.Redis.Addr})
limiter = ratelimitSvc.NewRedisLimiter(client, ratelimitConfig.Config{
    Algorithm: algorithm.SlidingWindow,
    Limit:     100,
    Window:    60,
})
```

### 2. 实现中间件并注册

限流中间件由业务方自行实现并显式注册——是否启用、按什么维度限流、返回什么响应，完全由业务方决定：

```go
// RateLimitMiddleware 请求限流中间件 — 业务方实现。
type RateLimitMiddleware struct {
    Limiter ratelimitContract.RateLimiter
}

func (r *RateLimitMiddleware) Middleware() any {
    return func(c *gin.Context) {
        if !r.Limiter.Allow(c.Request.Context(), c.ClientIP()) {
            c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
                "code":    response.RateLimit,
                "message": response.CodeMap[response.RateLimit],
                "data":    nil,
            })
            return
        }
        c.Next()
    }
}

apiHandle.Middleware(&RateLimitMiddleware{Limiter: limiter})
```

## 配置

```go
// ratelimit/config/config.go
type Config struct {
    Algorithm algorithm.Algorithm `mapstructure:"algorithm"` // 默认 SlidingWindow
    Limit     int                 `mapstructure:"limit"`     // 默认 100
    Window    int                 `mapstructure:"window"`    // 默认 60
}

// Normalize 补全 Config 默认值，供各后端构造限流器时调用
func Normalize(cfg Config) Config
```

限流配置由业务方自持有（不属 framework 的 config），按需映射到自身配置结构：

```yaml
rate_limit:
  backend: memory            # memory | redis
  algorithm: sliding_window  # sliding_window | token_bucket
  limit: 100                 # 窗口内最大请求数 / 桶容量
  window: 60                 # 窗口时长（秒）
```

## 相关模块

| 模块 | 说明 |
|------|------|
| [framework](https://github.com/hecc-blot/framework) | `IMiddleware` 接口、统一响应 `response` |
