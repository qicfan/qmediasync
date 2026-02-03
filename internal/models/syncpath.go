package models

import (
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/helpers"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

type SourceType string

const (
	SourceType115       SourceType = "115"
	SourceTypeLocal     SourceType = "local"
	SourceType123       SourceType = "123"
	SourceTypeOpenList  SourceType = "openlist"
	SourceTypeEmbyMedia SourceType = "emby媒体信息提取" // emby媒体信息提取专用
)

type SyncPathSetting struct {
	MinVideoSize   int64    `json:"min_video_size"`                  // 最小视频大小，单位字节,-1表示使用STRM设置
	VideoExt       string   `json:"-"`                               // 视频文件扩展名，JSON格式
	VideoExtArr    []string `json:"video_ext_arr" gorm:"-"`          // 视频文件扩展名数组，不参与数据库操作，仅供前端使用
	MetaExt        string   `json:"-"`                               // 元数据文件扩展名，JSON格式
	MetaExtArr     []string `json:"meta_ext_arr" gorm:"-"`           // 元数据文件扩展名数组，不参与数据库操作，仅供前端使用
	ExcludeName    string   `json:"-"`                               // 排除的文件名，JSON格式
	ExcludeNameArr []string `json:"exclude_name_arr" gorm:"-"`       // 排除的文件名数组，不参与数据库操作，仅供前端使用
	UploadMeta     int      `json:"upload_meta" gorm:"default:-1"`   // 是否上传元数据，-1表示使用STRM设置，0表示保留，1表示上传，2-表示删除
	DownloadMeta   int      `json:"download_meta" gorm:"default:-1"` // 是否下载元数据，-1表示使用STRM设置，0表示不下载，1表示下载
	DeleteDir      int      `json:"delete_dir" gorm:"default:-1"`    // 是否删除目录，-1表示使用STRM设置，0表示不删除，1表示删除
	AddPath        int      `json:"add_path" gorm:"default:-1"`      // 是否添加路径，默认-1(使用settings的值), 1- 表示添加路径， 2-表示不添加路径
}

type SyncPath struct {
	BaseModel
	SyncPathSetting
	CustomConfig bool       `json:"custom_config"`          // 是否自定义配置
	BaseCid      string     `json:"base_cid" gorm:"unique"` // 同步源路径的目录ID,115网盘和123网盘需要该字段
	LocalPath    string     `json:"local_path"`             // 存放strm文件和元数据文件的本地路径
	RemotePath   string     `json:"remote_path"`            // 同步源路径
	SourceType   SourceType `json:"source_type"`            // 同步源类型，主要分为：115网盘，本地目录，123网盘，无法编辑
	AccountId    uint       `json:"account_id"`             // 115账号ID或者123账号ID，根据SourceType决定，无法编辑
	EnableCron   bool       `json:"enable_cron"`            // 是否启用定时同步
	LastSyncAt   int64      `json:"last_sync_at"`           // 上次同步时间
	AccountName  string     `json:"account_name" gorm:"-"`  // 115账号名或者123账号名，不参与数据库操作，仅供前端使用
	IsFullSync   bool       `json:"is_full_sync"`           // 是否全量同步，默认false
	IsRunning    int        `json:"is_running" gorm:"-"`    // 是否正在运行 0-未运行，1-已在队列，2-正在运行
}

func (sp *SyncPath) GetUploadMeta() int {
	if sp.UploadMeta == -1 {
		return SettingsGlobal.UploadMeta
	}
	return sp.UploadMeta
}

func (sp *SyncPath) GetDownloadMeta() int {
	if sp.DownloadMeta == -1 {
		return SettingsGlobal.DownloadMeta
	}
	return sp.DownloadMeta
}

func (sp *SyncPath) GetDeleteDir() int {
	if sp.DeleteDir == -1 {
		return SettingsGlobal.DeleteDir
	}
	return sp.DeleteDir
}

func (sp *SyncPath) GetMinVideoSize() int64 {
	if sp.MinVideoSize == -1 {
		return SettingsGlobal.MinVideoSize
	}
	return sp.MinVideoSize
}

func (sp *SyncPath) GetVideoExt() []string {
	if len(sp.VideoExtArr) == 0 {
		return SettingsGlobal.VideoExtArr
	}
	return sp.VideoExtArr
}

func (sp *SyncPath) GetMetaExt() []string {
	if len(sp.MetaExtArr) == 0 {
		return SettingsGlobal.MetaExtArr
	}
	return sp.MetaExtArr
}

func (sp *SyncPath) GetExcludeNameArr() []string {
	if len(sp.ExcludeNameArr) == 0 {
		return SettingsGlobal.ExcludeNameArr
	}
	return sp.ExcludeNameArr
}

func (sp *SyncPath) GetAddPath() int {
	if sp.AddPath != -1 {
		return sp.AddPath
	}
	return SettingsGlobal.AddPath
}

// 修改同步路径
func (sp *SyncPath) Update(sourceType SourceType, accountId uint, baseCid, localPath, remotePath string, enableCron bool, customConfig bool, syncPathSetting SyncPathSetting) bool {
	if runtime.GOOS != "windows" {
		localPath = strings.TrimRight(localPath, "/")
		remotePath = strings.Trim(remotePath, "/")
	} else {
		localPath = strings.TrimRight(localPath, "\\")
		remotePath = strings.TrimRight(remotePath, "\\")
	}
	if customConfig {
		// 全部转小写
		for i, v := range syncPathSetting.VideoExtArr {
			syncPathSetting.VideoExtArr[i] = strings.ToLower(v)
		}
		for i, v := range syncPathSetting.MetaExtArr {
			syncPathSetting.MetaExtArr[i] = strings.ToLower(v)
		}
		for i, v := range syncPathSetting.ExcludeNameArr {
			syncPathSetting.ExcludeNameArr[i] = strings.ToLower(v)
		}
		videoExtStr, err := json.Marshal(syncPathSetting.VideoExtArr)
		if err != nil {
			helpers.AppLogger.Errorf("视频扩展名转换为JSON失败: %v", err)
			return false
		}
		metaExtStr, err := json.Marshal(syncPathSetting.MetaExtArr)
		if err != nil {
			helpers.AppLogger.Errorf("元数据扩展名转换为JSON失败: %v", err)
			return false
		}
		// 排除的名字
		excludeNameStr, err := json.Marshal(syncPathSetting.ExcludeNameArr)
		if err != nil {
			helpers.AppLogger.Errorf("将排除的名字转换为JSON字符串失败: %v", err)
			return false
		}
		syncPathSetting.VideoExt = string(videoExtStr)
		syncPathSetting.MetaExt = string(metaExtStr)
		syncPathSetting.ExcludeName = string(excludeNameStr)
	} else {
		syncPathSetting.VideoExt = ""
		syncPathSetting.MetaExt = ""
		syncPathSetting.ExcludeName = ""
	}
	sp.CustomConfig = customConfig
	sp.BaseCid = baseCid
	sp.LocalPath = localPath
	sp.RemotePath = remotePath
	sp.EnableCron = enableCron
	sp.SyncPathSetting = syncPathSetting
	// 使用 map 保存需要更新的字段
	updates := map[string]interface{}{
		"custom_config":  customConfig,
		"base_cid":       baseCid,
		"local_path":     localPath,
		"remote_path":    remotePath,
		"enable_cron":    enableCron,
		"min_video_size": syncPathSetting.MinVideoSize,
		"video_ext":      syncPathSetting.VideoExt,
		"meta_ext":       syncPathSetting.MetaExt,
		"exclude_name":   syncPathSetting.ExcludeName,
		"upload_meta":    syncPathSetting.UploadMeta,
		"download_meta":  syncPathSetting.DownloadMeta,
		"delete_dir":     syncPathSetting.DeleteDir,
		"add_path":       syncPathSetting.AddPath,
	}
	result := db.Db.Model(sp).Updates(updates)
	// 创建同步路径
	fullPath := filepath.Join(localPath, remotePath)
	os.MkdirAll(fullPath, 0777)
	// 更新同步路径
	// helpers.AppLogger.Debugf("更新同步路径: %s", fullPath)
	return result.Error == nil
}

func (sp *SyncPath) SetIsFullSync(isFullSync bool) {
	sp.IsFullSync = isFullSync
	db.Db.Save(sp)
}

// 给同步路径创建一个同步任务
func (sp *SyncPath) CreateSyncTask() *Sync {
	// 新建同步任务
	sync := &Sync{
		SyncPathId: sp.ID,
		Status:     SyncStatusPending,
		SubStatus:  SyncSubStatusNone,
		FileOffset: 0,
		Total:      0,
		NewStrm:    0,
		NewMeta:    0,
		Logger:     nil,
		LocalPath:  sp.LocalPath,
		RemotePath: sp.RemotePath,
		BaseCid:    sp.BaseCid,
		SyncPath:   sp,
		FailReason: "",
		IsFullSync: sp.IsFullSync,
	}
	// 写入数据库
	if err := db.Db.Create(sync).Error; err != nil {
		helpers.AppLogger.Errorf("创建同步任务失败: %v", err)
		return nil
	}
	// helpers.AppLogger.Debugf("创建同步任务: %d", sync.ID)
	sync.SyncPath = sp // 赋值syncpath实例给sync
	return sync
}

// 获取完整的本地路径
func (sp *SyncPath) GetFullLocalPath() string {
	if sp.SourceType == SourceTypeLocal {
		return sp.LocalPath
	}
	return filepath.Join(sp.LocalPath, sp.RemotePath)
}

func (sp *SyncPath) ParseVideoAndMetaExt() {
	var err error
	sp.VideoExtArr = make([]string, 0)
	sp.MetaExtArr = make([]string, 0)
	sp.ExcludeNameArr = make([]string, 0)
	if sp.VideoExt != "" {
		// 解析视频扩展名
		err = json.Unmarshal([]byte(sp.VideoExt), &sp.VideoExtArr)
		if err != nil {
			helpers.AppLogger.Errorf("视频扩展名解析失败: %v", err)
		}
	}
	// 解析元数据扩展名
	if sp.MetaExt != "" {
		err = json.Unmarshal([]byte(sp.MetaExt), &sp.MetaExtArr)
		if err != nil {
			helpers.AppLogger.Errorf("元数据扩展名解析失败: %v", err)
		}
	}
	if sp.ExcludeName != "" {
		// 解析排除的名字
		err = json.Unmarshal([]byte(sp.ExcludeName), &sp.ExcludeNameArr)
		if err != nil {
			helpers.AppLogger.Errorf("排除的名字解析失败: %v", err)
		}
	}
}

func (sp *SyncPath) UpdateLastSync() {
	sp.LastSyncAt = time.Now().Unix()
	db.Db.Save(sp)
}

func (sp *SyncPath) ToggleCron() {
	sp.EnableCron = !sp.EnableCron
	db.Db.Save(sp)
}

func (sp *SyncPath) IsValidVideoExt(name string) bool {
	ext := filepath.Ext(name)
	ext = strings.ToLower(ext)
	if slices.Contains(sp.GetVideoExt(), ext) {
		return true
	}
	// return ext == ".strm"
	return false
}

func (sp *SyncPath) IsValidMetaExt(name string) bool {
	ext := filepath.Ext(name)
	ext = strings.ToLower(ext)
	return slices.Contains(sp.GetMetaExt(), ext)
}

func (sp *SyncPath) MakeFullLocalPath(pid, name string) string {
	if sp.IsValidVideoExt(name) {
		// 视频文件要转成.strm文件
		ext := filepath.Ext(name)
		baseName := strings.TrimSuffix(name, ext)
		// if ext == ".iso" {
		// 	name = name + ".strm"
		// } else {
		name = baseName + ".strm"
		// }
	}
	switch sp.SourceType {
	case SourceType115:
		return filepath.Join(sp.LocalPath, sp.RemotePath, pid, name)
	case SourceTypeOpenList:
		return filepath.Join(sp.LocalPath, pid, name)
	case SourceTypeLocal:
		return filepath.Join(sp.LocalPath, pid, name)
	}
	return ""
}

// 创建同步路径
func CreateSyncPath(sourceType SourceType, accountId uint, baseCid, localPath, remotePath string, enableCron bool, customConfig bool, syncPathSetting SyncPathSetting) *SyncPath {
	if runtime.GOOS != "windows" {
		localPath = strings.TrimRight(localPath, "/")
		remotePath = strings.TrimRight(remotePath, "/")
	} else {
		localPath = strings.TrimRight(localPath, "\\")
		remotePath = strings.TrimRight(remotePath, "\\")
	}

	var err error
	if customConfig {
		var videoExtBytes, metaExtBytes, excludeNameBytes []byte
		// 全部转小写
		for i, v := range syncPathSetting.VideoExtArr {
			syncPathSetting.VideoExtArr[i] = strings.ToLower(v)
		}
		for i, v := range syncPathSetting.MetaExtArr {
			syncPathSetting.MetaExtArr[i] = strings.ToLower(v)
		}
		for i, v := range syncPathSetting.ExcludeNameArr {
			syncPathSetting.ExcludeNameArr[i] = strings.ToLower(v)
		}
		videoExtBytes, err = json.Marshal(syncPathSetting.VideoExtArr)
		if err != nil {
			helpers.AppLogger.Errorf("视频扩展名转换为JSON失败: %v", err)
			return nil
		}
		syncPathSetting.VideoExt = string(videoExtBytes)
		metaExtBytes, err = json.Marshal(syncPathSetting.MetaExtArr)
		if err != nil {
			helpers.AppLogger.Errorf("元数据扩展名转换为JSON失败: %v", err)
			return nil
		}
		syncPathSetting.MetaExt = string(metaExtBytes)
		// 排除的名字
		excludeNameBytes, err = json.Marshal(syncPathSetting.ExcludeNameArr)
		if err != nil {
			helpers.AppLogger.Errorf("将排除的名字转换为JSON字符串失败: %v", err)
			return nil
		}
		syncPathSetting.ExcludeName = string(excludeNameBytes)
	} else {
		syncPathSetting.VideoExt = ""
		syncPathSetting.MetaExt = ""
		syncPathSetting.ExcludeName = ""
	}
	// 使用map[string]interface{}格式入库，避免0值不入库
	syncPathData := map[string]interface{}{
		"source_type":    sourceType,
		"base_cid":       baseCid,
		"local_path":     localPath,
		"remote_path":    remotePath,
		"account_id":     accountId,
		"enable_cron":    enableCron,
		"custom_config":  customConfig,
		"video_ext":      syncPathSetting.VideoExt,
		"meta_ext":       syncPathSetting.MetaExt,
		"exclude_name":   syncPathSetting.ExcludeName,
		"download_meta":  syncPathSetting.DownloadMeta,
		"upload_meta":    syncPathSetting.UploadMeta,
		"delete_dir":     syncPathSetting.DeleteDir,
		"min_video_size": syncPathSetting.MinVideoSize,
		"add_path":       syncPathSetting.AddPath,
		"created_at":     time.Now().Unix(),
		"updated_at":     time.Now().Unix(),
	}

	// helpers.AppLogger.Infof("创建同步路径数据: %+v", syncPathData)

	// 使用Create方法插入数据
	result := db.Db.Model(&SyncPath{}).Create(syncPathData)
	if result.Error != nil {
		helpers.AppLogger.Errorf("创建同步路径失败: %v", result.Error)
		return nil
	}

	// 获取创建的同步路径对象
	syncPath := &SyncPath{}
	if err := db.Db.Where("source_type = ? AND base_cid = ? AND local_path = ? AND remote_path = ?",
		sourceType, baseCid, localPath, remotePath).Order("id DESC").First(syncPath).Error; err != nil {
		helpers.AppLogger.Errorf("获取创建的同步路径失败: %v", err)
		return nil
	}
	return syncPath
}

// 使用ID删除同步路径
func DeleteSyncPathById(id uint) bool {
	syncPath := GetSyncPathById(id)
	if syncPath == nil {
		return false
	}
	tx := db.Db.Begin()
	result := tx.Delete(&SyncPath{}, id)
	if result.Error != nil || result.RowsAffected <= 0 {
		helpers.AppLogger.Errorf("删除同步路径失败: %v", result.Error)
		tx.Rollback()
		return false
	}
	if syncPath.SourceType == SourceType115 {
		// 清空数据表
		// Delete by ID
		result = tx.Delete(SyncFile{}, "sync_path_id = ?", syncPath.ID)
		// result = tx.Exec("DELETE FROM sync_files WHERE sync_path_id = ?", syncPath.ID)
		if result.Error != nil {
			helpers.AppLogger.Errorf("删除同步路径数据失败: %v", result.Error)
			tx.Rollback()
			return false
		}
	}
	tx.Delete(EmbyLibrarySyncPath{}, "sync_path_id = ?", syncPath.ID)
	tx.Delete(EmbyMediaSyncFile{}, "sync_path_id = ?", syncPath.ID)
	// // 删除SyncPath和Emby Library的关联
	// tx.Exec("DELETE FROM emby_library_sync_paths WHERE sync_path_id = ?", syncPath.ID)
	// // 删除SyncFiles和Emby Media的关联
	// tx.Exec("DELETE FROM emby_media_sync_files WHERE sync_path_id = ?", syncPath.ID)
	tx.Commit()
	// 其他类型删除localpath/remotePath
	fullPath := filepath.Join(syncPath.LocalPath, syncPath.RemotePath)
	if syncPath.SourceType == SourceTypeLocal {
		// 本地目录类型直接删除localpath
		fullPath = syncPath.LocalPath
	}

	helpers.AppLogger.Infof("暂时不删除目标路径，先观察是否稳定: %s", fullPath)
	// err := os.RemoveAll(fullPath)
	// if err != nil {
	// 	helpers.AppLogger.Errorf("删除本地目录失败: %v", err)
	// 	return false
	// }
	// helpers.AppLogger.Debugf("删除本地目录成功: %s", fullPath)
	return true
}

// 根据ID获取同步路径
func GetSyncPathById(id uint) *SyncPath {
	var syncPath SyncPath
	db.Db.First(&syncPath, id)
	if syncPath.ID == 0 {
		helpers.AppLogger.Errorf("同步路径不存在: %v", id)
		return nil
	}
	syncPath.ParseVideoAndMetaExt()
	return &syncPath
}

// 查询同步路径列表
func GetSyncPathList(page, pageSize int, enableCron bool) ([]*SyncPath, int64) {
	var syncPaths []*SyncPath
	var total int64

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}

	offset := (page - 1) * pageSize
	if enableCron {
		db.Db.Model(&SyncPath{}).Where("enable_cron = ?", enableCron).Count(&total)
		db.Db.Model(&SyncPath{}).Where("enable_cron = ?", enableCron).Offset(offset).Limit(pageSize).Order("id DESC").Find(&syncPaths)
	} else {
		db.Db.Model(&SyncPath{}).Count(&total)
		db.Db.Offset(offset).Limit(pageSize).Order("id DESC").Find(&syncPaths)
	}
	accountCache := make(map[uint]*Account)
	for _, syncPath := range syncPaths {
		syncPath.ParseVideoAndMetaExt()
		if syncPath.AccountId == 0 {
			continue
		}
		if account, ok := accountCache[syncPath.AccountId]; ok {
			syncPath.AccountName = account.Name
			// helpers.AppLogger.Infof("从缓存获取账号成功: %s", account.Name)
			continue
		}
		account, err := GetAccountById(syncPath.AccountId)
		if err != nil {
			helpers.AppLogger.Errorf("获取账号失败: %v", err)
			continue
		}
		accountCache[syncPath.AccountId] = account
		syncPath.AccountName = account.Name
		if account.Name == "" {
			syncPath.AccountName = account.Username
		}
		syncPath.ParseVideoAndMetaExt()
		// helpers.AppLogger.Infof("获取账号成功: %s", account.Name)
	}
	// // 清空accountCache
	// accountCache = nil
	return syncPaths, total
}

// 根据账号ID获取同步路径列表
func GetAllSyncPathByAccountId(accountId uint) []SyncPath {
	var syncPaths []SyncPath
	db.Db.Where("account_id = ?", accountId).Find(&syncPaths)
	return syncPaths
}
