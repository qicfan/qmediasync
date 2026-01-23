package notificationmanager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/notification"
)

// ChannelHandler 通知渠道处理器接口
type ChannelHandler interface {
	Send(ctx context.Context, notification *notification.Notification) error
	GetChannelType() string
	IsHealthy() bool
}

// TelegramChannelHandler Telegram渠道处理器
type TelegramChannelHandler struct {
	config   *notification.TelegramChannelConfig
	proxyURL string // 系统代理URL
}

func NewTelegramChannelHandler(config *notification.TelegramChannelConfig) *TelegramChannelHandler {
	return &TelegramChannelHandler{
		config:   config,
		proxyURL: "",
	}
}

func NewTelegramChannelHandlerWithProxy(config *notification.TelegramChannelConfig, proxyURL string) *TelegramChannelHandler {
	return &TelegramChannelHandler{
		config:   config,
		proxyURL: proxyURL,
	}
}

func (h *TelegramChannelHandler) GetChannelType() string {
	return "telegram"
}

func (h *TelegramChannelHandler) IsHealthy() bool {
	if h.config.BotToken == "" || h.config.ChatID == "" {
		return false
	}
	return true
}

func (h *TelegramChannelHandler) Send(ctx context.Context, notification *notification.Notification) error {
	message := h.formatMessage(notification)

	// 验证配置
	if h.config == nil {
		return fmt.Errorf("Telegram渠道配置为空")
	}
	if h.config.BotToken == "" {
		return fmt.Errorf("Telegram Bot Token为空")
	}
	if h.config.ChatID == "" {
		return fmt.Errorf("Telegram ChatID为空")
	}

	// 使用实例中的代理配置
	var bot *helpers.TelegramBot
	var err error
	if h.proxyURL != "" {
		helpers.AppLogger.Debugf("使用系统代理发送Telegram消息: %s", h.proxyURL)
		bot, err = helpers.NewTelegramBotWithProxy(h.config.BotToken, h.config.ChatID, h.proxyURL)
		if err != nil {
			return fmt.Errorf("创建代理Telegram机器人失败: %v", err)
		}
	} else {
		helpers.AppLogger.Debugf("不使用代理，直接发送Telegram消息")
		bot = helpers.NewTelegramBot(h.config.BotToken, h.config.ChatID)
	}
	if bot == nil {
		return fmt.Errorf("创建Telegram机器人失败，请检查Token或ChatID是否有效")
	}

	// 如果有图片，先尝试发送图片（带标题作为caption）
	if notification.Image != "" {
		if perr := bot.SendPhoto(notification.Image, message); perr != nil {
			helpers.AppLogger.Errorf("发送Telegram图片失败: %v", perr)
			// 不中断，继续发送文本
			return perr
		}
		return nil
	}

	// 再发送文本消息
	retries := 2
	if h.proxyURL != "" {
		retries = 3
	}
	return bot.SendMessageWithRetry(message, retries)
}

func (h *TelegramChannelHandler) formatMessage(notification *notification.Notification) string {
	// timestamp := notification.Timestamp.Format("2006-01-02 15:04:05")

	message := fmt.Sprintf("<b>%s</b>\n\n", notification.Title)
	message += fmt.Sprintf("%s\n\n", notification.Content)

	if len(notification.Metadata) > 0 {
		for key, value := range notification.Metadata {
			message += fmt.Sprintf("<b>%s:</b> %v\n", key, value)
		}
		message += "\n"
	}

	// message += fmt.Sprintf("⏰ <b>时间:</b> %s", timestamp)
	return message
}

// MeoWChannelHandler MeoW渠道处理器
type MeoWChannelHandler struct {
	config *notification.MeoWChannelConfig
}

func NewMeoWChannelHandler(config *notification.MeoWChannelConfig) *MeoWChannelHandler {
	return &MeoWChannelHandler{
		config: config,
	}
}

func (h *MeoWChannelHandler) GetChannelType() string {
	return "meow"
}

func (h *MeoWChannelHandler) IsHealthy() bool {
	if h.config.Nickname == "" {
		return false
	}
	return true
}

func (h *MeoWChannelHandler) Send(ctx context.Context, notification *notification.Notification) error {
	if h.config.Endpoint == "" {
		h.config.Endpoint = "http://api.chuckfang.com"
	}

	// 构建请求 URL
	endpoint := fmt.Sprintf("%s/%s", h.config.Endpoint, h.config.Nickname)

	// 构建 POST 请求体 (JSON 格式)
	payload := map[string]interface{}{
		"title": notification.Title,
		"msg":   notification.Content,
	}

	if notification.Image != "" {
		payload["url"] = notification.Image
	}

	// msgType 默认为 text
	msgType := "text"
	if _, hasHtml := notification.Metadata["html"]; hasHtml {
		msgType = "html"
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("MeoW 消息编码失败: %v", err)
	}

	// 添加查询参数
	queryParams := url.Values{}
	queryParams.Set("msgType", msgType)

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	fullURL := endpoint
	if len(queryParams) > 0 {
		fullURL = endpoint + "?" + queryParams.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fullURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("MeoW 创建请求失败: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("MeoW 发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("MeoW 返回错误: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if status, ok := result["status"].(float64); ok && int(status) != 200 {
		return fmt.Errorf("MeoW 响应错误: %v", result)
	}

	return nil
}

// BarkChannelHandler Bark渠道处理器
type BarkChannelHandler struct {
	config *notification.BarkChannelConfig
}

func NewBarkChannelHandler(config *notification.BarkChannelConfig) *BarkChannelHandler {
	return &BarkChannelHandler{
		config: config,
	}
}

func (h *BarkChannelHandler) GetChannelType() string {
	return "bark"
}

func (h *BarkChannelHandler) IsHealthy() bool {
	if h.config.DeviceKey == "" {
		return false
	}
	return true
}

func (h *BarkChannelHandler) Send(ctx context.Context, notification *notification.Notification) error {
	if h.config.ServerURL == "" {
		h.config.ServerURL = "https://api.day.app"
	}

	endpoint := fmt.Sprintf("%s/push", h.config.ServerURL)

	payload := map[string]interface{}{
		"device_key": h.config.DeviceKey,
		"title":      notification.Title,
		"body":       notification.Content,
		"sound":      h.config.Sound,
		"group":      string(notification.Type),
	}

	if h.config.Icon != "" {
		payload["icon"] = h.config.Icon
	}

	if notification.Image != "" {
		payload["url"] = notification.Image
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("Bark 消息编码失败: %v", err)
	}

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("Bark 创建请求失败: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Bark 发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("Bark 返回错误: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// ServerChanChannelHandler Server酱渠道处理器
type ServerChanChannelHandler struct {
	config *notification.ServerChanChannelConfig
}

func NewServerChanChannelHandler(config *notification.ServerChanChannelConfig) *ServerChanChannelHandler {
	return &ServerChanChannelHandler{
		config: config,
	}
}

func (h *ServerChanChannelHandler) GetChannelType() string {
	return "serverchan"
}

func (h *ServerChanChannelHandler) IsHealthy() bool {
	if h.config.SCKEY == "" {
		return false
	}
	return true
}

func (h *ServerChanChannelHandler) Send(ctx context.Context, notification *notification.Notification) error {
	if h.config.Endpoint == "" {
		h.config.Endpoint = "https://sc.ftqq.com"
	}

	endpoint := fmt.Sprintf("%s/%s.send", h.config.Endpoint, h.config.SCKEY)

	payload := map[string]string{
		"text": notification.Title,
		"desp": notification.Content,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("Server酱 消息编码失败: %v", err)
	}

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("Server酱 创建请求失败: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Server酱 发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Server酱 返回错误: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	json.Unmarshal(body, &result)

	if code, ok := result["code"].(float64); ok && int(code) != 0 {
		return fmt.Errorf("Server酱 响应错误: %v", result)
	}

	return nil
}

// CustomWebhookChannelHandler 自定义 Webhook 渠道处理器
type CustomWebhookChannelHandler struct {
	config *notification.CustomWebhookChannelConfig
}

func NewCustomWebhookChannelHandler(config *notification.CustomWebhookChannelConfig) *CustomWebhookChannelHandler {
	return &CustomWebhookChannelHandler{config: config}
}

func (h *CustomWebhookChannelHandler) GetChannelType() string {
	return "webhook"
}

func (h *CustomWebhookChannelHandler) IsHealthy() bool {
	if h == nil || h.config == nil {
		return false
	}
	if strings.TrimSpace(h.config.Endpoint) == "" {
		return false
	}
	method := strings.ToUpper(strings.TrimSpace(h.config.Method))
	if method != "GET" && method != "POST" {
		return false
	}
	if strings.TrimSpace(h.config.Template) == "" {
		return false
	}
	if method == "POST" && strings.TrimSpace(h.config.Format) == "" {
		return false
	}
	return true
}

func (h *CustomWebhookChannelHandler) Send(ctx context.Context, n *notification.Notification) error {
	if !h.IsHealthy() {
		return fmt.Errorf("Webhook 渠道配置不完整")
	}
	method := strings.ToUpper(strings.TrimSpace(h.config.Method))
	switch method {
	case "POST":
		return h.sendPOST(ctx, n)
	case "GET":
		return h.sendGET(ctx, n)
	default:
		return fmt.Errorf("不支持的 HTTP 方法: %s", method)
	}
}

func (h *CustomWebhookChannelHandler) sendPOST(ctx context.Context, n *notification.Notification) error {
	bodyStr, contentType, err := h.renderTemplate(n)
	if err != nil {
		return fmt.Errorf("模板渲染失败: %v", err)
	}

	// 处理 query 鉴权（在 URL 上追加）
	endpoint := h.config.Endpoint
	if strings.ToLower(strings.TrimSpace(h.config.AuthType)) == "query" && strings.TrimSpace(h.config.AuthQueryKey) != "" {
		u, err := url.Parse(endpoint)
		if err != nil {
			return fmt.Errorf("解析 Endpoint 失败: %v", err)
		}
		q := u.Query()
		q.Set(h.config.AuthQueryKey, h.config.AuthToken)
		u.RawQuery = q.Encode()
		endpoint = u.String()
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBufferString(bodyStr))
	if err != nil {
		return fmt.Errorf("创建 POST 请求失败: %v", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	// 设置鉴权与额外头
	h.applyAuthAndHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("POST 发送失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("POST 返回状态异常: status=%d, body=%s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (h *CustomWebhookChannelHandler) sendGET(ctx context.Context, n *notification.Notification) error {
	bodyStr, _, err := h.renderTemplate(n)
	if err != nil {
		return fmt.Errorf("模板渲染失败: %v", err)
	}

	u, err := url.Parse(h.config.Endpoint)
	if err != nil {
		return fmt.Errorf("解析 Endpoint 失败: %v", err)
	}
	q := u.Query()
	param := strings.TrimSpace(h.config.QueryParam)
	if param == "" {
		param = "q"
	}
	q.Set(param, bodyStr)
	// 处理 query 鉴权（追加另一个参数）
	if strings.ToLower(strings.TrimSpace(h.config.AuthType)) == "query" && strings.TrimSpace(h.config.AuthQueryKey) != "" {
		q.Set(h.config.AuthQueryKey, h.config.AuthToken)
	}
	u.RawQuery = q.Encode()

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("创建 GET 请求失败: %v", err)
	}

	// 设置鉴权与额外头
	h.applyAuthAndHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("GET 发送失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GET 返回状态异常: status=%d, body=%s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (h *CustomWebhookChannelHandler) renderTemplate(n *notification.Notification) (string, string, error) {
	format := strings.ToLower(strings.TrimSpace(h.config.Format))
	tpl := h.config.Template
	title := n.Title
	content := n.Content
	timestamp := n.Timestamp.Format("2006-01-02 15:04:05")
	image := n.Image

	switch format {
	case "json":
		// 对字符串进行 JSON 转义，避免破坏 JSON 结构
		escape := func(s string) string {
			b, _ := json.Marshal(s)
			if len(b) >= 2 {
				return string(b[1 : len(b)-1])
			}
			return s
		}
		tpl = strings.ReplaceAll(tpl, "{{title}}", escape(title))
		tpl = strings.ReplaceAll(tpl, "{{content}}", escape(content))
		tpl = strings.ReplaceAll(tpl, "{{timestamp}}", escape(timestamp))
		tpl = strings.ReplaceAll(tpl, "{{image}}", escape(image))
		return tpl, "application/json", nil

	case "form":
		// 对值进行 URL 编码
		encode := url.QueryEscape
		tpl = strings.ReplaceAll(tpl, "{{title}}", encode(title))
		tpl = strings.ReplaceAll(tpl, "{{content}}", encode(content))
		tpl = strings.ReplaceAll(tpl, "{{timestamp}}", encode(timestamp))
		tpl = strings.ReplaceAll(tpl, "{{image}}", encode(image))
		return tpl, "application/x-www-form-urlencoded", nil

	case "text", "":
		tpl = strings.ReplaceAll(tpl, "{{title}}", title)
		tpl = strings.ReplaceAll(tpl, "{{content}}", content)
		tpl = strings.ReplaceAll(tpl, "{{timestamp}}", timestamp)
		tpl = strings.ReplaceAll(tpl, "{{image}}", image)
		return tpl, "text/plain; charset=utf-8", nil

	default:
		return "", "", fmt.Errorf("不支持的模板格式: %s", format)
	}
}

// applyAuthAndHeaders 根据配置设置请求鉴权与额外头
func (h *CustomWebhookChannelHandler) applyAuthAndHeaders(req *http.Request) {
	authType := strings.ToLower(strings.TrimSpace(h.config.AuthType))
	switch authType {
	case "bearer":
		if strings.TrimSpace(h.config.AuthToken) != "" {
			req.Header.Set("Authorization", "Bearer "+h.config.AuthToken)
		}
	case "basic":
		if h.config.AuthUser != "" || h.config.AuthPass != "" {
			req.SetBasicAuth(h.config.AuthUser, h.config.AuthPass)
		}
	case "header":
		if strings.TrimSpace(h.config.AuthHeaderKey) != "" && strings.TrimSpace(h.config.AuthToken) != "" {
			req.Header.Set(h.config.AuthHeaderKey, h.config.AuthToken)
		}
	case "query":
		// 已在构建 URL 处处理
	default:
		// none 或未设置，不处理
	}
	// 额外 headers（JSON 字符串对象）
	if strings.TrimSpace(h.config.Headers) != "" {
		var m map[string]string
		if err := json.Unmarshal([]byte(h.config.Headers), &m); err == nil {
			for k, v := range m {
				if strings.TrimSpace(k) != "" {
					req.Header.Set(k, v)
				}
			}
		}
	}
}
