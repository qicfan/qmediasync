package models

import (
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/helpers"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// UploadStatus 上传状态
type UploadStatus int

const (
	UploadStatusPending   UploadStatus = iota // 等待中
	UploadStatusUploading                     // 上传中
	UploadStatusCompleted                     // 已完成
	UploadStatusFailed                        // 失败
	UploadStatusCancelled                     // 已取消
	UploadStatusAll       = -1                // 所有状态
)

type UploadSource string

const (
	UploadSourceStrm   UploadSource = "strm同步"
	UploadSourceScrape UploadSource = "刮削整理"
)

type DbUploadTask struct {
	BaseModel
	Source               UploadSource     `json:"source"` // 任务来源
	AccountId            uint             `json:"account_id"`
	SyncFileId           uint             `json:"sync_file_id"`                                     // 同步文件ID
	ScrapeMediaFileId    uint             `json:"scrape_media_file_id"`                             // 刮削文件ID
	SourceType           SourceType       `json:"source_type"`                                      // 任务来源类型
	LocalFullPath        string           `json:"local_full_path" gorm:"index:idx_local_full_path"` // 本地完整文件路径，包含文件名
	RemoteFileId         string           `json:"remote_file_id" gorm:"index:idx_remote_file_id"`   // 远程文件ID，包含完整路径
	RemotePathId         string           `json:"remote_path_id"`                                   // 父目录CID，如果115就是文件夹ID，如果是openlist就是父文件夹路径
	FileName             string           `json:"file_name"`                                        // 要上传的文件名
	Status               UploadStatus     `json:"status" gorm:"index:idx_status_new"`               // 任务状态
	FileSize             int64            `json:"file_size"`                                        // 文件大小
	Error                string           `json:"error"`                                            // 错误信息
	StartTime            int64            `json:"start_time"`                                       // 开始时间
	EndTime              int64            `json:"end_time"`                                         // 结束时间
	IsSeasonOrTvshowFile bool             `json:"is_season_or_tvshow_file"`                         // 是否是剧集或电视剧文件
	SyncFile             *SyncFile        `json:"-" gorm:"-"`                                       // 同步文件
	ScrapeMediaFile      *ScrapeMediaFile `json:"-" gorm:"-"`                                       // 刮削文件
	Account              *Account         `json:"-" gorm:"-"`                                       // 账户
}

// String 返回状态的字符串表示
func (s UploadStatus) String() string {
	switch s {
	case UploadStatusPending:
		return "pending"
	case UploadStatusUploading:
		return "uploading"
	case UploadStatusCompleted:
		return "completed"
	case UploadStatusFailed:
		return "failed"
	case UploadStatusCancelled:
		return "cancelled"
	default:
		return "unknown"
	}
}

func (task *DbUploadTask) Complete() {
	// 标记为已完成
	task.Status = UploadStatusCompleted
	task.EndTime = time.Now().Unix()
	err := db.Db.Save(task).Error
	if err != nil {
		helpers.AppLogger.Warnf("[上传] 标记为已完成失败: %s", err.Error())
	}
}

func (task *DbUploadTask) Fail(err error) {
	// 标记为失败
	task.Status = UploadStatusFailed
	task.EndTime = time.Now().Unix()
	task.Error = err.Error()
	err = db.Db.Save(task).Error
	if err != nil {
		helpers.AppLogger.Warnf("[上传] 标记为失败失败: %s", err.Error())
	}
}

func (task *DbUploadTask) Cancel() {
	// 标记为已取消
	task.Status = UploadStatusCancelled
	task.EndTime = time.Now().Unix()
	err := db.Db.Save(task).Error
	if err != nil {
		helpers.AppLogger.Warnf("[上传] 标记为已取消失败: %s", err.Error())
	}
}

func (task *DbUploadTask) Uploading() {
	task.Status = UploadStatusUploading
	task.StartTime = time.Now().Unix()
	err := db.Db.Save(task).Error
	if err != nil {
		helpers.AppLogger.Warnf("[上传] 标记为上传中失败: %s", err.Error())
	}
}

func (task *DbUploadTask) GetAccount() *Account {
	if task.Account != nil {
		return task.Account
	}
	// 通过AccountId查询账户，然后判断是什么来源
	account, err := GetAccountById(task.AccountId)
	if err != nil {
		task.Fail(err)
		return nil
	}
	task.Account = account
	return account
}

// 执行上传
func (task *DbUploadTask) Upload() {
	if !helpers.PathExists(task.LocalFullPath) {
		task.Fail(fmt.Errorf("本地文件 %s 不存在", task.LocalFullPath))
		return
	}
	switch task.SourceType {
	case SourceType115:
		if !task.Upload115File() {
			return
		}
	case SourceTypeOpenList:
		if !task.UploadOpenListFile() {
			return
		}
	case SourceTypeLocal:
		if !task.UploadLocalFile() {
			return
		}
	case SourceTypeBaiduPan:
		if !task.UploadBaiduPanFile() {
			return
		}
	default:
		task.Fail(fmt.Errorf("未知的上传来源类型 %s", task.SourceType))
		return
	}
	// 标记为已完成
	task.Complete()
	// 如果是刮削类型,需要进行后续通知
	if task.Source == UploadSourceScrape {
		// 通知刮削整理完成
		scrapeMediaFile := GetScrapeMediaFileById(task.ScrapeMediaFileId)
		if scrapeMediaFile == nil {
			helpers.AppLogger.Errorf("刮削文件 %d 不存在", task.ScrapeMediaFileId)
			return
		}
		scrapeMediaFile.RemoveTmpFiles(task)
	}
}

func (task *DbUploadTask) Upload115File() bool {
	// 检查账户是否存在
	account := task.GetAccount()
	if account == nil {
		task.Fail(fmt.Errorf("账户 %d 不存在", task.AccountId))
		return false
	}
	// 上传文件
	client := account.Get115Client()
	if client == nil {
		task.Fail(fmt.Errorf("账户 %s 115客户端不存在", account.Name))
		return false
	}
	task.Uploading()
	// var file *SyncFile
	// if task.Source == UploadSourceStrm {
	// 	file = GetSyncFileById(task.SyncFileId)
	// 	if file == nil {
	// 		task.Fail(fmt.Errorf("同步文件 %d 不存在", task.SyncFileId))
	// 		return false
	// 	}
	// }
	// 检查远程文件是否存在
	detail, existsErr := client.GetFsDetailByPath(context.Background(), task.RemoteFileId)

	if existsErr == nil && detail.FileId != "" {
		if task.Source == UploadSourceStrm {
			return true
		}
		if task.Source == UploadSourceScrape {
			// 回调
			scrapeMediaFile := GetScrapeMediaFileById(task.ScrapeMediaFileId)
			if scrapeMediaFile == nil {
				helpers.AppLogger.Errorf("刮削文件 %d 不存在", task.ScrapeMediaFileId)
				task.Fail(fmt.Errorf("同步文件 %d 不存在", task.SyncFileId))
				return false
			}
			helpers.AppLogger.Infof("回调刮削整理：刮削文件 %d 上传成功, 文件ID: %s", task.ScrapeMediaFileId, task.RemoteFileId)
			scrapeMediaFile.RemoveTmpFiles(task)
			return true
		}
	}
	// 检查父目录是否存在
	detail, existsErr = client.GetFsDetailByCid(context.Background(), task.RemotePathId)
	if existsErr != nil {
		task.Fail(fmt.Errorf("115检查父目录 %s 失败: %s", task.RemotePathId, existsErr.Error()))
		return false
	}
	if detail.FileId == "" {
		task.Fail(fmt.Errorf("115检查父目录 %s 失败: 返回空文件ID", task.RemotePathId))
		return false
	}
	helpers.AppLogger.Infof("准备将文件 %s 上传到115目录 %s", task.LocalFullPath, task.RemotePathId)
	// 上传文件
	fileId, err := client.Upload(context.Background(), task.LocalFullPath, task.RemotePathId, "", "")
	if err != nil {
		task.Fail(fmt.Errorf("调用115上传API失败: %v", err))
		return false
	}
	if fileId == "" {
		task.Fail(fmt.Errorf("115上传文件 %s 失败: 返回空文件ID", task.FileName))
		return false
	}
	helpers.AppLogger.Infof("115上传文件 %s 成功, 新的文件ID: %s", task.LocalFullPath, fileId)
	if task.Source == UploadSourceStrm {
		// 查询文件详情，然后更新本地文件的修改时间
		detail, err = client.GetFsDetailByCid(context.Background(), fileId)
		if err != nil {
			task.Fail(fmt.Errorf("115查询文件详情 %s 失败: %s", fileId, err.Error()))
			return false
		}
		if detail.FileId == "" {
			task.Fail(fmt.Errorf("115查询文件详情 %s 失败: 返回空文件ID", fileId))
			return false
		}
		mtime := helpers.StringToInt64(detail.Ptime)
		// 更新本地文件的修改时间
		err = os.Chtimes(task.LocalFullPath, time.Unix(mtime, 0), time.Unix(mtime, 0))
		if err != nil {
			task.Fail(fmt.Errorf("更新本地文件 %s 修改时间失败: %v", task.LocalFullPath, err))
			return false
		}
	}
	return true
}

// 百度网盘上传文件
func (task *DbUploadTask) UploadBaiduPanFile() bool {
	// 检查账户是否存在
	account := task.GetAccount()
	if account == nil {
		task.Fail(fmt.Errorf("账户 %d 不存在", task.AccountId))
		return false
	}
	// 上传文件
	client := account.GetBaiDuPanClient()
	if client == nil {
		task.Fail(fmt.Errorf("账户 %s 百度网盘客户端不存在", account.Name))
		return false
	}
	task.Uploading()
	// 调用上传方法
	resp, err := client.Upload(context.Background(), task.LocalFullPath, task.RemoteFileId)
	if err != nil {
		task.Fail(fmt.Errorf("百度网盘上传文件 %s 失败: %v", task.FileName, err))
		return false
	}
	if task.Source == UploadSourceStrm {
		t := time.Unix(int64(*resp.Mtime), 0)
		// 更新本地文件的修改时间
		err = os.Chtimes(task.LocalFullPath, t, t)
		if err != nil {
			task.Fail(fmt.Errorf("更新本地文件 %s 修改时间失败: %v", task.LocalFullPath, err))
			return false
		}
	}
	return true
}

func (task *DbUploadTask) UploadOpenListFile() bool {
	// 检查账户是否存在
	account := task.GetAccount()
	if account == nil {
		task.Fail(fmt.Errorf("账户 %d 不存在", task.AccountId))
		return false
	}
	// 上传文件
	client := account.GetOpenListClient()
	if client == nil {
		task.Fail(fmt.Errorf("账户 %s OpenList客户端不存在", account.Name))
		return false
	}
	task.Uploading()
	_, err := client.Upload(task.LocalFullPath, task.RemoteFileId)
	if err != nil {
		task.Fail(fmt.Errorf("OpenList上传文件 %s 失败: %v", task.FileName, err))
		return false
	}
	if task.Source == UploadSourceStrm {
		// 查询文件详情
		detail, err := client.FileDetail(task.RemoteFileId)
		if err != nil {
			task.Fail(fmt.Errorf("OpenList查询文件详情 %s 失败: %s", task.RemoteFileId, err.Error()))
			return false
		}
		// 将ISO 8601格式的日期字符串转换为时间戳
		t, err := time.Parse(time.RFC3339, detail.Modified)
		if err != nil {
			helpers.AppLogger.Warnf("解析时间格式失败: %v, 时间字符串: %s", err, detail.Modified)
			return true
		}
		// 更新本地文件的修改时间
		err = os.Chtimes(task.LocalFullPath, t, t)
		if err != nil {
			task.Fail(fmt.Errorf("更新本地文件 %s 修改时间失败: %v", task.LocalFullPath, err))
			return false
		}
	}
	return true
}

func (task *DbUploadTask) UploadLocalFile() bool {
	task.Uploading()
	err := helpers.CopyFile(task.LocalFullPath, task.RemoteFileId)
	if err != nil {
		task.Fail(fmt.Errorf("本地文件 %s 复制到 %s 失败: %v", task.LocalFullPath, task.RemoteFileId, err))
		return false
	}
	return true
}

func CheckCanUploadByLocalPath(source UploadSource, localPath string) bool {
	var task *DbUploadTask
	err := db.Db.Model(&DbUploadTask{}).Where("source = ? AND local_full_path = ?", source, localPath).Order("id DESC").First(&task).Error
	if err != nil || task == nil {
		return true
	}
	if task.Status == UploadStatusUploading || task.Status == UploadStatusPending {
		// 待上传或者上传中，不能再次添加任务
		return false
	}
	// 其他状态都可以再次上传
	return true
}

// 检查任务是否已经存在，通过Source + RemoteFileId
func CheckUploadTaskExist(source UploadSource, remoteFileId string) *DbUploadTask {
	var task *DbUploadTask
	err := db.Db.Model(&DbUploadTask{}).
		Where("source = ? AND remote_file_id = ?", source, remoteFileId).
		First(&task).Error
	if err != nil {
		return nil
	}
	return task
}

// 添加strm同步产生的上传任务
func AddUploadTaskFromSyncFile(file *SyncFile) error {
	// 先检查是否存在
	if task := CheckUploadTaskExist(UploadSourceStrm, file.FileId); task != nil {
		if task.Status == UploadStatusPending {
			return errors.New("任务已存在，状态为待上传")
		}
		if task.Status == UploadStatusUploading {
			return errors.New("任务已存在，状态为上传中")
		}
	}
	// if file.SyncPath == nil {
	// 	file.SyncPath = GetSyncPathById(file.SyncPathId)
	// }
	remoteFileId := file.FileId
	if file.SourceType == SourceType115 {
		remoteFileId = filepath.Join(file.Path, file.FileName)
	}
	// 插入新纪录
	task := &DbUploadTask{
		AccountId:     file.AccountId,
		SourceType:    file.SourceType,
		SyncFileId:    file.ID,
		RemoteFileId:  remoteFileId,
		FileName:      file.FileName,
		RemotePathId:  file.ParentId,
		LocalFullPath: file.LocalFilePath,
		Source:        UploadSourceStrm,
		Status:        UploadStatusPending,
		FileSize:      file.FileSize,
	}
	err := db.Db.Create(task).Error
	if err != nil {
		helpers.AppLogger.Errorf("添加上传任务 %s => %s 失败: %s", file.LocalFilePath, remoteFileId, err.Error())
		return err
	}
	helpers.AppLogger.Infof("添加上传任务 %s => %s 成功", file.LocalFilePath, remoteFileId)
	return nil
}

// 添加刮削整理产生的上传任务
func AddUploadTaskFromMediaFile(mediaFile *ScrapeMediaFile, scrapePath *ScrapePath, fileName, localFullPath, remoteFileId, remotePathId string, isSeasonOrTvshowFile bool) error {
	stat, err := os.Stat(localFullPath)
	if err != nil {
		helpers.AppLogger.Errorf("要上传的文件 %s 无法获取到文件信息，错误：%vs", localFullPath, err.Error())
		return err
	}
	size := stat.Size()
	// 先检查是否存在
	if task := CheckUploadTaskExist(UploadSourceScrape, remoteFileId); task != nil {
		if task.Status == UploadStatusPending {
			return errors.New("任务已存在，状态为待上传")
		}
		if task.Status == UploadStatusUploading {
			return errors.New("任务已存在，状态为上传中")
		}
	}
	// 插入新纪录
	task := &DbUploadTask{
		AccountId:            scrapePath.AccountId,
		ScrapeMediaFileId:    mediaFile.ID,
		SourceType:           scrapePath.SourceType,
		RemoteFileId:         remoteFileId,
		FileName:             fileName,
		RemotePathId:         remotePathId,
		LocalFullPath:        localFullPath,
		Source:               UploadSourceScrape,
		Status:               UploadStatusPending,
		FileSize:             size,
		IsSeasonOrTvshowFile: isSeasonOrTvshowFile,
	}
	derr := db.Db.Create(task).Error
	return derr
}

func GetPendingUploadTasks(limit int) []*DbUploadTask {
	var tasks []*DbUploadTask
	db.Db.Model(&DbUploadTask{}).
		Where("status = ?", UploadStatusPending).
		Limit(limit).
		Order("id ASC").
		Find(&tasks)
	return tasks
}

func GetUploadingCount() int64 {
	var count int64
	db.Db.Model(&DbUploadTask{}).
		Where("status = ?", UploadStatusUploading).
		Count(&count)
	return count
}

// 查询上传队列任务列表
func GetUploadTaskList(status UploadStatus, page, pageSize int) ([]*DbUploadTask, int64) {
	var tasks []*DbUploadTask
	var total int64
	tx := db.Db.Model(&DbUploadTask{})
	if status >= 0 {
		tx.Where("status = ?", status)
	}
	tx.Count(&total).
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Order("id DESC").
		Find(&tasks)
	return tasks, total
}

func ClearPendingUploadTasks() error {
	err := db.Db.Model(&DbUploadTask{}).
		Where("status = ?", UploadStatusPending).
		Delete(&DbUploadTask{}).Error
	if err != nil {
		helpers.AppLogger.Errorf("清除待上传任务失败: %v", err)
		return err
	}
	return err
}

func ClearExpireUploadTasks() error {
	err := db.Db.Model(&DbUploadTask{}).
		Where("created_at < ?", time.Now().AddDate(0, 0, -3).Unix()).
		Delete(&DbUploadTask{}).Error
	if err != nil {
		helpers.AppLogger.Errorf("清除3天前的上传任务失败: %v", err)
		return err
	} else {
		helpers.AppLogger.Infof("清除3天前的上传任务成功")
	}
	return err
}

func ClearUploadSuccessAndFailed() error {
	err := db.Db.Model(&DbUploadTask{}).
		Where("status IN (?, ?)", UploadStatusCompleted, UploadStatusFailed).
		Delete(&DbUploadTask{}).Error
	if err != nil {
		helpers.AppLogger.Errorf("清除上传成功和失败任务失败: %v", err)
		return err
	} else {
		helpers.AppLogger.Infof("清除上传成功和失败任务成功")
	}
	return err
}

func UpdateUploadingToPending() error {
	err := db.Db.Model(&DbUploadTask{}).
		Where("status = ?", UploadStatusUploading).
		Update("status", UploadStatusPending).Error
	if err != nil {
		helpers.AppLogger.Errorf("更新上传中的任务为待上传失败: %v", err)
		return err
	} else {
		helpers.AppLogger.Infof("更新上传中的任务为待上传成功")
	}
	return err
}

func RetryFailedUploadTasks() error {
	udpateData := map[string]interface{}{
		"status": UploadStatusPending,
		"error":  "",
	}
	err := db.Db.Model(&DbUploadTask{}).
		Where("status = ?", UploadStatusFailed).
		Updates(udpateData).Error
	if err != nil {
		helpers.AppLogger.Errorf("重试失败的上传任务失败: %v", err)
		return err
	} else {
		helpers.AppLogger.Infof("重试失败的上传任务成功")
	}
	return err
}

func GetUnFinishUploadTaskCountByScrapeMediaId(scrapeMediaFileId uint) int64 {
	var count int64
	db.Db.Model(&DbUploadTask{}).
		Where("scrape_media_file_id = ? AND status IN (?, ?)", scrapeMediaFileId, UploadStatusPending, UploadStatusUploading).
		Count(&count)
	return count
}
