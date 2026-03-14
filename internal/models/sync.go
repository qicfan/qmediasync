package models

import (
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/notificationmanager"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gorm.io/gorm"
)

type SyncStatus int

const (
	SyncStatusPending    SyncStatus = iota // 待处理
	SyncStatusInProgress                   // 进行中
	SyncStatusCompleted                    // 已完成
	SyncStatusFailed                       // 失败
)

var SyncStatusText map[SyncStatus]string = map[SyncStatus]string{
	SyncStatusPending:    "待处理",
	SyncStatusInProgress: "进行中",
	SyncStatusCompleted:  "已完成",
	SyncStatusFailed:     "失败",
}

type SyncResultStatus int // 同步结果的状态，标记文件要做的操作

const (
	SyncResultStatusNew    SyncResultStatus = iota // 新增
	SyncResultStatusUpdate                         // 更新
	SyncResultStatusDelete                         // 删除
	SyncResultStatusSame                           // 相同
)

type SyncSubStatus int

const (
	SyncSubStatusNone                 SyncSubStatus = iota // 无子状态
	SyncSubStatusProcessNetFileList                        // 正在处理网盘文件
	SyncSubStatusProcessLocalFileList                      // 正在处理本地文件列表
)

var SyncSubStatusText map[SyncSubStatus]string = map[SyncSubStatus]string{
	SyncSubStatusNone:                 "无子状态",
	SyncSubStatusProcessNetFileList:   "正在处理网盘文件",
	SyncSubStatusProcessLocalFileList: "正在处理本地文件列表",
}

// 同步任务
type Sync struct {
	BaseModel
	SyncPathId        uint             `json:"sync_path_id"`
	Status            SyncStatus       `json:"status"`
	SubStatus         SyncSubStatus    `json:"sub_status"`  // 子状态，记录当前同步的子任务状态
	FileOffset        int              `json:"file_offset"` // 文件偏移量，用于继续任务时的定位
	Total             int              `json:"total"`
	FinishAt          int64            `json:"finish_at"`
	NewStrm           int              `json:"new_strm"`
	NewMeta           int              `json:"new_meta"`
	NewUpload         int              `json:"new_upload" gorm:"default:0"` // 新增上传的文件数量
	NetFileStartAt    int64            `json:"net_file_start_at"`           // 开始处理网盘文件时间
	NetFileFinishAt   int64            `json:"net_file_finish_at"`          // 处理网盘文件完成时间
	LocalFileStartAt  int64            `json:"local_file_start_at"`         // 开始处理本地文件列表时间
	LocalFileFinishAt int64            `json:"local_file_finish_at"`        // 处理本地文件列表完成时间
	LocalPath         string           `json:"local_path"`                  // 本地同步路径
	RemotePath        string           `json:"remote_path"`                 // 远程同步路径
	BaseCid           string           `json:"base_cid"`                    // 基础CID，用于标识同步的根目录
	FailReason        string           `json:"fail_reason"`                 // 失败原因
	IsFullSync        bool             `json:"is_full_sync"`                // 是否全量同步
	SyncPath          *SyncPath        `gorm:"-" json:"-"`                  // 同步路径实例
	Logger            *helpers.QLogger `gorm:"-" json:"-"`                  // 日志句柄，不参与数据读写
}

// 完成本地同步任务
func (s *Sync) Complete(sourceType SourceType) bool {
	s.Status = SyncStatusCompleted
	s.FinishAt = time.Now().Unix()
	s.LocalFileFinishAt = s.FinishAt
	// 回写数据库
	if err := db.Db.Save(s).Error; err != nil {
		s.Logger.Errorf("完成同步失败: %v", err)
		return false
	}
	// s.SyncPath.SetIsFullSync(false) // 改回默认值，下次非全量同步
	s.Logger.Infof("同步任务已完成: %d", s.ID)
	if s.NewUpload > 0 || s.NewMeta > 0 || s.NewStrm > 0 {
		ctx := context.Background()

		notif := &Notification{
			Type:      SyncFinished,
			Title:     fmt.Sprintf("✅ %s %s 同步完成", sourceType.String(), s.RemotePath),
			Content:   fmt.Sprintf("📊 耗时: %s, 生成STRM: %s, 下载: %s, 上传: %s\n⏰ 时间: %s", s.GetDuration(), helpers.IntToString(s.NewStrm), helpers.IntToString(s.NewMeta), helpers.IntToString(s.NewUpload), time.Now().Format("2006-01-02 15:04:05")),
			Timestamp: time.Now(),
			Priority:  NormalPriority,
		}
		if notificationmanager.GlobalEnhancedNotificationManager != nil {
			if err := notificationmanager.GlobalEnhancedNotificationManager.SendNotification(ctx, notif); err != nil {
				s.Logger.Errorf("发送同步完成通知失败: %v", err)
			}
		}
	}
	// 关闭日志
	s.Logger.Close()
	return true
}

func (s *Sync) Failed(reason string) {
	s.FailReason = reason
	s.FinishAt = time.Now().Unix()
	s.LocalFileFinishAt = s.FinishAt
	s.UpdateStatus(SyncStatusFailed)
	ctx := context.Background()
	notif := &Notification{
		Type:      SyncError,
		Title:     "❌ 同步错误",
		Content:   fmt.Sprintf("🔍 错误: %s\n⏰ 时间: %s", reason, time.Now().Format("2006-01-02 15:04:05")),
		Timestamp: time.Now(),
		Priority:  HighPriority,
	}
	if notificationmanager.GlobalEnhancedNotificationManager != nil {
		if err := notificationmanager.GlobalEnhancedNotificationManager.SendNotification(ctx, notif); err != nil {
			s.Logger.Errorf("发送同步错误通知失败: %v", err)
		}
	}
}

func (s *Sync) GetDuration() string {
	return helpers.FormatDuration(s.FinishAt - s.CreatedAt)
}

func (s *Sync) UpdateTotal() {
	// 回写数据库
	ctx := context.Background()
	_, err := gorm.G[Sync](db.Db).Where("id = ?", s.ID).Updates(ctx, Sync{
		Total: s.Total,
	})
	if err != nil {
		s.Logger.Errorf("更新文件总数失败: %v", err)
		return
	}
	// s.Logger.Infof("更新文件总数: %d", s.Total)
}

// 修改同步任务的状态
func (s *Sync) UpdateStatus(status SyncStatus) bool {
	oldStatus := s.Status
	s.Status = status
	// 回写数据库
	ctx := context.Background()
	_, err := gorm.G[Sync](db.Db).Where("id = ?", s.ID).Updates(ctx, Sync{
		Status:            status,
		FailReason:        s.FailReason,
		FinishAt:          s.FinishAt,
		LocalFileFinishAt: s.LocalFileFinishAt,
	})
	if err != nil {
		s.Logger.Errorf("更新同步状态失败: %v", err)
		return false
	}
	s.Logger.Infof("更新任务状态: %s => %s", SyncStatusText[oldStatus], SyncStatusText[status])
	return true
}

func (s *Sync) UpdateSubStatus(subStatus SyncSubStatus) bool {
	oldSubStatus := s.SubStatus
	s.SubStatus = subStatus
	var updateSync Sync
	switch subStatus {
	case SyncSubStatusProcessNetFileList:
		// 开始查找文件列表，修改NetFileFinishAt
		s.NetFileStartAt = time.Now().Unix()
		updateSync = Sync{
			SubStatus:      subStatus,
			NetFileStartAt: s.NetFileStartAt,
		}
	case SyncSubStatusProcessLocalFileList:
		// 开始对比文件，修改FetchFileFinishAt
		s.NetFileFinishAt = time.Now().Unix()
		s.LocalFileStartAt = s.NetFileFinishAt
		updateSync = Sync{
			SubStatus:        subStatus,
			NetFileFinishAt:  s.NetFileFinishAt,
			LocalFileStartAt: s.LocalFileStartAt,
		}
	}
	err := db.Db.Model(&Sync{}).Where("id = ?", s.ID).Updates(&updateSync).Error
	if err != nil {
		s.Logger.Errorf("更新同步子状态失败: %v", err)
		return false
	}
	s.Logger.Infof("更新任务子状态: %s => %s", SyncSubStatusText[oldSubStatus], SyncSubStatusText[subStatus])
	return true
}

func (s *Sync) InitLogger() {
	logDir := filepath.Join(helpers.ConfigDir, "logs", "libs")
	os.MkdirAll(logDir, 0755)
	logFileName := filepath.Join("logs", "libs", fmt.Sprintf("sync_%d.log", s.ID))
	s.Logger = helpers.NewLogger(logFileName, true, false)
	s.Logger.Infof("创建同步日志文件: %s", logFileName)
}

// 获取所有同步记录
func GetSyncRecords(page, pageSize int) ([]*Sync, int64, error) {
	var count int64
	if err := db.Db.Model(&Sync{}).Count(&count).Error; err != nil {
		helpers.AppLogger.Errorf("统计同步记录总数失败: %v", err)
		return nil, 0, err
	}
	var syncs []*Sync
	if err := db.Db.Offset((page - 1) * pageSize).Limit(pageSize).Order("id DESC").Find(&syncs).Error; err != nil {
		helpers.AppLogger.Errorf("获取同步记录失败: %v", err)
		return nil, 0, err
	}
	return syncs, count, nil
}

func GetSyncByID(id uint) (*Sync, error) {
	sync := &Sync{}
	if err := db.Db.First(sync, id).Error; err != nil {
		return nil, err
	}
	// 根据sync.SyncPathId查询SyncPath
	var syncPath SyncPath
	if err := db.Db.First(&syncPath, sync.SyncPathId).Error; err != nil {
		return nil, err
	}
	sync.SyncPath = &syncPath
	return sync, nil
}

// 获取最后一个同步任务
func GetLastSyncTask() *Sync {
	var sync Sync
	if err := db.Db.Order("id desc").First(&sync).Error; err != nil {
		helpers.AppLogger.Errorf("获取最后一个同步任务失败: %v", err)
		return nil
	}
	return &sync
}

func FailAllRunningSyncTasks() {

	// 查找所有运行中的同步任务
	var runningSyncs []Sync
	if err := db.Db.Where("status IN (?, ?)", SyncStatusPending, SyncStatusInProgress).Find(&runningSyncs).Error; err != nil {
		helpers.AppLogger.Errorf("查询运行中的同步任务失败: %v", err)
		return
	}

	if len(runningSyncs) == 0 {
		return
	}

	helpers.AppLogger.Infof("发现 %d 个运行中的同步任务，将设置为失败状态", len(runningSyncs))

	// 批量更新状态为失败
	if err := db.Db.Model(&Sync{}).Where("status IN (?, ?)", SyncStatusPending, SyncStatusInProgress).Updates(map[string]interface{}{
		"status": SyncStatusFailed,
	}).Error; err != nil {
		helpers.AppLogger.Errorf("批量更新运行中的同步任务状态失败: %v", err)
		return
	}
	syncPathId := make([]uint, 0)
	for _, sync := range runningSyncs {
		syncPathId = append(syncPathId, sync.SyncPathId)
	}
	// 批量更新同步路径的IsFullSync为false
	if err := db.Db.Model(&SyncPath{}).Where("id IN ?", syncPathId).Updates(map[string]interface{}{
		"is_full_sync": false,
	}).Error; err != nil {
		helpers.AppLogger.Errorf("批量更新同步路径状态失败: %v", err)
		return
	}
	helpers.AppLogger.Infof("成功将 %d 个运行中的同步任务设置为失败状态", len(runningSyncs))
}

// 使用ID删除同步记录和相关文件
func DeleteSyncRecordById(id uint) error {
	sync := &Sync{BaseModel: BaseModel{ID: id}}
	if err := db.Db.Delete(sync).Error; err != nil {
		helpers.AppLogger.Errorf("删除同步记录失败: %v", err)
		return err
	}
	// 删除同步结果文件
	logFile := filepath.Join(helpers.ConfigDir, "logs", "libs", fmt.Sprintf("sync_%d.log", id))
	// 删除相关的日志和同步结果文件
	os.Remove(logFile)
	helpers.AppLogger.Infof("删除同步记录成功: %d", id)
	return nil

}

// 清除过期的同步记录和相关文件，默认保留最近7天的记录
func ClearExpiredSyncRecords(days int) {
	cutoff := time.Now().AddDate(0, 0, -days).Unix()
	var expiredSyncs []Sync
	if err := db.Db.Where("created_at < ?", cutoff).Find(&expiredSyncs).Error; err != nil {
		helpers.AppLogger.Errorf("查询过期的同步记录失败: %v", err)
		return
	}
	if len(expiredSyncs) == 0 {
		helpers.AppLogger.Infof("没有找到过期的同步记录")
		return
	}
	for _, sync := range expiredSyncs {
		if err := DeleteSyncRecordById(sync.ID); err != nil {
			helpers.AppLogger.Errorf("删除过期的同步记录失败: %v", err)
		} else {
			helpers.AppLogger.Infof("删除过期的同步记录成功: %d", sync.ID)
		}
	}
}

func CreateSync(syncPathId uint, sourcePath, sourcePathId, targetPath string) *Sync {
	// 新建同步任务
	sync := &Sync{
		SyncPathId: syncPathId,
		Status:     SyncStatusPending,
		SubStatus:  SyncSubStatusNone,
		FileOffset: 0,
		Total:      0,
		NewStrm:    0,
		NewMeta:    0,
		Logger:     nil,
		LocalPath:  targetPath,
		RemotePath: sourcePath,
		BaseCid:    sourcePathId,
		FailReason: "",
		IsFullSync: false,
	}
	// 写入数据库
	if err := db.Db.Save(sync).Error; err != nil {
		helpers.AppLogger.Errorf("创建同步任务失败: %v", err)
		return nil
	}
	return sync
}

func GetTodayFirstSyncByPathId(syncPathId uint) *Sync {
	var sync Sync
	// 计算今天0点的时间戳
	today := time.Now().Truncate(24 * time.Hour).Unix()
	if err := db.Db.Where("sync_path_id = ? AND created_at >= ?", syncPathId, today).First(&sync).Error; err != nil {
		return nil
	}
	return &sync
}
