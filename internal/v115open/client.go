package v115open

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

// OpenClient HTTP客户端
type OpenClient struct {
	AppId           string // 应用ID
	AccountId       uint   // 账号ID
	client          *resty.Client
	AccessToken     string // 访问令牌
	RefreshTokenStr string // 刷新令牌
}

// 全局HTTP客户端实例
var cachedClients map[string]*OpenClient = make(map[string]*OpenClient, 0)
var cachedClientsMutex sync.RWMutex

func UpdateToken(accountId uint, token string, refreshToken string) {
	for key, client := range cachedClients {
		if client.AccountId == accountId {
			client.SetAuthToken(token, refreshToken)
			helpers.AppLogger.Infof("更新115客户端 %s 的token成功", key)
		}
	}
}

// NewHttpClient 创建新的HTTP客户端
func GetClient(accountId uint, appId string, token string, refreshToken string) *OpenClient {
	cachedClientsMutex.RLock()
	defer cachedClientsMutex.RUnlock()
	clientKey := fmt.Sprintf("%d", accountId)
	if client, exists := cachedClients[clientKey]; exists {
		client.SetAuthToken(token, refreshToken)
		return client
	}

	client := resty.New()
	openClient := &OpenClient{
		client:    client,
		AppId:     appId,
		AccountId: accountId,
	}
	openClient.SetAuthToken(token, refreshToken)
	cachedClients[clientKey] = openClient
	return openClient
}

// SetAuthToken 设置认证令牌
func (c *OpenClient) SetAuthToken(token string, refreshToken string) {
	c.AccessToken = token
	c.RefreshTokenStr = refreshToken
}

// request 执行HTTP请求的核心方法
func (c *OpenClient) request(url string, req *resty.Request) (*resty.Response, *RespBase[json.RawMessage], error) {
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
	helpers.V115Log.Infof("非认证访问 %s %s\nstate=%d, code=%d, msg=%s, data=%s\n", req.Method, req.URL, resp.State, resp.Code, resp.Message, string(resp.Data))
	switch resp.Code {
	case REFRESH_TOKEN_INVALID:
		return response, nil, fmt.Errorf("token invalid")
	case REQUEST_MAX_LIMIT_CODE:
		// 访问频率过高
		return response, nil, fmt.Errorf("访问频率过高")
	}

	return response, resp, nil
}

// doRequest 带重试的请求方法（使用全局队列）
func (c *OpenClient) doRequest(url string, req *resty.Request, options *RequestConfig) (*resty.Response, *RespBase[json.RawMessage], error) {
	// 设置超时时间
	req.SetTimeout(options.Timeout)
	// 设置默认头
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", DEFAULTUA)
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
			helpers.V115Log.Warn("检测到限流，停止重试")
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
			helpers.V115Log.Warnf("%s %s 请求失败:%+v", req.Method, url, lastErr)
			helpers.V115Log.Warnf("%s %s 请求失败，%+v秒后重试 (第%d次尝试)", req.Method, url, options.RetryDelay.Seconds(), attempt+1)
			time.Sleep(options.RetryDelay)
		}
	}
	return nil, nil, lastErr
}

// request 执行HTTP请求的核心方法
func (c *OpenClient) authRequest(ctx context.Context, url string, req *resty.Request, respData any, options *RequestConfig) (*resty.Response, []byte, error) {
	helpers.V115Log.Debugf("执行认证请求: %s %s", req.Method, url)
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
	helpers.V115Log.Debugf("完成认证请求: %s %s", req.Method, url)
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
			helpers.V115Log.Errorf("解析响应失败: %s", bodyErr.Error())
			return response, resBytes, bodyErr
		}
		// 重新赋值状态码、错误码、错误信息、数据
		resp.Code = respBase.Code
		resp.Message = respBase.Message
		resp.Data = respBase.Data
	}
	helpers.V115Log.Infof("认证访问 %s %s\nstate=%v, code=%d, msg=%s, data=%s\n", req.Method, req.URL, resp.State, resp.Code, resp.Message, string(resp.Data))
	switch resp.Code {
	case ACCESS_TOKEN_AUTH_FAIL:
		helpers.V115Log.Warn("访问凭证已过期1")
		return response, nil, fmt.Errorf("token expired")
	case ACCESS_AUTH_INVALID:
		helpers.V115Log.Warn("访问凭证已过期2")
		return response, nil, fmt.Errorf("token expired")
	case ACCESS_TOKEN_EXPIRY_CODE:
		helpers.V115Log.Warn("访问凭证已过期3")
		return response, nil, fmt.Errorf("token expired")
	case REFRESH_TOKEN_INVALID:
		// 不需要重试，直接返回
		helpers.V115Log.Error("访问凭证无效，请重新登录")
		return response, nil, fmt.Errorf("token expired")
	case REQUEST_MAX_LIMIT_CODE:
		return response, nil, fmt.Errorf("访问频率过高")
	}
	if respData != nil && resp.State {
		// 解包
		if unmarshalErr := json.Unmarshal(resp.Data, respData); unmarshalErr != nil {
			respData = nil
			helpers.V115Log.Errorf("解包响应数据失败: %s", unmarshalErr.Error())
			return response, resBytes, nil
		}
	}
	if resp.Code != 0 {
		return response, resBytes, fmt.Errorf("错误码：%d，错误信息：%s", resp.Code, resp.Message)
	}
	return response, resBytes, nil
}

// doAuthRequest 带重试的认证请求方法（使用全局队列）
func (c *OpenClient) doAuthRequest(ctx context.Context, url string, req *resty.Request, options *RequestConfig, respData any) (*resty.Response, []byte, error) {
	if c.AccessToken == "" {
		// 没有token，直接报错
		return nil, nil, fmt.Errorf("115账号授权失效，请在网盘账号管理中重新授权")
	}
	req.SetTimeout(options.Timeout)
	// 设置默认头
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", DEFAULTUA)
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
					helpers.V115Log.Errorf("解包响应数据失败: %s", unmarshalErr.Error())
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
				helpers.V115Log.Errorf("访问凭证过期，等待自动刷新后下次重试")
				lastErr = fmt.Errorf("访问凭证（Token）过期")
			case REFRESH_TOKEN_INVALID:
				lastErr = fmt.Errorf("访问凭证（Token）无效，请重新登录")
				return queueResp.Response, queueResp.RespBytes, lastErr
			}
		}

		// 如果是限流错误，不重试
		if queueResp.IsThrottled {
			helpers.V115Log.Warn("检测到限流，停止重试")
			return queueResp.Response, queueResp.RespBytes, lastErr
		}

		// 其他错误开始重试
		if attempt < options.MaxRetries && lastErr != nil {
			helpers.V115Log.Warnf("%s %s 请求失败:%+v", req.Method, url, lastErr)
			helpers.V115Log.Warnf("%s %s 请求失败，%+v秒后重试 (第%d次尝试)", req.Method, url, options.RetryDelay.Seconds(), attempt+1)
			time.Sleep(options.RetryDelay)
		}
	}
	return nil, nil, lastErr
}
