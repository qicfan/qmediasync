package controllers

import (
	"Q115-STRM/internal/emby"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/synccron"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// func UpdateEmby(c *gin.Context) {
// 	type updateEmbyRequest struct {
// 		EmbyUrl    string `form:"emby_url" json:"emby_url"`         // Emby Url
// 		EmbyApiKey string `form:"emby_api_key" json:"emby_api_key"` // Emby API Key
// 	}
// 	// 获取请求参数
// 	var req updateEmbyRequest
// 	if err := c.ShouldBind(&req); err != nil {
// 		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "请求参数错误: " + err.Error(), Data: nil})
// 		return
// 	}
// 	// 更新设置
// 	if !models.SettingsGlobal.UpdateEmbyUrl(req.EmbyUrl, req.EmbyApiKey) {
// 		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "更新Emby Url失败", Data: nil})
// 		return
// 	}

// 	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "更新Emby Url成功", Data: nil})
// }

// func GetEmby(c *gin.Context) {
// 	// 获取设置
// 	models.LoadSettings() // 确保设置已加载
// 	emby := make(map[string]string)
// 	emby["emby_url"] = models.GlobalEmbyConfig.EmbyUrl
// 	emby["emby_api_key"] = models.GlobalEmbyConfig.EmbyApiKey
// 	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "获取Emby设置成功", Data: emby})
// }

// ParseEmby 手动解析Emby媒体信息
// @Summary 解析Emby媒体信息
// @Description 手动触发Emby媒体信息解析任务
// @Tags 系统设置
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/emby/parse [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func ParseEmby(c *gin.Context) {
	if models.GlobalEmbyConfig.EmbyUrl == "" || models.GlobalEmbyConfig.EmbyApiKey == "" {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "Emby Url和Emby API Key没有填写，无法提取媒体信息", Data: nil})
		return
	}
	if emby.EmbyMediaInfoStart {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "Emby媒体信息解析任务已在运行", Data: nil})
		return
	}
	emby.StartParseEmbyMediaInfo()
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "解析Emby媒体信息成功", Data: nil})
}

// UpdateTelegram 更新Telegram Bot配置
// @Summary 更新Telegram配置
// @Description 启用或配置Telegram通知Bot
// @Tags 系统设置
// @Accept json
// @Produce json
// @Param enabled body integer true "是否启用，1启用 0禁用"
// @Param token body string false "Telegram Bot Token"
// @Param chat_id body string false "Telegram Chat ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/telegram [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func UpdateTelegram(c *gin.Context) {
	type updateTelegramRequest struct {
		Enabled int    `form:"enabled" json:"enabled"` // 是否启用Telegram通知，"1"表示启用，"0"表示禁用
		Token   string `form:"token" json:"token"`     // Telegram Bot的Token
		ChatId  string `form:"chat_id" json:"chat_id"` // Telegram Chat ID
	}
	// 获取请求参数
	var req updateTelegramRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "请求参数错误: " + err.Error(), Data: nil})
		return
	}
	enabled := req.Enabled == 1
	token := req.Token
	chatId := req.ChatId

	// 如果启用Telegram，则需要验证token和chatId
	if enabled && (token == "" || chatId == "") {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "启用Telegram通知时，Token和Chat ID不能为空", Data: nil})
		return
	}
	// 更新设置
	if !models.SettingsGlobal.UpdateTelegramBot(enabled, token, chatId) {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "更新Telegram Bot设置失败", Data: nil})
		return
	}

	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "更新Telegram Bot设置成功", Data: nil})
}

// GetTelegram 获取Telegram Bot配置
// @Summary 获取Telegram配置
// @Description 获取当前的Telegram Bot通知配置
// @Tags 系统设置
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/telegram [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetTelegram(c *gin.Context) {
	// 获取设置
	models.LoadSettings() // 确保设置已加载
	telegramBot := make(map[string]string)
	if models.SettingsGlobal.UseTelegram == 1 {
		telegramBot["enabled"] = "1"
	} else {
		telegramBot["enabled"] = "0"
	}
	telegramBot["token"] = models.SettingsGlobal.TelegramBotToken
	telegramBot["chat_id"] = models.SettingsGlobal.TelegramChatId
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "获取Telegram Bot设置成功", Data: telegramBot})
}

// UpdateHttpProxy 更新HTTP代理设置
// @Summary 更新HTTP代理
// @Description 更新系统使用的HTTP代理配置
// @Tags 系统设置
// @Accept json
// @Produce json
// @Param http_proxy body string false "HTTP代理地址"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/http-proxy [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func UpdateHttpProxy(c *gin.Context) {
	type updateHttpProxyRequest struct {
		HttpProxy string `form:"http_proxy" json:"http_proxy"` // HTTP代理地址
	}
	var req updateHttpProxyRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "请求参数错误: " + err.Error(), Data: nil})
		return
	}
	httpProxy := req.HttpProxy
	// 更新设置
	if !models.SettingsGlobal.UpdateHttpProxy(httpProxy) {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "更新HTTP代理设置失败", Data: nil})
		return
	}

	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "更新HTTP代理设置成功", Data: nil})
}

// GetHttpProxy 获取HTTP代理设置
// @Summary 获取HTTP代理
// @Description 获取当前生效的HTTP代理配置
// @Tags 系统设置
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/http-proxy [get]
// @Security JwtAuth
// @Security ApiKeyAuth
// GetHttpProxy 获取HTTP代理设置
// @Summary 获取HTTP代理
// @Description 获取当前系统配置的HTTP代理
// @Tags 系统设置
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/http-proxy [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetHttpProxy(c *gin.Context) {
	// 获取设置
	models.LoadSettings() // 确保设置已加载
	httpProxy := make(map[string]string)
	httpProxy["http_proxy"] = models.SettingsGlobal.HttpProxy
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "获取HTTP代理设置成功", Data: httpProxy})
}

// TestHttpProxy 测试HTTP代理连接
// @Summary 测试HTTP代理
// @Description 测试指定HTTP代理的连接有效性
// @Tags 系统设置
// @Accept json
// @Produce json
// @Param http_proxy body string true "HTTP代理地址"
// @Param detailed body integer false "是否返回详细测试结果，1返回 0不返回"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/test-http-proxy [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func TestHttpProxy(c *gin.Context) {
	type testHttpProxyRequest struct {
		HttpProxy string `form:"http_proxy" json:"http_proxy" binding:"required"` // HTTP代理地址
		Detailed  int    `form:"detailed" json:"detailed"`                        // 是否返回详细测试结果，"1"表示返回，"0"表示不返回
	}
	var req testHttpProxyRequest
	// 获取请求参数
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "请求参数错误: " + err.Error(), Data: nil})
		return
	}
	httpProxy := req.HttpProxy
	detailed := req.Detailed == 1

	// 数据校验
	if httpProxy == "" {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "HTTP代理地址不能为空", Data: nil})
		return
	}

	if detailed {
		// 使用高级测试，返回详细结果
		result, err := helpers.TestHttpProxyAdvanced(httpProxy)
		if err != nil {
			c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "连接失败: " + err.Error(), Data: nil})
			return
		}

		if result.Success {
			c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "HTTP代理连接测试成功", Data: result})
		} else {
			c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "连接失败: " + result.ErrorMessage, Data: nil})
		}
	} else {
		// 使用简单测试
		success, err := helpers.TestHttpProxy(httpProxy)
		if err != nil {
			c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "连接失败: " + err.Error(), Data: nil})
			return
		}

		if success {
			c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "HTTP代理连接测试成功", Data: nil})
		} else {
			c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "HTTP代理连接测试失败", Data: nil})
		}
	}
}

// TestTelegram 测试Telegram Bot连接
// @Summary 测试Telegram连接
// @Description 测试指定Telegram Bot的连接有效性
// @Tags 系统设置
// @Accept json
// @Produce json
// @Param token body string true "Telegram Bot Token"
// @Param chat_id body string true "Telegram Chat ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /telegram/test [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func TestTelegram(c *gin.Context) {
	type testTelegramRequest struct {
		Token  string `form:"token" json:"token" binding:"required"`     // Telegram Bot的Token
		ChatId string `form:"chat_id" json:"chat_id" binding:"required"` // Telegram Chat ID
	}
	// 获取请求参数
	var req testTelegramRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "请求参数错误: " + err.Error(), Data: nil})
		return
	}
	token := req.Token
	chatId := req.ChatId

	// 数据校验
	if token == "" {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "Telegram Bot Token不能为空", Data: nil})
		return
	}
	if chatId == "" {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "Telegram Chat ID不能为空", Data: nil})
		return
	}

	// 测试Telegram机器人连接
	err := helpers.TestTelegramBot(token, chatId, models.SettingsGlobal.HttpProxy)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "连接失败: " + err.Error(), Data: nil})
		return
	}

	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "Telegram机器人连接测试成功", Data: nil})
}

// GetStrmConfig 获取STRM配置
// @Summary 获取STRM配置
// @Description 获取STRM同步相关的配置项
// @Tags 系统设置
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/strm-config [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetStrmConfig(c *gin.Context) {
	// 获取设置
	models.LoadSettings() // 确保设置已加载
	strmConfig := make(map[string]interface{})
	strmConfig["strm_base_url"] = models.SettingsGlobal.StrmBaseUrl
	// 如果strmConfig["strm_base_url"]以/结尾，那删除掉末尾的/
	if strings.HasSuffix(strmConfig["strm_base_url"].(string), "/") {
		strmConfig["strm_base_url"] = strings.TrimSuffix(strmConfig["strm_base_url"].(string), "/")
	}
	if models.SettingsGlobal.Cron == "" {
		strmConfig["cron"] = helpers.GlobalConfig.Strm.Cron // 使用默认配置
	} else {
		strmConfig["cron"] = models.SettingsGlobal.Cron
	}
	if models.SettingsGlobal.MetaExt != "" {
		strmConfig["meta_ext"] = models.SettingsGlobal.MetaExtArr
	} else {
		// 从config.yml中读取默认的metaExt
		strmConfig["meta_ext"] = helpers.GlobalConfig.Strm.MetaExt
	}
	if models.SettingsGlobal.VideoExt != "" {
		strmConfig["video_ext"] = models.SettingsGlobal.VideoExtArr
	} else {
		// 从config.yml中读取默认的视频扩展名
		strmConfig["video_ext"] = helpers.GlobalConfig.Strm.VideoExt
	}
	strmConfig["exclude_name"] = models.SettingsGlobal.ExcludeNameArr
	strmConfig["min_video_size"] = models.SettingsGlobal.MinVideoSize
	strmConfig["upload_meta"] = models.SettingsGlobal.UploadMeta
	strmConfig["delete_dir"] = models.SettingsGlobal.DeleteDir
	strmConfig["local_proxy"] = models.SettingsGlobal.LocalProxy
	strmConfig["exclude_name"] = models.SettingsGlobal.ExcludeNameArr
	strmConfig["download_meta"] = models.SettingsGlobal.DownloadMeta
	strmConfig["add_path"] = models.SettingsGlobal.AddPath
	strmConfig["check_meta_mtime"] = models.SettingsGlobal.CheckMetaMtime
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "获取STRM配置成功", Data: strmConfig})
}

// UpdateStrmConfig 更新STRM配置
// @Summary 更新STRM配置
// @Description 更新STRM同步相关的配置项（包括URL、Cron、扩展名等）
// @Tags 系统设置
// @Accept json
// @Produce json
// @Param strm_base_url body string true "STRM基础URL"
// @Param cron body string true "Cron表达式"
// @Param meta_ext body []string true "元数据扩展名"
// @Param video_ext body []string true "视频扩展名"
// @Param min_video_size body integer false "最小视频大小（MB）"
// @Param upload_meta body integer false "是否上传元数据，1上传 0不上传"
// @Param delete_dir body integer false "是否删除空目录，1删除 0不删除"
// @Param local_proxy body integer false "是否启用本地代理，1启用 0禁用"
// @Param exclude_name body []string false "排除的文件名"
// @Param download_meta body integer false "是否下载元数据，1下载 0不下载"
// @Param add_path body integer false "是否添加路径，1添加 2不添加"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/strm-config [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func UpdateStrmConfig(c *gin.Context) {
	type updateStrmConfigRequest struct {
		StrmBaseUrl    string   `form:"strm_base_url" json:"strm_base_url" binding:"required"` // STRM基础URL
		Cron           string   `form:"cron" json:"cron" binding:"required"`                   // Cron表达式
		MetaExt        []string `form:"meta_ext" json:"meta_ext" binding:"required"`           // 元数据扩展名，JSON数组字符串格式，例如：["nfo","txt"]
		VideoExt       []string `form:"video_ext" json:"video_ext" binding:"required"`         // 视频扩展名，JSON数组字符串格式，例如：["mp4","mkv"]
		MinVideoSize   int64    `form:"min_video_size" json:"min_video_size"`                  // 最小视频大小，单位：MB
		UploadMeta     int      `form:"upload_meta" json:"upload_meta"`                        // 是否上传元数据文件，"1"表示上传，"0"表示不上传
		DeleteDir      int      `form:"delete_dir" json:"delete_dir"`                          // 是否删除空目录，"1"表示删除，"0"表示不删除
		LocalProxy     int      `form:"local_proxy" json:"local_proxy"`                        // 是否启用本地代理，"1"表示启用，"0"表示禁用
		ExcludeName    []string `form:"exclude_name" json:"exclude_name"`                      // 排除的文件名，JSON数组字符串格式，例如：["sample","test"]
		DownloadMeta   int      `form:"download_meta" json:"download_meta"`                    // 是否下载元数据文件，"1"表示下载，"0"表示不下载
		AddPath        int      `form:"add_path" json:"add_path"`                              // 是否添加路径，1- 表示添加路径， 2-表示不添加路径
		CheckMetaMtime int      `form:"check_meta_mtime" json:"check_meta_mtime"`              // 是否检查元数据修改时间，"1"表示检查，"0"表示不检查
	}
	// 获取请求参数
	var req updateStrmConfigRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "请求参数错误: " + err.Error(), Data: nil})
		return
	}
	oldCron := models.SettingsGlobal.Cron
	// 获取请求参数
	strmBaseUrl := req.StrmBaseUrl
	cron := req.Cron
	metaExt := req.MetaExt
	videoExt := req.VideoExt
	minVideoSize := req.MinVideoSize
	uploadMeta := req.UploadMeta
	deleteDir := req.DeleteDir
	localProxy := req.LocalProxy
	excludeName := req.ExcludeName
	downloadMeta := req.DownloadMeta
	checkMetaMtime := req.CheckMetaMtime
	addPath := req.AddPath
	// 数据校验
	if strmBaseUrl == "" {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "STRM基础URL不能为空", Data: nil})
		return
	}
	if cron == "" {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "Cron表达式不能为空", Data: nil})
		return
	}
	if minVideoSize < 0 {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "最小视频大小必须大于等于0", Data: nil})
		return
	}
	// 检查cron是否正确，并且不能小于1小时一次
	runTimes := helpers.GetNextTimeByCronStr(cron, 2)
	if runTimes == nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "Cron表达式格式不正确", Data: nil})
		return
	}
	// 更新设置
	if !models.SettingsGlobal.UpdateStrm(strmBaseUrl, cron, metaExt, videoExt, minVideoSize, uploadMeta, deleteDir, localProxy, excludeName, downloadMeta, addPath, checkMetaMtime) {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "更新STRM配置失败", Data: nil})
		return
	}
	if oldCron != models.SettingsGlobal.Cron {
		// 如果Cron发生变化，重启任务
		synccron.InitCron()
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "更新STRM配置成功", Data: nil})
}

// GetCronNextTime 获取Cron表达式的下次执行时间
// @Summary 获取Cron执行时间
// @Description 计算Cron表达式的下5次执行时间
// @Tags 系统设置
// @Accept json
// @Produce json
// @Param cron query string true "Cron表达式"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/cron [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetCronNextTime(c *gin.Context) {
	type getCronNextTimeRequest struct {
		Cron string `form:"cron" json:"cron" binding:"required"` // Cron表达式
	}
	var req getCronNextTimeRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "请求参数错误: " + err.Error(), Data: nil})
		return
	}
	times := helpers.GetNextTimeByCronStr(req.Cron, 5)
	if times == nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "Cron表达式格式不正确", Data: nil})
		return
	}
	var timeStrs []string
	for _, t := range times {
		timeStrs = append(timeStrs, t.Format("2006-01-02 15:04:05"))
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "获取下次执行时间成功", Data: timeStrs})
}

// GetThreads 获取线程配置
// @Summary 获取线程数配置
// @Description 获取当前下载和文件详情查询的线程数配置
// @Tags 系统设置
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/threads [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetThreads(c *gin.Context) {
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "获取线程数成功", Data: models.SettingsGlobal.GetThreads()})
}

// UpdateThreads 更新线程配置
// @Summary 更新线程数配置
// @Description 更新下载和文件详情查询的线程数
// @Tags 系统设置
// @Accept json
// @Produce json
// @Param download_threads body integer true "下载QPS"
// @Param file_detail_threads body integer true "115接口QPS"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /setting/threads [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func UpdateThreads(c *gin.Context) {
	type updateThreadsRequest struct {
		DownloadThreads    int `form:"download_threads" json:"download_threads" binding:"required"`         // 下载QPS
		FileDetailThreads  int `form:"file_detail_threads" json:"file_detail_threads" binding:"required"`   // 115接口QPS
		OpenlistQPS        int `form:"openlist_qps" json:"openlist_qps" binding:"required"`                 // OpenList QPS
		OpenlistRetry      int `form:"openlist_retry" json:"openlist_retry" binding:"required"`             // OpenList 重试次数
		OpenlistRetryDelay int `form:"openlist_retry_delay" json:"openlist_retry_delay" binding:"required"` // OpenList 重试间隔，单位秒
	}
	var req updateThreadsRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse[any]{Code: BadRequest, Message: "请求参数错误: " + err.Error(), Data: nil})
		return
	}
	downloadThreads := req.DownloadThreads
	fileDetailThreads := req.FileDetailThreads
	// 更新设置，传递当前的百度网盘限速值
	if !models.SettingsGlobal.UpdateThreads(
		downloadThreads, 
		fileDetailThreads, 
		req.OpenlistQPS, 
		req.OpenlistRetry, 
		req.OpenlistRetryDelay,
		models.SettingsGlobal.BaiDuPanQPS,
		models.SettingsGlobal.BaiDuPanQPM,
		models.SettingsGlobal.BaiDuPanQPH,
		models.SettingsGlobal.BaiDuPanQPT,
	) {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "更新线程数失败", Data: nil})
		return
	}

	// 动态更新下载队列的并发数
	models.UpdateGlobalDownloadQueueConcurrency(downloadThreads)

	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "更新线程数成功", Data: nil})
}
