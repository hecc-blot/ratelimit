package config

import (
	algorithm "github.com/hecc-blot/ratelimit/enum/algorithm"
)

// 默认限流参数。
const (
	DefaultLimit  = 100 // 默认窗口内最大请求数 / 令牌桶容量
	DefaultWindow = 60  // 默认窗口时长（秒）
)

// Config 限流参数，内存后端与 Redis 后端共用。
type Config struct {
	Algorithm algorithm.Algorithm `mapstructure:"algorithm"` // 限流算法，默认 SlidingWindow
	Limit     int                 `mapstructure:"limit"`     // 窗口内最大请求数 / 令牌桶容量
	Window    int                 `mapstructure:"window"`    // 窗口时长（秒）
}

// Normalize 补全 Config 默认值，供各后端构造限流器时调用。
func Normalize(cfg Config) Config {
	if cfg.Algorithm == "" {
		cfg.Algorithm = algorithm.SlidingWindow
	}
	if cfg.Limit <= 0 {
		cfg.Limit = DefaultLimit
	}
	if cfg.Window <= 0 {
		cfg.Window = DefaultWindow
	}
	return cfg
}
