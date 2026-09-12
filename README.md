# hecc-blot-ratelimit

限流模块：内存（滑动窗口 / 令牌桶）与 Redis（滑动窗口）两种后端。

## 安装

```bash
go get github.com/hecc-blot/ratelimit
```

## 快速开始

```go
import (
	ratelimitContract "github.com/hecc-blot/ratelimit/contract"
	ratelimitConfig "github.com/hecc-blot/ratelimit/config"
	algorithm "github.com/hecc-blot/ratelimit/enum/algorithm"
	ratelimit "github.com/hecc-blot/ratelimit/service"
)

func main() {
	// 1. 构造（无 cleanup、无 error）
	limiter := ratelimit.NewMemoryLimiter(ratelimitConfig.Config{
		Algorithm: algorithm.SlidingWindow,
		Limit:     100,
		Window:    60, // 秒
	})

	// 2. 挂到路由上（中间件实现见下）
	apiHandle.Middleware(&RateLimitMiddleware{Limiter: limiter})
}
```

完整的应用组装顺序（日志 → trace → ratelimit → 容器 → 路由）见[组织主页](https://github.com/hecc-blot)。

## 接口定义

```go
import ratelimitContract "github.com/hecc-blot/ratelimit/contract"

// 判断 key 本次请求是否放行。实现方保证 Allow 并发安全。
type RateLimiter interface {
	Allow(ctx context.Context, key string) bool
}
```

## 使用

### 内存限流器

```go
// 滑动窗口：窗口内计数，边界平滑，无突发
limiter := ratelimit.NewMemoryLimiter(ratelimitConfig.Config{
	Algorithm: algorithm.SlidingWindow,
	Limit:     100,
	Window:    60,
})

// 令牌桶：恒定速率放行，允许短时突发
limiter = ratelimit.NewMemoryLimiter(ratelimitConfig.Config{
	Algorithm: algorithm.TokenBucket,
	Limit:     100,
	Window:    60,
})
```

### Redis 限流器（集群）

```go
import "github.com/redis/go-redis/v9"

client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
limiter := ratelimit.NewRedisLimiter(client, ratelimitConfig.Config{
	Limit:  100,
	Window: 60,
})
```

> **Redis 后端只实现滑动窗口**，`Config.Algorithm` 会被忽略（内存后端才按算法分支）。
>
> **需要自备 `redis.UniversalClient`**：`cache` 的 `IRedisCache` 不暴露底层 client，所以无法直接复用缓存模块的连接，会另建一条连接。

### 实现中间件并注册

```go
import (
	"net/http"

	iCoreApi "github.com/hecc-blot/framework/contract/api"
	"github.com/hecc-blot/framework/enum/response"
	ratelimitContract "github.com/hecc-blot/ratelimit/contract"
)

type RateLimitMiddleware struct {
	Limiter ratelimitContract.RateLimiter
}

func (r *RateLimitMiddleware) Middleware() iCoreApi.MiddlewareFunc {
	return func(ctx iCoreApi.IContext) {
		if !r.Limiter.Allow(ctx, ctx.ClientIP()) {
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, map[string]any{
				"code":    response.RateLimit,
				"message": response.CodeMap[response.RateLimit],
			})
			return
		}
		ctx.Next()
	}
}

// 组装期
apiHandle.Middleware(&RateLimitMiddleware{Limiter: limiter})
```

限流粒度由 `Allow` 的 `key` 决定：按 IP（`ctx.ClientIP()`）、按用户（`ctx.Get(...)` 取）、按接口（`ctx.FullPath()`）都可以，也常拼成 `方法:路径:IP`。

## 配置

`Config` 由业务方自持有（模块不定义 yaml 段）：

```go
type Config struct {
	Algorithm algorithm.Algorithm // 限流算法，默认 SlidingWindow
	Limit     int                 // 窗口内最大请求数 / 令牌桶容量，默认 100
	Window    int                 // 窗口时长（秒），默认 60
}
```

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `Algorithm` | `algorithm.Algorithm` | `sliding_window` | `sliding_window` / `token_bucket`（**仅内存后端生效**） |
| `Limit` | int | 100 | 窗口内最大请求数 / 桶容量，`<= 0` 回填默认 |
| `Window` | int | 60 | 窗口时长（秒），`<= 0` 回填默认 |

`Normalize` 会补全全部默认值。

## 语义与约定

- **Redis 后端 fail-open**：Redis 异常时 `Allow` 直接返回 `true`（放行），**既不报错也不打日志**。可用性优先，但意味着限流在 Redis 故障期间静默失效。
- **Redis key 前缀**：实际写入的 key 是 `hecc:ratelimit:<你的key>`。
- **并发安全**：`RateLimiter` 实例可跨 goroutine 共享；内存后端内部对每个 key 的计数结构加锁。
- **内存后端无清理**：每个不同的 key 都会在内存中留一份计数结构，**不会被回收**。用高基数 key（如完整 URL）会导致内存持续增长。
- **panic 条件**：`NewRedisLimiter(nil, ...)` 会 panic。
- **无释放语义**：不持有需关闭的资源（Redis client 由调用方管理），不需要 cleanup。

## 相关模块

| 模块 | 说明 |
|------|------|
| [framework](https://github.com/hecc-blot/framework) | `IContext` / `MiddlewareFunc`，限流中间件的载体 |
| [cache](https://github.com/hecc-blot/cache) | 原子原语的来源（但 Redis 连接无法直接复用） |
