package controllers

import (
	"Q115-STRM/internal/emby"
	embyclientrestgo "Q115-STRM/internal/embyclient-rest-go"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// StartEmbySync 手动触发同步
// @Summary 启动Emby同步
// @Description 手动触发Emby媒体库同步任务
// @Tags Emby管理
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /emby/sync-start [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func StartEmbySync(c *gin.Context) {
	// 检查是否已有任务在运行
	if emby.IsEmbySyncRunning() {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "已有Emby同步任务正在运行，请稍候"})
		return
	}

	go func() {
		if _, err := emby.PerformEmbySync(); err != nil {
			helpers.AppLogger.Warnf("Emby同步失败: %v", err)
		}
	}()
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "Emby同步任务已启动"})
}

// GetEmbySyncStatus 同步状态
// @Summary 获取Emby同步状态
// @Description 获取Emby媒体库同步的当前状态和信息
// @Tags Emby管理
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /emby/sync-status [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetEmbySyncStatus(c *gin.Context) {
	config, err := models.GetEmbyConfig()
	if err == gorm.ErrRecordNotFound {
		c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "尚未配置Emby", Data: gin.H{"exists": false}})
		return
	}
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "获取配置失败: " + err.Error()})
		return
	}
	helpers.AppLogger.Infof("获取Emby同步状态，最后同步时间: %d", config.LastSyncTime)
	total, _ := models.GetEmbyMediaItemsCount()
	c.JSON(http.StatusOK, APIResponse[any]{
		Code:    Success,
		Message: "获取同步状态成功",
		Data:    gin.H{"last_sync_time": config.LastSyncTime, "sync_cron": config.SyncCron, "total_items": total, "sync_enabled": config.SyncEnabled, "is_running": emby.IsEmbySyncRunning()},
	})
}

// GetEmbyLibraries 获取所有可用的Emby媒体库
// @Summary 获取Emby媒体库列表
// @Description 获取所有可用的Emby媒体库列表
// @Tags Emby管理
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /emby/libraries [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetEmbyLibraries(c *gin.Context) {
	config, err := models.GetEmbyConfig()
	if err == gorm.ErrRecordNotFound {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "尚未配置Emby"})
		return
	}
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "获取Emby配置失败: " + err.Error()})
		return
	}
	if config.EmbyUrl == "" || config.EmbyApiKey == "" {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "Emby Url或ApiKey为空"})
		return
	}

	// 直接从 Emby 查询媒体库，并写入本地 emby_libraries 表
	client := embyclientrestgo.NewClient(config.EmbyUrl, config.EmbyApiKey)
	libs, err := client.GetAllMediaLibraries()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "查询Emby媒体库失败: " + err.Error()})
		return
	}
	if err := models.UpsertEmbyLibraries(libs); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "写入媒体库表失败: " + err.Error()})
		return
	}

	// 清理已不在Emby中存在的媒体库记录
	activeLibraryIds := make([]string, 0, len(libs))
	for _, lib := range libs {
		activeLibraryIds = append(activeLibraryIds, lib.ID)
	}
	if err := models.CleanupDeletedEmbyLibraries(activeLibraryIds); err != nil {
		helpers.AppLogger.Warnf("清理已删除媒体库记录失败: %v", err)
	}

	libraries, err := models.GetAllEmbyLibraries()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "获取媒体库列表失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "获取媒体库列表成功", Data: libraries})
}
