package backup

import (
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/emby"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/synccron"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

var isRuning int32 = 0

type BackupOrRestoreResult struct {
	Type      string    `json:"type"`       // 备份类型: backup or restore
	Desc      string    `json:"desc"`       // 当前操作描述
	Total     int       `json:"total"`      // 需要备份的数量
	Count     int       `json:"count"`      // 已备份的数量
	ErrorMsg  string    `json:"error_msg"`  // 错误信息（如果有错误发生，则会包含错误信息）
	IsRunning bool      `json:"is_running"` // 是否正在运行（如果为否，则备份完成）
	StartTime time.Time `json:"start_time"` // 开始时间
	Elapsed   float64   `json:"elapsed"`    // 已用时间（秒）
}

var runningResult *BackupOrRestoreResult

func GetRunningResult() *BackupOrRestoreResult {
	return runningResult
}

func SetRunningResult(t string, desc string, total int, count int, errorMsg string, new bool) {
	if runningResult == nil {
		runningResult = &BackupOrRestoreResult{}
	}
	if new {
		runningResult.StartTime = time.Now()
		runningResult.Elapsed = 0
	}
	runningResult.Type = t
	runningResult.Desc = desc
	runningResult.Total = total
	runningResult.Count = count
	runningResult.ErrorMsg = errorMsg
	runningResult.IsRunning = IsRunning()
	runningResult.Elapsed = time.Since(runningResult.StartTime).Seconds()
	helpers.AppLogger.Infof("%s %s %d/%d张表", runningResult.Type, runningResult.Desc, runningResult.Count, runningResult.Total)
}

func IsRunning() bool {
	return atomic.LoadInt32(&isRuning) == 1
}

func SetRunning(running bool) {
	if running {
		atomic.StoreInt32(&isRuning, 1)
	} else {
		atomic.StoreInt32(&isRuning, 0)
	}
}

// 备份之前先停止所有同步任务、上传下载任务、定时任务
func stopAllTasks() error {
	synccron.PauseAllNewSyncQueues()
	if synccron.SyncCron != nil {
		synccron.SyncCron.Stop()
	}
	if synccron.GlobalCron != nil {
		synccron.GlobalCron.Stop()
	}
	models.GlobalDownloadQueue.Stop()
	models.GlobalUploadQueue.Stop()
	emby.SetEmbySyncRunning(true)
	return nil
}

func startAllTasks() error {
	synccron.ResumeAllNewSyncQueues()
	synccron.InitSyncCron()
	synccron.InitSyncCron()
	models.GlobalDownloadQueue.Start()
	models.GlobalUploadQueue.Start()
	emby.SetEmbySyncRunning(false)
	return nil
}

// 每个表一个文件
// 每个文件的文件名格式为：模型名.json
// 文件中每一行都是一个json格式的字符串，代表一条数据
// 首先生成一个备份记录
// 然后将运行中状态为1

// 遍历每一个模型，生成json格式的备份文件
func Backup(backupType string, reason string) error {
	totalTable := 35
	count := 0
	// config := models.GetOrCreateBackupConfig()
	backupDir := filepath.Join(helpers.ConfigDir, "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		helpers.AppLogger.Errorf("创建备份目录失败: %v", err)
	}
	if IsRunning() {
		return fmt.Errorf("备份任务正在运行中")
	}
	SetRunning(true)
	defer SetRunning(false)
	SetRunningResult("backup", fmt.Sprintf("开始%s备份", backupType), totalTable, count, "", true)
	// 清理旧备份
	models.GetBackupService().CleanupOldBackups()
	record := &models.BackupRecord{
		Status:        models.BackupStatusRunning,
		BackupType:    backupType,
		CreatedReason: reason,
	}
	if err := db.Db.Create(record).Error; err != nil {
		helpers.AppLogger.Errorf("创建备份记录失败: %v", err)
		return err
	}
	startTime := time.Now()
	helpers.AppLogger.Infof("开始%s备份，备份记录ID: %d", backupType, record.ID)
	if err := stopAllTasks(); err != nil {
		return err
	}
	SetRunningResult("backup", "已停止所有同步任务、上传下载任务、定时任务", totalTable, count, "", false)
	defer startAllTasks()

	// 创建备份目录
	backupRecordDir := filepath.Join(backupDir, fmt.Sprintf("%d", record.ID))
	if err := os.MkdirAll(backupRecordDir, 0755); err != nil {
		helpers.AppLogger.Errorf("创建备份目录失败: %v", err)
		return err
	}
	// 如果本方法返回的不是nil，defer中删除这个目录
	defer func() {
		if r := recover(); r != nil {
			helpers.AppLogger.Errorf("备份任务Panic: %v", r)
			record.Status = models.BackupStatusFailed
			record.BackupDuration = int64(time.Since(startTime).Seconds())
			db.Db.Save(record)
			// 删除目录
			os.RemoveAll(backupRecordDir)
		}
	}()
	if err := backupToJsonFile(backupRecordDir, "Account", totalTable, &count, models.Account{}); err != nil {
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
	if err := backupToJsonFile(backupRecordDir, "SyncFile", totalTable, &count, models.SyncFile{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "ScrapeSettings", totalTable, &count, models.ScrapeSettings{}); err != nil {
		return err
	}

	if err := backupToJsonFile(backupRecordDir, "ScrapePath", totalTable, &count, models.ScrapePath{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "ScrapeMediaFile", totalTable, &count, models.ScrapeMediaFile{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "ScrapePathCategory", totalTable, &count, models.ScrapePathCategory{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "MovieCategory", totalTable, &count, models.MovieCategory{}); err != nil {
		return err
	}
	if err := backupToJsonFile(backupRecordDir, "TvShowCategory", totalTable, &count, models.TvShowCategory{}); err != nil {
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
	if err := backupToJsonFile(backupRecordDir, "EmbyMediaItem", totalTable, &count, models.EmbyMediaItem{}); err != nil {
		return err
	}
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

	if err := backupToJsonFile(backupRecordDir, "Migrator", totalTable, &count, models.Migrator{}); err != nil {
		return err
	}

	record.Status = models.BackupStatusCompleted
	record.BackupDuration = int64(time.Since(startTime).Seconds())
	var fileName string
	timestamp := time.Now().Format("20060102_150405")
	fileName = fmt.Sprintf("backup_%s_%s.zip", backupType, timestamp)
	filePath := filepath.Join(backupDir, fileName)
	// 打包目录中所有文件，生成一个压缩包
	if err := helpers.ZipDir(backupRecordDir, filePath); err != nil {
		helpers.AppLogger.Errorf("打包备份目录失败: %v", err)
		return err
	}
	stat, err := os.Stat(filePath)
	if err != nil {
		helpers.AppLogger.Errorf("获取备份文件状态失败: %v", err)
		return err
	}
	record.FilePath = filePath
	record.FileSize = stat.Size()
	record.BackupDuration = int64(time.Since(startTime).Seconds())
	record.TableCount = totalTable
	db.Db.Save(record)
	// 删除目录
	os.RemoveAll(backupRecordDir)
	return nil
}

// 备份账号信息
func backupToJsonFile[T any](backupDir string, modelName string, totalTable int, count *int, model T) error {
	// 打开一个文件用来写入
	backupFilePath := filepath.Join(backupDir, modelName+".json")
	backupFile, err := os.OpenFile(backupFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		helpers.AppLogger.Errorf("创建%s备份文件失败: %v", modelName, err)
		return err
	}
	defer backupFile.Close()
	// 从数据库中分页查询所有数据，每页100条
	pageSize := 100
	page := 0
	totalCount := 0
	for {
		var records []T
		if err := db.Db.Model(&model).Offset(page * pageSize).Limit(pageSize).Find(&records).Error; err != nil {
			helpers.AppLogger.Errorf("查询%s失败: %v", modelName, err)
			return err
		}
		if len(records) == 0 {
			helpers.AppLogger.Infof("查询%s完成", modelName)
			break
		}

		// 写入文件
		for _, record := range records {
			// helpers.AppLogger.Infof("备份%s: %v", modelName, record)
			// 序列化成json，然后写入文件中的末尾
			jsonStr := helpers.JsonString(record)
			_, err := backupFile.WriteString(jsonStr + "\n")
			if err != nil {
				helpers.AppLogger.Errorf("写入%s备份文件失败: %v", modelName, err)
				return err
			}
			totalCount++
		}
		page++
	}
	*count++
	SetRunningResult("backup", fmt.Sprintf("已备份%s %d条", modelName, totalCount), totalTable, *count, "", false)
	return nil
}
