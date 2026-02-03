package syncstrm

import (
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/v115open"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"time"
)

type driverImpl interface {
	GetNetFileFiles(ctx context.Context, parentPath, parentPathId string) ([]*SyncFileCache, error)
	GetPathIdByPath(ctx context.Context, path string) (string, error)
	SetSyncStrm(s *SyncStrm)
	MakeStrmContent(sf *SyncFileCache) string
	CreateDirRecursively(ctx context.Context, parentDir string) (parentPathId, remotePath string, err error)
	GetTotalFileCount(ctx context.Context) (int64, string, error)
	GetDirsByPathId(ctx context.Context, pathId string) ([]pathQueueItem, error)
	GetFilesByPathId(ctx context.Context, rootPathId string, offset, limit int) ([]v115open.File, error)
	// 所有文件详情，含路径
	DetailByFileId(ctx context.Context, fileId string) (*v115open.FileDetail, error)
}

type SyncStrm struct {
	SyncDriver   driverImpl
	Account      *models.Account // 网盘账号，如果是本地类型则为nil
	Sync         *models.Sync    // 同步记录，Start方法会生成
	SourcePath   string          // 来源路径
	SourcePathId string
	TmpSyncPath  bool
	TargetPath   string
	Config       SyncStrmConfig
	Context      context.Context
	Cancel       context.CancelFunc
	FullSync     bool // 是否是全量同步

	// 路径队列
	PathWorkerMax int64
	PathErrChan   chan error

	// 临时表
	TempTableName string
	SyncPathId    uint

	// 计数
	NewMeta   int64
	NewStrm   int64
	NewUpload int64
	TotalFile int64

	// 停止状态：避免多次触发停止
	stopped atomic.Bool

	// 115 同步器
	sync115 *Sync115

	memSyncCache *MemorySyncCache // 同步缓存
}

type pathQueueItem struct {
	Path   string // 路径
	PathId string // 路径ID, Openlist和本地Path和PathId是相同的
	Depth  uint   // 路径深度
	Mtime  int64  // 最后修改时间
}

func NewSyncStrm(account *models.Account, syncPathId uint, sourcePath, sourcePathId, targetPath string, config SyncStrmConfig, IsFullSync bool) *SyncStrm {
	var syncDriver driverImpl
	switch account.SourceType {
	case models.SourceType115:
		syncDriver = NewOpen115Driver(account.Get115Client())
	case models.SourceTypeOpenList:
		syncDriver = NewOpenListDriver(account.GetOpenListClient())
	case models.SourceTypeLocal:
		syncDriver = NewLocalDriver()
	}
	pathWorkerMax := int64(models.SettingsGlobal.FileDetailThreads)
	if account.SourceType == models.SourceTypeLocal {
		pathWorkerMax = 10 // 本地类型（CD2会自己限制并发），限制为10个并发
	} else {
		if pathWorkerMax == 1 {
			pathWorkerMax = 2 // 最少为2，否则errgroup递归会卡住
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &SyncStrm{
		Context:       ctx,
		Cancel:        cancel,
		SyncDriver:    syncDriver,
		Account:       account,
		SourcePath:    sourcePath,
		SourcePathId:  sourcePathId,
		TargetPath:    targetPath,
		TmpSyncPath:   false,
		PathWorkerMax: pathWorkerMax,
		Config:        config,
		SyncPathId:    syncPathId,
		FullSync:      IsFullSync,
		PathErrChan:   make(chan error, 1),
	}
	s.memSyncCache = NewMemorySyncCache(syncPathId)
	if s.Account == nil {
		s.Account = &models.Account{SourceType: models.SourceTypeLocal}
	}
	// 如果SyncPathId = 0, 则生成一个唯一的临时ID
	if s.SyncPathId == 0 {
		s.SyncPathId = uint(time.Now().UnixNano())
		s.TmpSyncPath = true
	}
	// 新增一条Sync
	s.Sync = models.CreateSync(s.SyncPathId, s.SourcePath, s.SourcePathId, s.TargetPath)
	if s.Sync == nil {
		return nil
	}
	s.Sync.InitLogger()
	s.SyncDriver.SetSyncStrm(s)
	return s
}

func NewSyncStrmFromSyncPath(syncPath *models.SyncPath) *SyncStrm {
	var account *models.Account
	var err error
	if syncPath.AccountId != 0 {
		account, err = models.GetAccountById(syncPath.AccountId)
		if err != nil {
			return nil
		}
	} else {
		account = &models.Account{SourceType: models.SourceTypeLocal}
	}
	config := SyncStrmConfig{
		EnableDownloadMeta:    int64(syncPath.GetDownloadMeta()),
		MinVideoSize:          syncPath.GetMinVideoSize(),
		VideoExt:              syncPath.GetVideoExt(),
		MetaExt:               syncPath.GetMetaExt(),
		ExcludeNames:          syncPath.GetExcludeNameArr(),
		NetNotFoundFileAction: models.SyncTreeItemMetaAction(syncPath.GetUploadMeta()),
		StrmUrlNeedPath:       syncPath.GetAddPath(),
		DelEmptyLocalDir:      syncPath.GetDeleteDir() == 1,
	}
	return NewSyncStrm(account, syncPath.ID, syncPath.RemotePath, syncPath.BaseCid, syncPath.LocalPath, config, syncPath.IsFullSync)
}

// 直接同步某个路径（可以是目录，也可以是文件）
func NewSyncStrmByPath(account *models.Account, sourcePath, sourcePathId string) *SyncStrm {
	config := SyncStrmConfig{
		EnableDownloadMeta:    int64(models.SettingsGlobal.DownloadMeta),
		MinVideoSize:          int64(models.SettingsGlobal.MinVideoSize),
		VideoExt:              models.SettingsGlobal.VideoExtArr,
		MetaExt:               models.SettingsGlobal.MetaExtArr,
		ExcludeNames:          models.SettingsGlobal.ExcludeNameArr,
		NetNotFoundFileAction: models.SyncTreeItemMetaAction(models.SettingsGlobal.UploadMeta),
		StrmUrlNeedPath:       models.SettingsGlobal.AddPath,
		DelEmptyLocalDir:      models.SettingsGlobal.DeleteDir == 1,
	}
	return NewSyncStrm(account, 0, sourcePath, sourcePathId, "", config, false)
}

func (s *SyncStrm) Stop() {
	if !s.stopped.CompareAndSwap(false, true) {
		s.Sync.Logger.Warnf("同步任务已停止或正在停止")
		return
	}

	s.Sync.Logger.Infof("正在停止同步任务...")
	s.Cancel()
}

func (s *SyncStrm) Start() error {
	// 开始任务时先暂停下载和上传队列
	// 关闭上传下载队列
	models.GlobalDownloadQueue.Stop()
	models.GlobalUploadQueue.Stop()
	defer func() {
		// 任务完成后启动上传下载队列
		models.GlobalDownloadQueue.Start()
		models.GlobalUploadQueue.Start()
	}()
	atomic.StoreInt64(&s.NewMeta, 0)
	atomic.StoreInt64(&s.NewStrm, 0)
	atomic.StoreInt64(&s.NewUpload, 0)
	atomic.StoreInt64(&s.TotalFile, 0)
	s.Sync.UpdateStatus(models.SyncStatusInProgress)
	newPathId, err := s.SyncDriver.GetPathIdByPath(s.Context, s.SourcePath)
	if err != nil {
		reason := err.Error()
		s.Sync.Failed(reason)
		return errors.New(reason)
	}
	if newPathId != s.SourcePathId {
		s.SourcePathId = newPathId
	}
	if !s.checkPathExists(s.TargetPath) {
		reason := fmt.Sprintf("目标路径 %s 不存在", s.TargetPath)
		s.Sync.Failed(reason)
		return errors.New(reason)
	}
	// 创建本地根目录
	localBaseDir := s.GetLocalBaseDir()
	if !s.checkPathExists(localBaseDir) {
		if err := os.MkdirAll(localBaseDir, 0777); err != nil {
			reason := fmt.Sprintf("创建本地根目录失败: %s %v", localBaseDir, err)
			s.Sync.Failed(reason)
			return errors.New(reason)
		}
	}
	if s.Account.SourceType != models.SourceType115 {
		// 其他来源走一套逻辑
		s.StartOther()
	} else {
		// 115 单独处理
		s.Start115Sync()
	}
	s.Sync.Logger.Info("完成所有路径和文件的处理，检查是否有错误发生")
	select {
	case <-s.Context.Done():
		s.Sync.Failed(fmt.Sprintf("同步任务被取消: %v", s.Context.Err()))
		return nil
	case err := <-s.PathErrChan:
		s.Sync.Failed(fmt.Sprintf("路径队列处理失败: %v", err))
		return err
	default:
	}
	// 开始添加需要下载的文件到下载队列
	s.Sync.Logger.Info("开始将要下载的任务添加到下载队列")
	s.AddDownloadTaskFromMemCache()
	s.Sync.Logger.Infof("开始对比本地文件和临时表中的文件，删除多余的本地文件")
	if err := s.compareLocalFilesWithTempTable(); err != nil {
		return err
	}
	s.Sync.NewMeta = int(s.NewMeta)
	s.Sync.NewStrm = int(s.NewStrm)
	s.Sync.NewUpload = int(s.NewUpload)
	s.Sync.Total = int(s.TotalFile)
	s.Sync.Complete()

	if !s.TmpSyncPath {
		// 有syncPathId,将IsFullSync改为false
		if s.FullSync {
			db.Db.Model(&models.SyncPath{}).Where("id = ?", s.SyncPathId).Update("is_full_sync", false)
		}
		db.Db.Model(&models.SyncPath{}).Where("id = ?", s.SyncPathId).Update("last_sync_at", s.Sync.FinishAt)
		// 触发刷新Emby媒体库，延迟30s，等待文件下载完成
		go func() {
			time.Sleep(30 * time.Second)
			if s.NewMeta > 0 || s.NewStrm > 0 {
				s.Sync.Logger.Info("有新的元数据文件或STRM文件，触发刷新Emby媒体库，是否可以刷新受到 Emby设置 - STRM同步完成后刷新媒体库 选项是否开启的影响")
				models.RefreshEmbyLibraryBySyncPathId(s.SyncPathId)
			}
		}()
		// 处理差异
		go func() {
			s.Sync.Logger.Info("115路径和文件同步完成，开始处理SyncFile表和临时表的数据差异")
			s.handleTempTableDiff()
			s.Sync.Logger.Info("完成差异比对，并更新了SyncFile表，任务彻底完成")
		}()
	}
	return nil
}

// 处理网盘文件，生成strm或者添加下载任务
func (s *SyncStrm) processNetFile(file *SyncFileCache) error {
	// 1. 检查对应的本地文件是否存在
	// 先处理视频文件
	defer func() {
		file.Processed = true // 文件已处理
	}()
	// s.Sync.Logger.Infof("正在处理网盘文件 %s => %s", file.FileId, file.FileName)
	localFilePath := file.GetLocalFilePath(s.TargetPath, s.SourcePath)
	// s.Sync.Logger.Infof("本地文件路径: %s", localFilePath)
	// 先处理重命名，只有非临时同步才会处理重命名，临时同步只会删除重建
	var existingFile models.SyncFile
	if !s.TmpSyncPath {
		err := db.Db.Where("file_id = ? AND sync_path_id = ?", file.GetFileId(), s.SyncPathId).First(&existingFile).Error
		if err == nil {
			// 如果SyncFiles存在，检查是否需要重命名，所在目录必须相同才可以重命名，否则只能走删除重建流程
			if existingFile.FileName != file.FileName && existingFile.Path == file.Path {
				// 需要重命名
				err := os.Rename(existingFile.LocalFilePath, localFilePath)
				if err != nil {
					// 只记录日志，不报错（因为重命名失败不影响后续处理，会自动转入删除、重建流程）
					s.Sync.Logger.Errorf("重命名失败 %s => %s: %w", existingFile.LocalFilePath, localFilePath, err)
				} else {
					s.Sync.Logger.Infof("重命名成功 %s => %s", existingFile.LocalFilePath, localFilePath)
				}
			}
		}
	}
	if file.IsVideo {
		// s.Sync.Logger.Infof("正在处理视频文件 %s", file.FileId)
		return s.ProcessStrmFile(file)
	}
	// 再处理元数据文件
	if file.IsMeta {
		if !helpers.PathExists(localFilePath) {
			// 如果文件不存在，则判断是否需要下载，使用strm设置
			if s.Config.EnableDownloadMeta == 1 {
				// 允许下载，添加到下载列表
				s.AddDownloadTaskTemp(file)
				// err := models.AddDownloadTaskFromSyncFile(file)

			}
		}
		return nil
	}
	return nil
}

// 添加下载任务（不实际添加，先记录起来，任务完成后，统一处理）
func (s *SyncStrm) AddDownloadTaskTemp(file *SyncFileCache) {
	file.NeedDownload = true
	// 生成下载索引
	s.memSyncCache.InsertDownloadIndex(file)
}

// 遍历同步缓存，添加下载任务
// 首先将下载队列表中没有完成的下载任务全部提取到map中，然后遍历内存同步缓存，检查哪些文件需要下载且不在下载任务列表中，添加下载任务
func (s *SyncStrm) AddDownloadTaskFromMemCache() {
	// 获取未完成的下载任务
	existingDownloads := make(map[string]bool)
	offset := 0
	limit := 1000
	type existDownloadTask struct {
		RemoteFileId string `json:"remote_file_id"`
	}
	for {
		var batch []existDownloadTask
		err := db.Db.Model(models.DbDownloadTask{}).Select("remote_file_id").Where("source_type = ? AND status IN ?", s.Account.SourceType, []int{int(models.DownloadStatusPending), int(models.DownloadStatusDownloading)}).
			Offset(offset).Limit(limit).Order("id ASC").Find(&batch).Error
		if err != nil {
			s.Sync.Logger.Errorf("获取未完成的下载任务失败: %v", err)
			break
		}
		if len(batch) == 0 || len(batch) < limit {
			break
		}
		for _, item := range batch {
			existingDownloads[item.RemoteFileId] = true
		}
		offset += limit
	}
	// 遍历内存同步缓存的下载索引
	s.memSyncCache.mu.RLock()
	for _, file := range s.memSyncCache.downloadIndex {
		if _, exists := existingDownloads[file.GetPickCode(s.Account.BaseUrl)]; exists {
			// 已经存在下载任务，跳过
			continue
		}
		// 添加下载任务
		err := models.AddDownloadTaskFromSyncFile(file.GetSyncFile(s, s.Account.BaseUrl))
		if err == nil {
			s.Sync.Logger.Infof("添加下载任务成功: %s=>%s", file.Path+"/"+file.FileName, file.GetLocalFilePath(s.TargetPath, s.SourcePath))
			atomic.AddInt64(&s.NewMeta, 1)
		}
	}
	s.memSyncCache.mu.RUnlock()
}

// 对比本地文件和临时表中的文件
func (s *SyncStrm) compareLocalFilesWithTempTable() error {
	s.Sync.UpdateSubStatus(models.SyncSubStatusProcessLocalFileList)
	select {
	case <-s.Context.Done():
		return nil
	default:
		rootPath := filepath.Join(s.TargetPath, s.SourcePath)
		s.Sync.Logger.Infof("开始对比本地文件和临时表中的文件，根目录: %s", rootPath)
		// 对比本地文件和临时表中的文件
		filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
			path = filepath.ToSlash(path)
			select {
			case <-s.Context.Done():
				return nil
			default:
				// 不处理目录（只处理文件）
				if err != nil || path == "." || strings.Contains(path, ".verysync") || strings.Contains(path, ".deletedByTMM") {
					// 跳过根目录本身
					// 跳过微力同步和TMM的临时目录中的文件
					return nil
				}
				if info.IsDir() {
					if s.Config.DelEmptyLocalDir {
						// 如果目录是空的则删除目录
						dirEntries, rerr := os.ReadDir(path)
						if rerr != nil {
							s.Sync.Logger.Errorf("读取目录 %s 的文件列表失败: %v", path, err)
							return nil
						}
						if len(dirEntries) == 0 {
							os.Remove(path)
							s.Sync.Logger.Infof("删除空目录 %s", path)
						}
					}
					return nil
				}
				ext := filepath.Ext(info.Name())
				isVideo := ext == ".strm"
				isMeta := s.IsValidMetaExt(info.Name())
				if isMeta && s.Config.EnableDownloadMeta == 0 {
					// 如果是元数据文件且设置为不下载，则跳过检查（代表着不上传）
					s.Sync.Logger.Infof("本地元数据文件 %s 由于关闭了元数据下载所以不需要处理", info.Name())
					return nil
				}
				if !isVideo && !isMeta {
					// 非视频文件和元数据文件，跳过
					s.Sync.Logger.Debugf("本地文件 %s 既不是STRM文件也不是元数据文件，跳过", path)
					return nil
				}
				// 检查文件在临时表是否存在
				// existsFile, _ := s.queryTempTableByLocalPath(path)
				existsFile, err := s.memSyncCache.GetByLocalPath(path)
				if err != nil {
					s.Sync.Logger.Warnf("查询同步缓存失败 %s: %v", path, err)
				}
				// s.Sync.Logger.Infof("对比本地文件 %s，是否存在于网盘: %v", path, existsFile)
				if isVideo {
					// STRM文件，检查文件在临时表是否存在，不存在需要删除临时文件
					if existsFile != nil {
						return nil
					}
					// s.Sync.Logger.Warnf("本地文件在网盘不存在，删除本地STRM文件: %s", path)
					s.RemoveFileAndCheckDirEmtry(path)
					return nil
				}
				if isMeta {
					// 如果选择忽略，则跳过
					if s.Config.NetNotFoundFileAction == models.SyncTreeItemMetaActionKeep {
						s.Sync.Logger.Infof("本地元数据文件 %s 由于设置为保留所以不需要处理", path)
						return nil
					}
					// 如果选择删除，则检查是否存在，不存在则删除
					if s.Config.NetNotFoundFileAction == models.SyncTreeItemMetaActionDelete && existsFile == nil {
						s.RemoveFileAndCheckDirEmtry(path)
						return nil
					}
					// 如果允许上传，则检查是否需要上传（文件在网盘不存在）
					if s.Config.NetNotFoundFileAction == models.SyncTreeItemMetaActionUpload && existsFile == nil {
						// 检查是否已经添加了上传任务，检查dbupload表中是否存在对应的记录
						canUpload := models.CheckCanUploadByLocalPath(models.UploadSourceStrm, path)
						if !canUpload {
							s.Sync.Logger.Infof("本地元数据文件 %s 由于存在上传任务所以不需要处理", path)
							return nil
						}
						sourceRootPath := filepath.ToSlash(filepath.Join(s.TargetPath, s.Sync.RemotePath))
						// 添加上传任务
						// 检查文件是否可以上传
						// 普通元数据文件需要父目录存在才可以上传，允许上传目录下的文件需要循环创建目录上传
						parentDir := filepath.Dir(path)
						parentName := filepath.Base(parentDir)
						if parentDir != "" {
							parentDir = filepath.ToSlash(parentDir)
						}
						isAllowedUploadDir := slices.Contains(uploadDirNames, strings.ToLower(parentName))
						// 检查父目录是否在网盘存在
						existsPath, _ := s.memSyncCache.GetByLocalPath(parentDir)
						// 如果不存在，检查是否可以创建目录
						var parentPath, parentPathId, remotePath string
						s.Sync.Logger.Infof("准备上传本地元数据文件 %s，检查父目录 %s 是否存在网盘", parentDir, sourceRootPath)
						if existsPath == nil && parentDir != sourceRootPath {
							if !isAllowedUploadDir {
								s.Sync.Logger.Infof("父目录 %s 不存在网盘，无法上传子文件 %s", parentDir, path)
								return nil
							} else {
								// 递归创建目录, 调用不同的driver
								parentPathId, remotePath, err = s.SyncDriver.CreateDirRecursively(s.Context, parentDir)
								if err != nil {
									s.Sync.Logger.Errorf("创建目录 %s 失败: %v", parentDir, err)
									return nil
								}
								parentPath = parentDir
							}
						} else {
							if parentDir == sourceRootPath {
								parentPath = sourceRootPath
								parentPathId = s.SourcePathId
								remotePath = s.SourcePath
							} else {
								parentPath = parentDir
								parentPathId = existsPath.GetFileId()
								remotePath = fmt.Sprintf("%s/%s", existsPath.Path, existsPath.FileName)
							}
						}
						// 加入上传队列
						db115File := &models.SyncFile{
							AccountId:     s.Account.ID,
							SyncPathId:    s.SyncPathId,
							SourceType:    s.Account.SourceType,
							FileType:      v115open.TypeFile,
							FileId:        "", // 上传前FileId为空
							ParentId:      parentPathId,
							FileName:      info.Name(),
							Path:          remotePath,
							FileSize:      info.Size(),
							MTime:         info.ModTime().Unix(),
							IsMeta:        isMeta,
							IsVideo:       isVideo,
							LocalFilePath: filepath.Join(parentPath, info.Name()),
						}
						if s.Account.SourceType != models.SourceType115 {
							db115File.FileId = filepath.ToSlash(filepath.Join(db115File.Path, db115File.FileName))
						}
						models.AddUploadTaskFromSyncFile(db115File)
						return nil
					}
				}
			}
			return nil
		})
	}
	return nil
}

// 处理SyncFile表和内存同步缓存的数据差异
// 临时表存在SyncFile没有的插入
// 临时表没有SyncFile有的删除
func (s *SyncStrm) handleTempTableDiff() error {
	// 先删除SyncFile表中有，但是同步缓存中没有的记录，过程中顺便更新两边都有的数据，然后从同步缓存中删除该条数据（最后同步缓存中留下的就是新增的数据）
	offset := 0
	limit := 1000
	// 要删除的ID
	waitDeleteIds := make([]uint, 0)
	for {
		var batch []models.SyncFile
		err := db.Db.Where("sync_path_id = ?", s.SyncPathId).Offset(offset).Limit(limit).Order("id ASC").Find(&batch).Error
		if err != nil {
			s.Sync.Logger.Warnf("获取SyncFile表数据失败: %v", err)
			return err
		}
		if len(batch) == 0 {
			s.Sync.Logger.Info("SyncFile表数据全部处理完毕")
			break
		}
		for _, file := range batch {
			syncFileCache, _ := s.memSyncCache.GetByFileId(file.FileId)
			if syncFileCache == nil {
				// 同步缓存中没有该文件，删除SyncFile记录
				waitDeleteIds = append(waitDeleteIds, file.ID)
				// s.Sync.Logger.Infof("SyncFile表数据 ID=%d 在同步缓存中不存在，已标记为删除", file.ID)
			} else {
				// 双方都有，更新SyncFile记录
				// 主要更新数据name, size, m_time, path, local_file_path
				udpateData := map[string]interface{}{
					"file_name":       syncFileCache.FileName,
					"file_size":       syncFileCache.FileSize,
					"m_time":          syncFileCache.MTime,
					"path":            syncFileCache.GetPath(),
					"local_file_path": syncFileCache.LocalFilePath,
					"thumb_url":       syncFileCache.ThumbUrl,
					"openlist_sign":   syncFileCache.OpenlistSign,
					"sha1":            syncFileCache.Sha1,
					"parent_id":       syncFileCache.ParentId,
				}
				err := db.Db.Model(&models.SyncFile{}).Where("id = ?", file.ID).Updates(udpateData).Error
				if err != nil {
					s.Sync.Logger.Errorf("更新SyncFile表数据失败 ID=%d: %v", file.ID, err)
					continue
				}
				// 然后从同步缓存中移除该记录
				s.memSyncCache.DeleteByFileId(file.FileId)
				// s.Sync.Logger.Infof("SyncFile表数据 ID=%d 在同步缓存中存在，已更新并移除同步缓存记录", file.ID)
			}
		}
		if len(batch) < limit {
			break
		}
		offset += limit
	}
	s.Sync.Logger.Infof("SyncFile表中共有 %d 条多余数据需要删除，开始分批删除，每批1000条", len(waitDeleteIds))
	// 分批删除
	batchSize := 1000
	if len(waitDeleteIds) <= batchSize {
		// 一次性删除
		err := db.Db.Where("id IN ?", waitDeleteIds).Delete(&models.SyncFile{}).Error
		if err != nil {
			s.Sync.Logger.Errorf("删除SyncFile表数据失败: %v", err)
			return err
		}
	} else {
		for i := 0; i < len(waitDeleteIds); i += batchSize {
			end := i + batchSize
			if end > len(waitDeleteIds) {
				end = len(waitDeleteIds)
			}
			batchIds := waitDeleteIds[i:end]
			err := db.Db.Where("id IN ?", batchIds).Delete(&models.SyncFile{}).Error
			if err != nil {
				s.Sync.Logger.Errorf("删除SyncFile表数据失败: %v", err)
				return err
			}
		}
		s.Sync.Logger.Infof("删除SyncFile表中 %d 条多余数据成功", len(waitDeleteIds))
	}
	s.Sync.Logger.Infof("已删除所有网盘不存在的文件记录，开始插入新增的文件记录")
	waitDeleteIds = nil // 清空切片

	// 然后插入同步缓存中剩余的新增数据
	// 不会并发执行该方法，所以可以直接读取
	offset = 0
	for {
		fileItems, err := s.memSyncCache.Query(offset, limit)
		if err != nil {
			s.Sync.Logger.Errorf("查询内存同步缓存数据失败: %v", err)
			return err
		}
		if len(fileItems) == 0 {
			s.Sync.Logger.Info("内存同步缓存数据全部处理完毕")
			break
		}
		for _, file := range fileItems {
			syncFile := file.GetSyncFile(s, s.Account.BaseUrl)
			err := db.Db.Create(syncFile).Error
			if err != nil {
				s.Sync.Logger.Errorf("插入SyncFile表数据失败 FileID=%s: %v", file.GetFileId(), err)
				continue
			}
			// s.Sync.Logger.Infof("插入SyncFile表数据成功 FileID=%s", file.FileId)
		}
		if len(fileItems) < limit {
			break
		}
		offset += limit
	}
	return nil
}
