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
	"reflect"
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
		helpers.AppLogger.Infof("已将任务设置为进行中: %d", atomic.LoadInt32(&isRuning))
		if runningResult != nil {
			runningResult.IsRunning = true
		}
	} else {
		atomic.StoreInt32(&isRuning, 0)
		helpers.AppLogger.Infof("已将任务设置为完成: %d", atomic.LoadInt32(&isRuning))
		if runningResult != nil {
			runningResult.IsRunning = false
		}
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
	if synccron.ScrapeCron != nil {
		synccron.ScrapeCron.Stop()
	}
	if models.GlobalDownloadQueue != nil {
		models.GlobalDownloadQueue.Stop()
	}
	if models.GlobalUploadQueue != nil {
		models.GlobalUploadQueue.Stop()
	}
	emby.SetEmbySyncRunning(true)
	return nil
}

func startAllTasks() error {
	synccron.ResumeAllNewSyncQueues()
	synccron.InitCron()
	synccron.InitSyncCron()
	synccron.InitScrapeCron()
	if models.GlobalDownloadQueue != nil {
		models.GlobalDownloadQueue.Start()
	}
	if models.GlobalUploadQueue != nil {
		models.GlobalUploadQueue.Start()
	}
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
	totalTable := len(models.AllTables)
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
	if err := db.Db.Save(record).Error; err != nil {
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
	for _, table := range models.AllTables {
		if err := backupToJsonFile(backupRecordDir, helpers.GetStructName(table), totalTable, &count, table); err != nil {
			return err
		}
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
func backupToJsonFile(backupDir string, modelName string, totalTable int, count *int, model any) error {
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
	typ := reflect.TypeOf(model)
	sliceType := reflect.SliceOf(typ)
	setCount := 0
	for {
		records := reflect.New(sliceType).Interface()
		if err := db.Db.Model(model).Offset(page * pageSize).Limit(pageSize).Order("id").Find(records).Error; err != nil {
			helpers.AppLogger.Errorf("查询%s失败: %v", modelName, err)
			return err
		}
		recordsValue := reflect.ValueOf(records).Elem()
		if recordsValue.Len() == 0 {
			helpers.AppLogger.Infof("查询%s完成", modelName)
			break
		}

		for i := 0; i < recordsValue.Len(); i++ {
			record := recordsValue.Index(i).Interface()
			jsonStr := helpers.JsonString(record)
			_, err := backupFile.WriteString(jsonStr + "\n")
			if err != nil {
				helpers.AppLogger.Errorf("写入%s备份文件失败: %v", modelName, err)
			}
			totalCount++
			setCount++
			if setCount >= 10 {
				setCount = 0
				SetRunningResult("backup", fmt.Sprintf("已备份%s %d条", modelName, totalCount), totalTable, *count, "", false)
			}
		}
		page++
	}
	*count++
	SetRunningResult("backup", fmt.Sprintf("已备份%s %d条", modelName, totalCount), totalTable, *count, "", false)
	return nil
}
