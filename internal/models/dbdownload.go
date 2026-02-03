package models

import (
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/v115open"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"
)

type DownloadSource string

const (
	DownloadSourceStrm      = "strm同步"
	DownloadSourceLocalFile = "本地文件"
	DownloadSourceEmbyMedia = "emby媒体信息提取"
)

// DownloadStatus 下载状态
type DownloadStatus int

const (
	DownloadStatusPending     DownloadStatus = iota // 等待中
	DownloadStatusDownloading                       // 下载中
	DownloadStatusCompleted                         // 已完成
	DownloadStatusFailed                            // 失败
	DownloadStatusCancelled                         // 已取消
	DownloadStatusAll         DownloadStatus = -1   // 所有状态
)

// 数据库下载队列
type DbDownloadTask struct {
	BaseModel
	AccountId     uint           `json:"account_id"`
	SyncFileId    uint           `json:"sync_file_id"`                           // 115文件ID
	SourceType    SourceType     `json:"source_type"`                            // 任务来源类型
	RemoteFileId  string         `json:"remote_file_id" gorm:"index:idx_source"` // 远程文件ID，用来提取实际下载链接，或者这本身就是下载链接
	FileName      string         `json:"file_name"`                              // 文件名，用来显示
	RemotePath    string         `json:"remote_path"`                            // 远程路径，不含文件名
	LocalFullPath string         `json:"local_full_path"`                        // 本地文件路径，下载到这个位置，如果已存在不覆盖，下载前先检查
	Source        DownloadSource `json:"source" gorm:"index:idx_source"`         // 下载来源，目前只有strm同步
	Status        DownloadStatus `json:"status" gorm:"index:idx_status"`         // 下载状态
	Size          int64          `json:"size"`                                   // 文件大小
	StartTime     int64          `json:"start_time"`                             // 开始时间
	EndTime       int64          `json:"end_time"`                               // 结束时间
	Error         string         `json:"error"`                                  // 错误信息
	MTime         int64          `json:"mtime"`                                  // 文件修改时间，下载完文件后要设置为这个时间
	Account       *Account       `json:"-" gorm:"-"`                             // 账户信息
}

func (task *DbDownloadTask) GetAccount() *Account {
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

func (task *DbDownloadTask) Complete() {
	// 标记为已完成
	task.Status = DownloadStatusCompleted
	task.EndTime = time.Now().Unix()
	err := db.Db.Save(task).Error
	if err != nil {
		helpers.AppLogger.Warnf("[下载] 标记为已完成失败: %s", err.Error())
	}
}

func (task *DbDownloadTask) Fail(err error) {
	// 标记为失败
	task.Status = DownloadStatusFailed
	task.EndTime = time.Now().Unix()
	task.Error = err.Error()
	err = db.Db.Save(task).Error
	if err != nil {
		helpers.AppLogger.Warnf("[下载] 标记为失败失败: %s", err.Error())
	}
}

func (task *DbDownloadTask) Cancel() {
	// 标记为已取消
	task.Status = DownloadStatusCancelled
	task.EndTime = time.Now().Unix()
	err := db.Db.Save(task).Error
	if err != nil {
		helpers.AppLogger.Warnf("[下载] 标记为已取消失败: %s", err.Error())
	}
}

func (task *DbDownloadTask) Downloading() {
	task.Status = DownloadStatusDownloading
	task.StartTime = time.Now().Unix()
	err := db.Db.Save(task).Error
	if err != nil {
		helpers.AppLogger.Warnf("[下载] 标记为下载中失败: %s", err.Error())
	}
}

// 执行下载
func (task *DbDownloadTask) Download() {
	if helpers.PathExists(task.LocalFullPath) {
		task.Complete()
		helpers.AppLogger.Infof("文件已存在，无需下载：%s", task.LocalFullPath)
		// 设置文件修改时间
		// task.SetMTime()
		return
	}
	switch task.Source {
	case DownloadSourceStrm:
		account := task.GetAccount()
		if account == nil {
			task.Fail(fmt.Errorf("账户不存在，无法下载文件%s", task.LocalFullPath))
			return
		}
		switch account.SourceType {
		case SourceType115:
			task.Download115File()
		case SourceTypeOpenList:
			task.DownloadOpenListFile()
		case SourceType123:
		}
	case DownloadSourceEmbyMedia:
		// emby媒体信息提取，从emby下载
		task.DownloadEmbyMedia()
	case DownloadSourceLocalFile:
		// 复制本地文件到指定位置
		// 标记为下载中
		task.Downloading()
		err := helpers.CopyFile(task.RemoteFileId, task.LocalFullPath)
		if err != nil {
			helpers.AppLogger.Warnf("[下载] 复制文件失败: %s", err.Error())
			task.Fail(err)
			return
		}
		// 设置文件修改时间
		task.SetMTime()
		task.Complete()
	}

}

func (task *DbDownloadTask) SetMTime() {
	if task.MTime > 0 {
		err := os.Chtimes(task.LocalFullPath, time.Unix(task.MTime, 0), time.Unix(task.MTime, 0))
		if err != nil {
			helpers.AppLogger.Warnf("[下载] 修改文件时间失败: %s", err.Error())
		}
	}
}

func (task *DbDownloadTask) Download115File() {
	account := task.GetAccount()
	if account == nil {
		task.Fail(fmt.Errorf("账户不存在，无法下载文件%s", task.LocalFullPath))
		return
	}
	// if task.SyncFileId == 0 {
	// 	task.Fail(fmt.Errorf("115文件ID为空，无法下载文件%s", task.LocalFullPath))
	// 	return
	// }
	// 先根据pickcode查询115文件
	// file := GetSyncFileById(task.SyncFileId)
	// if file == nil {
	// 	task.Fail(fmt.Errorf("115文件ID不存在，无法下载文件%s", task.LocalFullPath))
	// 	return
	// }
	// 再次检查文件是否已存在
	if helpers.PathExists(task.LocalFullPath) {
		helpers.AppLogger.Infof("[下载] 文件已存在，无需下载：%s", task.LocalFullPath)
		task.Complete()
		return
	}
	// 标记为下载中
	task.Downloading()
	// 查询下载链接
	v115Client := account.Get115Client()
	// 首先获取到下载链接
	url := v115Client.GetDownloadUrl(context.Background(), task.RemoteFileId, v115open.DEFAULTUA, false)
	if url == "" {
		helpers.AppLogger.Warnf("[下载] 获取下载链接失败: %s", task.RemoteFileId)
		task.Fail(fmt.Errorf("获取 %s => %s 的下载链接失败", task.RemoteFileId, task.FileName))
		return
	}
	// 下载文件到指定位置
	downloadErr := helpers.DownloadFile(url, task.LocalFullPath, v115open.DEFAULTUA)
	if downloadErr != nil {
		helpers.AppLogger.Warnf("[下载] 下载文件失败: %s", downloadErr.Error())
		task.Fail(downloadErr)
		return
	}
	// 设置文件修改时间
	task.SetMTime()
	// 下载完成
	task.Complete()
}

func (task *DbDownloadTask) DownloadOpenListFile() {
	account := task.GetAccount()
	if account == nil {
		task.Fail(fmt.Errorf("账户不存在，无法下载文件%s", task.LocalFullPath))
		return
	}
	// 标记为下载中
	task.Downloading()
	// 拼接url
	// syncFile := GetSyncFileById(task.SyncFileId)
	// if syncFile == nil {
	// 	task.Fail(fmt.Errorf("openlist文件ID不存在，无法下载文件%s", task.LocalFullPath))
	// 	return
	// }
	// remoteFileId := strings.ReplaceAll(task.RemoteFileId, "\\", "/")
	// // 去掉remoteFileId的开头的/
	// remoteFileId = strings.TrimPrefix(remoteFileId, "/")
	// 将remoteFileId中的每一段都做urlencode
	// remoteFileIdParts := strings.Split(remoteFileId, "/")
	// for i, part := range remoteFileIdParts {
	// 	remoteFileIdParts[i] = url.QueryEscape(part)
	// }
	// url := fmt.Sprintf("%s/d/%s", account.BaseUrl, strings.Join(remoteFileIdParts, "/"))
	// url := fmt.Sprintf("%s/d/%s", account.BaseUrl, remoteFileId)
	// if syncFile.OpenlistSign != "" {
	// 	url += "?sign=" + syncFile.OpenlistSign
	// }
	// 下载文件到指定位置
	downloadErr := helpers.DownloadFile(task.RemoteFileId, task.LocalFullPath, v115open.DEFAULTUA)
	if downloadErr != nil {
		helpers.AppLogger.Warnf("[下载] 下载文件失败: %s", downloadErr.Error())
		task.Fail(downloadErr)
		return
	}
	// 设置文件修改时间
	task.SetMTime()
	// 下载完成
	task.Complete()
}

// 访问Emby下载链接
func (task *DbDownloadTask) DownloadEmbyMedia() {
	// 标记为下载中
	task.Downloading()
	// 发送一个POST请求
	// 创建请求并设置User-Agent
	client := &http.Client{
		Timeout: 30 * time.Second, // 30秒超时
	}
	req, err := http.NewRequest(http.MethodPost, task.RemoteFileId, nil)
	if err != nil {
		helpers.AppLogger.Errorf("[下载] 创建 %s 的http request失败: %v", task.FileName, err)
		task.Fail(err)
		return
	}
	req.Header.Set("User-Agent", v115open.DEFAULTUA)
	// 发送请求
	_, doErr := client.Do(req)
	if doErr != nil {
		helpers.AppLogger.Errorf("[Emby媒体信息提取] 失败，名称 %s, Emby ItemID: %s 错误: %v", task.FileName, task.RemoteFileId, doErr)
		task.Fail(doErr)
		return
	}
	if helpers.IsRelease {
		helpers.AppLogger.Infof("[Emby媒体信息提取] 成功, 名称 %s, Emby ItemID: %s", task.FileName, task.RemoteFileId)
	}
	task.Complete()
}

// 检查任务是否已经存在，通过Source + RemoteFileId
func CheckDownloadTaskExist(source DownloadSource, remoteFileId string) *DbDownloadTask {
	var task *DbDownloadTask
	err := db.Db.Model(&DbDownloadTask{}).
		Where("source = ? AND remote_file_id = ?", source, remoteFileId).
		First(&task).Error
	if err != nil {
		return nil
	}
	return task
}

// 添加任务
func AddDownloadTaskFromSyncFile(file *SyncFile) error {
	// 先检查是否存在
	if task := CheckDownloadTaskExist(DownloadSourceStrm, file.PickCode); task != nil {
		if task.Status == DownloadStatusPending {
			return errors.New("任务已存在，状态为待下载")
		}
		if task.Status == DownloadStatusDownloading {
			return errors.New("任务已存在，状态为下载中")
		}
	}
	if file.SyncPath == nil {
		file.SyncPath = GetSyncPathById(file.SyncPathId)
	}
	source := DownloadSourceStrm
	switch file.SourceType {
	case SourceTypeLocal:
		source = DownloadSourceLocalFile
	case SourceTypeOpenList:
		// openlist文件，直接使用远程路径作为下载链接

	}
	// 插入新纪录
	task := &DbDownloadTask{
		AccountId:     file.AccountId,
		SyncFileId:    file.ID,
		RemoteFileId:  file.PickCode,
		FileName:      file.FileName,
		RemotePath:    file.Path,
		LocalFullPath: file.LocalFilePath,
		Source:        DownloadSource(source),
		Status:        DownloadStatusPending,
		Size:          file.FileSize,
		SourceType:    file.SourceType,
		MTime:         file.MTime,
	}
	err := db.Db.Create(task).Error
	return err
}

func AddDownloadTaskFromEmbyMedia(url, itemId, itemName string) error {
	// 先检查是否存在
	if task := CheckDownloadTaskExist(DownloadSourceEmbyMedia, url); task != nil {
		if task.Status == DownloadStatusPending {
			return errors.New("任务已存在，状态为待下载")
		}
		if task.Status == DownloadStatusDownloading {
			return errors.New("任务已存在，状态为下载中")
		}
	}
	// 插入新纪录
	task := &DbDownloadTask{
		AccountId:     0,
		RemoteFileId:  url,
		FileName:      itemName,
		RemotePath:    itemId,
		LocalFullPath: "",
		Source:        DownloadSourceEmbyMedia,
		Status:        DownloadStatusPending,
		Size:          0,
		SourceType:    SourceTypeEmbyMedia,
	}
	err := db.Db.Create(task).Error
	return err
}

func GetPendingDownloadTasks(limit int) []*DbDownloadTask {
	var tasks []*DbDownloadTask
	db.Db.Model(&DbDownloadTask{}).
		Where("status = ?", DownloadStatusPending).
		Limit(limit).
		Order("id ASC").
		Find(&tasks)
	return tasks
}

func GetDownloadingCount() int64 {
	var count int64
	db.Db.Model(&DbDownloadTask{}).
		Where("status = ?", DownloadStatusDownloading).
		Count(&count)
	return count
}

// 查询下载队列任务列表
func GetDownloadTaskList(status DownloadStatus, page, pageSize int) ([]*DbDownloadTask, int64) {
	var tasks []*DbDownloadTask
	var total int64
	tx := db.Db.Model(&DbDownloadTask{})
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

func ClearDownloadPendingTasks() error {
	err := db.Db.Model(&DbDownloadTask{}).
		Where("status = ?", DownloadStatusPending).
		Delete(&DbDownloadTask{}).Error
	if err != nil {
		helpers.AppLogger.Errorf("清除待下载任务失败: %v", err)
		return err
	}
	return err
}

func ClearExpireDownloadTasks() error {
	err := db.Db.Model(&DbDownloadTask{}).
		Where("created_at < ?", time.Now().AddDate(0, 0, -3).Unix()).
		Delete(&DbDownloadTask{}).Error
	if err != nil {
		helpers.AppLogger.Errorf("清除3天前的下载任务失败: %v", err)
		return err
	} else {
		helpers.AppLogger.Infof("清除3天前的下载任务成功")
	}
	return err
}

func ClearDownloadSuccessAndFailed() error {
	err := db.Db.Model(&DbDownloadTask{}).
		Where("status IN ?", []DownloadStatus{DownloadStatusCompleted, DownloadStatusFailed}).
		Delete(&DbDownloadTask{}).Error
	if err != nil {
		helpers.AppLogger.Errorf("清除待下载任务失败: %v", err)
		return err
	}
	return err
}

func UpdateDownloadingToPending() error {
	// 把所有下载中的记录改为待下载
	err := db.Db.Model(&DbDownloadTask{}).
		Where("status = ?", DownloadStatusDownloading).
		Update("status", DownloadStatusPending).Error
	if err != nil {
		helpers.AppLogger.Errorf("更新下载中的任务为待下载失败: %v", err)
		return err
	} else {
		helpers.AppLogger.Infof("更新下载中的任务为待下载成功")
	}
	return err
}
