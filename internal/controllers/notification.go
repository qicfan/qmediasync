package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"Q115-STRM/internal/db"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/notification"
	"Q115-STRM/internal/notificationmanager"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetNotificationChannels 获取所有通知渠道
// @Summary 获取通知渠道列表
// @Description 获取已配置的通知渠道列表
// @Tags 通知管理
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/notification/channels [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetNotificationChannels(c *gin.Context) {
	var channels []models.NotificationChannel
	if err := db.Db.Find(&channels).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    1,
			"message": "获取通知渠道失败",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "获取成功",
		"data":    channels,
	})
}

// CreateTelegramChannel 创建Telegram渠道
// @Summary 创建Telegram渠道
// @Description 创建Telegram通知渠道并保存配置
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param channel_name body string true "渠道名称"
// @Param bot_token body string true "机器人Token"
// @Param chat_id body string true "聊天ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/notification/channels/telegram [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func CreateTelegramChannel(c *gin.Context) {
	type req struct {
		ChannelName string `json:"channel_name" binding:"required"`
		BotToken    string `json:"bot_token" binding:"required"`
		ChatID      string `json:"chat_id" binding:"required"`
	}

	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "参数错误",
			"data":    nil,
		})
		return
	}

	// 创建渠道
	channel := models.NotificationChannel{
		ChannelType: "telegram",
		ChannelName: r.ChannelName,
		IsEnabled:   true,
	}

	if err := db.Db.Save(&channel).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    1,
			"message": "创建渠道失败",
			"data":    nil,
		})
		return
	}

	// 创建配置
	config := models.TelegramChannelConfig{
		ChannelID: channel.ID,
		BotToken:  r.BotToken,
		ChatID:    r.ChatID,
	}
	if err := db.Db.Save(&config).Error; err != nil {
		// 回滚
		db.Db.Delete(&channel)
		c.JSON(http.StatusOK, gin.H{
			"code":    1,
			"message": "创建配置失败",
			"data":    nil,
		})
		return
	}

	// 创建默认规则
	for _, eventType := range notification.AllNotificationTypes {
		rule := models.NotificationRule{
			ChannelID: channel.ID,
			EventType: string(eventType),
			IsEnabled: true,
		}
		db.Db.Save(&rule)
	}

	// 重新加载管理器
	if notificationmanager.GlobalEnhancedNotificationManager != nil {
		notificationmanager.GlobalEnhancedNotificationManager.ReloadChannel(channel.ID)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "创建成功",
		"data":    channel,
	})
}

// CreateMeoWChannel 创建MeoW渠道
// @Summary 创建MeoW渠道
// @Description 创建MeoW通知渠道并保存配置
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param channel_name body string true "渠道名称"
// @Param nickname body string true "昵称"
// @Param endpoint body string false "接口地址"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/notification/channels/meow [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func CreateMeoWChannel(c *gin.Context) {
	type req struct {
		ChannelName string `json:"channel_name" binding:"required"`
		Nickname    string `json:"nickname" binding:"required"`
		Endpoint    string `json:"endpoint"`
	}

	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "参数错误",
			"data":    nil,
		})
		return
	}

	// 创建渠道
	channel := models.NotificationChannel{
		ChannelType: "meow",
		ChannelName: r.ChannelName,
		IsEnabled:   true,
	}

	if err := db.Db.Save(&channel).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    1,
			"message": "创建渠道失败",
			"data":    nil,
		})
		return
	}

	// 创建配置
	if r.Endpoint == "" {
		r.Endpoint = "http://api.chuckfang.com"
	}

	config := models.MeoWChannelConfig{
		ChannelID: channel.ID,
		Nickname:  r.Nickname,
		Endpoint:  r.Endpoint,
	}
	if err := db.Db.Save(&config).Error; err != nil {
		// 回滚
		db.Db.Delete(&channel)
		c.JSON(http.StatusOK, gin.H{
			"code":    1,
			"message": "创建配置失败",
			"data":    nil,
		})
		return
	}

	// 创建默认规则
	for _, eventType := range notification.AllNotificationTypes {
		rule := models.NotificationRule{
			ChannelID: channel.ID,
			EventType: string(eventType),
			IsEnabled: true,
		}
		db.Db.Save(&rule)
	}

	// 重新加载管理器
	if notificationmanager.GlobalEnhancedNotificationManager != nil {
		notificationmanager.GlobalEnhancedNotificationManager.ReloadChannel(channel.ID)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "创建成功",
		"data":    channel,
	})
}

// CreateBarkChannel 创建Bark渠道
// @Summary 创建Bark渠道
// @Description 创建Bark通知渠道并保存配置
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param channel_name body string true "渠道名称"
// @Param device_key body string true "设备Key"
// @Param server_url body string false "服务器地址"
// @Param sound body string false "提示音"
// @Param icon body string false "图标URL"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/notification/channels/bark [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func CreateBarkChannel(c *gin.Context) {
	type req struct {
		ChannelName string `json:"channel_name" binding:"required"`
		DeviceKey   string `json:"device_key" binding:"required"`
		ServerURL   string `json:"server_url"`
		Sound       string `json:"sound"`
		Icon        string `json:"icon"`
	}

	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "参数错误",
			"data":    nil,
		})
		return
	}

	// 创建渠道
	channel := models.NotificationChannel{
		ChannelType: "bark",
		ChannelName: r.ChannelName,
		IsEnabled:   true,
	}

	if err := db.Db.Save(&channel).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    1,
			"message": "创建渠道失败",
			"data":    nil,
		})
		return
	}

	// 创建配置
	if r.ServerURL == "" {
		r.ServerURL = "https://api.day.app"
	}
	if r.Sound == "" {
		r.Sound = "alert"
	}

	config := models.BarkChannelConfig{
		ChannelID: channel.ID,
		DeviceKey: r.DeviceKey,
		ServerURL: r.ServerURL,
		Sound:     r.Sound,
		Icon:      r.Icon,
	}
	if err := db.Db.Save(&config).Error; err != nil {
		// 回滚
		db.Db.Delete(&channel)
		c.JSON(http.StatusOK, gin.H{
			"code":    1,
			"message": "创建配置失败",
			"data":    nil,
		})
		return
	}

	// 创建默认规则
	for _, eventType := range notification.AllNotificationTypes {
		rule := models.NotificationRule{
			ChannelID: channel.ID,
			EventType: string(eventType),
			IsEnabled: true,
		}
		db.Db.Save(&rule)
	}

	// 重新加载管理器
	if notificationmanager.GlobalEnhancedNotificationManager != nil {
		notificationmanager.GlobalEnhancedNotificationManager.ReloadChannel(channel.ID)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "创建成功",
		"data":    channel,
	})
}

// CreateServerChanChannel 创建Server酱渠道
// @Summary 创建Server酱渠道
// @Description 创建Server酱通知渠道并保存配置
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param channel_name body string true "渠道名称"
// @Param sc_key body string true "SCKEY"
// @Param endpoint body string false "接口地址"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/notification/channels/serverchan [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func CreateServerChanChannel(c *gin.Context) {
	type req struct {
		ChannelName string `json:"channel_name" binding:"required"`
		SCKEY       string `json:"sc_key" binding:"required"`
		Endpoint    string `json:"endpoint"`
	}

	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "参数错误",
			"data":    nil,
		})
		return
	}

	// 创建渠道
	channel := models.NotificationChannel{
		ChannelType: "serverchan",
		ChannelName: r.ChannelName,
		IsEnabled:   true,
	}

	if err := db.Db.Save(&channel).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    1,
			"message": "创建渠道失败",
			"data":    nil,
		})
		return
	}

	// 创建配置
	if r.Endpoint == "" {
		r.Endpoint = "https://sc.ftqq.com"
	}

	config := models.ServerChanChannelConfig{
		ChannelID: channel.ID,
		SCKEY:     r.SCKEY,
		Endpoint:  r.Endpoint,
	}
	if err := db.Db.Save(&config).Error; err != nil {
		// 回滚
		db.Db.Delete(&channel)
		c.JSON(http.StatusOK, gin.H{
			"code":    1,
			"message": "创建配置失败",
			"data":    nil,
		})
		return
	}

	// 创建默认规则
	for _, eventType := range notification.AllNotificationTypes {
		rule := models.NotificationRule{
			ChannelID: channel.ID,
			EventType: string(eventType),
			IsEnabled: true,
		}
		db.Db.Save(&rule)
	}

	// 重新加载管理器
	if notificationmanager.GlobalEnhancedNotificationManager != nil {
		notificationmanager.GlobalEnhancedNotificationManager.ReloadChannel(channel.ID)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "创建成功",
		"data":    channel,
	})
}

// CreateCustomWebhookChannel 创建自定义 Webhook 渠道
// @Summary 创建Webhook渠道
// @Description 创建自定义Webhook通知渠道并保存配置
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param channel_name body string true "渠道名称"
// @Param endpoint body string true "Webhook地址"
// @Param method body string true "请求方法(GET/POST)"
// @Param template body string true "模板内容"
// @Param format body string false "POST格式(json|form|text)"
// @Param query_param body string false "GET参数名，默认q"
// @Param auth_type body string false "鉴权类型 none|bearer|basic|header|query"
// @Param auth_token body string false "鉴权Token"
// @Param auth_user body string false "Basic用户名"
// @Param auth_pass body string false "Basic密码"
// @Param auth_header_key body string false "Header键"
// @Param auth_query_key body string false "Query键"
// @Param headers body object false "附加请求头"
// @Param description body string false "描述"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/notification/channels/webhook [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func CreateCustomWebhookChannel(c *gin.Context) {
	type req struct {
		ChannelName string `json:"channel_name" binding:"required"`
		Endpoint    string `json:"endpoint" binding:"required"`
		Method      string `json:"method" binding:"required"`   // GET | POST
		Template    string `json:"template" binding:"required"` // 模板字符串
		Format      string `json:"format"`                      // POST: json|form|text；GET 可忽略
		QueryParam  string `json:"query_param"`                 // GET 参数名，默认 q
		// 鉴权与扩展
		AuthType      string            `json:"auth_type"` // none|bearer|basic|header|query
		AuthToken     string            `json:"auth_token"`
		AuthUser      string            `json:"auth_user"`
		AuthPass      string            `json:"auth_pass"`
		AuthHeaderKey string            `json:"auth_header_key"`
		AuthQueryKey  string            `json:"auth_query_key"`
		Headers       map[string]string `json:"headers"` // 额外请求头
		Description   string            `json:"description"`
	}

	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "参数错误", "data": nil})
		return
	}

	method := strings.ToUpper(strings.TrimSpace(r.Method))
	format := strings.ToLower(strings.TrimSpace(r.Format))
	if r.QueryParam == "" {
		r.QueryParam = "q"
	}

	// 鉴权字段基本校验
	switch strings.ToLower(strings.TrimSpace(r.AuthType)) {
	case "", "none":
		// 无需额外校验
	case "bearer":
		if strings.TrimSpace(r.AuthToken) == "" {
			c.JSON(http.StatusOK, gin.H{"code": 1, "message": "bearer 方式需提供 auth_token", "data": nil})
			return
		}
	case "basic":
		if strings.TrimSpace(r.AuthUser) == "" && strings.TrimSpace(r.AuthPass) == "" {
			c.JSON(http.StatusOK, gin.H{"code": 1, "message": "basic 方式需提供 auth_user 或 auth_pass", "data": nil})
			return
		}
	case "header":
		if strings.TrimSpace(r.AuthHeaderKey) == "" || strings.TrimSpace(r.AuthToken) == "" {
			c.JSON(http.StatusOK, gin.H{"code": 1, "message": "header 方式需提供 auth_header_key 与 auth_token", "data": nil})
			return
		}
	case "query":
		if strings.TrimSpace(r.AuthQueryKey) == "" || strings.TrimSpace(r.AuthToken) == "" {
			c.JSON(http.StatusOK, gin.H{"code": 1, "message": "query 方式需提供 auth_query_key 与 auth_token", "data": nil})
			return
		}
	default:
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "auth_type 必须是 none|bearer|basic|header|query", "data": nil})
		return
	}

	// 模板校验
	switch method {
	case "POST":
		switch format {
		case "json":
			s := replaceVarsWithEmpty(r.Template)
			var js interface{}
			if err := json.Unmarshal([]byte(s), &js); err != nil {
				c.JSON(http.StatusOK, gin.H{"code": 1, "message": "JSON 模板无效: " + err.Error(), "data": nil})
				return
			}
		case "form":
			re := regexp.MustCompile(`^[A-Za-z0-9_.-]+=[^&]*(?:&[A-Za-z0-9_.-]+=[^&]*)*$`)
			if !re.MatchString(strings.TrimSpace(r.Template)) {
				c.JSON(http.StatusOK, gin.H{"code": 1, "message": "Form 模板无效: 必须为 key=value&key2=value2 格式", "data": nil})
				return
			}
		case "text", "":
		default:
			c.JSON(http.StatusOK, gin.H{"code": 1, "message": "format 必须是 json|form|text", "data": nil})
			return
		}
	case "GET":
		// GET 模板不做特殊格式校验
	default:
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "method 必须是 GET 或 POST", "data": nil})
		return
	}

	// 创建渠道
	channel := models.NotificationChannel{
		ChannelType: "webhook",
		ChannelName: r.ChannelName,
		Description: r.Description,
		IsEnabled:   true,
	}
	if err := db.Db.Save(&channel).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "创建渠道失败", "data": nil})
		return
	}

	// 创建配置
	// 额外头序列化
	var headersJSON string
	if r.Headers != nil {
		if b, err := json.Marshal(r.Headers); err == nil {
			headersJSON = string(b)
		} else {
			c.JSON(http.StatusOK, gin.H{"code": 1, "message": "headers 必须为对象", "data": nil})
			return
		}
	}

	cfg := models.CustomWebhookChannelConfig{
		ChannelID:     channel.ID,
		Endpoint:      r.Endpoint,
		Method:        method,
		Template:      r.Template,
		Format:        format,
		QueryParam:    strings.TrimSpace(r.QueryParam),
		AuthType:      strings.ToLower(strings.TrimSpace(r.AuthType)),
		AuthToken:     r.AuthToken,
		AuthUser:      r.AuthUser,
		AuthPass:      r.AuthPass,
		AuthHeaderKey: r.AuthHeaderKey,
		AuthQueryKey:  r.AuthQueryKey,
		Headers:       headersJSON,
	}
	if err := db.Db.Save(&cfg).Error; err != nil {
		db.Db.Delete(&channel)
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "创建配置失败", "data": nil})
		return
	}

	// 创建默认规则
	for _, eventType := range notification.AllNotificationTypes {
		db.Db.Save(&models.NotificationRule{ChannelID: channel.ID, EventType: string(eventType), IsEnabled: true})
	}

	// 刷新通知管理器
	if notificationmanager.GlobalEnhancedNotificationManager != nil {
		notificationmanager.GlobalEnhancedNotificationManager.ReloadChannel(channel.ID)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "创建成功", "data": channel})
}

// UpdateCustomWebhookChannel 更新自定义 Webhook 渠道配置
// @Summary 更新Webhook渠道
// @Description 更新自定义Webhook渠道的基础信息和模板
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param channel_id body integer true "渠道ID"
// @Param channel_name body string false "渠道名称"
// @Param endpoint body string false "Webhook地址"
// @Param method body string false "请求方法(GET/POST)"
// @Param template body string false "模板内容"
// @Param format body string false "POST格式(json|form|text)"
// @Param query_param body string false "GET参数名"
// @Param auth_type body string false "鉴权类型"
// @Param auth_token body string false "鉴权Token"
// @Param auth_user body string false "Basic用户名"
// @Param auth_pass body string false "Basic密码"
// @Param auth_header_key body string false "Header键"
// @Param auth_query_key body string false "Query键"
// @Param headers body object false "附加请求头"
// @Param description body string false "描述"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/notification/channels/webhook [put]
// @Security JwtAuth
// @Security ApiKeyAuth
func UpdateCustomWebhookChannel(c *gin.Context) {
	type req struct {
		ChannelID     uint              `json:"channel_id" binding:"required"`
		ChannelName   string            `json:"channel_name"`
		Endpoint      string            `json:"endpoint"`
		Method        string            `json:"method"`
		Template      string            `json:"template"`
		Format        string            `json:"format"`
		QueryParam    string            `json:"query_param"`
		AuthType      string            `json:"auth_type"`
		AuthToken     string            `json:"auth_token"`
		AuthUser      string            `json:"auth_user"`
		AuthPass      string            `json:"auth_pass"`
		AuthHeaderKey string            `json:"auth_header_key"`
		AuthQueryKey  string            `json:"auth_query_key"`
		Headers       map[string]string `json:"headers"`
		Description   string            `json:"description"`
	}

	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "参数错误", "data": nil})
		return
	}

	// 查找渠道
	var channel models.NotificationChannel
	if err := db.Db.First(&channel, r.ChannelID).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "渠道不存在", "data": nil})
		return
	}
	if channel.ChannelType != "webhook" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "该渠道不是 Webhook 类型", "data": nil})
		return
	}

	// 查找配置
	var cfg models.CustomWebhookChannelConfig
	if err := db.Db.Where("channel_id = ?", r.ChannelID).First(&cfg).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "配置不存在", "data": nil})
		return
	}

	// 更新渠道基本信息
	if r.ChannelName != "" {
		channel.ChannelName = r.ChannelName
	}
	if r.Description != "" {
		channel.Description = r.Description
	}

	// 准备更新配置字段
	updates := make(map[string]interface{})

	if r.Endpoint != "" {
		updates["endpoint"] = r.Endpoint
	}
	if r.Method != "" {
		method := strings.ToUpper(strings.TrimSpace(r.Method))
		if method != "GET" && method != "POST" {
			c.JSON(http.StatusOK, gin.H{"code": 1, "message": "method 必须是 GET 或 POST", "data": nil})
			return
		}
		updates["method"] = method
	}
	if r.Template != "" {
		// 执行模板校验
		method := cfg.Method
		if r.Method != "" {
			method = strings.ToUpper(strings.TrimSpace(r.Method))
		}
		format := cfg.Format
		if r.Format != "" {
			format = strings.ToLower(strings.TrimSpace(r.Format))
		}

		if method == "POST" {
			switch format {
			case "json":
				s := replaceVarsWithEmpty(r.Template)
				var js interface{}
				if err := json.Unmarshal([]byte(s), &js); err != nil {
					c.JSON(http.StatusOK, gin.H{"code": 1, "message": "JSON 模板无效: " + err.Error(), "data": nil})
					return
				}
			case "form":
				re := regexp.MustCompile(`^[A-Za-z0-9_.-]+=[^&]*(?:&[A-Za-z0-9_.-]+=[^&]*)*$`)
				if !re.MatchString(strings.TrimSpace(r.Template)) {
					c.JSON(http.StatusOK, gin.H{"code": 1, "message": "Form 模板无效: 必须为 key=value&key2=value2 格式", "data": nil})
					return
				}
			case "text", "":
			default:
				c.JSON(http.StatusOK, gin.H{"code": 1, "message": "format 必须是 json|form|text", "data": nil})
				return
			}
		}
		updates["template"] = r.Template
	}
	if r.Format != "" {
		updates["format"] = strings.ToLower(strings.TrimSpace(r.Format))
	}
	if r.QueryParam != "" {
		updates["query_param"] = strings.TrimSpace(r.QueryParam)
	}

	// 鉴权字段更新
	if r.AuthType != "" {
		authType := strings.ToLower(strings.TrimSpace(r.AuthType))
		switch authType {
		case "", "none":
		case "bearer":
			if strings.TrimSpace(r.AuthToken) == "" {
				c.JSON(http.StatusOK, gin.H{"code": 1, "message": "bearer 方式需提供 auth_token", "data": nil})
				return
			}
		case "basic":
			if strings.TrimSpace(r.AuthUser) == "" && strings.TrimSpace(r.AuthPass) == "" {
				c.JSON(http.StatusOK, gin.H{"code": 1, "message": "basic 方式需提供 auth_user 或 auth_pass", "data": nil})
				return
			}
		case "header":
			if strings.TrimSpace(r.AuthHeaderKey) == "" || strings.TrimSpace(r.AuthToken) == "" {
				c.JSON(http.StatusOK, gin.H{"code": 1, "message": "header 方式需提供 auth_header_key 与 auth_token", "data": nil})
				return
			}
		case "query":
			if strings.TrimSpace(r.AuthQueryKey) == "" || strings.TrimSpace(r.AuthToken) == "" {
				c.JSON(http.StatusOK, gin.H{"code": 1, "message": "query 方式需提供 auth_query_key 与 auth_token", "data": nil})
				return
			}
		default:
			c.JSON(http.StatusOK, gin.H{"code": 1, "message": "auth_type 必须是 none|bearer|basic|header|query", "data": nil})
			return
		}
		updates["auth_type"] = authType
	}
	if r.AuthToken != "" {
		updates["auth_token"] = r.AuthToken
	}
	if r.AuthUser != "" {
		updates["auth_user"] = r.AuthUser
	}
	if r.AuthPass != "" {
		updates["auth_pass"] = r.AuthPass
	}
	if r.AuthHeaderKey != "" {
		updates["auth_header_key"] = r.AuthHeaderKey
	}
	if r.AuthQueryKey != "" {
		updates["auth_query_key"] = r.AuthQueryKey
	}
	if r.Headers != nil {
		if b, err := json.Marshal(r.Headers); err == nil {
			updates["headers"] = string(b)
		} else {
			c.JSON(http.StatusOK, gin.H{"code": 1, "message": "headers 必须为对象", "data": nil})
			return
		}
	}

	// 更新配置
	if len(updates) > 0 {
		if err := db.Db.Model(&cfg).Updates(updates).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 1, "message": "更新配置失败", "data": nil})
			return
		}
	}

	// 更新渠道
	if err := db.Db.Save(&channel).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "更新渠道失败", "data": nil})
		return
	}

	// 刷新通知管理器
	if notificationmanager.GlobalEnhancedNotificationManager != nil {
		notificationmanager.GlobalEnhancedNotificationManager.ReloadChannel(channel.ID)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "更新成功", "data": channel})
}

// UpdateTelegramChannel 更新 Telegram 渠道配置
// @Summary 更新Telegram渠道
// @Description 更新Telegram渠道名称与配置
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param channel_id body integer true "渠道ID"
// @Param channel_name body string false "渠道名称"
// @Param bot_token body string false "机器人Token"
// @Param chat_id body string false "聊天ID"
// @Param description body string false "描述"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/notification/channels/telegram [put]
// @Security JwtAuth
// @Security ApiKeyAuth
func UpdateTelegramChannel(c *gin.Context) {
	type req struct {
		ChannelID   uint   `json:"channel_id" binding:"required"`
		ChannelName string `json:"channel_name"`
		BotToken    string `json:"bot_token"`
		ChatID      string `json:"chat_id"`
		Description string `json:"description"`
	}

	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "参数错误", "data": nil})
		return
	}

	// 查找渠道
	var channel models.NotificationChannel
	if err := db.Db.First(&channel, r.ChannelID).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "渠道不存在", "data": nil})
		return
	}
	if channel.ChannelType != "telegram" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "该渠道不是 Telegram 类型", "data": nil})
		return
	}

	// 查找配置
	var cfg models.TelegramChannelConfig
	if err := db.Db.Where("channel_id = ?", r.ChannelID).First(&cfg).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "配置不存在", "data": nil})
		return
	}

	// 更新渠道基本信息
	if r.ChannelName != "" {
		channel.ChannelName = r.ChannelName
	}
	if r.Description != "" {
		channel.Description = r.Description
	}

	// 准备更新配置字段
	updates := make(map[string]interface{})
	if r.BotToken != "" {
		updates["bot_token"] = r.BotToken
	}
	if r.ChatID != "" {
		updates["chat_id"] = r.ChatID
	}

	// 更新配置
	if len(updates) > 0 {
		if err := db.Db.Model(&cfg).Updates(updates).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 1, "message": "更新配置失败", "data": nil})
			return
		}
	}

	// 更新渠道
	if err := db.Db.Save(&channel).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "更新渠道失败", "data": nil})
		return
	}

	// 刷新通知管理器
	if notificationmanager.GlobalEnhancedNotificationManager != nil {
		notificationmanager.GlobalEnhancedNotificationManager.ReloadChannel(channel.ID)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "更新成功", "data": channel})
}

// UpdateMeoWChannel 更新 MeoW 渠道配置
// @Summary 更新MeoW渠道
// @Description 更新MeoW渠道名称与配置
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param channel_id body integer true "渠道ID"
// @Param channel_name body string false "渠道名称"
// @Param nickname body string false "昵称"
// @Param endpoint body string false "接口地址"
// @Param description body string false "描述"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/notification/channels/meow [put]
// @Security JwtAuth
// @Security ApiKeyAuth
func UpdateMeoWChannel(c *gin.Context) {
	type req struct {
		ChannelID   uint   `json:"channel_id" binding:"required"`
		ChannelName string `json:"channel_name"`
		Nickname    string `json:"nickname"`
		Endpoint    string `json:"endpoint"`
		Description string `json:"description"`
	}

	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "参数错误", "data": nil})
		return
	}

	// 查找渠道
	var channel models.NotificationChannel
	if err := db.Db.First(&channel, r.ChannelID).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "渠道不存在", "data": nil})
		return
	}
	if channel.ChannelType != "meow" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "该渠道不是 MeoW 类型", "data": nil})
		return
	}

	// 查找配置
	var cfg models.MeoWChannelConfig
	if err := db.Db.Where("channel_id = ?", r.ChannelID).First(&cfg).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "配置不存在", "data": nil})
		return
	}

	// 更新渠道基本信息
	if r.ChannelName != "" {
		channel.ChannelName = r.ChannelName
	}
	if r.Description != "" {
		channel.Description = r.Description
	}

	// 准备更新配置字段
	updates := make(map[string]interface{})
	if r.Nickname != "" {
		updates["nickname"] = r.Nickname
	}
	if r.Endpoint != "" {
		updates["endpoint"] = r.Endpoint
	}

	// 更新配置
	if len(updates) > 0 {
		if err := db.Db.Model(&cfg).Updates(updates).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 1, "message": "更新配置失败", "data": nil})
			return
		}
	}

	// 更新渠道
	if err := db.Db.Save(&channel).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "更新渠道失败", "data": nil})
		return
	}

	// 刷新通知管理器
	if notificationmanager.GlobalEnhancedNotificationManager != nil {
		notificationmanager.GlobalEnhancedNotificationManager.ReloadChannel(channel.ID)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "更新成功", "data": channel})
}

// UpdateBarkChannel 更新 Bark 渠道配置
// @Summary 更新Bark渠道
// @Description 更新Bark渠道名称与配置
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param channel_id body integer true "渠道ID"
// @Param channel_name body string false "渠道名称"
// @Param device_key body string false "设备Key"
// @Param server_url body string false "服务器地址"
// @Param sound body string false "提示音"
// @Param icon body string false "图标URL"
// @Param description body string false "描述"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/notification/channels/bark [put]
// @Security JwtAuth
// @Security ApiKeyAuth
func UpdateBarkChannel(c *gin.Context) {
	type req struct {
		ChannelID   uint   `json:"channel_id" binding:"required"`
		ChannelName string `json:"channel_name"`
		DeviceKey   string `json:"device_key"`
		ServerURL   string `json:"server_url"`
		Sound       string `json:"sound"`
		Icon        string `json:"icon"`
		Description string `json:"description"`
	}

	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "参数错误", "data": nil})
		return
	}

	// 查找渠道
	var channel models.NotificationChannel
	if err := db.Db.First(&channel, r.ChannelID).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "渠道不存在", "data": nil})
		return
	}
	if channel.ChannelType != "bark" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "该渠道不是 Bark 类型", "data": nil})
		return
	}

	// 查找配置
	var cfg models.BarkChannelConfig
	if err := db.Db.Where("channel_id = ?", r.ChannelID).First(&cfg).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "配置不存在", "data": nil})
		return
	}

	// 更新渠道基本信息
	if r.ChannelName != "" {
		channel.ChannelName = r.ChannelName
	}
	if r.Description != "" {
		channel.Description = r.Description
	}

	// 准备更新配置字段
	updates := make(map[string]interface{})
	if r.DeviceKey != "" {
		updates["device_key"] = r.DeviceKey
	}
	if r.ServerURL != "" {
		updates["server_url"] = r.ServerURL
	}
	if r.Sound != "" {
		updates["sound"] = r.Sound
	}
	if r.Icon != "" {
		updates["icon"] = r.Icon
	}

	// 更新配置
	if len(updates) > 0 {
		if err := db.Db.Model(&cfg).Updates(updates).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 1, "message": "更新配置失败", "data": nil})
			return
		}
	}

	// 更新渠道
	if err := db.Db.Save(&channel).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "更新渠道失败", "data": nil})
		return
	}

	// 刷新通知管理器
	if notificationmanager.GlobalEnhancedNotificationManager != nil {
		notificationmanager.GlobalEnhancedNotificationManager.ReloadChannel(channel.ID)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "更新成功", "data": channel})
}

// UpdateServerChanChannel 更新 Server酱 渠道配置
// @Summary 更新Server酱渠道
// @Description 更新Server酱渠道名称与配置
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param channel_id body integer true "渠道ID"
// @Param channel_name body string false "渠道名称"
// @Param sc_key body string false "SCKEY"
// @Param endpoint body string false "接口地址"
// @Param description body string false "描述"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/notification/channels/serverchan [put]
// @Security JwtAuth
// @Security ApiKeyAuth
func UpdateServerChanChannel(c *gin.Context) {
	type req struct {
		ChannelID   uint   `json:"channel_id" binding:"required"`
		ChannelName string `json:"channel_name"`
		SCKEY       string `json:"sc_key"`
		Endpoint    string `json:"endpoint"`
		Description string `json:"description"`
	}

	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": "参数错误", "data": nil})
		return
	}

	// 查找渠道
	var channel models.NotificationChannel
	if err := db.Db.First(&channel, r.ChannelID).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "渠道不存在", "data": nil})
		return
	}
	if channel.ChannelType != "serverchan" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "该渠道不是 Server酱 类型", "data": nil})
		return
	}

	// 查找配置
	var cfg models.ServerChanChannelConfig
	if err := db.Db.Where("channel_id = ?", r.ChannelID).First(&cfg).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "配置不存在", "data": nil})
		return
	}

	// 更新渠道基本信息
	if r.ChannelName != "" {
		channel.ChannelName = r.ChannelName
	}
	if r.Description != "" {
		channel.Description = r.Description
	}

	// 准备更新配置字段
	updates := make(map[string]interface{})
	if r.SCKEY != "" {
		updates["sckey"] = r.SCKEY
	}
	if r.Endpoint != "" {
		updates["endpoint"] = r.Endpoint
	}

	// 更新配置
	if len(updates) > 0 {
		if err := db.Db.Model(&cfg).Updates(updates).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 1, "message": "更新配置失败", "data": nil})
			return
		}
	}

	// 更新渠道
	if err := db.Db.Save(&channel).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "更新渠道失败", "data": nil})
		return
	}

	// 刷新通知管理器
	if notificationmanager.GlobalEnhancedNotificationManager != nil {
		notificationmanager.GlobalEnhancedNotificationManager.ReloadChannel(channel.ID)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "更新成功", "data": channel})
}

func replaceVarsWithEmpty(s string) string {
	s = strings.ReplaceAll(s, "{{title}}", "")
	s = strings.ReplaceAll(s, "{{content}}", "")
	s = strings.ReplaceAll(s, "{{timestamp}}", "")
	s = strings.ReplaceAll(s, "{{image}}", "")
	return s
}

// UpdateChannelStatus 启用/禁用渠道
// @Summary 更新渠道启用状态
// @Description 启用或禁用指定通知渠道
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param channel_id body integer true "渠道ID"
// @Param is_enabled body boolean false "是否启用"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/notification/channels/status [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func UpdateChannelStatus(c *gin.Context) {
	type req struct {
		ChannelID uint `json:"channel_id" binding:"required"`
		IsEnabled bool `json:"is_enabled"`
	}

	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "参数错误",
			"data":    nil,
		})
		return
	}

	if err := db.Db.Model(&models.NotificationChannel{}).
		Where("id = ?", r.ChannelID).
		Update("is_enabled", r.IsEnabled).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    1,
			"message": "更新失败",
			"data":    nil,
		})
		return
	}

	// 重新加载管理器
	if notificationmanager.GlobalEnhancedNotificationManager != nil {
		notificationmanager.GlobalEnhancedNotificationManager.ReloadChannel(r.ChannelID)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
		"data":    nil,
	})
}

// GetTelegramChannel 查询单个 Telegram 渠道配置
// @Summary 获取Telegram渠道
// @Description 根据ID获取Telegram渠道及配置
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param id path integer true "渠道ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/notification/channels/telegram/{id} [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetTelegramChannel(c *gin.Context) {
	channelID := c.Param("id")
	var channel models.NotificationChannel
	if err := db.Db.First(&channel, channelID).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "渠道不存在", "data": nil})
		return
	}
	if channel.ChannelType != "telegram" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "渠道类型不匹配", "data": nil})
		return
	}
	var cfg models.TelegramChannelConfig
	if err := db.Db.Where("channel_id = ?", channel.ID).First(&cfg).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "配置不存在", "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "获取成功", "data": gin.H{
		"channel": channel,
		"config":  cfg,
	}})
}

// GetMeoWChannel 查询单个 MeoW 渠道配置
// @Summary 获取MeoW渠道
// @Description 根据ID获取MeoW渠道及配置
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param id path integer true "渠道ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/notification/channels/meow/{id} [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetMeoWChannel(c *gin.Context) {
	channelID := c.Param("id")
	var channel models.NotificationChannel
	if err := db.Db.First(&channel, channelID).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "渠道不存在", "data": nil})
		return
	}
	if channel.ChannelType != "meow" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "渠道类型不匹配", "data": nil})
		return
	}
	var cfg models.MeoWChannelConfig
	if err := db.Db.Where("channel_id = ?", channel.ID).First(&cfg).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "配置不存在", "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "获取成功", "data": gin.H{
		"channel": channel,
		"config":  cfg,
	}})
}

// GetBarkChannel 查询单个 Bark 渠道配置
// @Summary 获取Bark渠道
// @Description 根据ID获取Bark渠道及配置
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param id path integer true "渠道ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/notification/channels/bark/{id} [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetBarkChannel(c *gin.Context) {
	channelID := c.Param("id")
	var channel models.NotificationChannel
	if err := db.Db.First(&channel, channelID).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "渠道不存在", "data": nil})
		return
	}
	if channel.ChannelType != "bark" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "渠道类型不匹配", "data": nil})
		return
	}
	var cfg models.BarkChannelConfig
	if err := db.Db.Where("channel_id = ?", channel.ID).First(&cfg).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "配置不存在", "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "获取成功", "data": gin.H{
		"channel": channel,
		"config":  cfg,
	}})
}

// GetServerChanChannel 查询单个 Server酱 渠道配置
// @Summary 获取Server酱渠道
// @Description 根据ID获取Server酱渠道及配置
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param id path integer true "渠道ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/notification/channels/serverchan/{id} [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetServerChanChannel(c *gin.Context) {
	channelID := c.Param("id")
	var channel models.NotificationChannel
	if err := db.Db.First(&channel, channelID).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "渠道不存在", "data": nil})
		return
	}
	if channel.ChannelType != "serverchan" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "渠道类型不匹配", "data": nil})
		return
	}
	var cfg models.ServerChanChannelConfig
	if err := db.Db.Where("channel_id = ?", channel.ID).First(&cfg).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "配置不存在", "data": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "获取成功", "data": gin.H{
		"channel": channel,
		"config":  cfg,
	}})
}

// GetCustomWebhookChannel 查询单个 Webhook 渠道配置
// @Summary 获取Webhook渠道
// @Description 根据ID获取Webhook渠道及配置
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param id path integer true "渠道ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/notification/channels/webhook/{id} [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetCustomWebhookChannel(c *gin.Context) {
	channelID := c.Param("id")
	var channel models.NotificationChannel
	if err := db.Db.First(&channel, channelID).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "渠道不存在", "data": nil})
		return
	}
	if channel.ChannelType != "webhook" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "渠道类型不匹配", "data": nil})
		return
	}
	var cfg models.CustomWebhookChannelConfig
	if err := db.Db.Where("channel_id = ?", channel.ID).First(&cfg).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "配置不存在", "data": nil})
		return
	}
	// 解析 headers JSON 字符串为对象
	var headers map[string]string
	if cfg.Headers != "" {
		json.Unmarshal([]byte(cfg.Headers), &headers)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "获取成功", "data": gin.H{
		"channel": channel,
		"config": gin.H{
			"id":              cfg.ID,
			"channel_id":      cfg.ChannelID,
			"endpoint":        cfg.Endpoint,
			"method":          cfg.Method,
			"template":        cfg.Template,
			"format":          cfg.Format,
			"query_param":     cfg.QueryParam,
			"auth_type":       cfg.AuthType,
			"auth_token":      cfg.AuthToken,
			"auth_user":       cfg.AuthUser,
			"auth_pass":       cfg.AuthPass,
			"auth_header_key": cfg.AuthHeaderKey,
			"auth_query_key":  cfg.AuthQueryKey,
			"headers":         headers,
			"created_at":      cfg.CreatedAt,
			"updated_at":      cfg.UpdatedAt,
		},
	}})
}

// DeleteChannel 删除渠道
// @Summary 删除通知渠道
// @Description 删除渠道及其配置与规则
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param id path integer true "渠道ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/notification/channels/{id} [delete]
// @Security JwtAuth
// @Security ApiKeyAuth
func DeleteChannel(c *gin.Context) {
	channelID := c.Param("id")
	if channelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "参数错误",
			"data":    nil,
		})
		return
	}

	// 删除渠道及其关联的配置和规则
	if err := db.Db.Transaction(func(tx *gorm.DB) error {
		// 删除规则
		if err := tx.Where("channel_id = ?", channelID).Delete(&models.NotificationRule{}).Error; err != nil {
			return err
		}
		// 删除特定类型的配置
		var channel models.NotificationChannel
		if err := tx.Where("id = ?", channelID).First(&channel).Error; err != nil {
			return err
		}

		switch channel.ChannelType {
		case "telegram":
			tx.Where("channel_id = ?", channelID).Delete(&models.TelegramChannelConfig{})
		case "meow":
			tx.Where("channel_id = ?", channelID).Delete(&models.MeoWChannelConfig{})
		case "bark":
			tx.Where("channel_id = ?", channelID).Delete(&models.BarkChannelConfig{})
		case "serverchan":
			tx.Where("channel_id = ?", channelID).Delete(&models.ServerChanChannelConfig{})
		case "webhook":
			tx.Where("channel_id = ?", channelID).Delete(&models.CustomWebhookChannelConfig{})
		}

		// 删除渠道
		return tx.Delete(&channel).Error
	}); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    1,
			"message": "删除失败",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
		"data":    nil,
	})
}

// GetNotificationRules 获取通知规则
// @Summary 获取通知规则
// @Description 获取指定渠道的通知规则列表
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param channel_id query integer false "渠道ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/notification/rules [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetNotificationRules(c *gin.Context) {
	channelID := c.Query("channel_id")

	var rules []models.NotificationRule
	query := db.Db
	if channelID != "" {
		query = query.Where("channel_id = ?", channelID)
	}
	if err := query.Find(&rules).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    1,
			"message": "获取规则失败",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "获取成功",
		"data":    rules,
	})
}

// UpdateNotificationRule 更新通知规则
// @Summary 更新通知规则
// @Description 创建或更新渠道的事件通知规则
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param channel_id body integer true "渠道ID"
// @Param event_type body string true "事件类型"
// @Param is_enabled body boolean false "是否启用"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/notification/rules [put]
// @Security JwtAuth
// @Security ApiKeyAuth
func UpdateNotificationRule(c *gin.Context) {
	type req struct {
		ChannelID uint   `json:"channel_id" binding:"required"`
		EventType string `json:"event_type" binding:"required"`
		IsEnabled bool   `json:"is_enabled"`
	}

	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "参数错误",
			"data":    nil,
		})
		return
	}

	// 先检查规则是否存在
	var rule models.NotificationRule
	if err := db.Db.Where("channel_id = ? AND event_type = ?", r.ChannelID, r.EventType).First(&rule).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 不存在则创建
			rule = models.NotificationRule{
				ChannelID: r.ChannelID,
				EventType: r.EventType,
				IsEnabled: r.IsEnabled,
			}
			if err := db.Db.Save(&rule).Error; err != nil {
				c.JSON(http.StatusOK, gin.H{
					"code":    1,
					"message": "创建规则失败",
					"data":    nil,
				})
				return
			}
		} else {
			c.JSON(http.StatusOK, gin.H{
				"code":    1,
				"message": "查询规则失败",
				"data":    nil,
			})
			return
		}
	} else {
		// 存在则更新
		if err := db.Db.Model(&rule).Update("is_enabled", r.IsEnabled).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    1,
				"message": "更新规则失败",
				"data":    nil,
			})
			return
		}
	}

	// 重新加载规则
	if notificationmanager.GlobalEnhancedNotificationManager != nil {
		notificationmanager.GlobalEnhancedNotificationManager.LoadChannels()
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
		"data":    nil,
	})
}

// TestChannelConnection 测试渠道连接
// @Summary 测试通知渠道
// @Description 发送测试消息验证通知渠道可用性
// @Tags 通知管理
// @Accept json
// @Produce json
// @Param channel_id body integer true "渠道ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/notification/channels/test [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func TestChannelConnection(c *gin.Context) {
	type req struct {
		ChannelID uint `json:"channel_id" binding:"required"`
	}

	var r req
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "参数错误",
			"data":    nil,
		})
		return
	}

	var channel models.NotificationChannel
	if err := db.Db.Where("id = ?", r.ChannelID).First(&channel).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    1,
			"message": "渠道不存在",
			"data":    nil,
		})
		return
	}

	// 构建测试通知
	testNotif := &notification.Notification{
		Type:      notification.SystemAlert,
		Title:     "通知渠道测试",
		Content:   "这是一条测试消息",
		Timestamp: time.Now(),
		Priority:  notification.NormalPriority,
	}

	// 创建处理器并发送测试消息
	var handler notificationmanager.ChannelHandler

	switch channel.ChannelType {
	case "telegram":
		var config models.TelegramChannelConfig
		if err := db.Db.Where("channel_id = ?", r.ChannelID).First(&config).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    1,
				"message": "配置不存在",
				"data":    nil,
			})
			return
		}
		handler = notificationmanager.NewTelegramChannelHandlerWithProxy(&config, models.SettingsGlobal.HttpProxy)

	case "meow":
		var config models.MeoWChannelConfig
		if err := db.Db.Where("channel_id = ?", r.ChannelID).First(&config).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    1,
				"message": "配置不存在",
				"data":    nil,
			})
			return
		}
		handler = notificationmanager.NewMeoWChannelHandler(&config)

	case "bark":
		var config models.BarkChannelConfig
		if err := db.Db.Where("channel_id = ?", r.ChannelID).First(&config).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    1,
				"message": "配置不存在",
				"data":    nil,
			})
			return
		}
		handler = notificationmanager.NewBarkChannelHandler(&config)

	case "serverchan":
		var config models.ServerChanChannelConfig
		if err := db.Db.Where("channel_id = ?", r.ChannelID).First(&config).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    1,
				"message": "配置不存在",
				"data":    nil,
			})
			return
		}
		handler = notificationmanager.NewServerChanChannelHandler(&config)

	case "webhook":
		var config models.CustomWebhookChannelConfig
		if err := db.Db.Where("channel_id = ?", r.ChannelID).First(&config).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    1,
				"message": "配置不存在",
				"data":    nil,
			})
			return
		}
		handler = notificationmanager.NewCustomWebhookChannelHandler(&config)

	default:
		c.JSON(http.StatusOK, gin.H{
			"code":    1,
			"message": "未知的渠道类型",
			"data":    nil,
		})
		return
	}

	// 发送测试消息
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := handler.Send(ctx, testNotif); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    1,
			"message": "测试失败: " + err.Error(),
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "测试成功",
		"data":    nil,
	})
}
