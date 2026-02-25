package controllers

import (
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/synccron"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetEmbyConfig 获取Emby配置
// @Summary 获取Emby配置
// @Description 获取Emby媒体服务器的配置信息
// @Tags Emby管理
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /emby/config [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetEmbyConfig(c *gin.Context) {
	config, err := models.GetEmbyConfig()
	if err == gorm.ErrRecordNotFound {
		c.JSON(http.StatusOK, APIResponse[any]{
			Code:    Success,
			Message: "获取Emby配置成功",
			Data:    gin.H{"exists": false},
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "获取Emby配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{
		Code:    Success,
		Message: "获取Emby配置成功",
		Data:    gin.H{"exists": true, "config": config},
	})
}

type updateEmbyConfigRequest struct {
	EmbyUrl                 string `json:"emby_url"`
	EmbyApiKey              string `json:"emby_api_key"`
	EnableDeleteNetdisk     int    `json:"enable_delete_netdisk"`
	EnableRefreshLibrary    int    `json:"enable_refresh_library"`
	EnableMediaNotification int    `json:"enable_media_notification"`
	EnableExtractMediaInfo  int    `json:"enable_extract_media_info"`
	EnableAuth              int    `json:"enable_auth"`
	SyncEnabled             int    `json:"sync_enabled"`
	SyncCron                string `json:"sync_cron"`
}

// UpdateEmbyConfig 更新Emby配置
// @Summary 更新Emby配置
// @Description 更新Emby媒体服务器的配置信息
// @Tags Emby管理
// @Accept json
// @Produce json
// @Param emby_url body string false "Emby服务器地址"
// @Param emby_api_key body string false "Emby API密钥"
// @Param enable_delete_netdisk body integer false "是否启用网盘删除"
// @Param enable_refresh_library body integer false "是否启用库刷新"
// @Param enable_media_notification body integer false "是否启用媒体通知"
// @Param enable_extract_media_info body integer false "是否启用提取媒体信息"
// @Param enable_auth body integer false "是否启用Webhook鉴权"
// @Param sync_enabled body integer false "是否启用同步"
// @Param sync_cron body string false "同步Cron表达式"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /emby/config [put]
// @Security JwtAuth
// @Security ApiKeyAuth
func UpdateEmbyConfig(c *gin.Context) {
	var req updateEmbyConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "请求参数错误: " + err.Error()})
		return
	}

	config, err := models.GetEmbyConfig()
	if err != nil && err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "查询Emby配置失败: " + err.Error()})
		return
	}
	isNew := err == gorm.ErrRecordNotFound
	oldSyncEnabled := 0
	if !isNew {
		oldSyncEnabled = config.SyncEnabled
	}
	if isNew {
		config = &models.EmbyConfig{}
	}
	config.EmbyUrl = req.EmbyUrl
	config.EmbyApiKey = req.EmbyApiKey
	config.EnableDeleteNetdisk = req.EnableDeleteNetdisk
	config.EnableRefreshLibrary = req.EnableRefreshLibrary
	config.EnableMediaNotification = req.EnableMediaNotification
	config.EnableExtractMediaInfo = req.EnableExtractMediaInfo
	config.EnableAuth = req.EnableAuth
	config.SyncEnabled = req.SyncEnabled
	config.SyncCron = "0 * * * *"
	if config.SyncEnabled == 0 {
		config.EnableDeleteNetdisk = 0
		config.EnableRefreshLibrary = 0
	}

	if isNew {
		if err := db.Db.Create(config).Error; err != nil {
			c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "创建Emby配置失败: " + err.Error()})
			return
		}
	} else {
		updates := map[string]interface{}{
			"emby_url":                  config.EmbyUrl,
			"emby_api_key":              config.EmbyApiKey,
			"enable_delete_netdisk":     config.EnableDeleteNetdisk,
			"enable_refresh_library":    config.EnableRefreshLibrary,
			"enable_media_notification": config.EnableMediaNotification,
			"enable_extract_media_info": config.EnableExtractMediaInfo,
			"enable_auth":               config.EnableAuth,
			"sync_enabled":              config.SyncEnabled,
		}
		if err := config.Update(updates); err != nil {
			c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "更新Emby配置失败: " + err.Error()})
			return
		}
	}
	if oldSyncEnabled != config.SyncEnabled {
		// 同步状态改变，需要重新加载cron
		synccron.InitCron()
	}

	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "Emby配置更新成功"})
}
