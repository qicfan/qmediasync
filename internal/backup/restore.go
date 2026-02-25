package backup

import (
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 从文件还原到数据库
func Restore(filePath string) error {
	totalTable := 35
	count := 0
	// 检查是否正在运行
	if IsRunning() {
		return fmt.Errorf("备份或还原任务正在运行中")
	}
	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("备份文件不存在")
	}
	SetRunning(true)
	defer SetRunning(false)
	// 停止所有任务
	stopAllTasks()
	defer startAllTasks()
	// 解压到临时目录
	tempDir, err := os.MkdirTemp(filepath.Join(helpers.ConfigDir, "backups"), "backup-restore-*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)
	// 解压文件
	if err := helpers.ExtractZip(filePath, tempDir); err != nil {
		return fmt.Errorf("解压文件失败: %v", err)
	}
	// 开始还原
	SetRunningResult("restore", "开始还原数据库", totalTable, count, "", true)
	if err := restoreFromJsonFile(tempDir, "Account", totalTable, &count, models.Account{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "ApiKey", totalTable, &count, models.ApiKey{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "DbDownloadTask", totalTable, &count, models.DbDownloadTask{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "DbUploadTask", totalTable, &count, models.DbUploadTask{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "Settings", totalTable, &count, models.Settings{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "User", totalTable, &count, models.User{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "Sync", totalTable, &count, models.Sync{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "SyncPath", totalTable, &count, models.SyncPath{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "SyncFile", totalTable, &count, models.SyncFile{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "ScrapeSettings", totalTable, &count, models.ScrapeSettings{}); err != nil {
		return err
	}

	if err := restoreFromJsonFile(tempDir, "ScrapePath", totalTable, &count, models.ScrapePath{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "ScrapeMediaFile", totalTable, &count, models.ScrapeMediaFile{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "ScrapePathCategory", totalTable, &count, models.ScrapePathCategory{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "MovieCategory", totalTable, &count, models.MovieCategory{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "TvShowCategory", totalTable, &count, models.TvShowCategory{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "Media", totalTable, &count, models.Media{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "MediaSeason", totalTable, &count, models.MediaSeason{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "MediaEpisode", totalTable, &count, models.MediaEpisode{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "ScrapeStrmPath", totalTable, &count, models.ScrapeStrmPath{}); err != nil {
		return err
	}

	if err := restoreFromJsonFile(tempDir, "EmbyConfig", totalTable, &count, models.EmbyConfig{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "EmbyLibrary", totalTable, &count, models.EmbyLibrary{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "EmbyMediaItem", totalTable, &count, models.EmbyMediaItem{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "EmbyMediaSyncFile", totalTable, &count, models.EmbyMediaSyncFile{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "EmbyLibrarySyncPath", totalTable, &count, models.EmbyLibrarySyncPath{}); err != nil {
		return err
	}

	if err := restoreFromJsonFile(tempDir, "RequestStat", totalTable, &count, models.RequestStat{}); err != nil {
		return err
	}

	if err := restoreFromJsonFile(tempDir, "BackupConfig", totalTable, &count, models.BackupConfig{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "BackupRecord", totalTable, &count, models.BackupRecord{}); err != nil {
		return err
	}

	if err := restoreFromJsonFile(tempDir, "BarkChannelConfig", totalTable, &count, models.BarkChannelConfig{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "CustomWebhookChannelConfig", totalTable, &count, models.CustomWebhookChannelConfig{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "MeowChannelConfig", totalTable, &count, models.MeoWChannelConfig{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "TelegramChannelConfig", totalTable, &count, models.TelegramChannelConfig{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "NotificationChannel", totalTable, &count, models.NotificationChannel{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "ServerChanChannelConfig", totalTable, &count, models.ServerChanChannelConfig{}); err != nil {
		return err
	}
	if err := restoreFromJsonFile(tempDir, "NotificationRule", totalTable, &count, models.NotificationRule{}); err != nil {
		return err
	}

	if err := restoreFromJsonFile(tempDir, "Migrator", totalTable, &count, models.Migrator{}); err != nil {
		return err
	}
	helpers.AppLogger.Infof("完成恢复任务")
	return nil
}

// 从json文件还原到数据库
func restoreFromJsonFile[T any](backupDir string, modelName string, totalTable int, count *int, model T) error {
	backupFilePath := filepath.Join(backupDir, modelName+".json")
	// 检查文件是否存在
	if _, err := os.Stat(backupFilePath); os.IsNotExist(err) {
		helpers.AppLogger.Warnf("备份文件不存在: %s", backupFilePath)
		return nil
	}
	// 读取文件，一行是一条json
	file, err := os.Open(backupFilePath)
	if err != nil {
		helpers.AppLogger.Warnf("打开备份文件 %s 失败: %v", backupFilePath, err)
		return fmt.Errorf("打开备份文件 %s 失败: %v", backupFilePath, err)
	}
	defer file.Close()
	// 1. 删除表（如果存在）
	err = db.Db.Migrator().DropTable(&model)
	if err != nil {
		// 处理错误
		helpers.AppLogger.Warnf("删除表 %s 失败: %v", modelName, err)
		return fmt.Errorf("删除表 %s 失败: %v", modelName, err)
	} else {
		helpers.AppLogger.Infof("表 %s 已删除", modelName)
	}

	// 2. 重新创建表
	err = db.Db.AutoMigrate(&model)
	if err != nil {
		// 处理错误
		helpers.AppLogger.Warnf("创建表 %s 失败: %v", modelName, err)
		if strings.Contains(err.Error(), "index") {
			helpers.AppLogger.Infof("表 %s 索引创建失败，跳过错误，继续导入数据", modelName)
		} else {
			return fmt.Errorf("创建表 %s 失败: %v", modelName, err)
		}
	} else {
		helpers.AppLogger.Infof("表 %s 已创建", modelName)
	}
	// 读取文件内容
	scanner := bufio.NewScanner(file)
	// 统计还原数量
	var restoredCount int
	for scanner.Scan() {
		line := scanner.Text()
		// 解析json
		var item T
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			return fmt.Errorf("解析json失败: %v", err)
		}
		// 插入数据库
		if err := db.Db.Create(&item).Error; err != nil {
			return fmt.Errorf("插入数据库失败: %v", err)
		}
		restoredCount++
	}
	*count++
	SetRunningResult("restore", fmt.Sprintf("已还原 %d 条 %s 记录", restoredCount, modelName), totalTable, *count, "", false)
	return nil
}
