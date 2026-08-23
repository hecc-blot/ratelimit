package algorithm

// Algorithm 限流算法类型。
type Algorithm string

const (
	// SlidingWindow 滑动窗口：窗口内计数，边界平滑，无突发。
	SlidingWindow Algorithm = "sliding_window"
	// TokenBucket 令牌桶：恒定速率放行，允许短时突发。
	TokenBucket Algorithm = "token_bucket"
)
