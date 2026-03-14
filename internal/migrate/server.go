package migrate

import (
	"Q115-STRM/internal/backup"
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/db/database"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"context"
	"database/sql"
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

var migrateFiles embed.FS

func SetMigrateFiles(fs embed.FS) {
	migrateFiles = fs
}

type MigrateServer struct {
	config      *database.Config
	dbManager   *database.EmbeddedManager
	httpServer  *http.Server
	backupPath  string
	isCompleted bool
}

func NewMigrateServer(dbManager *database.EmbeddedManager, config *database.Config) *MigrateServer {
	return &MigrateServer{
		config:     config,
		dbManager:  dbManager,
		backupPath: filepath.Join(helpers.ConfigDir, "backups", "migrate.zip"),
	}
}

const MigrateBackupPath = "migrate.zip"

func (s *MigrateServer) Start() error {
	if helpers.IsRelease {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()

	data, err := migrateFiles.ReadFile("assets/migrate.html")
	if err != nil {
		return fmt.Errorf("读取迁移模板失败: %v", err)
	}
	tmpl := template.Must(template.New("migrate.html").Parse(string(data)))
	r.SetHTMLTemplate(tmpl)

	r.GET("/", func(c *gin.Context) {
		step := s.getCurrentStep()
		c.HTML(200, "migrate.html", gin.H{
			"title":      "数据库迁移",
			"step":       step,
			"isDocker":   helpers.IsRunningInDocker(),
			"isWindows":  runtime.GOOS == "windows",
			"backupPath": s.backupPath,
		})
	})

	r.GET("/api/step", func(c *gin.Context) {
		step := s.getCurrentStep()
		c.JSON(200, gin.H{
			"step":       step,
			"backupPath": s.backupPath,
		})
	})

	r.POST("/api/backup", s.handleBackup)
	r.POST("/api/test-db", s.handleTestDB)
	r.POST("/api/save-config", s.handleSaveConfig)
	r.GET("/api/backup-status", s.handleBackupStatus)

	s.httpServer = &http.Server{
		Addr:    helpers.GlobalConfig.HttpHost,
		Handler: r,
	}

	fmt.Printf("迁移服务已启动，请在浏览器中访问: http://ip%s\n", helpers.GlobalConfig.HttpHost)
	go func() {
		time.Sleep(1 * time.Second)
		helpers.OpenBrowser("http://127.0.0.1" + helpers.GlobalConfig.HttpHost)
	}()

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("启动迁移服务失败: %v", err)
	}

	return nil
}

func (s *MigrateServer) getCurrentStep() int {
	if s.isCompleted {
		return 4
	}
	if helpers.PathExists(s.backupPath) {
		return 3
	}
	return 1
}

func (s *MigrateServer) handleBackup(c *gin.Context) {
	if backup.IsRunning() {
		c.JSON(200, gin.H{
			"success": false,
			"error":   "备份任务正在运行中",
		})
		return
	}

	backupDir := filepath.Join(helpers.ConfigDir, "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		c.JSON(200, gin.H{
			"success": false,
			"error":   "创建备份目录失败: " + err.Error(),
		})
		return
	}

	go func() {
		if err := s.performMigrateBackup(); err != nil {
			log.Printf("迁移备份失败: %v", err)
		}
	}()

	c.JSON(200, gin.H{
		"success": true,
		"message": "备份任务已启动",
	})
}

func (s *MigrateServer) performMigrateBackup() error {
	totalTable := 35
	count := 0
	backupDir := filepath.Join(helpers.ConfigDir, "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		helpers.AppLogger.Errorf("创建备份目录失败: %v", err)
		return err
	}
	if backup.IsRunning() {
		return fmt.Errorf("备份任务正在运行中")
	}
	backup.SetRunning(true)
	defer backup.SetRunning(false)
	backup.SetRunningResult("backup", "开始迁移备份", totalTable, count, "", true)

	backupRecordDir := filepath.Join(backupDir, "migrate_temp")
	if err := os.MkdirAll(backupRecordDir, 0755); err != nil {
		backup.SetRunningResult("backup", "创建备份目录失败", totalTable, count, err.Error(), false)
		return err
	}
	defer os.RemoveAll(backupRecordDir)

	if err := backupToJsonFile(backupRecordDir, "Account", totalTable, &count, models.Account{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "Migrator", totalTable, &count, models.Migrator{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "ApiKey", totalTable, &count, models.ApiKey{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "DbDownloadTask", totalTable, &count, models.DbDownloadTask{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "DbUploadTask", totalTable, &count, models.DbUploadTask{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "Settings", totalTable, &count, models.Settings{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "User", totalTable, &count, models.User{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "Sync", totalTable, &count, models.Sync{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "SyncPath", totalTable, &count, models.SyncPath{}); err != nil {
		return err
	}

	if err := backupToJsonFile(backupRecordDir, "ScrapePathCategories", totalTable, &count, models.ScrapePathCategory{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "MovieCategories", totalTable, &count, models.MovieCategory{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "TvShowCategories", totalTable, &count, models.TvShowCategory{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "Media", totalTable, &count, models.Media{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "MediaSeason", totalTable, &count, models.MediaSeason{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "MediaEpisode", totalTable, &count, models.MediaEpisode{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "ScrapeStrmPath", totalTable, &count, models.ScrapeStrmPath{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "EmbyConfig", totalTable, &count, models.EmbyConfig{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "EmbyLibrary", totalTable, &count, models.EmbyLibrary{}); err != nil {
		return err
	}
	// if err := backupToJsonFile(backupRecordDir, "EmbyMediaItem", totalTable, &count, models.EmbyMediaItem{}); err != nil {
	// 	return err
	// }
	if err := backupToJsonFile(backupRecordDir, "EmbyMediaSyncFile", totalTable, &count, models.EmbyMediaSyncFile{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "EmbyLibrarySyncPath", totalTable, &count, models.EmbyLibrarySyncPath{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "RequestStat", totalTable, &count, models.RequestStat{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "BackupConfig", totalTable, &count, models.BackupConfig{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "BackupRecord", totalTable, &count, models.BackupRecord{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "BarkChannelConfig", totalTable, &count, models.BarkChannelConfig{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "CustomWebhookChannelConfig", totalTable, &count, models.CustomWebhookChannelConfig{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "MeowChannelConfig", totalTable, &count, models.MeoWChannelConfig{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "TelegramChannelConfig", totalTable, &count, models.TelegramChannelConfig{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "NotificationChannel", totalTable, &count, models.NotificationChannel{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "ServerChanChannelConfig", totalTable, &count, models.ServerChanChannelConfig{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "NotificationRule", totalTable, &count, models.NotificationRule{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "SyncPathScrapePath", totalTable, &count, models.SyncPathScrapePath{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "ScrapePath", totalTable, &count, models.ScrapePath{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "SyncFile", totalTable, &count, models.SyncFile{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "ScrapeSettings", totalTable, &count, models.ScrapeSettings{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "ScrapeMediaFile", totalTable, &count, models.ScrapeMediaFile{}); err != nil {
		return err
	}

	if err := helpers.ZipDir(backupRecordDir, s.backupPath); err != nil {
		backup.SetRunningResult("backup", "打包备份失败", totalTable, count, err.Error(), false)
		return err
	}

	backup.SetRunningResult("backup", "备份完成，正在停止内嵌数据库...", totalTable, count, "", false)
	helpers.AppLogger.Infof("迁移备份完成，文件保存到: %s", s.backupPath)

	if s.dbManager != nil {
		if err := s.dbManager.Stop(); err != nil {
			helpers.AppLogger.Warnf("停止内嵌数据库失败: %v", err)
		} else {
			helpers.AppLogger.Info("内嵌数据库已停止")
		}
	}

	backup.SetRunningResult("backup", "备份完成", totalTable, count, "", false)
	return nil
}

func backupToJsonFile[T any](backupDir string, modelName string, totalTable int, count *int, model T) error {
	backupFilePath := filepath.Join(backupDir, modelName+".json")
	backupFile, err := os.OpenFile(backupFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		helpers.AppLogger.Errorf("创建%s备份文件失败: %v", modelName, err)
		return err
	}
	defer backupFile.Close()

	pageSize := 100
	page := 0
	totalCount := 0
	for {
		var records []T
		if err := db.Db.Model(&model).Offset(page * pageSize).Limit(pageSize).Find(&records).Error; err != nil {
			helpers.AppLogger.Errorf("查询%s失败: %v", modelName, err)
			break
		}
		if len(records) == 0 {
			helpers.AppLogger.Infof("查询%s完成", modelName)
			break
		}

		for _, record := range records {
			jsonStr := helpers.JsonString(record)
			if _, err := backupFile.WriteString(jsonStr + "\n"); err != nil {
				helpers.AppLogger.Errorf("写入%s备份文件失败: %v", modelName, err)
				return err
			}
			totalCount++
		}
		page++
	}

	*count++
	backup.SetRunningResult("backup", fmt.Sprintf("已备份%s %d条", modelName, totalCount), totalTable, *count, "", false)
	return nil
}

func (s *MigrateServer) handleBackupStatus(c *gin.Context) {
	result := backup.GetRunningResult()
	c.JSON(200, result)
}

func (s *MigrateServer) handleTestDB(c *gin.Context) {
	var req struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		Database string `json:"database"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"success": false, "error": err.Error()})
		return
	}

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=disable connect_timeout=5",
		req.Host, req.Port, req.User, req.Password)
	sqlDB, err := sql.Open("postgres", connStr)
	if err != nil {
		c.JSON(200, gin.H{"success": false, "error": "连接失败: " + err.Error()})
		return
	}
	defer sqlDB.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		c.JSON(200, gin.H{"success": false, "error": "连接失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "数据库连接成功"})
}

func (s *MigrateServer) handleSaveConfig(c *gin.Context) {
	var req struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		Database string `json:"database"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	helpers.GlobalConfig.Db.PostgresType = helpers.PostgresTypeExternal
	helpers.GlobalConfig.Db.PostgresConfig = helpers.PostgresConfig{
		Host:         req.Host,
		Port:         req.Port,
		User:         req.User,
		Password:     req.Password,
		Database:     req.Database,
		MaxOpenConns: 25,
		MaxIdleConns: 25,
	}

	if err := helpers.SaveConfig(&helpers.GlobalConfig); err != nil {
		c.JSON(500, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}

	postgresDir := filepath.Join(helpers.ConfigDir, "postgres")
	postgresBackupDir := filepath.Join(helpers.ConfigDir, "postgres-backup")
	if helpers.PathExists(postgresDir) {
		if err := os.Rename(postgresDir, postgresBackupDir); err != nil {
			helpers.AppLogger.Warnf("重命名postgres目录失败: %v", err)
		} else {
			helpers.AppLogger.Info("已将postgres目录重命名为postgres-backup")
		}
	}

	s.isCompleted = true
	c.JSON(200, gin.H{
		"success": true,
		"message": "配置已保存，程序即将退出，请重新启动",
	})

	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
}

func (s *MigrateServer) Stop() {
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.httpServer.Shutdown(ctx)
	}
}

func ShouldMigrate() bool {
	return helpers.GlobalConfig.Db.Engine == helpers.DbEnginePostgres &&
		helpers.GlobalConfig.Db.PostgresType == helpers.PostgresTypeEmbedded
}

func ShouldRestore() bool {
	backupPath := filepath.Join(helpers.ConfigDir, "backups", "migrate.zip")
	return helpers.PathExists(backupPath) &&
		helpers.GlobalConfig.Db.Engine == helpers.DbEnginePostgres &&
		helpers.GlobalConfig.Db.PostgresType == helpers.PostgresTypeExternal
}

func GetMigrateBackupPath() string {
	return filepath.Join(helpers.ConfigDir, "backups", "migrate.zip")
}
