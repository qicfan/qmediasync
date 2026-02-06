package baidupan

import (
	"time"
)

// RespBase 基础响应结构
type RespBase[T any] struct {
	State   int    `json:"state"`
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

// RespBaseBool 基础响应结构（布尔状态）
type RespBaseBool[T any] struct {
	State   bool   `json:"state"`
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

// RequestConfig 请求配置
type RequestConfig struct {
	MaxRetries      int           `json:"max_retries"`
	RetryDelay      time.Duration `json:"retry_delay"`
	Timeout         time.Duration `json:"timeout"`
	BypassRateLimit bool          `json:"bypass_rate_limit"` // 是否绕过速率限制
}

// DefaultRequestConfig 默认请求配置
func DefaultRequestConfig() *RequestConfig {
	return &RequestConfig{
		MaxRetries: DEFAULT_MAX_RETRIES,
		RetryDelay: DEFAULT_RETRY_DELAY * time.Second,
		Timeout:    DEFAULT_TIMEOUT * time.Second,
	}
}

func MakeRequestConfig(maxRetries int, retryDelay time.Duration, timeout time.Duration) *RequestConfig {
	config := DefaultRequestConfig()
	if maxRetries > 0 {
		config.MaxRetries = maxRetries
	}
	if retryDelay > 0 {
		config.RetryDelay = retryDelay * time.Second
	}
	if timeout > 0 {
		config.Timeout = timeout * time.Second
	}
	return config
}

// RateLimitConfig 限速配置
type RateLimitConfig struct {
	QPSLimit int64 // 每秒请求数限制
	QPMLimit int64 // 每分钟请求数限制
	QPHLimit int64 // 每小时请求数限制
	QPTLimit int64 // 每天请求数限制
}

// DefaultRateLimitConfig 默认限速配置
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		QPSLimit: DEFAULT_QPS_LIMIT,
		QPMLimit: DEFAULT_QPM_LIMIT,
		QPHLimit: DEFAULT_QPH_LIMIT,
		QPTLimit: DEFAULT_QPT_LIMIT,
	}
}
