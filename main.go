package main

import (
	"Q115-STRM/emby302/config"
	"Q115-STRM/emby302/util/logs/colors"
	"Q115-STRM/emby302/web"
	"Q115-STRM/internal/backup"
	"Q115-STRM/internal/controllers"
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/db/database"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/synccron"
	"Q115-STRM/internal/v115open"
	"context"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
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
var FANART_API_KEY = ""
var DEFAULT_TMDB_ACCESS_TOKEN = ""
var DEFAULT_TMDB_API_KEY = ""
var DEFAULT_SC_API_KEY = ""
var ENCRYPTION_KEY = ""
var Update bool = false

var AppName string = "QMediaSync"
var QMSApp *App

type App struct {
	isRelease   bool
	dbManager   *database.EmbeddedManager
	httpServer  *http.Server
	httpsServer *http.Server
	version     string
	publishDate string
}

func (app *App) Start() {
	// 启动外网302服务
	startEmby302()
	if helpers.IsRelease {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(controllers.Cors())
	setRouter(r)
	app.StartHttpServer(r)
	app.StartHttpsServer(r)
	go func() {
		http.ListenAndServe(":12330", nil)
	}()
	// if runtime.GOOS == "windows" {
	// 	helpers.AppLogger.Infof("QMediaSync 启动完成，现在可以关闭终端窗口。如果要退出请在通知栏（右下角）找到QMediaSync图标右键退出。")
	// }
	if runtime.GOOS == "windows" {
		// 监听Ctrl+C信号
		go func() {
			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
			<-quit
			log.Println("收到Ctrl+C信号")
			helpers.ExitChan <- struct{}{}
		}()
		<-helpers.ExitChan
		log.Println("收到停止信号")
		app.Stop()
		close(helpers.ExitChan)
		log.Println("应用程序正常退出")
		return
	} else {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("收到停止信号")
		// 停止应用
		app.Stop()
		log.Println("应用程序正常退出")
	}
}

func (app *App) Stop() {
	// 关闭同步任务执行队列
	synccron.PauseAllNewSyncQueues()
	// 关闭上传下载队列
	models.GlobalDownloadQueue.Stop()
	models.GlobalUploadQueue.Stop()
	// 关闭定时任务（包含备份定时任务）
	synccron.GlobalCron.Stop()
	// 关闭数据库
	if app.dbManager != nil {
		app.dbManager.Stop()
	}
	helpers.CloseLogger() // 关闭日志
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
			sslHost = helpers.GlobalConfig.HttpsHost
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
	host := helpers.GlobalConfig.HttpHost
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

func (app *App) StartDatabase() error {
	defer models.Migrate()
	// 根据配置启动数据库连接
	if helpers.GlobalConfig.Db.Engine == helpers.DbEngineSqlite {
		// 如果是sqlite，直接初始化sqlite连接
		sqliteFile := filepath.Join(helpers.ConfigDir, helpers.GlobalConfig.Db.SqliteFile)
		log.Printf("sqlite数据库文件路径：%s", sqliteFile)
		db.Db = db.InitSqlite3(sqliteFile)
		return nil
	}

	// 初始化数据库配置
	dbConfig := &database.Config{
		Mode:         helpers.GlobalConfig.Db.PostgresType,
		Host:         helpers.GlobalConfig.Db.PostgresConfig.Host,
		Port:         helpers.GlobalConfig.Db.PostgresConfig.Port,
		User:         helpers.GlobalConfig.Db.PostgresConfig.User,
		Password:     helpers.GlobalConfig.Db.PostgresConfig.Password,
		DBName:       helpers.GlobalConfig.Db.PostgresConfig.Database,
		SSLMode:      "disable",
		LogDir:       filepath.Join(helpers.ConfigDir, "postgres", "log"),
		DataDir:      filepath.Join(helpers.ConfigDir, "postgres", "data"),
		BinaryPath:   db.GetPostgresBinaryPath(helpers.DataDir),
		MaxOpenConns: helpers.GlobalConfig.Db.PostgresConfig.MaxOpenConns,
		MaxIdleConns: helpers.GlobalConfig.Db.PostgresConfig.MaxIdleConns,
	}
	if dbConfig.Mode == helpers.PostgresTypeEmbedded {
		// 如果使用内置数据库，则需要启动和初始化数据库
		app.dbManager = database.NewEmbeddedManager(dbConfig)
		// 启动数据库
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		if err := app.dbManager.Start(ctx); err != nil {
			return err
		}
		db.InitPostgres(app.dbManager.GetDB())
	} else {
		// 初始化PostgreSQL数据库连接
		if err := db.ConnectPostgres(dbConfig); err != nil {
			return err
		}
	}

	return nil
}

func newApp() {
	if QMSApp != nil {
		log.Println("App已经初始化，不能再次初始化")
		return
	}
	// 初始化APP
	QMSApp = &App{
		isRelease:   helpers.IsRelease,
		version:     Version,
		publishDate: PublishDate,
	}
}

func initTimeZone() {
	cstZone := time.FixedZone("CST", 8*3600)
	time.Local = cstZone
}

func checkRelease() {
	if helpers.IsRunningInDocker() {
		helpers.IsRelease = true
	}
	arg1 := strings.ToLower(os.Args[0])
	name := strings.ToLower(filepath.Base(arg1))
	helpers.IsRelease = strings.Index(name, "qmediasync") == 0 && !strings.Contains(arg1, "go-build")
}

func getRootDir() string {
	var exPath string = "/app" // 默认使用docker的路径
	checkRelease()
	if os.Getenv("TRIM_APPDEST") != "" {
		helpers.RootDir = os.Getenv("TRIM_APPDEST")
		return helpers.RootDir
	}
	if helpers.IsRelease {
		ex, err := os.Executable()
		if err != nil {
			panic(err)
		}
		exPath = filepath.Dir(ex)
	} else {
		if runtime.GOOS == "windows" {
			exPath, _ = os.Getwd()
		} else {
			exPath = "/home/qicfan/dev/qmediasync"
		}
	}
	helpers.RootDir = exPath // 获取当前工作目录
	return exPath
}

// 获取用户数据目录
func getDataAndConfigDir() {
	var appData string
	var dataDir string
	var configDir string
	needMk := false
	if runtime.GOOS == "windows" {
		// 使用AppData目录，用户有完全控制权限
		appData := os.Getenv("LOCALAPPDATA")
		if appData == "" {
			appData = os.Getenv("APPDATA")
		}
		dataDir = filepath.Join(helpers.RootDir, "postgres")      // 数据库目录
		oldConfigDir := filepath.Join(appData, AppName, "config") // 配置目录
		configDir = filepath.Join(helpers.RootDir, "config")      // 配置目录
		err := os.MkdirAll(dataDir, 0755)
		if err != nil {
			fmt.Printf("创建数据目录失败: %v\n", err)
			panic("创建数据目录失败")
		}
		err = os.MkdirAll(configDir, 0755)
		if err != nil {
			fmt.Printf("创建配置目录失败: %v\n", err)
			panic("创建配置目录失败")
		}
		helpers.DataDir = dataDir
		helpers.ConfigDir = configDir
		if helpers.PathExists(oldConfigDir) {
			// 迁移旧配置
			err := helpers.MoveDir(oldConfigDir, configDir)
			if err != nil {
				fmt.Printf("迁移旧配置目录失败: %v\n", err)
				panic("迁移旧配置目录失败")
			}
			// 删除旧目录
			err = os.RemoveAll(oldConfigDir)
			if err != nil {
				fmt.Printf("删除旧配置目录失败: %v\n", err)
				panic("删除旧配置目录失败")
			}
		}
	} else {
		if os.Getenv("TRIM_PKGETC") == "" {
			appData = helpers.RootDir
			configDir = filepath.Join(appData, "config") // 配置目录
			dataDir = filepath.Join(appData, "postgres") // 数据库目录
			needMk = true
			helpers.DataDir = dataDir
			helpers.ConfigDir = configDir
		} else {
			configDir = os.Getenv("TRIM_PKGETC")
			dataDir = filepath.Join(configDir, "postgres") // 数据库目录
			needMk = false
			helpers.DataDir = dataDir
			helpers.ConfigDir = configDir
		}
	}
	if needMk {
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			log.Printf("创建配置目录失败: %v\n", err)
			panic("创建配置目录失败")
		}
	}
}

//go:embed emby302.yml
var s string

func startEmby302() {
	dataRoot := helpers.ConfigDir
	if err := config.ReadFromFile([]byte(s)); err != nil {
		log.Fatal(err)
	}
	if models.GlobalEmbyConfig == nil || models.GlobalEmbyConfig.EmbyUrl == "" {
		helpers.AppLogger.Warnf("Emby302未配置Emby地址，跳过启动emby302服务")
		return
	}
	config.C.Emby.Host = models.GlobalEmbyConfig.EmbyUrl
	config.C.Emby.EpisodesUnplayPrior = false // 关闭剧集排序
	certFile := filepath.Join(dataRoot, "server.crt")
	keyFile := filepath.Join(dataRoot, "server.key")
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

func initLogger() {
	logPath := filepath.Join(helpers.ConfigDir, "logs")
	os.MkdirAll(logPath, 0755) // 如果没有logs目录则创建
	libLogPath := filepath.Join(logPath, "libs")
	os.MkdirAll(libLogPath, 0755) // 如果没有logs/libs目录则创建
	helpers.AppLogger = helpers.NewLogger(helpers.GlobalConfig.Log.File, true, true)
	helpers.V115Log = helpers.NewLogger(helpers.GlobalConfig.Log.V115, false, true)
	helpers.OpenListLog = helpers.NewLogger(helpers.GlobalConfig.Log.OpenList, false, true)
	helpers.TMDBLog = helpers.NewLogger(helpers.GlobalConfig.Log.TMDB, false, true)
	helpers.BaiduPanLog = helpers.NewLogger(helpers.GlobalConfig.Log.BaiduPan, false, true)
}

func initOthers() {
	helpers.InitEventBus() // 初始化事件总线
	models.LoadSettings()  // 从数据库加载设置
	helpers.AppLogger.Infof("已加载配置，准备初始化115请求队列，线程数: %d", models.SettingsGlobal.FileDetailThreads)
	qps := models.SettingsGlobal.FileDetailThreads
	if qps <= 0 {
		qps = 2
	}
	v115open.SetGlobalExecutorConfig(qps, qps*60, qps*3600)
	models.LoadScrapeSettings()          // 从数据库加载刮削设置
	models.InitDQ()                      // 初始化下载队列
	models.InitUQ()                      // 初始化上传队列
	models.InitNotificationManager()     // 初始化通知管理器
	controllers.StartListenTelegramBot() // 初始化TelegramBot监听
	models.GetEmbyConfig()               // 加载Emby配置
	helpers.SubscribeSync(helpers.V115TokenInValidEvent, models.HandleV115TokenInvalid)
	helpers.SubscribeSync(helpers.SaveOpenListTokenEvent, models.HandleOpenListTokenSaveSync)
	models.FailAllRunningSyncTasks()   // 将所有运行中的同步任务设置为失败状态
	synccron.RefreshOAuthAccessToken() // 启动时刷新一次115的访问凭证，防止有过期的token导致同步失败

	// 设置115请求队列的统计保存回调函数
	v115open.SetGlobalExecutorStatSaver(func(requestTime int64, url, method string, duration int64, isThrottled bool) {
		stat := &models.RequestStat{
			RequestTime: requestTime,
			URL:         url,
			Method:      method,
			Duration:    duration,
			IsThrottled: isThrottled,
			AccountID:   0, // 可以后续扩展传入账号ID
		}
		if err := models.CreateRequestStat(stat); err != nil {
			helpers.V115Log.Errorf("写入请求统计失败: %v", err)
		}
	})

	// if helpers.IsRelease {
	// 启动同步任务队列管理器
	synccron.InitNewSyncQueueManager()
	synccron.InitCron()     // 初始化定时任务（包含备份定时任务）
	synccron.InitSyncCron() // 初始化同步目录的定时任务
	// 初始化备份服务
	models.InitBackupService()
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
	helpers.Subscribe(helpers.BackupCronEevent, func(event helpers.Event) {
		backup.Backup("定时", "定时备份")
	})
	// backup.Backup("手动", "手动备份")

	// account, _ := models.GetAccountById(1)
	// client := account.Get115Client()
	// _, err := client.GetFsDetailByCid(context.Background(), "31123123123123")
	// helpers.AppLogger.Errorf("获取文件详情失败: %v", err)
	// panic("test")
}

// 设置路由
func setRouter(r *gin.Engine) {
	webStatisPath := filepath.Join(helpers.RootDir, "web_statics")
	// if helpers.IsFnOS {
	// 	webStatisPath = filepath.Join(helpers.RootDir, "www")
	// }
	r.LoadHTMLFiles(filepath.Join(webStatisPath, "index.html"))
	r.StaticFile("/favicon.ico", filepath.Join(webStatisPath, "favicon.ico"))
	r.StaticFS("/assets", http.Dir(filepath.Join(webStatisPath, "assets")))
	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", gin.H{})
	})
	r.POST("/emby/webhook", controllers.Webhook)
	r.POST("/api/login", controllers.LoginAction)
	r.GET("/115/url/*filename", controllers.Get115UrlByPickCode)           // 查询115直链 by pickcode 支持iso，路径最后一部分是.扩展名格式
	r.GET("/115/newurl", controllers.Get115UrlByPickCode)                  // 查询115直链 by pickcode
	r.GET("/baidupan/url/*filename", controllers.GetBaiduPanUrlByPickCode) // 查询百度网盘直链 by fsid 支持iso，路径最后一部分是.扩展名格式

	r.GET("/openlist/url", controllers.GetOpenListFileUrl) // 查询OpenList直链

	r.GET("/proxy-115", controllers.Proxy115) // 115CDN反代路由

	r.GET("/api/scrape/tmp-image", controllers.ScrapeTmpImage)           // 获取临时图片
	r.GET("/api/scrape/records/export", controllers.ExportScrapeRecords) // 导出刮削记录
	r.GET("/api/logs/ws", controllers.LogWebSocket)                      // WebSocket日志查看
	r.GET("/api/logs/old", controllers.GetOldLogs)                       // HTTP获取旧日志
	r.GET("/api/logs/download", controllers.DownloadLogFile)             // 下载日志文件

	r.POST("/api/update-fn-access-path", controllers.UpdateFNPath) // 更新Fn访问路径
	r.GET("/api/path/is-fn-os", controllers.IsFnOS)                // 查询是否是飞牛环境

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
		// api.GET("/announce", controllers.GetAnnounce) // 获取公告

		api.POST("/auth/115-qrcode-open", controllers.GetLoginQrCodeOpen) // 获取115开放平台登录二维码
		api.POST("/auth/115-qrcode-status", controllers.GetQrCodeStatus)  // 查询115二维码扫码状态
		api.GET("/115/status", controllers.Get115Status)                  // 查询115状态
		api.GET("/115/oauth-url", controllers.GetOAuthUrl)                // 获取115 OAuth登录地址
		api.POST("115/oauth-confirm", controllers.ConfirmOAuthCode)       // 确认OAuth登录
		api.GET("/115/queue/stats", controllers.GetQueueStats)            // 获取115 OpenAPI请求队列统计数据
		api.POST("/115/queue/rate-limit", controllers.SetQueueRateLimit)  // 设置115 OpenAPI请求队列速率限制
		api.GET("/115/stats/daily", controllers.GetRequestStatsByDay)     // 获取115请求统计（按天）
		api.GET("/115/stats/hourly", controllers.GetRequestStatsByHour)   // 获取115请求统计（按小时）
		api.POST("/115/stats/clean", controllers.CleanOldRequestStats)    // 清理旧的请求统计数据
		// 百度网盘相关路由
		api.GET("/baidupan/oauth-url", controllers.GetBaiDuPanOAuthUrl)           // 获取百度网盘OAuth登录地址
		api.POST("/baidupan/oauth-confirm", controllers.ConfirmBaiDuPanOAuthCode) // 确认百度网盘OAuth登录
		api.GET("/baidupan/status", controllers.GetBaiDuPanStatus)                // 查询百度网盘状态

		api.GET("/update/last", controllers.GetLastRelease)         // 获取最新版本
		api.POST("/update/to-version", controllers.UpdateToVersion) // 获取更新版本
		api.GET("/update/progress", controllers.UpdateProgress)     // 获取更新进度
		api.POST("/update/cancel", controllers.CancelUpdate)        // 取消更新

		api.GET("/user/info", controllers.GetUserInfo)
		api.GET("/path/list", controllers.GetPathList)     // 目录列表
		api.POST("/path/create", controllers.CreateDir)    // 创建目录接口
		api.GET("/path/files", controllers.GetNetFileList) // 查询网盘文件列表
		api.POST("/user/change", controllers.ChangePassword)

		api.POST("/setting/http-proxy", controllers.UpdateHttpProxy)    // 更改HTTP代理
		api.GET("/setting/http-proxy", controllers.GetHttpProxy)        // 获取HTTP代理
		api.POST("/setting/test-http-proxy", controllers.TestHttpProxy) // 测试HTTP代理
		// api.GET("/setting/telegram", controllers.GetTelegram)                                      // 获取telegram消息通知配置
		// api.POST("/setting/telegram", controllers.UpdateTelegram)                                  // 更改telegram消息通知配置
		// api.POST("/telegram/test", controllers.TestTelegram)                                       // 测试telegram连通性
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

		api.POST("/emby/sync/start", controllers.StartEmbySync)     // 手动启动Emby同步
		api.GET("/emby/sync/status", controllers.GetEmbySyncStatus) // 获取Emby同步状态           // 删除媒体库与同步目录关联

		api.POST("/sync/start", controllers.StartSync)                   // 启动同步
		api.GET("/sync/records", controllers.GetSyncRecords)             // 同步列表
		api.GET("/sync/task", controllers.GetSyncTask)                   // 获取同步任务详情
		api.GET("/sync/path-list", controllers.GetSyncPathList)          // 获取同步路径列表
		api.POST("/sync/path-add", controllers.AddSyncPath)              // 创建同步路径
		api.POST("/sync/path-update", controllers.UpdateSyncPath)        // 更新同步路径
		api.POST("/sync/path-delete", controllers.DeleteSyncPath)        // 删除同步路径
		api.POST("/sync/path/stop", controllers.StopSyncByPath)          // 停止同步路径的同步任务
		api.POST("/sync/path/start", controllers.StartSyncByPath)        // 启动同步路径的同步任务
		api.POST("/sync/path/full-start", controllers.FullStart115Sync)  // 启动115的全量同步任务
		api.POST("/sync/delete-records", controllers.DelSyncRecords)     // 批量删除同步记录
		api.POST("/sync/path/toggle-cron", controllers.ToggleSyncByPath) // 关闭或开启同步目录的定时同步
		api.GET("/sync/path/:id", controllers.GetSyncPathById)           // 获取同步路径详情

		api.GET("/account/list", controllers.GetAccountList)             // 获取开放平台账号列表
		api.POST("/account/add", controllers.CreateTmpAccount)           // 创建开放平台账号
		api.POST("/account/delete", controllers.DeleteAccount)           // 删除开放平台账号
		api.POST("/account/openlist", controllers.CreateOpenListAccount) // 创建openlist账号

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
		api.POST("/scrape/sync-pathes", controllers.SaveScrapeStrmPath)               // 保存刮削目录关联的同步目录
		api.GET("/scrape/sync-pathes", controllers.GetScrapeStrmPaths)                // 获取刮削目录关联的同步目录
		api.GET("/scrape/tmdb-search", controllers.TmdbSearch)                        // 搜索TMDB媒体

		api.GET("/upload/queue", controllers.UploadList)                                             // 获取上传队列列表
		api.POST("/upload/queue/clear-pending", controllers.ClearPendingUploadTasks)                 // 清除上传队列中未开始的任务
		api.POST("/upload/queue/start", controllers.StartUploadQueue)                                // 启动上传队列
		api.POST("/upload/queue/stop", controllers.StopUploadQueue)                                  // 停止上传队列
		api.GET("/upload/queue/status", controllers.UploadQueueStatus)                               // 查询上传队列状态
		api.POST("/upload/queue/clear-success-failed", controllers.ClearUploadSuccessAndFailedTasks) // 清除上传队列中已完成和失败的任务
		api.POST("/upload/queue/retry-failed", controllers.RetryFailedUploadTasks)                   // 重试所有失败的上传任务

		api.GET("/download/queue", controllers.DownloadList)                                             // 获取下载队列列表
		api.POST("/download/queue/clear-pending", controllers.ClearPendingDownloadTasks)                 // 清除下载队列中未开始的任务
		api.POST("/download/queue/start", controllers.StartDownloadQueue)                                // 启动下载队列
		api.POST("/download/queue/stop", controllers.StopDownloadQueue)                                  // 停止下载队列
		api.GET("/download/queue/status", controllers.DownloadQueueStatus)                               // 查询下载队列状态
		api.POST("/download/queue/clear-success-failed", controllers.ClearDownloadSuccessAndFailedTasks) // 清除下载队列中已完成和失败的任务

		// 备份与恢复相关路由
		api.GET("/backup/list", controllers.GetBackupList)               // 获取备份列表
		api.GET("/backup/records/:id", controllers.GetBackupRecord)      // 获取备份记录详情
		api.POST("/backup/create", controllers.CreateBackup)             // 创建手动备份
		api.DELETE("/backup/records/:id", controllers.DeleteBackup)      // 删除备份记录
		api.POST("/backup/restore", controllers.RestoreFromBackup)       // 从备份恢复
		api.POST("/backup/upload-restore", controllers.UploadAndRestore) // 上传文件并恢复
		api.GET("/backup/download/:id", controllers.DownloadBackup)      // 下载备份文件
		api.GET("/backup/config", controllers.GetBackupConfig)           // 获取备份配置
		api.PUT("/backup/config", controllers.UpdateBackupConfig)        // 更新备份配置
		api.GET("/backup/status", controllers.GetBackupStatus)           // 获取备份状态

	}
}

func initEnv() bool {
	log.Printf("当前版本号:%s, 发布日期:%s\n", Version, PublishDate)
	// 将版本写入helper
	helpers.Version = Version
	helpers.ReleaseDate = PublishDate
	// 加载环境变量配置
	helpers.LoadEnvFromFile(filepath.Join(helpers.RootDir, "config", ".env"))
	if DEFAULT_SC_API_KEY != "" {
		helpers.DEFAULT_SC_API_KEY = DEFAULT_SC_API_KEY
	} else {
		helpers.DEFAULT_SC_API_KEY = os.Getenv("DEFAULT_SC_API_KEY")
	}
	if DEFAULT_TMDB_API_KEY != "" {
		helpers.DEFAULT_TMDB_API_KEY = DEFAULT_TMDB_API_KEY
	} else {
		helpers.DEFAULT_TMDB_API_KEY = os.Getenv("DEFAULT_TMDB_API_KEY")
	}
	if DEFAULT_TMDB_ACCESS_TOKEN != "" {
		helpers.DEFAULT_TMDB_ACCESS_TOKEN = DEFAULT_TMDB_ACCESS_TOKEN
	} else {
		helpers.DEFAULT_TMDB_ACCESS_TOKEN = os.Getenv("DEFAULT_TMDB_ACCESS_TOKEN")
	}
	if FANART_API_KEY != "" {
		helpers.FANART_API_KEY = FANART_API_KEY
	} else {
		helpers.FANART_API_KEY = os.Getenv("FANART_API_KEY")
	}
	if ENCRYPTION_KEY != "" {
		helpers.ENCRYPTION_KEY = ENCRYPTION_KEY
	} else {
		helpers.ENCRYPTION_KEY = os.Getenv("ENCRYPTION_KEY")
	}
	initTimeZone()        // 设置东8区
	getDataAndConfigDir() // 获取数据库数据目录和配置文件目录
	log.Printf("当前工作目录:%s\n", helpers.RootDir)
	log.Printf("当前数据目录：%s\n", helpers.DataDir)
	log.Printf("当前配置文件目录: %s\n", helpers.ConfigDir)
	ipv4, _ := helpers.GetLocalIP()
	log.Printf("本机IPv4地址是 <%s>\n", ipv4)
	// 检查配置文件是否存在
	configPath := filepath.Join(helpers.ConfigDir, "config.yml")
	helpers.IsFirstRun = !helpers.PathExists(configPath)
	// 如果不存在，启动一个简易web服务来配置数据库连接信息
	if helpers.IsFirstRun {
		// 检查是否有旧的数据库配置和记录，有的话生成配置文件，跳过配置流程
		oldPostgresDataDir := filepath.Join(helpers.ConfigDir, "postgres")
		if helpers.PathExists(oldPostgresDataDir) {
			log.Printf("发现旧的数据库数据目录: %s", oldPostgresDataDir)
			// 生成新的配置文件
			if err := helpers.MakeOldConfig(); err != nil {
				log.Printf("生成新的配置文件失败: %v", err)
				return false
			}
			log.Printf("已生成配置文件: %s", configPath)
			helpers.IsFirstRun = false
		} else {
			log.Printf("配置文件不存在，启动简单配置服务: %s", configPath)
			StartConfigWebServer()
			return false
		}
	}
	log.Printf("配置文件存在，加载配置文件: %s", configPath)
	// 如果存在，则加载配置文件，进行其他的初始化工作
	err := helpers.InitConfig()
	if err != nil {
		log.Printf("初始化配置文件失败: %v", err)
		return false
	}
	initLogger()
	// 创建App
	newApp()
	helpers.AppLogger.Infof("当前版本号:%s, 发布日期:%s\n", Version, PublishDate)
	if err := QMSApp.StartDatabase(); err != nil {
		log.Println("数据库启动失败:", err)
		return false
	}
	db.InitCache() // 初始化内存缓存
	initOthers()
	return true
}

func parseParams() {
	// 定义 guid 参数
	var update string
	flag.StringVar(&helpers.Guid, "guid", "", "GUID 参数")
	flag.BoolVar(&helpers.IsFnOS, "fnos", false, "是否是飞牛环境")
	flag.StringVar(&update, "update", "", "更新参数")
	// 解析命令行参数
	flag.Parse()
	// 使用参数
	if helpers.IsFnOS {
		log.Printf("当前环境为飞牛环境\n")
	}
	if helpers.Guid == "" {
		// 检查是否有GUID环境变量，有的话直接使用
		guidEnv := os.Getenv("GUID")
		if guidEnv != "" {
			log.Printf("使用环境变量 GUID: %s 执行操作\n", guidEnv)
			helpers.Guid = guidEnv
		} else {
			log.Printf("使用 root 执行操作\n")
			helpers.Guid = ""
		}
	}
	// 检查是否是更新模式
	if update != "" && runtime.GOOS == "windows" {
		Update = true
	}
}

// @title QMediaSync API
// @version 1.0
// @description 媒体同步和刮削系统API
// @host localhost:8115
// @BasePath /
// @securityDefinitions.apikey JwtAuth
// @in header
// @name Authorization
// @securityDefinitions.apikey ApiKeyAuth
// @in query
// @name api_key
func main() {
	parseParams()
	getRootDir()
	if Update {
		runUpdateProcess()
		return
	}
	if !initEnv() {
		return
	}
	if runtime.GOOS == "windows" {
		if helpers.IsRelease {
			go QMSApp.Start()
			helpers.StartApp(func() {
				QMSApp.Stop()
			})
		} else {
			QMSApp.Start()
		}
	} else {
		QMSApp.Start()
	}
}

func runUpdateProcess() {
	if len(os.Args) < 3 {
		fmt.Println("更新参数不足")
		return
	}

	updateDir := os.Args[2]

	fmt.Println("开始更新流程...")

	parentPID := os.Getppid()
	fmt.Printf("等待父进程退出 (PID: %d)...\n", parentPID)

	if err := waitForProcessExit(parentPID); err != nil {
		fmt.Printf("等待父进程退出失败: %v\n", err)
	}

	fmt.Println("父进程已退出，开始更新...")

	backupDir := filepath.Join(helpers.RootDir, "old")

	if helpers.PathExists(backupDir) {
		fmt.Println("删除旧的备份目录...")
		os.RemoveAll(backupDir)
	}

	os.MkdirAll(backupDir, 0777)

	appName := "QMediaSync.exe"
	appPath := filepath.Join(helpers.RootDir, appName)
	newAppPath := filepath.Join(updateDir, appName)
	if helpers.PathExists(newAppPath) {
		fmt.Printf("更新 %s...\n", appName)
		// 将老的可执行文件改名为old.exe
		oldAppPath := appPath + ".old.exe"
		if helpers.PathExists(oldAppPath) {
			if err := os.Remove(oldAppPath); err != nil {
				fmt.Printf("删除旧 %s 失败: %v\n", appName, err)
				os.Exit(1)
			}
		}
		if err := os.Rename(appPath, oldAppPath); err != nil {
			fmt.Printf("重命名旧 %s 失败: %v\n", appName, err)
			os.Exit(1)
		}
		if err := helpers.CopyFile(newAppPath, appPath); err != nil {
			fmt.Printf("更新主程序失败: %v\n", err)
		}
	} else {
		fmt.Printf("更新目录中未找到 %s\n", appName)
	}

	replaceDir(filepath.Join(updateDir, "web_statics"), filepath.Join(helpers.RootDir, "web_statics"), backupDir)
	replaceDir(filepath.Join(updateDir, "scripts"), filepath.Join(helpers.RootDir, "scripts"), backupDir)
	// 删除临时exe
	tempExePath := newAppPath + ".temp.exe"
	if helpers.PathExists(tempExePath) {
		if err := os.Remove(tempExePath); err != nil {
			fmt.Printf("删除临时exe文件失败: %v\n", err)
		}
	}
	fmt.Println("更新完成!")
	fmt.Println("启动新版本...")
	// 启动新进程
	if !helpers.StartNewProcess(appPath, "") {
		fmt.Printf("启动新版本失败\n")
	}
}

func waitForProcessExit(pid int) error {
	maxWait := 30 * time.Second
	deadline := time.Now().Add(maxWait)

	for time.Now().Before(deadline) {

		alive, err := helpers.IsProcessAlive(pid)
		if err != nil {
			return nil
		}
		// 检查process已经退出
		if !alive {
			fmt.Printf("父进程已退出，等待资源释放...\n")
			time.Sleep(2 * time.Second)
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}

	return nil
}

func replaceDir(srcDir, dstDir, backupDir string) {
	if !helpers.PathExists(srcDir) {
		return
	}

	dirName := filepath.Base(dstDir)
	backupPath := filepath.Join(backupDir, dirName)

	if helpers.PathExists(dstDir) {
		fmt.Printf("备份 %s 目录...\n", dirName)
		os.RemoveAll(backupPath)
		if err := helpers.CopyDir(dstDir, backupPath); err != nil {
			fmt.Printf("备份 %s 目录失败: %v\n", dirName, err)
		}

		fmt.Printf("删除旧 %s 目录...\n", dirName)
		os.RemoveAll(dstDir)
	}

	fmt.Printf("更新 %s 目录...\n", dirName)
	if err := helpers.CopyDir(srcDir, dstDir); err != nil {
		fmt.Printf("更新 %s 目录失败: %v\n", dirName, err)
	}
}

func isInRestrictedDirectory() (bool, string) {
	if runtime.GOOS != "windows" {
		return false, ""
	}

	exePath, err := os.Executable()
	if err != nil {
		return false, ""
	}
	exeDir := filepath.Dir(exePath)

	driveLetter := strings.ToUpper(string(exeDir[0]))
	log.Printf("应用程序路径: %s, 盘符: %s", exePath, driveLetter)
	if driveLetter == "C" {
		return true, "应用程序位于 C 盘，建议将应用程序移动到其他盘符（如 D 盘、E 盘等）以避免权限问题"
	}

	restrictedPaths := []string{
		"Program Files",
		"Program Files (x86)",
		"ProgramData",
		"Windows",
	}

	for _, restrictedPath := range restrictedPaths {
		log.Printf("检查目录: %s, 是否包含受限路径: %s", exeDir, restrictedPath)
		if strings.Contains(exeDir, restrictedPath) {
			return true, fmt.Sprintf("应用程序位于受限目录 '%s' 中，建议将应用程序移动到普通用户目录或其他非系统目录", restrictedPath)
		}
	}

	return false, ""
}

func StartConfigWebServer() {
	if helpers.IsRelease {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()
	r.GET("/", func(c *gin.Context) {
		isRestricted, warningMsg := isInRestrictedDirectory()
		c.HTML(200, "db_config.html", gin.H{
			"title":        "数据库配置",
			"isRestricted": isRestricted,
			"warningMsg":   warningMsg,
		})
	})

	r.POST("/api/config/save", func(c *gin.Context) {
		var req struct {
			Engine       string `json:"engine"`
			PostgresType string `json:"postgresType"`
			Host         string `json:"host"`
			Port         int    `json:"port"`
			User         string `json:"user"`
			Password     string `json:"password"`
			Database     string `json:"database"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		yamlConfig := helpers.MakeDefaultConfig()
		if req.Engine == string(helpers.DbEnginePostgres) {
			yamlConfig.Db.PostgresType = helpers.PostgresType(req.PostgresType)
			if req.PostgresType == string(helpers.PostgresTypeExternal) {
				yamlConfig.Db.PostgresConfig = helpers.PostgresConfig{
					Host:         req.Host,
					Port:         req.Port,
					User:         req.User,
					Password:     req.Password,
					Database:     req.Database,
					MaxOpenConns: 25,
					MaxIdleConns: 25,
				}
			} else {
				yamlConfig.Db.PostgresConfig = helpers.PostgresConfig{
					Host:         "localhost",
					Port:         5432,
					User:         "qms",
					Password:     "qms123456",
					Database:     "qms",
					MaxOpenConns: 25,
					MaxIdleConns: 25,
				}
			}
		} else {
			yamlConfig.Db.Engine = helpers.DbEngineSqlite
		}

		if err := helpers.SaveConfig(yamlConfig); err != nil {
			c.JSON(500, gin.H{"error": "保存配置失败: " + err.Error()})
			return
		}

		c.JSON(200, gin.H{"success": true, "message": "配置已保存，配置服务已退出，请重启软件或者容器"})
		go func() {
			time.Sleep(1 * time.Second)
			os.Exit(0)
		}()
	})

	r.LoadHTMLGlob(filepath.Join(helpers.RootDir, "web_statics", "*.html"))

	fmt.Printf("配置服务已启动，请在浏览器中访问: http://ip:12333\n")
	if err := r.Run(":12333"); err != nil {
		log.Fatalf("启动配置服务失败: %v", err)
	}
}
