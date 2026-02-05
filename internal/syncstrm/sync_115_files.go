package syncstrm

import (
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/v115open"

	"context"

	"golang.org/x/sync/errgroup"
)

// 启动115文件处理调度器
// 启动N个文件处理，将所有文件查询回来入临时表
func (s *SyncStrm) Start115FileDispathcer(total int64) error {
	limit := 1150
	// 根据文件总数来计算需要多少个处理器
	pageCount := total / int64(limit)
	if total%int64(limit) != 0 {
		pageCount++
	}
	if pageCount == 0 {
		s.Sync.Logger.Infof("无需处理文件列表，总数为 0")
		return nil
	}
	// 取pageCount和s.PathWorkerMax之间的较小值
	workerMax := min(pageCount, s.PathWorkerMax)

	// 使用 errgroup 管理并发
	eg, ctx := errgroup.WithContext(s.Context)
	eg.SetLimit(int(workerMax))

	// 加入文件任务
	for page := 0; page < int(pageCount); page++ {
		page := page // 捕获循环变量
		eg.Go(func() error {
			return s.process115FilePage(ctx, page, limit)
		})
	}
	s.Sync.Logger.Infof("所有文件任务已加入队列，等待处理器完成")
	return eg.Wait()
}

// 115文件页处理器
func (s *SyncStrm) process115FilePage(ctx context.Context, page, limit int) error {
	// 检查 context 是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	offset := page * int(limit)
	s.Sync.Logger.Infof("文件处理器开始处理文件列表，offset=%d, limit=%d", offset, limit)
	// 查询115网盘文件
	var files []v115open.File
	var err error
	files, err = s.SyncDriver.GetFilesByPathId(ctx, s.SourcePathId, offset, limit)
	if err != nil {
		s.Sync.Logger.Errorf("获取115网盘文件列表失败: 目录ID %s, offset=%d, limit=%d, %v", s.SourcePathId, offset, limit, err)
		return err
	}
	// 处理查询到的文件
	for _, file := range files {
		// s.Sync.Logger.Infof("文件 %s => %s 开始处理", file.FileId, file.FileName)
		// 检查文件是否被排除
		if s.IsExcludeName(file.FileName) {
			s.Sync.Logger.Warnf("文件 %s 被排除", file.FileName)
			continue
		}
		// 检查目录ID是否被排除
		if _, excluded := s.sync115.excludePathId.Load(file.Pid); excluded {
			s.Sync.Logger.Warnf("文件 %s 的父目录ID %s 被排除", file.FileName, file.Pid)
			continue
		}
		// 处理文件
		// 生成一个临时的SyncFile
		syncFile := SyncFileCache{
			Path:       "",
			FileId:     file.FileId,
			FileName:   file.FileName,
			SourceType: models.SourceType115,
			ParentId:   file.Pid,
			FileSize:   file.FileSize,
			FileType:   file.FileCategory,
			PickCode:   file.PickCode,
			Sha1:       file.Sha1,
			MTime:      file.Ptime,
			ThumbUrl:   file.Thumbnail,
		}
		// 验证文件本身，然后入临时表
		if !s.ValidFile(&syncFile) {
			continue
		}
		if parentPath, ok := s.sync115.existsPathes.Load(file.Pid); ok {
			syncFile.Path = parentPath.(string)
			// s.Sync.Logger.Infof("文件 %s 的父路径已存在，路径为 %s", file.FileName, syncFile.Path)
			syncFile.GetLocalFilePath(s.TargetPath, s.SourcePath)
			// 检查是否被排除
			if s.IsExcludePath(syncFile.Path) {
				s.Sync.Logger.Warnf("文件 %s 的路径 %s 中有排除项，被排除", file.FileName, syncFile.LocalFilePath)
				continue
			}
		}
		// 放入同步缓存
		err := s.memSyncCache.Insert(&syncFile)
		if err != nil {
			s.Sync.Logger.Errorf("文件 %s => %s 插入同步缓存失败: %v", syncFile.FileId, syncFile.FileName, err)
			return err
		}
		// s.Sync.Logger.Infof("文件 %s => %s 插入同步缓存成功, 路径 %s", syncFile.FileId, syncFile.FileName, syncFile.LocalFilePath)
		// 如果路径完整，直接处理文件
		if syncFile.LocalFilePath != "" {
			s.processNetFile(&syncFile)
		}
		// s.Sync.Logger.Infof("文件 %s => %s 处理完成", syncFile.FileId, syncFile.FileName)
	}
	s.Sync.Logger.Infof("文件处理器处理完成offset=%d, limit=%d，共处理 %d 个文件", offset, limit, len(files))
	return nil
}
