package baidupan

import (
	"Q115-STRM/internal/helpers"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"resty.dev/v3"
)

// AuthResponse 认证响应
type AuthResponse struct {
	AccessToken   string `json:"access_token"`
	ExpiresIn     int    `json:"expires_in"`
	RefreshToken  string `json:"refresh_token"`
	Scope         string `json:"scope"`
	SessionKey    string `json:"session_key"`
	SessionSecret string `json:"session_secret"`
}

// GetAccessToken 获取访问令牌
func GetAccessToken(ctx context.Context, appId, appSecret, code string) (*AuthResponse, error) {
	url := fmt.Sprintf("%s/oauth/2.0/token", API_BASE_URL)

	req := resty.New().R()
	req.SetFormData(map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"client_id":     appId,
		"client_secret": appSecret,
		"redirect_uri":  "oob",
	})

	resp, err := req.Post(url)
	if err != nil {
		return nil, fmt.Errorf("获取访问令牌失败: %v", err)
	}

	// 解析响应
	defer resp.Body.Close()
	resBytes, ioErr := io.ReadAll(resp.Body)
	if ioErr != nil {
		return nil, fmt.Errorf("读取响应失败: %v", ioErr)
	}

	var authResp AuthResponse
	if err := json.Unmarshal(resBytes, &authResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	return &authResp, nil
}

// RefreshAccessToken 刷新访问令牌
func RefreshAccessToken(ctx context.Context, refreshToken string) (*AuthResponse, error) {
	// TODO 调用授权服务器来刷新token,避免暴露client_id和client_secret

	// req := resty.New().R()
	// req.SetFormData(map[string]string{
	// 	"grant_type":    "refresh_token",
	// 	"refresh_token": refreshToken,
	// })

	// resp, err := req.Post(url)
	// if err != nil {
	// 	return nil, fmt.Errorf("刷新访问令牌失败: %v", err)
	// }

	// // 解析响应
	// defer resp.Body.Close()
	// resBytes, ioErr := io.ReadAll(resp.Body)
	// if ioErr != nil {
	// 	return nil, fmt.Errorf("读取响应失败: %v", ioErr)
	// }

	// var authResp AuthResponse
	// if err := json.Unmarshal(resBytes, &authResp); err != nil {
	// 	return nil, fmt.Errorf("解析响应失败: %v", err)
	// }

	// return &authResp, nil
	return nil, nil
}

// ValidateToken 验证访问令牌
func ValidateToken(ctx context.Context, accessToken string) (bool, error) {
	url := fmt.Sprintf("%s/oauth/2.0/tokeninfo", API_BASE_URL)

	req := resty.New().R()
	req.SetQueryParam("access_token", accessToken)

	resp, err := req.Get(url)
	if err != nil {
		return false, fmt.Errorf("验证访问令牌失败: %v", err)
	}

	// 解析响应
	defer resp.Body.Close()
	resBytes, ioErr := io.ReadAll(resp.Body)
	if ioErr != nil {
		return false, fmt.Errorf("读取响应失败: %v", ioErr)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resBytes, &result); err != nil {
		return false, fmt.Errorf("解析响应失败: %v", err)
	}

	// 检查是否有错误
	if _, ok := result["error"]; ok {
		return false, fmt.Errorf("访问令牌无效: %v", result["error_description"])
	}

	return true, nil
}

// RefreshTokenIfExpired 检查并刷新过期的令牌
func (c *BaiDuPanClient) RefreshTokenIfExpired(ctx context.Context) error {
	// 先验证令牌是否有效
	valid, err := ValidateToken(ctx, c.AccessToken)
	if err != nil {
		helpers.AppLogger.Warnf("验证令牌失败: %v，尝试刷新", err)
	} else if valid {
		// 令牌有效，不需要刷新
		return nil
	}

	// 令牌无效，尝试刷新
	authResp, err := RefreshAccessToken(ctx, c.RefreshTokenStr)
	if err != nil {
		return fmt.Errorf("刷新令牌失败: %v", err)
	}

	// 更新令牌
	c.SetAuthToken(authResp.AccessToken, authResp.RefreshToken)
	helpers.AppLogger.Infof("令牌刷新成功，新令牌有效期: %d秒", authResp.ExpiresIn)

	return nil
}
