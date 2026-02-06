package controllers

import (
	"Q115-STRM/internal/baidupan"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// BaiDuPanStatusResp 百度网盘状态响应
type BaiDuPanStatusResp struct {
	LoggedIn    bool        `json:"logged_in"`
	UserId      json.Number `json:"user_id"`
	Username    string      `json:"username"`
	UsedSpace   int64       `json:"used_space"`
	TotalSpace  int64       `json:"total_space"`
	MemberLevel string      `json:"member_level"`
	ExpireTime  string      `json:"expire_time"`
}

// GetBaiDuPanStatus 查询百度网盘账号状态
// @Summary 查询百度网盘账号状态
// @Description 获取指定百度网盘账号的登录状态及存储信息
// @Tags 百度网盘
// @Accept json
// @Produce json
// @Param account_id query integer true "账号ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /auth/baidupan-status [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetBaiDuPanStatus(c *gin.Context) {
	type statusReq struct {
		AccountId uint `json:"account_id" form:"account_id"`
	}
	var req statusReq
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "参数错误", Data: nil})
		return
	}
	_, err := models.GetAccountById(req.AccountId)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "账号ID不存在", Data: nil})
		return
	}
	// TODO: 实现百度网盘客户端的UserInfo方法
	// client := account.GetBaiDuPanClient()
	// var resp BaiDuPanStatusResp
	// // 获取用户信息
	// userInfo, err := client.UserInfo()
	// if err != nil {
	// 	c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "获取百度网盘用户信息失败: " + err.Error(), Data: nil})
	// 	return
	// }
	// resp.LoggedIn = true
	// resp.UserId = json.Number(strconv.FormatUint(uint64(userInfo.UserId), 10))
	// resp.Username = userInfo.Username
	// resp.UsedSpace = userInfo.UsedSpace
	// resp.TotalSpace = userInfo.TotalSpace
	// resp.MemberLevel = userInfo.MemberLevel
	// resp.ExpireTime = userInfo.ExpireTime
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "成功", Data: BaiDuPanStatusResp{
		LoggedIn:    true,
		UserId:      json.Number("123456"),
		Username:    "test_user",
		UsedSpace:   1024 * 1024 * 1024,      // 1GB
		TotalSpace:  10 * 1024 * 1024 * 1024, // 10GB
		MemberLevel: "普通会员",
		ExpireTime:  "2026-12-31",
	}})
}

// GetBaiDuPanOAuthUrl 获取百度网盘OAuth登录地址
// @Summary 获取百度网盘OAuth登录地址
// @Description 生成跳转到百度OAuth授权服务器的连接给客户端
// @Tags 百度网盘
// @Accept json
// @Produce json
// @Param account_id query string true "账号ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /baidupan/oauth-url [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetBaiDuPanOAuthUrl(c *gin.Context) {
	accountId := c.Query("account_id")

	if accountId == "" {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "缺少账号ID参数", Data: nil})
		return
	}
	account, err := models.GetAccountById(uint(helpers.StringToInt(accountId)))
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "账号ID不存在", Data: nil})
		return
	}

	clientId := account.AppId
	if clientId == "" {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "账号缺少AppId配置", Data: nil})
		return
	}

	// 百度网盘OAuth授权地址
	authUrl := "https://api.mqfamily.top/baidupan/oauth-url"

	// 生成state参数
	type stateData struct {
		State     string `json:"state"`
		Time      int64  `json:"time"`
		ClientId  string `json:"client_id"`
		AccountId string `json:"account_id"`
	}
	stateObj := stateData{
		State:     helpers.RandStr(16),
		Time:      time.Now().Unix(),
		ClientId:  clientId,
		AccountId: accountId,
	}
	stateJson, _ := json.Marshal(stateObj)
	stateEncoded, err := helpers.Encrypt(string(stateJson))
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "生成OAuth登录地址失败: " + err.Error(), Data: nil})
		return
	}

	// 构建授权URL
	// 注意：redirect_uri需要与百度开放平台配置的一致
	oauthUrl := fmt.Sprintf("%s?action=code&state=%s", authUrl, stateEncoded)
	c.JSON(http.StatusOK, APIResponse[string]{Code: Success, Message: "获取百度网盘OAuth登录地址成功", Data: oauthUrl})
}

// ConfirmBaiDuPanOAuthCode 确认百度网盘OAuth登录
// @Summary 确认百度网盘OAuth登录
// @Description 客户端将授权服务器返回的数据发送过来换取access token和refresh token并入库
// @Tags 百度网盘
// @Accept json
// @Produce json
// @Param account_id body string true "账号ID"
// @Param code body string true "授权码"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /baidupan/oauth-confirm [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func ConfirmBaiDuPanOAuthCode(c *gin.Context) {
	type oauthReq struct {
		AccountId uint   `json:"account_id" form:"account_id"`
		Data      string `json:"data" form:"data"`
	}
	var req oauthReq
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "参数错误", Data: nil})
		return
	}
	account, err := models.GetAccountById(req.AccountId)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "账号ID不存在", Data: nil})
		return
	}
	// 对req.Data解密
	decryptedData, err := helpers.Decrypt(req.Data)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "确认OAuth登录失败: " + err.Error(), Data: nil})
		return
	}
	// 	{
	// expires_in: 2592000,
	// refresh_token: "122.2959fe0da8c91d522099c5dca1b5608f.YDUwKsc1DS89VaP2DogevEN15cD65vXLtZ7bHHe.DbEWAW",
	// access_token: "121.fd4b4277dba7a65a51cf370d0e83f567.Y74pa1cYlIOT_Vdp2xuWOqeasckh1tWtxT9Ouw5.LPOBOA",
	// session_secret: "",
	// session_key: "",
	// scope: "basic netdisk"
	// }
	type oauthData struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	var data oauthData
	err = json.Unmarshal([]byte(decryptedData), &data)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "确认OAuth登录失败: " + err.Error(), Data: nil})
		return
	}
	// 将token和刷新token保存到账号
	account.UpdateToken(data.AccessToken, data.RefreshToken, data.ExpiresIn)
	// 调用接口获取百度用户信息
	client := account.GetBaiDuPanClient()
	userInfo, err := client.GetUserInfo()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "确认OAuth登录失败: " + err.Error(), Data: nil})
		return
	}
	rs := account.UpdateUser(string(userInfo.UserId), userInfo.UserName)
	if !rs {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "更新用户信息失败", Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "确认OAuth登录成功", Data: nil})
}

// GetBaiDuPanQueueStats 获取百度网盘请求队列统计数据
// @Summary 获取百度网盘请求队列统计数据
// @Description 获取百度网盘请求队列统计数据
// @Tags 百度网盘
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /baidupan/queue/stats [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetBaiDuPanQueueStats(c *gin.Context) {
	executor := baidupan.GetGlobalExecutor()
	stats := executor.GetStats(24 * time.Hour)
	throttleStatus := executor.GetThrottleStatus()

	responseData := gin.H{
		"queue_length": 0,
		"stats": map[string]any{
			"total_requests": stats.TotalRequests,
			"qps_count":      stats.QPSCount,
			"qpm_count":      stats.QPMCount,
			"qph_count":      stats.QPHCount,
			"qpt_count":      stats.QPTCount,
		},
		"throttle_status": map[string]any{
			"is_throttled":   throttleStatus.IsThrottled,
			"elapsed_time":   throttleStatus.ElapsedTime,
			"remaining_time": throttleStatus.RemainingTime,
			"current_qps":    throttleStatus.CurrentQPS,
			"current_qpm":    throttleStatus.CurrentQPM,
			"current_qph":    throttleStatus.CurrentQPH,
			"current_qpt":    throttleStatus.CurrentQPT,
		},
	}

	c.JSON(http.StatusOK, APIResponse[gin.H]{Code: Success, Message: "获取队列统计数据成功", Data: responseData})
}

// SetBaiDuPanQueueRateLimit 设置百度网盘请求队列速率限制
// @Summary 设置百度网盘请求队列速率限制
// @Description 设置百度网盘请求队列速率限制
// @Tags 百度网盘
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /baidupan/queue/rate-limit [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func SetBaiDuPanQueueRateLimit(c *gin.Context) {
	type rateLimitReq struct {
		QPS int `json:"qps" binding:"required,min=1,max=1000"`
		QPM int `json:"qpm" binding:"required,min=1,max=100000"`
		QPH int `json:"qph" binding:"required,min=1,max=1000000"`
		QPT int `json:"qpt" binding:"required,min=1,max=10000000"`
	}
	var req rateLimitReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "请求参数错误: " + err.Error(), Data: nil})
		return
	}

	// 设置全局执行器的速率限制配置
	baidupan.SetGlobalExecutorConfig(req.QPS, req.QPM, req.QPH, req.QPT)

	helpers.AppLogger.Infof("百度网盘队列速率限制已更新: QPS=%d, QPM=%d, QPH=%d, QPT=%d", req.QPS, req.QPM, req.QPH, req.QPT)

	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "速率限制配置成功", Data: map[string]int{
		"qps": req.QPS,
		"qpm": req.QPM,
		"qph": req.QPH,
		"qpt": req.QPT,
	}})
}

// GetBaiDuPanRequestStatsByDay 获取百度网盘请求统计（按天）
// @Summary 获取百度网盘请求统计（按天）
// @Description 获取指定日期范围内的百度网盘请求统计（按天分组）
// @Tags 百度网盘
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /baidupan/stats/daily [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetBaiDuPanRequestStatsByDay(c *gin.Context) {
	// TODO: 实现百度网盘请求统计（按天）
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "成功", Data: []map[string]any{
		{
			"date":  "2026-02-07",
			"count": 100,
		},
	}})
}

// GetBaiDuPanRequestStatsByHour 获取百度网盘请求统计（按小时）
// @Summary 获取百度网盘请求统计（按小时）
// @Description 获取指定日期范围内的百度网盘请求统计（按小时分组）
// @Tags 百度网盘
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /baidupan/stats/hourly [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetBaiDuPanRequestStatsByHour(c *gin.Context) {
	// TODO: 实现百度网盘请求统计（按小时）
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "成功", Data: []map[string]any{
		{
			"hour":  "2026-02-07 00",
			"count": 10,
		},
	}})
}

// CleanOldBaiDuPanRequestStats 清理旧的百度网盘请求统计数据
// @Summary 清理旧的百度网盘请求统计数据
// @Description 清理指定天数之前的百度网盘请求统计数据
// @Tags 百度网盘
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /baidupan/stats/clean [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func CleanOldBaiDuPanRequestStats(c *gin.Context) {
	// TODO: 实现清理旧的百度网盘请求统计数据
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "成功", Data: nil})
}
