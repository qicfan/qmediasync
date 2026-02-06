package baidupan

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"resty.dev/v3"
)

// QueuedRequest 队列中的请求
type QueuedRequest struct {
	// 请求的URL
	URL string
	// HTTP方法（GET/POST）
	Method string
	// Resty请求对象
	Request *resty.Request
	// 是否绕过速率限制
	BypassRateLimit bool
	// 响应数据接收通道
	ResponseChan chan *RequestResponse
	// 创建时间
	CreatedAt time.Time
	// 上下文
	Ctx context.Context
}

// RequestResponse 请求响应
type RequestResponse struct {
	Response  *resty.Response
	RespData  *RespBaseBool[json.RawMessage]
	RespBytes []byte
	Error     error
	// 响应耗时（毫秒）
	Duration int64
	// 是否是限流响应
	IsThrottled bool
}

// RequestStats 请求统计数据
type RequestStats struct {
	sync.RWMutex
	// 请求计数
	TotalRequests int64 // 总请求数
	// 限流相关
	ThrottledCount      int64         // 限流次数
	ThrottledStartTime  time.Time     // 最后一次限流开始时间
	ThrottledWaitTime   time.Duration // 本次限流等待时间
	LastThrottleTime    time.Time     // 最后一次触发限流的时间
	ThrottleRecoverTime time.Time     // 限流恢复时间
	// 响应时间统计
	ResponseTimes []int64 // 最近响应时间记录（毫秒）
	MaxRecords    int     // 最多保留记录数
	// 时间窗口统计
	RequestLog []RequestLogEntry // 请求日志，用于统计qps/qpm/qph/qpt
}

// RequestLogEntry 请求日志条目
type RequestLogEntry struct {
	Timestamp   time.Time // 请求时间
	Duration    int64     // 响应时间（毫秒）
	IsThrottled bool      // 是否限流
	URL         string    // 请求URL
	Method      string    // 请求方法
}

// NewRequestStats 创建新的请求统计
func NewRequestStats(maxRecords int) *RequestStats {
	if maxRecords <= 0 {
		maxRecords = 10000
	}
	return &RequestStats{
		ResponseTimes: make([]int64, 0, maxRecords),
		MaxRecords:    maxRecords,
		RequestLog:    make([]RequestLogEntry, 0, maxRecords),
	}
}

// RecordRequest 记录一个请求
func (s *RequestStats) RecordRequest(entry RequestLogEntry) {
	s.Lock()
	defer s.Unlock()

	s.TotalRequests++

	// 记录响应时间
	if len(s.ResponseTimes) >= s.MaxRecords {
		// 移除最老的响应时间
		s.ResponseTimes = s.ResponseTimes[1:]
	}
	s.ResponseTimes = append(s.ResponseTimes, entry.Duration)

	// 记录请求日志
	if len(s.RequestLog) >= s.MaxRecords {
		// 移除最老的日志条目
		s.RequestLog = s.RequestLog[1:]
	}
	s.RequestLog = append(s.RequestLog, entry)

	// 记录限流统计
	if entry.IsThrottled {
		s.ThrottledCount++
		s.LastThrottleTime = entry.Timestamp
	}
}

// RecordThrottle 记录限流事件
func (s *RequestStats) RecordThrottle(startTime time.Time, waitTime time.Duration) {
	s.Lock()
	defer s.Unlock()

	s.ThrottledStartTime = startTime
	s.ThrottledWaitTime = waitTime
	s.ThrottleRecoverTime = time.Now()
}

// GetStats 获取统计数据
func (s *RequestStats) GetStats(duration time.Duration) *StatsSnapshot {
	s.RLock()
	defer s.RUnlock()

	now := time.Now()
	cutoff := now.Add(-duration)

	var (
		qpsCount         int64 // 最近1秒的请求数
		qpmCount         int64 // 最近1分钟的请求数
		qphCount         int64 // 最近1小时的请求数
		qptCount         int64 // 最近24小时的请求数
		windowCount      int64 // 指定时间窗口内的请求数
		throttledCount   int64 // 限流请求数
		totalDuration    int64 // 总耗时
		avgResponseTime  int64 // 平均响应时间
		lastThrottleTime *time.Time
	)

	// 遍历请求日志，统计
	for _, entry := range s.RequestLog {
		// 统计指定时间窗口内的请求
		if entry.Timestamp.After(cutoff) {
			windowCount++
			totalDuration += entry.Duration
			if entry.IsThrottled {
				throttledCount++
			}
		}

		// 分别统计不同时间窗口
		// 最近1秒
		if entry.Timestamp.After(now.Add(-time.Second)) {
			qpsCount++
		}
		// 最近1分钟
		if entry.Timestamp.After(now.Add(-time.Minute)) {
			qpmCount++
		}
		// 最近1小时
		if entry.Timestamp.After(now.Add(-time.Hour)) {
			qphCount++
		}
		// 最近24小时
		if entry.Timestamp.After(now.Add(-24 * time.Hour)) {
			qptCount++
		}
	}

	// 计算平均响应时间
	if windowCount > 0 {
		avgResponseTime = totalDuration / windowCount
	}

	// 获取最后一次限流时间
	if !s.LastThrottleTime.IsZero() {
		lastThrottleTime = &s.LastThrottleTime
	}

	return &StatsSnapshot{
		TotalRequests:       s.TotalRequests,
		QPSCount:            qpsCount,
		QPMCount:            qpmCount,
		QPHCount:            qphCount,
		QPTCount:            qptCount,
		ThrottledCount:      throttledCount,
		AvgResponseTime:     avgResponseTime,
		LastThrottleTime:    lastThrottleTime,
		ThrottledWaitTime:   s.ThrottledWaitTime,
		ThrottleRecoverTime: &s.ThrottleRecoverTime,
	}
}

// StatsSnapshot 统计数据快照
type StatsSnapshot struct {
	TotalRequests       int64         // 总请求数
	QPSCount            int64         // 最近1秒请求数
	QPMCount            int64         // 最近1分钟请求数
	QPHCount            int64         // 最近1小时请求数
	QPTCount            int64         // 最近24小时请求数
	ThrottledCount      int64         // 限流总次数
	AvgResponseTime     int64         // 平均响应时间（毫秒）
	LastThrottleTime    *time.Time    // 最后一次限流时间
	ThrottledWaitTime   time.Duration // 本次限流等待时间
	ThrottleRecoverTime *time.Time    // 限流恢复时间
}

// RequestQueue 请求队列
type RequestQueue struct {
	sync.Mutex
	queue []*QueuedRequest
}

// NewRequestQueue 创建新的请求队列
func NewRequestQueue() *RequestQueue {
	return &RequestQueue{
		queue: make([]*QueuedRequest, 0),
	}
}

// Enqueue 入队
func (q *RequestQueue) Enqueue(req *QueuedRequest) {
	q.Lock()
	defer q.Unlock()
	q.queue = append(q.queue, req)
}

// Dequeue 出队
func (q *RequestQueue) Dequeue() *QueuedRequest {
	q.Lock()
	defer q.Unlock()
	if len(q.queue) == 0 {
		return nil
	}
	req := q.queue[0]
	q.queue = q.queue[1:]
	return req
}

// Length 获取队列长度
func (q *RequestQueue) Length() int {
	q.Lock()
	defer q.Unlock()
	return len(q.queue)
}
