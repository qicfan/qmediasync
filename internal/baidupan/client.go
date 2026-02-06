package baidupan

import (
	"Q115-STRM/internal/helpers"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"resty.dev/v3"
)

// BaiDuPanClient 百度网盘客户端
type BaiDuPanClient struct {
	AccountId       uint // 账号ID
	client          *resty.Client
	AccessToken     string // 访问令牌
	RefreshTokenStr string // 刷新令牌
	throttleManager *ThrottleManager
	stats           *RequestStats
}

// 全局HTTP客户端实例
var cachedClients map[string]*BaiDuPanClient = make(map[string]*BaiDuPanClient, 0)
var cachedClientsMutex sync.RWMutex

// 全局请求队列执行器
var globalExecutor *RequestQueueExecutor
var executorOnce sync.Once

// RequestQueueExecutor 请求队列执行器
type RequestQueueExecutor struct {
	queue           *RequestQueue
	throttleManager *ThrottleManager
	stats           *RequestStats
	wg              sync.WaitGroup
	ctx             context.Context
	cancel          context.CancelFunc
}

// GetGlobalExecutor 获取全局请求队列执行器
func GetGlobalExecutor() *RequestQueueExecutor {
	executorOnce.Do(func() {
		stats := NewRequestStats(10000)
		// 使用默认限速配置，避免与models模块的循环引用
		rateLimit := &RateLimitConfig{
			QPSLimit: int64(10),     // 默认值
			QPMLimit: int64(600),    // 默认值
			QPHLimit: int64(36000),  // 默认值
			QPTLimit: int64(864000), // 默认值
		}

		throttleManager := NewThrottleManager(rateLimit, stats)
		ctx, cancel := context.WithCancel(context.Background())
		globalExecutor = &RequestQueueExecutor{
			queue:           NewRequestQueue(),
			throttleManager: throttleManager,
			stats:           stats,
			ctx:             ctx,
			cancel:          cancel,
		}
		globalExecutor.start()
	})
	return globalExecutor
}

// start 启动请求队列执行器
func (e *RequestQueueExecutor) start() {
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		for {
			select {
			case <-e.ctx.Done():
				return
			default:
				req := e.queue.Dequeue()
				if req == nil {
					time.Sleep(10 * time.Millisecond)
					continue
				}
				e.processRequest(req)
			}
		}
	}()
}

// EnqueueRequest 入队请求
func (e *RequestQueueExecutor) EnqueueRequest(req *QueuedRequest) {
	e.queue.Enqueue(req)
}

// processRequest 处理请求
func (e *RequestQueueExecutor) processRequest(req *QueuedRequest) {
	startTime := time.Now()

	// 检查限速（除非绕过）
	if !req.BypassRateLimit {
		isThrottled, reason := e.throttleManager.CheckRateLimit()
		if isThrottled {
			e.throttleManager.MarkThrottled(reason)
			resp := &RequestResponse{
				Error:       fmt.Errorf("百度网盘API限速: %s", reason),
				IsThrottled: true,
				Duration:    time.Since(startTime).Milliseconds(),
			}
			// 记录限流请求
			e.stats.RecordRequest(RequestLogEntry{
				Timestamp:   startTime,
				Duration:    resp.Duration,
				IsThrottled: true,
				URL:         req.URL,
				Method:      req.Method,
			})
			req.ResponseChan <- resp
			return
		}
	}

	// 执行请求
	var response *resty.Response
	var err error
	var respData *RespBaseBool[json.RawMessage]
	var respBytes []byte

	switch req.Method {
	case "GET":
		response, err = req.Request.Get(req.URL)
	case "POST":
		response, err = req.Request.Post(req.URL)
	default:
		err = fmt.Errorf("unsupported HTTP method: %s", req.Method)
	}

	// 处理响应
	if err == nil && response != nil {
		// 解析响应
		defer response.Body.Close()
		resBytes, ioErr := io.ReadAll(response.Body)
		if ioErr != nil {
			helpers.AppLogger.Errorf("读取响应失败: %s", ioErr.Error())
		} else {
			respBytes = resBytes
			respData = &RespBaseBool[json.RawMessage]{}
			if jsonErr := json.Unmarshal(respBytes, respData); jsonErr != nil {
				helpers.AppLogger.Errorf("解析响应失败: %s", jsonErr.Error())
			}
		}
	}

	// 检查是否是限流响应
	isThrottled := false
	if respData != nil && respData.Code == REQUEST_MAX_LIMIT_CODE {
		isThrottled = true
		e.throttleManager.MarkThrottled("API返回限流错误")
	}

	// 记录请求
	duration := time.Since(startTime).Milliseconds()
	e.stats.RecordRequest(RequestLogEntry{
		Timestamp:   startTime,
		Duration:    duration,
		IsThrottled: isThrottled,
		URL:         req.URL,
		Method:      req.Method,
	})

	// 返回响应
	resp := &RequestResponse{
		Response:    response,
		RespData:    respData,
		RespBytes:   respBytes,
		Error:       err,
		Duration:    duration,
		IsThrottled: isThrottled,
	}
	req.ResponseChan <- resp
}

// UpdateToken 更新令牌
func UpdateToken(accountId uint, token string, refreshToken string) {
	for key, client := range cachedClients {
		if client.AccountId == accountId {
			client.SetAuthToken(token, refreshToken)
			helpers.AppLogger.Infof("更新百度网盘客户端 %s 的token成功", key)
		}
	}
}

// GetClient 获取百度网盘客户端
func GetClient(accountId uint, token string, refreshToken string) *BaiDuPanClient {
	cachedClientsMutex.RLock()
	defer cachedClientsMutex.RUnlock()
	clientKey := fmt.Sprintf("%d", accountId)
	if client, exists := cachedClients[clientKey]; exists {
		client.SetAuthToken(token, refreshToken)
		return client
	}

	client := resty.New()
	stats := NewRequestStats(10000)
	rateLimit := DefaultRateLimitConfig()
	throttleManager := NewThrottleManager(rateLimit, stats)
	baiduClient := &BaiDuPanClient{
		client:          client,
		AccountId:       accountId,
		throttleManager: throttleManager,
		stats:           stats,
	}
	baiduClient.SetAuthToken(token, refreshToken)
	cachedClients[clientKey] = baiduClient
	return baiduClient
}

// SetAuthToken 设置认证令牌
func (c *BaiDuPanClient) SetAuthToken(token string, refreshToken string) {
	c.AccessToken = token
	c.RefreshTokenStr = refreshToken
}

// request 执行HTTP请求的核心方法
func (c *BaiDuPanClient) request(url string, req *resty.Request) (*resty.Response, *RespBase[json.RawMessage], error) {
	// 检查限速
	isThrottled, reason := c.throttleManager.CheckRateLimit()
	if isThrottled {
		c.throttleManager.MarkThrottled(reason)
		return nil, nil, fmt.Errorf("百度网盘API限速: %s", reason)
	}

	req.SetResult(&RespBase[json.RawMessage]{}).SetForceResponseContentType("application/json")
	var response *resty.Response
	var err error
	method := req.Method
	switch method {
	case "GET":
		response, err = req.Get(url)
	case "POST":
		response, err = req.Post(url)
	default:
		return nil, nil, fmt.Errorf("unsupported HTTP method: %s", method)
	}
	result := response.Result()
	resp := result.(*RespBase[json.RawMessage])
	if err != nil {
		return response, resp, err
	}
	helpers.AppLogger.Infof("非认证访问 %s %s\nstate=%d, code=%d, msg=%s, data=%s\n", req.Method, req.URL, resp.State, resp.Code, resp.Message, string(resp.Data))
	switch resp.Code {
	case REFRESH_TOKEN_INVALID:
		return response, nil, fmt.Errorf("token invalid")
	case REQUEST_MAX_LIMIT_CODE:
		// 访问频率过高
		c.throttleManager.MarkThrottled("API返回限流错误")
		return response, nil, fmt.Errorf("访问频率过高")
	}

	return response, resp, nil
}

// doRequest 带重试的请求方法（使用全局队列）
func (c *BaiDuPanClient) doRequest(url string, req *resty.Request, options *RequestConfig) (*resty.Response, *RespBase[json.RawMessage], error) {
	// 检查限速
	if !options.BypassRateLimit {
		isThrottled, reason := c.throttleManager.CheckRateLimit()
		if isThrottled {
			c.throttleManager.MarkThrottled(reason)
			return nil, nil, fmt.Errorf("百度网盘API限速: %s", reason)
		}
	}

	// 设置超时时间
	req.SetTimeout(options.Timeout)
	// 设置默认头
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	}

	var lastErr error
	for attempt := 0; attempt <= options.MaxRetries; attempt++ {
		// 使用全局队列执行器处理请求
		executor := GetGlobalExecutor()
		respChan := make(chan *RequestResponse, 1)

		queuedReq := &QueuedRequest{
			URL:             url,
			Method:          req.Method,
			Request:         req,
			BypassRateLimit: options.BypassRateLimit,
			ResponseChan:    respChan,
			CreatedAt:       time.Now(),
			Ctx:             context.Background(),
		}

		// 将请求加入队列
		executor.EnqueueRequest(queuedReq)

		// 等待响应
		queueResp := <-respChan

		if queueResp.Error == nil && queueResp.RespData != nil {
			// 请求成功，转换为RespBase格式
			respBase := &RespBase[json.RawMessage]{
				State:   0,
				Code:    queueResp.RespData.Code,
				Message: queueResp.RespData.Message,
				Data:    queueResp.RespData.Data,
			}
			if queueResp.RespData.State {
				respBase.State = 1
			}
			return queueResp.Response, respBase, nil
		}

		lastErr = queueResp.Error

		// Token相关错误不重试
		if queueResp.RespData != nil {
			switch queueResp.RespData.Code {
			case REFRESH_TOKEN_INVALID:
				// 转换为RespBase格式返回
				respBase := &RespBase[json.RawMessage]{
					State:   0,
					Code:    queueResp.RespData.Code,
					Message: queueResp.RespData.Message,
					Data:    queueResp.RespData.Data,
				}
				if queueResp.RespData.State {
					respBase.State = 1
				}
				return queueResp.Response, respBase, lastErr
			}
		}

		// 如果是限流错误，不重试
		if queueResp.IsThrottled {
			helpers.AppLogger.Warn("检测到限流，停止重试")
			if queueResp.RespData != nil {
				respBase := &RespBase[json.RawMessage]{
					State:   0,
					Code:    queueResp.RespData.Code,
					Message: queueResp.RespData.Message,
					Data:    queueResp.RespData.Data,
				}
				if queueResp.RespData.State {
					respBase.State = 1
				}
				return queueResp.Response, respBase, lastErr
			}
			return queueResp.Response, nil, lastErr
		}

		// 其他错误开始重试
		if attempt < options.MaxRetries && lastErr != nil {
			helpers.AppLogger.Warnf("%s %s 请求失败:%+v", req.Method, url, lastErr)
			helpers.AppLogger.Warnf("%s %s 请求失败，%+v秒后重试 (第%d次尝试)", req.Method, url, options.RetryDelay.Seconds(), attempt+1)
			time.Sleep(options.RetryDelay)
		}
	}
	return nil, nil, lastErr
}

// authRequest 执行HTTP请求的核心方法
func (c *BaiDuPanClient) authRequest(ctx context.Context, url string, req *resty.Request, respData any, options *RequestConfig) (*resty.Response, []byte, error) {
	// 检查限速
	if !options.BypassRateLimit {
		isThrottled, reason := c.throttleManager.CheckRateLimit()
		if isThrottled {
			c.throttleManager.MarkThrottled(reason)
			return nil, nil, fmt.Errorf("百度网盘API限速: %s", reason)
		}
	}

	helpers.AppLogger.Debugf("执行认证请求: %s %s", req.Method, url)
	req.SetForceResponseContentType("application/json")
	var response *resty.Response
	var err error
	method := req.Method
	req.SetContext(ctx)
	req.SetAuthToken(c.AccessToken).SetDoNotParseResponse(true)
	switch method {
	case "GET":
		response, err = req.Get(url)
	case "POST":
		response, err = req.Post(url)
	default:
		return nil, nil, fmt.Errorf("unsupported HTTP method: %s", method)
	}
	helpers.AppLogger.Debugf("完成认证请求: %s %s", req.Method, url)
	if err != nil {
		return response, nil, err
	}
	defer response.Body.Close() // ensure to close response body
	resBytes, ioErr := io.ReadAll(response.Body)
	if ioErr != nil {
		fmt.Println(ioErr)
		return response, nil, ioErr
	}
	resp := &RespBaseBool[json.RawMessage]{}

	bodyErr := json.Unmarshal(resBytes, resp)
	if bodyErr != nil {
		// 尝试用RespBase[json.RawMessage]解析
		respBase := &RespBase[json.RawMessage]{}
		bodyErr = json.Unmarshal(resBytes, respBase)
		if bodyErr != nil {
			helpers.AppLogger.Errorf("解析响应失败: %s", bodyErr.Error())
			return response, resBytes, bodyErr
		}
		// 重新赋值状态码、错误码、错误信息、数据
		resp.Code = respBase.Code
		resp.Message = respBase.Message
		resp.Data = respBase.Data
	}
	helpers.AppLogger.Infof("认证访问 %s %s\nstate=%v, code=%d, msg=%s, data=%s\n", req.Method, req.URL, resp.State, resp.Code, resp.Message, string(resp.Data))
	switch resp.Code {
	case ACCESS_TOKEN_AUTH_FAIL:
		helpers.AppLogger.Warn("访问凭证已过期1")
		return response, nil, fmt.Errorf("token expired")
	case ACCESS_AUTH_INVALID:
		helpers.AppLogger.Warn("访问凭证已过期2")
		return response, nil, fmt.Errorf("token expired")
	case ACCESS_TOKEN_EXPIRY_CODE:
		helpers.AppLogger.Warn("访问凭证已过期3")
		return response, nil, fmt.Errorf("token expired")
	case REFRESH_TOKEN_INVALID:
		// 不需要重试，直接返回
		helpers.AppLogger.Error("访问凭证无效，请重新登录")
		return response, nil, fmt.Errorf("token expired")
	case REQUEST_MAX_LIMIT_CODE:
		c.throttleManager.MarkThrottled("API返回限流错误")
		return response, nil, fmt.Errorf("访问频率过高")
	}
	if respData != nil && resp.State {
		// 解包
		if unmarshalErr := json.Unmarshal(resp.Data, respData); unmarshalErr != nil {
			respData = nil
			helpers.AppLogger.Errorf("解包响应数据失败: %s", unmarshalErr.Error())
			return response, resBytes, nil
		}
	}
	if resp.Code != 0 {
		return response, resBytes, fmt.Errorf("错误码：%d，错误信息：%s", resp.Code, resp.Message)
	}
	return response, resBytes, nil
}

// doAuthRequest 带重试的认证请求方法（使用全局队列）
func (c *BaiDuPanClient) doAuthRequest(ctx context.Context, url string, req *resty.Request, options *RequestConfig, respData any) (*resty.Response, []byte, error) {
	// 检查限速
	if !options.BypassRateLimit {
		isThrottled, reason := c.throttleManager.CheckRateLimit()
		if isThrottled {
			c.throttleManager.MarkThrottled(reason)
			return nil, nil, fmt.Errorf("百度网盘API限速: %s", reason)
		}
	}

	if c.AccessToken == "" {
		// 没有token，直接报错
		return nil, nil, fmt.Errorf("百度网盘账号授权失效，请在网盘账号管理中重新授权")
	}
	req.SetTimeout(options.Timeout)
	// 设置默认头
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	}
	req.SetAuthToken(c.AccessToken).SetDoNotParseResponse(true)

	var lastErr error
	for attempt := 0; attempt <= options.MaxRetries; attempt++ {
		// 使用全局队列执行器处理请求
		executor := GetGlobalExecutor()
		respChan := make(chan *RequestResponse, 1)

		queuedReq := &QueuedRequest{
			URL:             url,
			Method:          req.Method,
			Request:         req,
			BypassRateLimit: options.BypassRateLimit,
			ResponseChan:    respChan,
			CreatedAt:       time.Now(),
			Ctx:             ctx,
		}

		// 将请求加入队列
		executor.EnqueueRequest(queuedReq)

		// 等待响应
		queueResp := <-respChan

		if queueResp.Error == nil && queueResp.RespData != nil {
			// 请求成功
			if respData != nil && queueResp.RespData.State {
				// 解包响应数据
				if unmarshalErr := json.Unmarshal(queueResp.RespData.Data, respData); unmarshalErr != nil {
					helpers.AppLogger.Errorf("解包响应数据失败: %s", unmarshalErr.Error())
					return queueResp.Response, queueResp.RespBytes, nil
				}
			}
			return queueResp.Response, queueResp.RespBytes, nil
		}

		lastErr = queueResp.Error

		// Token相关错误处理
		if queueResp.RespData != nil {
			switch queueResp.RespData.Code {
			case ACCESS_TOKEN_AUTH_FAIL, ACCESS_AUTH_INVALID, ACCESS_TOKEN_EXPIRY_CODE:
				helpers.AppLogger.Errorf("访问凭证过期，等待自动刷新后下次重试")
				lastErr = fmt.Errorf("访问凭证（Token）过期")
			case REFRESH_TOKEN_INVALID:
				lastErr = fmt.Errorf("访问凭证（Token）无效，请重新登录")
				return queueResp.Response, queueResp.RespBytes, lastErr
			}
		}

		// 如果是限流错误，不重试
		if queueResp.IsThrottled {
			helpers.AppLogger.Warn("检测到限流，停止重试")
			return queueResp.Response, queueResp.RespBytes, lastErr
		}

		// 其他错误开始重试
		if attempt < options.MaxRetries && lastErr != nil {
			helpers.AppLogger.Warnf("%s %s 请求失败:%+v", req.Method, url, lastErr)
			helpers.AppLogger.Warnf("%s %s 请求失败，%+v秒后重试 (第%d次尝试)", req.Method, url, options.RetryDelay.Seconds(), attempt+1)
			time.Sleep(options.RetryDelay)
		}
	}
	return nil, nil, lastErr
}

// GetStats 获取客户端统计数据
func (c *BaiDuPanClient) GetStats() *StatsSnapshot {
	return c.stats.GetStats(24 * time.Hour)
}

// GetThrottleStatus 获取限流状态
func (c *BaiDuPanClient) GetThrottleStatus() ThrottleStatus {
	return c.throttleManager.GetThrottleStatus()
}

// SetGlobalExecutorConfig 设置全局执行器的速率限制配置
func SetGlobalExecutorConfig(qps, qpm, qph, qpt int) {
	executor := GetGlobalExecutor()
	// 更新限速配置
	rateLimit := &RateLimitConfig{
		QPSLimit: int64(qps),
		QPMLimit: int64(qpm),
		QPHLimit: int64(qph),
		QPTLimit: int64(qpt),
	}
	// 更新ThrottleManager的限速配置
	executor.throttleManager.UpdateRateLimit(rateLimit)
	helpers.AppLogger.Infof("百度网盘限速配置已更新: QPS=%d, QPM=%d, QPH=%d, QPT=%d", qps, qpm, qph, qpt)
}

// GetStats 获取统计数据
func (e *RequestQueueExecutor) GetStats(duration time.Duration) *StatsSnapshot {
	return e.stats.GetStats(duration)
}

// GetThrottleStatus 获取限流状态
func (e *RequestQueueExecutor) GetThrottleStatus() ThrottleStatus {
	return e.throttleManager.GetThrottleStatus()
}
