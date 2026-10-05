package openlist

import (
	"Q115-STRM/internal/helpers"
	"fmt"
	"net/http"
)

type TokenData struct {
	Token string `json:"token"`
}

// 获取开放平台token
// 用于自动登录开放平台
// POST /api/auth/login
func (c *Client) GetToken() (*TokenData, error) {
	type tokenReq struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	reqData := &tokenReq{
		Username: c.Username,
		Password: c.Password,
	}
	var result Resp[TokenData]
	req := c.client.R().SetBody(reqData).SetMethod(http.MethodPost).SetResult(&result)
	_, err := c.doRequest("/api/auth/login", req, MakeRequestConfig(0, 1, 5))
	if err != nil {
		helpers.OpenListLog.Errorf("开放平台获取访问凭证失败: %s", err.Error())
		return nil, err
	}
	tokenData := result.Data
	helpers.OpenListLog.Info("开放平台获取访问凭证成功")
	// 给客户端设置新的token
	c.SetAuthToken(tokenData.Token)
	// 通知models保存token到数据库
	helpers.PublishSync(helpers.SaveOpenListTokenEvent, map[string]any{
		"account_id": c.AccountId,
		"token":      tokenData.Token,
	})
	c.SetAuthToken(tokenData.Token)
	return &tokenData, nil
}

type UserInfoResp struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}
type RespWrapper struct {
	Code    int          `json:"code"`
	Message string       `json:"message"`
	Data    UserInfoResp `json:"data"`
}

// GetUserInfo 验证提供的 Token 是否有效
// 通过调用用户信息接口来验证 Token，并返回用户名和用户ID
func (c *Client) GetUserInfo(token string) (*UserInfoResp, error) {

	var result RespWrapper
	req := c.client.R().
		SetHeader("Authorization", token).
		SetMethod(http.MethodGet).
		SetResult(&result)
	_, err := c.doRequest("/api/me", req, MakeRequestConfig(0, 1, 5))
	if err != nil {
		helpers.OpenListLog.Errorf("验证 Token 失败: %s", err.Error())
		return nil, err
	}
	if result.Code != http.StatusOK {
		return nil, fmt.Errorf("Token 验证失败: %s", result.Message)
	}
	helpers.OpenListLog.Infof("Token 验证成功，用户: %s (ID: %d)", result.Data.Username, result.Data.ID)
	return &result.Data, nil
}
