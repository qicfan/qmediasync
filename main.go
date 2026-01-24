package main

import (
	"Q115-STRM/emby302/config"
	"Q115-STRM/emby302/util/logs/colors"
	"Q115-STRM/emby302/web"
	"Q115-STRM/internal/controllers"
	"Q115-STRM/internal/db"
	dbConfig "Q115-STRM/internal/db/config"
	"Q115-STRM/internal/db/database"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/synccron"
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

var Version string = "v0.0.1"
var PublishDate string = "2025-08-08"
var QMSApp *App

type App struct {
	isRelease   bool
	rootDir     string
	dbManager   *database.Manager
	config      *dbConfig.Config
	httpServer  *http.Server
	httpsServer *http.Server
	version     string
	publishDate string
}

func (app *App) Start() {
	// 启动外网302服务
	StartEmby302()
	// if helpers.IsRelease {
	gin.SetMode(gin.ReleaseMode)
	// }
	r := gin.New()
	r.Use(controllers.Cors())
	setRouter(r)
	app.StartHttpServer(r)
	app.StartHttpsServer(r)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("收到停止信号")

	// 停止应用
	app.Stop()
	log.Println("应用程序正常退出")
}

func (app *App) Stop() {
	helpers.CloseLogger() // 关闭日志
	// 关闭上传下载队列
	models.GlobalDownloadQueue.Stop()
	models.GlobalUploadQueue.Stop()
	// 关闭数据库
	if app.dbManager != nil {
		app.dbManager.Stop()
	}
	if app.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := app.httpServer.Shutdown(ctx); err != nil {
			log.Println("HTTP Server Shutdown:", err)
		}
	}
	if app.httpsServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := app.httpsServer.Shutdown(ctx); err != nil {
			log.Println("HTTPS Server Shutdown:", err)
		}
	}
}

func (app *App) StartHttpsServer(r *gin.Engine) {
	certFile := filepath.Join(helpers.RootDir, "config", "server.crt")
	keyFile := filepath.Join(helpers.RootDir, "config", "server.key")
	host := helpers.GlobalConfig.WebHost
	if !helpers.PathExists(certFile) || !helpers.PathExists(keyFile) {
		return
	}
	go func() {
		// 在12332端口上启动https服务
		sslHost := ""
		// 启动web server
		if !helpers.IsRelease {
			sslHost = "localhost:12332"
		} else {
			// 将12333替换为12332
			sslHost = strings.Replace(host, "12333", "12332", 1)
		}
		app.httpsServer = &http.Server{
			Addr:    sslHost,
			Handler: r,
		}
		// 没有证书则回退到普通 HTTP
		weberr := app.httpsServer.ListenAndServeTLS(certFile, keyFile)
		if weberr != nil {
			fmt.Println("ListenAndServe error:", weberr)
		}
	}()
}

func (app *App) StartHttpServer(r *gin.Engine) {
	host := helpers.GlobalConfig.WebHost
	// 同时在12333端口上启动http服务
	app.httpServer = &http.Server{
		Addr:    host,
		Handler: r,
	}
	go func() {
		weberr := app.httpServer.ListenAndServe()
		if weberr != nil {
			fmt.Println("ListenAndServe error:", weberr)
		}
	}()
}

func (a *App) getDBMode() string {
	if a.config.App.Mode == "docker" || a.config.DB.External || runtime.GOOS == "linux" {
		return "external"
	}
	return "embedded"
}

func (app *App) StartDatabase() error {
	// 初始化数据库管理器
	dbConfig := &database.Config{
		Mode:         app.getDBMode(),
		Host:         app.config.DB.Host,
		Port:         app.config.DB.Port,
		User:         app.config.DB.User,
		Password:     app.config.DB.Password,
		DBName:       app.config.DB.Name,
		SSLMode:      app.config.DB.SSLMode,
		LogDir:       app.config.DB.LogDir,
		DataDir:      app.config.DB.DataDir,
		BinaryPath:   app.config.DB.BinaryPath,
		MaxOpenConns: app.config.DB.MaxOpenConns,
		MaxIdleConns: app.config.DB.MaxIdleConns,
		External:     app.config.DB.External,
	}

	app.dbManager = database.NewManager(dbConfig)

	// 启动数据库
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := app.dbManager.Start(ctx); err != nil {
		return err
	}
	db.InitDb(app.dbManager.GetDB())
	// 设置全局管理器引用供其他包使用
	db.Manager = app.dbManager
	// 开始数据库版本维护
	models.Migrate()
	return nil
}

func NewApp(rootDir string) {
	if QMSApp != nil {
		log.Println("App already initialized")
		return
	}
	// 初始化APP
	QMSApp = &App{
		isRelease:   helpers.IsRelease,
		rootDir:     helpers.RootDir,
		version:     Version,
		publishDate: PublishDate,
	}
	QMSApp.config = dbConfig.Load(rootDir)
}

func initTimeZone() {
	cstZone := time.FixedZone("CST", 8*3600)
	time.Local = cstZone
}

func CheckRelease() {
	if helpers.IsRunningInDocker() {
		helpers.IsRelease = true
	}
	arg1 := strings.ToLower(os.Args[0])
	// fmt.Printf("arg1=%s\n", arg1)
	name := strings.ToLower(filepath.Base(arg1))
	// fmt.Printf("name=%s\n", name)
	helpers.IsRelease = strings.Index(name, "qmediasync") == 0 && !strings.Contains(arg1, "go-build")
}

func GetRootDir() string {
	var exPath string = "/app" // 默认使用docker的路径
	CheckRelease()
	if helpers.IsRelease {
		ex, err := os.Executable()
		if err != nil {
			panic(err)
		}
		exPath = filepath.Dir(ex)
	} else {
		if runtime.GOOS == "windows" {
			exPath = "D:\\Dev\\q115-strm-go"
		} else {
			exPath = "/home/qicfan/dev/q115-strm-go"
		}
	}
	helpers.RootDir = exPath // 获取当前工作目录
	return exPath
}

//go:embed emby302.yml
var s string

func StartEmby302() {
	dataRoot := filepath.Join(helpers.RootDir, "config")
	if err := config.ReadFromFile([]byte(s)); err != nil {
		log.Fatal(err)
	}
	if models.GlobalEmbyConfig == nil || models.GlobalEmbyConfig.EmbyUrl == "" {
		helpers.AppLogger.Warnf("Emby302未配置Emby地址，跳过启动emby302服务")
		return
	}
	config.C.Emby.Host = models.GlobalEmbyConfig.EmbyUrl
	config.C.Emby.EpisodesUnplayPrior = false // 关闭剧集排序
	certFile := filepath.Join(helpers.RootDir, "config", "server.crt")
	keyFile := filepath.Join(helpers.RootDir, "config", "server.key")
	if helpers.PathExists(certFile) && helpers.PathExists(keyFile) {
		config.C.Ssl.Enable = true
		config.C.Ssl.SinglePort = false
		config.C.Ssl.Crt = "server.crt"
		config.C.Ssl.Key = "server.key"
	}
	config.BasePath = dataRoot
	config.C.Emby.LocalMediaRoot = "/"
	config.C.VideoPreview.Enable = true
	config.C.VideoPreview.Containers = []string{"strm"}
	go func() {
		if err := web.Listen(); err != nil {
			log.Fatal(colors.ToRed(err.Error()))
		}
	}()

}

func InitLogger() {
	logPath := filepath.Join(helpers.RootDir, "config", "logs")
	os.MkdirAll(logPath, 0755) // 如果没有logs目录则创建
	libLogPath := filepath.Join(logPath, "libs")
	os.MkdirAll(libLogPath, 0755) // 如果没有logs/libs目录则创建
	helpers.AppLogger = helpers.NewLogger(helpers.GlobalConfig.Log.File, true, true)
	helpers.V115Log = helpers.NewLogger(helpers.GlobalConfig.Log.V115, false, true)
	helpers.OpenListLog = helpers.NewLogger(helpers.GlobalConfig.Log.OpenList, false, true)
	helpers.TMDBLog = helpers.NewLogger(helpers.GlobalConfig.Log.TMDB, false, true)
}

func InitOthers() {
	helpers.InitEventBus()           // 初始化事件总线
	models.LoadSettings()            // 从数据库加载设置
	models.LoadScrapeSettings()      // 从数据库加载刮削设置
	models.InitDQ()                  // 初始化下载队列
	models.InitUQ()                  // 初始化上传队列
	models.InitNotificationManager() // 初始化通知管理器
	models.GetEmbyConfig()           // 加载Emby配置
	// helpers.SubscribeSync(helpers.Save115TokenEvent, models.HandleOpen115TokenSaveSync)
	helpers.SubscribeSync(helpers.V115TokenInValidEvent, models.HandleV115TokenInvalid)
	helpers.SubscribeSync(helpers.SaveOpenListTokenEvent, models.HandleOpenListTokenSaveSync)
	models.FailAllRunningSyncTasks() // 将所有运行中的同步任务设置为失败状态
	synccron.Refresh115AccessToken() // 启动时刷新一次115的访问凭证，防止有过期的token导致同步失败
	// 清理应用启动时的未完成备份任务
	cleanupIncompleteBackupTasks()
	// if helpers.IsRelease {
	synccron.InitCron() // 初始化定时任务
	// }
	// 将所有刮削中和整理中的记录改为未执行
	models.ResetScrapePathStatus()
	// 将所有刮削中改为待刮削
	models.UpdateScrapeMediaStatus(models.ScrapeMediaStatusScraping, models.ScrapeMediaStatusScanned, 0)
	// 将所有整理中的记录改为待整理
	models.UpdateScrapeMediaStatus(models.ScrapeMediaStatusRenaming, models.ScrapeMediaStatusScraped, 0)
	// 上传中的任务改为待上传
	models.UpdateUploadingToPending()
	// 下载中的任务改为待下载
	models.UpdateDownloadingToPending()
}

// 设置路由
func setRouter(r *gin.Engine) {
	r.LoadHTMLFiles(filepath.Join(helpers.RootDir, "web_statics", "index.html"))
	r.StaticFile("/favicon.ico", filepath.Join(helpers.RootDir, "web_statics", "favicon.ico"))
	r.StaticFS("/assets", http.Dir(filepath.Join(helpers.RootDir, "web_statics", "assets")))
	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", gin.H{})
	})
	r.POST("/emby/webhook", controllers.Webhook)
	r.POST("/api/login", controllers.LoginAction)
	r.GET("/115/url/*filename", controllers.Get115UrlByPickCode) // 查询115直链 by pickcode 支持iso，路径最后一部分是.扩展名格式
	r.GET("/115/newurl", controllers.Get115UrlByPickCode)        // 查询115直链 by pickcode
	r.GET("/openlist/url", controllers.GetOpenListFileUrl)       // 查询OpenList直链

	r.GET("/proxy-115", controllers.Proxy115)                            // 115CDN反代路由
	r.GET("/api/scrape/tmp-image", controllers.ScrapeTmpImage)           // 获取临时图片
	r.GET("/api/scrape/records/export", controllers.ExportScrapeRecords) // 导出刮削记录
	r.GET("/api/logs/ws", controllers.LogWebSocket)                      // WebSocket日志查看
	r.GET("/api/logs/old", controllers.GetOldLogs)                       // HTTP获取旧日志
	r.GET("/api/logs/download", controllers.DownloadLogFile)             // 下载日志文件
	api := r.Group("/api")
	api.Use(controllers.JWTAuthMiddleware())
	{
		api.GET("/version", func(c *gin.Context) {
			c.JSON(http.StatusOK, map[string]interface{}{
				"version":   Version,
				"date":      PublishDate,
				"isWindows": runtime.GOOS == "windows",
				"isRelease": helpers.IsRelease,
			})
		})
		api.GET("/update/last", controllers.GetLastRelease)         // 获取最新版本
		api.POST("/update/to-version", controllers.UpdateToVersion) // 获取更新版本
		api.GET("/update/progress", controllers.UpdateProgress)     // 获取更新进度
		api.POST("/update/cancel", controllers.CancelUpdate)        // 取消更新
		api.GET("/user/info", controllers.GetUserInfo)
		api.GET("/path/list", controllers.GetPathList)
		api.POST("/user/change", controllers.ChangePassword)
		api.GET("/auth/115-status", controllers.Get115Status)                                      // 查询115状态
		api.POST("/auth/115-qrcode-open", controllers.GetLoginQrCodeOpen)                          // 获取115开放平台登录二维码
		api.POST("/auth/115-qrcode-status", controllers.GetQrCodeStatus)                           // 查询115二维码扫码状态
		api.POST("/setting/http-proxy", controllers.UpdateHttpProxy)                               // 更改HTTP代理
		api.GET("/setting/http-proxy", controllers.GetHttpProxy)                                   // 获取HTTP代理
		api.POST("/setting/test-http-proxy", controllers.TestHttpProxy)                            // 测试HTTP代理
		api.GET("/setting/telegram", controllers.GetTelegram)                                      // 获取telegram消息通知配置
		api.POST("/setting/telegram", controllers.UpdateTelegram)                                  // 更改telegram消息通知配置
		api.POST("/telegram/test", controllers.TestTelegram)                                       // 测试telegram连通性
		api.GET("/setting/notification/channels", controllers.GetNotificationChannels)             // 获取所有通知渠道
		api.POST("/setting/notification/channels/telegram", controllers.CreateTelegramChannel)     // 创建Telegram渠道
		api.GET("/setting/notification/channels/telegram/:id", controllers.GetTelegramChannel)     // 查询Telegram渠道
		api.PUT("/setting/notification/channels/telegram", controllers.UpdateTelegramChannel)      // 更新Telegram渠道
		api.POST("/setting/notification/channels/meow", controllers.CreateMeoWChannel)             // 创建MeoW渠道
		api.GET("/setting/notification/channels/meow/:id", controllers.GetMeoWChannel)             // 查询MeoW渠道
		api.PUT("/setting/notification/channels/meow", controllers.UpdateMeoWChannel)              // 更新MeoW渠道
		api.POST("/setting/notification/channels/bark", controllers.CreateBarkChannel)             // 创建Bark渠道
		api.GET("/setting/notification/channels/bark/:id", controllers.GetBarkChannel)             // 查询Bark渠道
		api.PUT("/setting/notification/channels/bark", controllers.UpdateBarkChannel)              // 更新Bark渠道
		api.POST("/setting/notification/channels/serverchan", controllers.CreateServerChanChannel) // 创建Server酱渠道
		api.GET("/setting/notification/channels/serverchan/:id", controllers.GetServerChanChannel) // 查询Server酱渠道
		api.PUT("/setting/notification/channels/serverchan", controllers.UpdateServerChanChannel)  // 更新Server酱渠道
		api.POST("/setting/notification/channels/webhook", controllers.CreateCustomWebhookChannel) // 创建自定义Webhook渠道
		api.GET("/setting/notification/channels/webhook/:id", controllers.GetCustomWebhookChannel) // 查询自定义Webhook渠道
		api.PUT("/setting/notification/channels/webhook", controllers.UpdateCustomWebhookChannel)  // 更新自定义Webhook渠道
		api.POST("/setting/notification/channels/status", controllers.UpdateChannelStatus)         // 启用/禁用渠道
		api.DELETE("/setting/notification/channels/:id", controllers.DeleteChannel)                // 删除渠道
		api.GET("/setting/notification/rules", controllers.GetNotificationRules)                   // 获取通知规则
		api.PUT("/setting/notification/rules", controllers.UpdateNotificationRule)                 // 更新通知规则
		api.POST("/setting/notification/channels/test", controllers.TestChannelConnection)         // 测试通知渠道连接
		api.GET("/setting/strm-config", controllers.GetStrmConfig)                                 // 获取STRM配置
		api.POST("/setting/strm-config", controllers.UpdateStrmConfig)                             // 更新STRM配置
		api.GET("/setting/cron", controllers.GetCronNextTime)                                      // 获取Cron表达式的下5次执行时间
		api.POST("/setting/emby/parse", controllers.ParseEmby)                                     // 解析Emby媒体信息
		api.GET("/setting/emby-config", controllers.GetEmbyConfig)                                 // 获取新的Emby配置
		api.POST("/setting/emby-config", controllers.UpdateEmbyConfig)                             // 更新新的Emby配置
		api.POST("/setting/threads", controllers.UpdateThreads)                                    // 更新线程数
		api.GET("/setting/threads", controllers.GetThreads)                                        // 获取线程数
		api.POST("/emby/sync/start", controllers.StartEmbySync)                                    // 手动启动Emby同步
		api.GET("/emby/sync/status", controllers.GetEmbySyncStatus)                                // 获取Emby同步状态
		api.GET("/emby/media", controllers.GetEmbyMediaItems)                                      // 分页获取Emby媒体项
		api.GET("/emby/library-sync-paths", controllers.GetEmbyLibrarySyncPaths)                   // 获取媒体库与同步目录关联
		api.POST("/emby/library-sync-paths", controllers.UpdateEmbyLibrarySyncPath)                // 更新媒体库与同步目录关联
		api.DELETE("/emby/library-sync-paths", controllers.DeleteEmbyLibrarySyncPath)              // 删除媒体库与同步目录关联
		api.POST("/sync/start", controllers.StartSync)                                             // 启动同步
		api.GET("/sync/records", controllers.GetSyncRecords)                                       // 同步列表
		api.GET("/sync/task", controllers.GetSyncTask)                                             // 获取同步任务详情
		api.GET("/sync/path-list", controllers.GetSyncPathList)                                    // 获取同步路径列表
		api.POST("/sync/path-add", controllers.AddSyncPath)                                        // 创建同步路径
		api.POST("/sync/path-update", controllers.UpdateSyncPath)                                  // 更新同步路径
		api.POST("/sync/path-delete", controllers.DeleteSyncPath)                                  // 删除同步路径
		api.POST("/sync/path/stop", controllers.StopSyncByPath)                                    // 停止同步路径的同步任务
		api.POST("/sync/path/start", controllers.StartSyncByPath)                                  // 启动同步路径的同步任务
		api.POST("/sync/path/full-start", controllers.FullStart115Sync)                            // 启动115的全量同步任务
		api.POST("/sync/delete-records", controllers.DelSyncRecords)                               // 批量删除同步记录
		api.POST("/sync/path/toggle-cron", controllers.ToggleSyncByPath)                           // 关闭或开启同步目录的定时同步
		api.GET("/account/list", controllers.GetAccountList)                                       // 获取开放平台账号列表
		api.POST("/account/add", controllers.CreateTmpAccount)                                     // 创建开放平台账号
		api.POST("/account/delete", controllers.DeleteAccount)                                     // 删除开放平台账号
		api.POST("/account/openlist", controllers.CreateOpenListAccount)                           // 创建openlist账号

		// API Key管理接口
		api.POST("/api-keys", controllers.CreateAPIKey)                 // 创建API Key
		api.GET("/api-keys", controllers.ListAPIKeys)                   // 获取API Key列表
		api.PUT("/api-keys/:id/status", controllers.UpdateAPIKeyStatus) // 更新API Key状态
		api.DELETE("/api-keys/:id", controllers.DeleteAPIKey)           // 删除API Key

		api.GET("/scrape/movie-genre", controllers.GetMovieGenre)                     // 获取电影类别
		api.GET("/scrape/tvshow-genre", controllers.GetTvshowGenre)                   // 获取电视剧类别
		api.GET("/scrape/language", controllers.GetLanguage)                          // 获取语言数组
		api.GET("/scrape/countries", controllers.GetCountries)                        // 获取国家数组
		api.GET("/scrape/tmdb", controllers.GetTmdbSettings)                          // 获取TMDB设置
		api.POST("/scrape/tmdb", controllers.SaveTmdbSettings)                        // 保存TMDB设置
		api.POST("/scrape/tmdb-test", controllers.TestTmdbSettings)                   // 测试TMDB设置
		api.GET("/scrape/ai-settings", controllers.GetAiSettings)                     // 获取AI识别设置
		api.POST("/scrape/ai-settings", controllers.SaveAiSettings)                   // 保存AI识别设置
		api.POST("/scrape/ai-test", controllers.TestAiSettings)                       // 测试AI识别设置
		api.GET("/scrape/movie-categories", controllers.GetMovieCategories)           // 获取电影分类列表
		api.GET("/scrape/tvshow-categories", controllers.GetTvshowCategories)         // 获取电视剧分类列表
		api.POST("/scrape/movie-categories", controllers.SaveMovieCategory)           // 保存电影分类
		api.POST("/scrape/tvshow-categories", controllers.SaveTvshowCategory)         // 保存电视剧分类
		api.DELETE("/scrape/movie-categories/:id", controllers.DeleteMovieCategory)   // 删除电影分类
		api.DELETE("/scrape/tvshow-categories/:id", controllers.DeleteTvshowCategory) // 删除电视剧分类
		api.GET("/scrape/pathes", controllers.GetScrapePathes)                        // 获取刮削路径列表
		api.POST("/scrape/pathes", controllers.SaveScrapePath)                        // 保存刮削路径列表
		api.DELETE("/scrape/pathes/:id", controllers.DeleteScrapePath)                // 删除刮削路径
		api.GET("/scrape/pathes/:id", controllers.GetScrapePath)                      // 获取刮削路径详情
		api.POST("/scrape/pathes/start", controllers.ScanScrapePath)                  // 扫描刮削路径
		api.POST("/scrape/pathes/stop", controllers.StopScrape)                       // 停止刮削任务
		api.POST("/scrape/pathes/toggle-cron", controllers.ToggleScrapePathCron)      // 关闭或开启刮削路径的定时刮削
		api.GET("/scrape/records", controllers.GetScrapeRecords)                      // 获取刮削记录
		api.POST("/scrape/re-scrape", controllers.ReScrape)                           // 重新刮削记录
		api.POST("/scrape/clear-failed", controllers.ClearFailedScrapeRecords)        // 清除所有刮削失败的记录
		api.POST("/scrape/truncate-all", controllers.TruncateAllScrapeRecords)        // 一键清空所有刮削记录
		api.DELETE("/scrape/records", controllers.DeleteScrapeMediaFile)              // 删除刮削记录
		api.POST("/scrape/finish", controllers.FinishScrapeMediaFile)                 // 完成刮削记录
		api.POST("/scrape/rename-failed", controllers.RenameFailedScrapeMediaFile)    // 标记所有失败的记录为待整理

		api.GET("/upload/queue", controllers.UploadList)                                             // 获取上传队列列表
		api.POST("/upload/queue/clear-pending", controllers.ClearPendingUploadTasks)                 // 清除上传队列中未开始的任务
		api.POST("/upload/queue/start", controllers.StartUploadQueue)                                // 启动上传队列
		api.POST("/upload/queue/stop", controllers.StopUploadQueue)                                  // 停止上传队列
		api.GET("/upload/queue/status", controllers.UploadQueueStatus)                               // 查询上传队列状态
		api.POST("/upload/queue/clear-success-failed", controllers.ClearUploadSuccessAndFailedTasks) // 清除上传队列中已完成和失败的任务

		api.GET("/download/queue", controllers.DownloadList)                                             // 获取下载队列列表
		api.POST("/download/queue/clear-pending", controllers.ClearPendingDownloadTasks)                 // 清除下载队列中未开始的任务
		api.POST("/download/queue/start", controllers.StartDownloadQueue)                                // 启动下载队列
		api.POST("/download/queue/stop", controllers.StopDownloadQueue)                                  // 停止下载队列
		api.GET("/download/queue/status", controllers.DownloadQueueStatus)                               // 查询下载队列状态
		api.POST("/download/queue/clear-success-failed", controllers.ClearDownloadSuccessAndFailedTasks) // 清除下载队列中已完成和失败的任务

		// 数据库备份和恢复接口
		api.GET("/database/backup-config", controllers.GetBackupConfig)            // 获取备份配置
		api.POST("/database/backup-config", controllers.UpdateBackupConfig)        // 更新备份配置
		api.POST("/database/backup/start", controllers.StartBackupTask)            // 启动备份任务
		api.POST("/database/backup/cancel", controllers.CancelBackupTask)          // 取消备份任务
		api.GET("/database/backup/progress", controllers.GetBackupProgress)        // 查询备份进度
		api.POST("/database/restore", controllers.RestoreDatabase)                 // 恢复数据库
		api.GET("/database/restore/progress", controllers.GetRestoreProgress)      // 查询恢复进度
		api.GET("/database/backups", controllers.ListBackups)                      // 列出所有备份文件
		api.DELETE("/database/backup", controllers.DeleteBackup)                   // 删除单个备份文件
		api.POST("/database/backup-record/delete", controllers.DeleteBackupRecord) // 删除备份记录
		api.GET("/database/backup-records", controllers.GetBackupRecords)          // 获取备份历史记录
	}
}

func Init() {
	fmt.Printf("当前版本号:%s, 发布日期:%s\n", Version, PublishDate)
	// 将版本写入helper
	helpers.Version = Version
	helpers.ReleaseDate = PublishDate
	initTimeZone() // 设置东8区
	GetRootDir()   // 获取当前工作目录
	fmt.Printf("当前工作目录:%s\n", helpers.RootDir)
	ipv4, _ := helpers.GetLocalIP()
	fmt.Printf("本机IPv4地址是 <%s>\n", ipv4)
	helpers.InitConfig() // 初始化配置文件
	InitLogger()
	// 创建App
	NewApp(helpers.RootDir)
	helpers.AppLogger.Infof("当前版本号:%s, 发布日期:%s\n", Version, PublishDate)
	if err := QMSApp.StartDatabase(); err != nil {
		log.Println("数据库启动失败:", err)
		return
	}
	db.InitCache() // 初始化内存缓存
	InitOthers()
}

func ParseParams() {
	// 定义 guid 参数
	var guid string
	flag.StringVar(&guid, "guid", "", "GUID 参数")
	// 解析命令行参数
	flag.Parse()
	fmt.Printf("传入的 GUID: %s\n", guid)
	// 使用参数
	if guid != "" {
		fmt.Printf("使用 GUID: %s 执行操作\n", guid)
		helpers.Guid = guid
	} else {
		// 检查是否有GUID环境变量，有的话直接使用
		guidEnv := os.Getenv("GUID")
		if guidEnv != "" {
			fmt.Printf("使用环境变量 GUID: %s 执行操作\n", guidEnv)
			helpers.Guid = guidEnv
		} else {
			fmt.Printf("使用默认用户: qms (12333) 执行操作\n")
			helpers.Guid = "12331"
		}
	}
}

func main() {
	ParseParams()
	Init()
	if runtime.GOOS == "windows" {
		go QMSApp.Start()
		helpers.StartApp(func() {
			QMSApp.Stop()
		})
	} else {
		QMSApp.Start()
	}
}

// cleanupIncompleteBackupTasks 清理应用启动时的未完成备份任务
func cleanupIncompleteBackupTasks() {
	// 查找所有未完成的备份任务
	var tasks []*models.BackupTask
	if err := db.Db.Where("status = ?", "running").Find(&tasks).Error; err != nil {
		helpers.AppLogger.Errorf("查询未完成的备份任务失败: %v", err)
		return
	}

	for _, task := range tasks {
		// 标记为失败
		if err := db.Db.Model(task).Updates(map[string]interface{}{
			"status":         "failed",
			"end_time":       time.Now().Unix(),
			"failure_reason": "应用重启导致任务中断",
		}).Error; err != nil {
			helpers.AppLogger.Errorf("更新备份任务状态失败: %v", err)
		}

		// 清理临时文件
		if task.FilePath != "" {
			config := &models.BackupConfig{}
			db.Db.First(config)
			if config.ID > 0 {
				backupDir := filepath.Join(helpers.RootDir, config.BackupPath)
				os.Remove(filepath.Join(backupDir, task.FilePath))
				os.Remove(filepath.Join(backupDir, task.FilePath+".gz"))
			}
		}

		helpers.AppLogger.Infof("已清理未完成的备份任务，任务ID: %d", task.ID)
	}

	// 检查维护模式是否需要恢复
	config := &models.BackupConfig{}
	if err := db.Db.First(config).Error; err == nil {
		if config.MaintenanceMode == 1 {
			// 自动退出维护模式
			db.Db.Model(config).Updates(map[string]interface{}{
				"maintenance_mode":      0,
				"maintenance_mode_time": 0,
			})
			helpers.AppLogger.Info("应用启动时已自动退出维护模式")
		}
	}
}
