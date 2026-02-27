package controllers

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/synccron"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// StartSync 启动同步
// @Summary 启动同步任务
// @Description 启动全局同步任务并添加到队列
// @Tags 同步管理
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /sync/start [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func StartSync(c *gin.Context) {
	// 启动同步
	synccron.StartSyncCron()
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "同步任务已添加到队列", Data: nil})
}

// GetSyncRecords 获取同步记录列表
// @Summary 获取同步记录
// @Description 分页获取同步任务记录列表
// @Tags 同步管理
// @Accept json
// @Produce json
// @Param page query integer false "页码"
// @Param page_size query integer false "每页数量"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /sync/records [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetSyncRecords(c *gin.Context) {
	type syncRecordsRequest struct {
		Page     int `form:"page" json:"page" binding:"omitempty,min=1"`           // 页码，默认1
		PageSize int `form:"page_size" json:"page_size" binding:"omitempty,min=1"` // 每页数量，默认50
	}

	var req syncRecordsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: 400, Message: "请求参数错误", Data: nil})
		return
	}
	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}

	// 获取同步记录
	records, total, err := models.GetSyncRecords(page, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: 500, Message: "获取同步记录失败", Data: nil})
		return
	}

	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "获取同步记录成功", Data: map[string]interface{}{
		"records": records,
		"total":   total,
	}})
}

// GetSyncTask 获取同步任务详情
// @Summary 获取同步任务详情
// @Description 根据ID获取指定同步任务的详细信息
// @Tags 同步管理
// @Accept json
// @Produce json
// @Param sync_id query integer true "同步任务ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /sync/task [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetSyncTask(c *gin.Context) {
	type syncTaskRequest struct {
		SyncID uint `form:"sync_id" json:"sync_id" binding:"required"` // 同步任务ID
	}
	var req syncTaskRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: 400, Message: "请求参数错误", Data: nil})
		return
	}

	sync, err := models.GetSyncByID(req.SyncID)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: 500, Message: "获取同步任务失败", Data: nil})
		return
	}

	if sync == nil {
		c.JSON(http.StatusNotFound, APIResponse[any]{Code: 404, Message: "未找到对应的同步任务", Data: nil})
		return
	}

	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "获取同步任务详情成功", Data: sync})
}

// GetSyncPathList 获取同步路径列表
// @Summary 获取同步路径列表
// @Description 分页获取所有配置的同步路径
// @Tags 同步管理
// @Accept json
// @Produce json
// @Param page query integer false "页码"
// @Param page_size query integer false "每页数量"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /sync/path-list [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetSyncPathList(c *gin.Context) {
	type syncPathListRequest struct {
		Page       int               `form:"page" json:"page" binding:"omitempty,min=1"`           // 页码，默认1
		PageSize   int               `form:"page_size" json:"page_size" binding:"omitempty,min=1"` // 每页数量，默认20
		SourceType models.SourceType `form:"source_type" json:"source_type" binding:"omitempty"`   // 来源类型
	}
	var req syncPathListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "请求参数错误", Data: nil})
		return
	}
	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	syncPaths, total := models.GetSyncPathList(page, pageSize, false, req.SourceType)

	for _, sp := range syncPaths {
		status := synccron.CheckNewTaskStatus(sp.ID, synccron.SyncTaskTypeStrm)
		sp.IsRunning = int(status)
	}

	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "获取同步路径列表成功", Data: map[string]any{
		"list":      syncPaths,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	}})
}

type addSyncPathRequest struct {
	SourceType   models.SourceType `json:"source_type" form:"source_type" binding:"required"` // 来源类型
	AccountId    uint              `json:"account_id" form:"account_id"`                      // 网盘账号ID
	BaseCid      string            `json:"base_cid" form:"base_cid" binding:"required"`       // 来源路径ID或者本地路径
	LocalPath    string            `json:"local_path" form:"local_path" binding:"required"`   // 本地路径
	RemotePath   string            `json:"remote_path" form:"remote_path" binding:"required"` // 同步源路径，115网盘和123网盘需要该字段
	EnableCron   bool              `json:"enable_cron" form:"enable_cron"`                    // 是否启用定时任务
	CustomConfig bool              `json:"custom_config" form:"custom_config"`                // 自定义配置
	models.SettingStrm
}

// AddSyncPath 添加同步路径
// @Summary 添加同步路径
// @Description 创建新的同步路径配置
// @Tags 同步管理
// @Accept json
// @Produce json
// @Param source_type body integer true "来源类型"
// @Param account_id body integer false "网盘账号ID"
// @Param base_cid body string true "来源路径ID或本地路径"
// @Param local_path body string true "本地路径"
// @Param remote_path body string true "同步源路径"
// @Param enable_cron body boolean false "是否启用定时任务"
// @Param custom_config body boolean false "是否自定义配置"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /sync/path-add [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func AddSyncPath(c *gin.Context) {
	var req addSyncPathRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: fmt.Sprintf("请求参数错误: %v", err), Data: nil})
		return
	}
	baseCid := req.BaseCid
	localPath := req.LocalPath
	if req.SourceType != models.SourceTypeLocal {
		// 检查accountId是否存在
		account, accountErr := models.GetAccountById(req.AccountId)
		if accountErr != nil || account == nil {
			c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "账号不存在", Data: nil})
			return
		}
		// 检查来源类型是否正确
		if req.SourceType != account.SourceType {
			c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "账号类型与同步源类型不一致", Data: nil})
			return
		}
	}
	remotePath := req.RemotePath
	if req.SourceType != models.SourceTypeLocal {
		remotePath = strings.TrimPrefix(req.RemotePath, "/")
		remotePath = strings.TrimPrefix(req.RemotePath, "/")
		remotePath = filepath.ToSlash(filepath.Clean(req.RemotePath))
	}
	// 非windows+本地类型，remotePath需要以/开头
	if runtime.GOOS != "windows" && req.SourceType == models.SourceTypeLocal {
		if !strings.HasPrefix(remotePath, "/") {
			remotePath = "/" + remotePath
		}
	}
	// if req.SourceType == models.SourceTypeOpenList {
	// 	// 将remotepath中的\都替换为/
	// 	remotePath = strings.ReplaceAll(remotePath, "\\", "/")
	// 	baseCid = strings.ReplaceAll(req.BaseCid, "\\", "/")
	// }
	// 创建同步路径
	syncPath := models.CreateSyncPath(req.SourceType, req.AccountId, baseCid, localPath, remotePath, req.EnableCron, req.CustomConfig, req.SettingStrm)
	if syncPath == nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "创建同步路径失败", Data: nil})
		return
	}
	if syncPath.EnableCron && syncPath.Cron != "" {
		synccron.InitSyncCron()
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "添加同步路径成功", Data: syncPath})
}

// UpdateSyncPath 更新同步路径
// @Summary 更新同步路径
// @Description 更新已有的同步路径配置
// @Tags 同步管理
// @Accept json
// @Produce json
// @Param id body integer true "同步路径ID"
// @Param source_type body integer true "来源类型"
// @Param account_id body integer false "网盘账号ID"
// @Param base_cid body string true "来源路径ID或本地路径"
// @Param local_path body string true "本地路径"
// @Param remote_path body string true "同步源路径"
// @Param enable_cron body boolean false "是否启用定时任务"
// @Param custom_config body boolean false "是否自定义配置"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /sync/path-update [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func UpdateSyncPath(c *gin.Context) {
	type updateSyncPathRequest struct {
		ID uint `json:"id" form:"id"` // 同步路径ID
		addSyncPathRequest
	}
	var req updateSyncPathRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "请求参数错误", Data: nil})
		return
	}
	id := req.ID
	// 获取并更新同步路径
	syncPath := models.GetSyncPathById(uint(id))
	if syncPath == nil {
		c.JSON(http.StatusNotFound, APIResponse[any]{Code: BadRequest, Message: "同步路径不存在", Data: nil})
		return
	}
	oldCron := syncPath.Cron
	if req.SourceType != models.SourceTypeLocal {
		// 检查accountId是否存在
		account, err := models.GetAccountById(syncPath.AccountId)
		if err != nil {
			c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "账号不存在", Data: nil})
			return
		}
		// 检查来源类型是否正确
		if req.SourceType != account.SourceType {
			c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "账号类型与同步源类型不一致", Data: nil})
			return
		}
	}
	remotePath := req.RemotePath
	if req.SourceType != models.SourceTypeLocal {
		remotePath = strings.TrimPrefix(req.RemotePath, "/")
		remotePath = strings.TrimPrefix(req.RemotePath, "/")
		remotePath = filepath.ToSlash(filepath.Clean(req.RemotePath))
	}
	// 非windows+本地类型，remotePath需要以/开头
	if runtime.GOOS != "windows" && req.SourceType == models.SourceTypeLocal {
		if !strings.HasPrefix(remotePath, "/") {
			remotePath = "/" + remotePath
		}
	}
	if req.SourceType == models.SourceTypeOpenList {
		// 将remotepath中的\都替换为/
		req.RemotePath = strings.ReplaceAll(req.RemotePath, "\\", "/")
		req.BaseCid = strings.ReplaceAll(req.BaseCid, "\\", "/")
	}
	// helpers.AppLogger.Infof("更新同步路径 %d 定时任务: %s", syncPath.ID, req.Cron)
	success := syncPath.Update(req.SourceType, req.AccountId, req.BaseCid, req.LocalPath, remotePath, req.EnableCron, req.CustomConfig, req.SettingStrm)
	if !success {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "更新同步路径失败", Data: nil})
		return
	}
	if oldCron != syncPath.Cron {
		synccron.InitSyncCron()
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "更新同步路径成功", Data: syncPath})
}

// DeleteSyncPath 删除同步路径
// @Summary 删除同步路径
// @Description 根据ID删除指定的同步路径
// @Tags 同步管理
// @Accept json
// @Produce json
// @Param id body integer true "同步路径ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /sync/path-delete [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func DeleteSyncPath(c *gin.Context) {
	type deleteSyncPathRequest struct {
		ID uint `json:"id" binding:"required"` // 同步路径ID
	}
	var req deleteSyncPathRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "请求参数错误", Data: nil})
		return
	}
	id := req.ID
	if id == 0 {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "id 参数不能为空", Data: nil})
		return
	}
	// 删除同步路径
	success := models.DeleteSyncPathById(id)
	if !success {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "删除同步路径失败", Data: nil})
		return
	}
	synccron.InitSyncCron()
	synccron.InitCron()
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "删除同步路径成功", Data: nil})
}

// GetSyncPathById 根据ID获取同步路径详情
// @Summary 获取同步路径详情
// @Description 根据ID获取指定同步路径的详细配置
// @Tags 同步管理
// @Accept json
// @Produce json
// @Param id body integer true "同步路径ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /sync/path/:id [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetSyncPathById(c *gin.Context) {
	// 改成从路径参数获取ID
	idStr := c.Param("id")
	id := uint(helpers.StringToInt(idStr))
	if id == 0 {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "id 参数格式错误", Data: nil})
		return
	}

	syncPath := models.GetSyncPathById(uint(id))
	if syncPath == nil {
		c.JSON(http.StatusNotFound, APIResponse[any]{Code: BadRequest, Message: "同步路径不存在", Data: nil})
		return
	}

	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "获取同步路径详情成功", Data: syncPath})
}

// DelSyncRecords 批量删除同步记录
// @Summary 删除同步记录
// @Description 批量删除已完成或失败的同步记录
// @Tags 同步管理
// @Accept json
// @Produce json
// @Param ids body []integer true "同步记录ID列表"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /sync/delete-records [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func DelSyncRecords(c *gin.Context) {
	type delSyncRecordsRequest struct {
		IDs []uint `json:"ids" form:"ids" binding:"required"` // 同步路径ID列表
	}
	var req delSyncRecordsRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "请求参数错误", Data: nil})
		return
	}
	ids := req.IDs
	if len(ids) == 0 {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "没有选择删除的记录", Data: nil})
		return
	}
	for _, id := range ids {
		deleteErr := models.DeleteSyncRecordById(id)
		if deleteErr != nil {
			c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "删除同步记录失败: " + deleteErr.Error(), Data: nil})
			continue
		}
	}
	synccron.InitSyncCron()
	synccron.InitCron()
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "删除同步记录成功", Data: nil})
}

// StartSyncByPath 启动指定路径的同步任务
// @Summary 启动同步路径
// @Description 启动指定同步目录的同步任务
// @Tags 同步管理
// @Accept json
// @Produce json
// @Param id body integer true "同步路径ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /sync/path/start [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func StartSyncByPath(c *gin.Context) {
	type startSyncRequest struct {
		ID uint `form:"id" json:"id" binding:"required"` // 同步路径ID
	}
	var req startSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "请求参数错误", Data: nil})
		return
	}
	if req.ID == 0 {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "id 参数不能为空", Data: nil})
		return
	}

	id := req.ID
	syncPath := models.GetSyncPathById(uint(id))
	if syncPath == nil {
		c.JSON(http.StatusNotFound, APIResponse[any]{Code: BadRequest, Message: "同步路径不存在", Data: nil})
		return
	}
	// syncPath.SetIsFullSync(false)
	if err := synccron.AddNewSyncTask(syncPath.ID, synccron.SyncTaskTypeStrm); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "添加同步任务失败: " + err.Error(), Data: nil})
		return
	}

	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "同步任务已添加到队列", Data: nil})
}

// StopSyncByPath 停止指定路径的同步任务
// @Summary 停止同步路径
// @Description 停止指定同步目录的同步任务
// @Tags 同步管理
// @Accept json
// @Produce json
// @Param id body integer true "同步路径ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /sync/path/stop [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func StopSyncByPath(c *gin.Context) {
	type startSyncRequest struct {
		ID uint `form:"id" json:"id" binding:"required"` // 同步路径ID
	}
	var req startSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "请求参数错误", Data: nil})
		return
	}
	if req.ID == 0 {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "id 参数不能为空", Data: nil})
		return
	}

	id := req.ID
	syncPath := models.GetSyncPathById(uint(id))
	if syncPath == nil {
		c.JSON(http.StatusNotFound, APIResponse[any]{Code: BadRequest, Message: "同步路径不存在", Data: nil})
		return
	}
	// syncPath.SetIsFullSync(false)
	synccron.CancelNewSyncTask(syncPath.ID, synccron.SyncTaskTypeStrm)

	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "同步任务已添加到队列", Data: nil})
}

// ToggleSyncByPath 切换同步路径的定时任务
// @Summary 切换定时同步
// @Description 开启或关闭同步目录的定时同步任务
// @Tags 同步管理
// @Accept json
// @Produce json
// @Param id body integer true "同步路径ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /sync/path/toggle-cron [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func ToggleSyncByPath(c *gin.Context) {
	type stopSyncRequest struct {
		ID uint `form:"id" json:"id" binding:"required"` // 同步路径ID
	}
	var req stopSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "请求参数错误", Data: nil})
		return
	}
	if req.ID == 0 {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "id 参数不能为空", Data: nil})
		return
	}
	// 将enable参数设置为false
	syncPath := models.GetSyncPathById(req.ID)
	if syncPath == nil {
		c.JSON(http.StatusNotFound, APIResponse[any]{Code: BadRequest, Message: "同步路径不存在", Data: nil})
		return
	}
	syncPath.ToggleCron()
	synccron.InitCron()
	// 重启自定义定时任务
	if syncPath.Cron != "" {
		synccron.InitSyncCron()
	}
	if syncPath.EnableCron {
		c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "定时同步已开启", Data: nil})
	} else {
		c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "定时同步已关闭", Data: nil})
	}

}

// FullStart115Sync 启动115全量同步
// @Summary 启动115全量同步
// @Description 删除本地缓存数据并触发115的全量同步
// @Tags 同步管理
// @Accept json
// @Produce json
// @Param id body integer true "同步路径ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /sync/path/full-start [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func FullStart115Sync(c *gin.Context) {
	type startSyncRequest struct {
		ID uint `form:"id" json:"id" binding:"required"` // 同步路径ID
	}
	var req startSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "请求参数错误", Data: nil})
		return
	}
	if req.ID == 0 {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "id 参数不能为空", Data: nil})
		return
	}
	id := req.ID
	syncPath := models.GetSyncPathById(uint(id))
	if syncPath == nil {
		c.JSON(http.StatusNotFound, APIResponse[any]{Code: BadRequest, Message: "同步路径不存在", Data: nil})
		return
	}
	// 删除所有的数据库记录，重新查询接口
	// if syncPath.SourceType == models.SourceType115 {
	// 	// 清空数据表
	// 	if err := models.DeleteAllFileBySyncPathId(syncPath.ID); err != nil {
	// 		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "清空同步目录的数据表失败: " + err.Error(), Data: nil})
	// 		return
	// 	}
	// }
	syncPath.SetIsFullSync(true)
	if err := synccron.AddNewSyncTask(syncPath.ID, synccron.SyncTaskTypeStrm); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "添加同步任务失败: " + err.Error(), Data: nil})
		return
	}

	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "同步任务已添加到队列", Data: nil})
}

func SaveRelScrapePath(c *gin.Context) {
	type saveScrapePathRequest struct {
		ID           uint   `form:"id" json:"id" binding:"required"`                         // 同步路径ID
		ScrapePathId []uint `form:"scrape_path_id" json:"scrape_path_id" binding:"required"` // 刮削路径ID
	}
	var req saveScrapePathRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "请求参数错误", Data: nil})
		return
	}
	if req.ID == 0 {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "id 参数不能为空", Data: nil})
		return
	}
	syncPath := models.GetSyncPathById(req.ID)
	if syncPath == nil {
		c.JSON(http.StatusNotFound, APIResponse[any]{Code: BadRequest, Message: "同步路径不存在", Data: nil})
		return
	}
	// 保存关联的刮削路径
	if err := syncPath.SaveScrapePaths(req.ScrapePathId); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "保存关联的刮削路径失败: " + err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "关联的刮削路径保存成功", Data: nil})
}

func GetRelScrapePath(c *gin.Context) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "id 参数不能为空", Data: nil})
		return
	}
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "id 参数格式错误", Data: nil})
		return
	}
	syncPath := models.GetSyncPathById(uint(id))
	if syncPath == nil {
		c.JSON(http.StatusNotFound, APIResponse[any]{Code: BadRequest, Message: "同步路径不存在", Data: nil})
		return
	}
	// 获取关联的刮削路径
	scrapePathIds := syncPath.GetScrapePathIds()
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "关联的刮削路径获取成功", Data: scrapePathIds})
}
