package controllers

import (
	"Q115-STRM/internal/models"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// CreateAPIKeyRequest 创建API Key请求
type CreateAPIKeyRequest struct {
	Name string `json:"name" binding:"required"`
}

// CreateAPIKeyResponse 创建API Key响应
type CreateAPIKeyResponse struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Key       string `json:"key"`        // 完整的API Key，仅此一次返回
	KeyPrefix string `json:"key_prefix"` // 前缀用于显示
	CreatedAt int64  `json:"created_at"`
	IsActive  bool   `json:"is_active"`
}

// APIKeyListItem API Key列表项（不包含完整密钥）
type APIKeyListItem struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	KeyPrefix  string `json:"key_prefix"`
	LastUsedAt int64  `json:"last_used_at"`
	CreatedAt  int64  `json:"created_at"`
	IsActive   bool   `json:"is_active"`
}

// CreateAPIKey 创建新的API Key
func CreateAPIKey(c *gin.Context) {
	var req CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: fmt.Sprintf("参数错误：%v", err), Data: nil})
		return
	}

	// 获取当前登录用户
	if LoginedUser == nil {
		c.JSON(http.StatusUnauthorized, APIResponse[any]{Code: BadRequest, Message: "用户未登录", Data: nil})
		return
	}

	// 创建API Key
	apiKey, rawKey, err := models.CreateAPIKey(LoginedUser.ID, req.Name)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: fmt.Sprintf("创建API Key失败：%v", err), Data: nil})
		return
	}

	// 返回包含完整密钥的响应（仅此一次）
	resp := CreateAPIKeyResponse{
		ID:        apiKey.ID,
		Name:      apiKey.Name,
		Key:       rawKey, // 完整密钥仅返回一次
		KeyPrefix: apiKey.KeyPrefix,
		CreatedAt: apiKey.CreatedAt,
		IsActive:  apiKey.IsActive,
	}

	c.JSON(http.StatusOK, APIResponse[CreateAPIKeyResponse]{
		Code:    Success,
		Message: "API Key创建成功，请妥善保管密钥，此密钥仅显示一次",
		Data:    resp,
	})
}

// ListAPIKeys 获取当前用户的所有API Keys
func ListAPIKeys(c *gin.Context) {
	// 获取当前登录用户
	if LoginedUser == nil {
		c.JSON(http.StatusUnauthorized, APIResponse[any]{Code: BadRequest, Message: "用户未登录", Data: nil})
		return
	}

	// 查询用户的API Keys
	apiKeys, err := models.GetAPIKeysByUserID(LoginedUser.ID)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: fmt.Sprintf("查询API Keys失败：%v", err), Data: nil})
		return
	}

	// 转换为响应格式（不包含完整密钥）
	resp := make([]APIKeyListItem, 0, len(apiKeys))
	for _, apiKey := range apiKeys {
		resp = append(resp, APIKeyListItem{
			ID:         apiKey.ID,
			Name:       apiKey.Name,
			KeyPrefix:  apiKey.KeyPrefix,
			LastUsedAt: apiKey.LastUsedAt,
			CreatedAt:  apiKey.CreatedAt,
			IsActive:   apiKey.IsActive,
		})
	}

	c.JSON(http.StatusOK, APIResponse[[]APIKeyListItem]{
		Code:    Success,
		Message: "查询成功",
		Data:    resp,
	})
}

// DeleteAPIKey 删除API Key
func DeleteAPIKey(c *gin.Context) {
	// 获取当前登录用户
	if LoginedUser == nil {
		c.JSON(http.StatusUnauthorized, APIResponse[any]{Code: BadRequest, Message: "用户未登录", Data: nil})
		return
	}

	// 获取API Key ID
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "无效的API Key ID", Data: nil})
		return
	}

	// 删除API Key（确保只能删除自己的）
	err = models.DeleteAPIKey(uint(id), LoginedUser.ID)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: fmt.Sprintf("删除API Key失败：%v", err), Data: nil})
		return
	}

	c.JSON(http.StatusOK, APIResponse[any]{
		Code:    Success,
		Message: "删除成功",
		Data:    nil,
	})
}

// UpdateAPIKeyStatusRequest 更新API Key状态请求
type UpdateAPIKeyStatusRequest struct {
	IsActive bool `json:"is_active"`
}

// UpdateAPIKeyStatus 更新API Key的启用/禁用状态
func UpdateAPIKeyStatus(c *gin.Context) {
	// 获取当前登录用户
	if LoginedUser == nil {
		c.JSON(http.StatusUnauthorized, APIResponse[any]{Code: BadRequest, Message: "用户未登录", Data: nil})
		return
	}

	// 获取API Key ID
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "无效的API Key ID", Data: nil})
		return
	}

	// 解析请求体
	var req UpdateAPIKeyStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: fmt.Sprintf("参数错误：%v", err), Data: nil})
		return
	}

	// 更新API Key状态（确保只能更新自己的）
	err = models.UpdateAPIKeyStatus(uint(id), LoginedUser.ID, req.IsActive)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: fmt.Sprintf("更新API Key状态失败：%v", err), Data: nil})
		return
	}

	statusText := "禁用"
	if req.IsActive {
		statusText = "启用"
	}

	c.JSON(http.StatusOK, APIResponse[any]{
		Code:    Success,
		Message: fmt.Sprintf("API Key已%s", statusText),
		Data:    nil,
	})
}
