package baidupan

import (
	"Q115-STRM/internal/helpers"
	"context"
	"sync"
	"time"
)

// ThrottleManager 全局限流管理器，用于管理API访问频率限制
type ThrottleManager struct {
	sync.RWMutex
	// 是否处于限流状态
	isThrottled bool
	// 限流开始时间
	throttleStartTime time.Time
	// 限流通知通道
	throttleNotify chan struct{}
	// 限流暂停时长
	throttleDuration time.Duration
	// 限速配置
	rateLimit *RateLimitConfig
	// 请求统计
	stats *RequestStats
}

// NewThrottleManager 创建新的限流管理器
func NewThrottleManager(rateLimit *RateLimitConfig, stats *RequestStats) *ThrottleManager {
	if rateLimit == nil {
		rateLimit = DefaultRateLimitConfig()
	}
	if stats == nil {
		stats = NewRequestStats(10000)
	}
	return &ThrottleManager{
		isThrottled:      false,
		throttleNotify:   make(chan struct{}),
		throttleDuration: 1 * time.Minute,
		rateLimit:        rateLimit,
		stats:            stats,
	}
}

// IsThrottled 检查是否处于限流状态
func (tm *ThrottleManager) IsThrottled() bool {
	tm.RLock()
	defer tm.RUnlock()

	if !tm.isThrottled {
		return false
	}

	// 检查是否已经恢复
	if time.Since(tm.throttleStartTime) >= tm.throttleDuration {
		// 时间已过，应该恢复
		return false
	}

	return true
}

// CheckRateLimit 检查是否达到限速
func (tm *ThrottleManager) CheckRateLimit() (bool, string) {
	// 先检查是否已经处于限流状态
	if tm.IsThrottled() {
		return true, "当前处于限流状态"
	}

	// 获取最新的统计数据
	stats := tm.stats.GetStats(24 * time.Hour)

	// 检查各个维度的限速
	if stats.QPSCount >= tm.rateLimit.QPSLimit {
		return true, "QPS超过限制"
	}
	if stats.QPMCount >= tm.rateLimit.QPMLimit {
		return true, "QPM超过限制"
	}
	if stats.QPHCount >= tm.rateLimit.QPHLimit {
		return true, "QPH超过限制"
	}
	if stats.QPTCount >= tm.rateLimit.QPTLimit {
		return true, "QPT超过限制"
	}

	return false, ""
}

// MarkThrottled 标记为限流状态，并启动恢复计时器
func (tm *ThrottleManager) MarkThrottled(reason string) {
	tm.Lock()
	defer tm.Unlock()

	if tm.isThrottled {
		// 已经在限流状态，不需要重复标记
		return
	}

	tm.isThrottled = true
	tm.throttleStartTime = time.Now()

	helpers.AppLogger.Warnf("百度网盘API达到限速: %s，将在 %v 秒后恢复", reason, tm.throttleDuration.Seconds())

	// 记录限流事件
	tm.stats.RecordThrottle(tm.throttleStartTime, tm.throttleDuration)

	// 启动恢复计时器
	go tm.startRecoveryTimer()
}

// startRecoveryTimer 启动恢复计时器
func (tm *ThrottleManager) startRecoveryTimer() {
	time.Sleep(tm.throttleDuration)

	tm.Lock()
	defer tm.Unlock()

	tm.isThrottled = false
	helpers.AppLogger.Infof("百度网盘API限流已恢复，继续处理请求")

	// 发送恢复通知
	select {
	case tm.throttleNotify <- struct{}{}:
	default:
		// 通道已满，不需要发送
	}
}

// WaitThrottleRecovery 等待限流恢复，如果当前不在限流状态则立即返回
func (tm *ThrottleManager) WaitThrottleRecovery(ctx context.Context) {
	for {
		if !tm.IsThrottled() {
			return
		}

		// 创建一个定时器来定期检查
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 继续检查
			if !tm.IsThrottled() {
				return
			}
		}
	}
}

// GetThrottleStatus 获取限流状态详情
func (tm *ThrottleManager) GetThrottleStatus() ThrottleStatus {
	tm.RLock()
	defer tm.RUnlock()

	status := ThrottleStatus{
		IsThrottled: tm.isThrottled,
		RateLimit:   *tm.rateLimit,
	}

	if tm.isThrottled {
		elapsed := time.Since(tm.throttleStartTime)
		status.ElapsedTime = elapsed
		status.RemainingTime = tm.throttleDuration - elapsed
		if status.RemainingTime < 0 {
			status.RemainingTime = 0
		}
	}

	// 获取当前速率统计
	stats := tm.stats.GetStats(24 * time.Hour)
	status.CurrentQPS = stats.QPSCount
	status.CurrentQPM = stats.QPMCount
	status.CurrentQPH = stats.QPHCount
	status.CurrentQPT = stats.QPTCount

	return status
}

// UpdateRateLimit 更新限速配置
func (tm *ThrottleManager) UpdateRateLimit(rateLimit *RateLimitConfig) {
	tm.Lock()
	defer tm.Unlock()
	
	tm.rateLimit = rateLimit
	helpers.AppLogger.Infof("百度网盘限速配置已更新: QPS=%d, QPM=%d, QPH=%d, QPT=%d", 
		rateLimit.QPSLimit, rateLimit.QPMLimit, rateLimit.QPHLimit, rateLimit.QPTLimit)
}

// ThrottleStatus 限流状态详情
type ThrottleStatus struct {
	IsThrottled    bool           // 是否处于限流状态
	ElapsedTime    time.Duration  // 已经限流的时长
	RemainingTime  time.Duration  // 剩余限流时长
	RateLimit      RateLimitConfig // 限速配置
	CurrentQPS     int64          // 当前QPS
	CurrentQPM     int64          // 当前QPM
	CurrentQPH     int64          // 当前QPH
	CurrentQPT     int64          // 当前QPT
}
