package syncstrm

import (
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/v115open"
	"path/filepath"

	"golang.org/x/sync/errgroup"
)

// 启动路径队列调度器
func (s *SyncStrm) StartOther() {
	s.Sync.UpdateSubStatus(models.SyncSubStatusProcessNetFileList)

	eg, ctx := errgroup.WithContext(s.Context)
	eg.SetLimit(int(s.PathWorkerMax) + 3) // 增加一些额外的协程数，避免死锁

	var processPath func(pathQueueItem) error
	processPath = func(pathItem pathQueueItem) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		s.Sync.Logger.Infof("正在处理目录 %s 下的文件列表", pathItem.Path)
		if s.IsExcludeName(filepath.Base(pathItem.Path)) {
			s.Sync.Logger.Warnf("目录 %s 被排除，跳过它和旗下所有内容", pathItem.Path)
			return nil
		}

		// GetNetFileFiles 返回该目录下的子目录和文件列表
		fileItems, err := s.SyncDriver.GetNetFileFiles(ctx, pathItem.Path, pathItem.PathId)
		if err != nil {
			s.PathErrChan <- err
			return err
		}
		if len(fileItems) == 0 {
			s.Sync.Logger.Infof("目录 %s 下没有文件，跳过", pathItem.Path)
			return nil
		}
		// 递归处理子目录
		for _, fileItem := range fileItems {
			if s.IsExcludeName(filepath.Base(fileItem.FileName)) {
				s.Sync.Logger.Warnf("文件 %s 被排除，跳过它和其下所有内容", fileItem.FileName)
				continue
			}
			if fileItem.FileType == v115open.TypeDir {
				fileItem.GetLocalFilePath(s.TargetPath, s.SourcePath) // 生成本地路径缓存
				// 放入临时表
				s.memSyncCache.Insert(fileItem)
				// 继续处理该目录下的文件
				subPath := pathQueueItem{
					Path:   fileItem.GetFullRemotePath(),
					PathId: fileItem.GetFileId(),
				}
				eg.Go(func(item pathQueueItem) func() error {
					return func() error {
						return processPath(item)
					}
				}(subPath))
			} else {
				// 处理文件
				if !s.ValidFile(fileItem) {
					continue
				}
				fileItem.GetLocalFilePath(s.TargetPath, s.SourcePath) // 生成本地路径缓存
				// s.Sync.Logger.Infof("发现文件: %s 文件名：%s", fileItem.LocalFilePath, fileItem.FileName)
				// 放入临时表
				s.memSyncCache.Insert(fileItem)
				// s.Sync.Logger.Infof("文件加入临时表: %s", fileItem.LocalFilePath)
				// 处理文件
				s.processNetFile(fileItem)
				// s.Sync.Logger.Infof("文件处理完成: %s", fileItem.LocalFilePath)
			}
		}
		return nil
	}

	eg.Go(func() error {
		return processPath(pathQueueItem{
			Path:   s.SourcePath,
			PathId: s.SourcePathId,
		})
	})

	if err := eg.Wait(); err != nil {
		s.Sync.Logger.Errorf("路径处理失败: %v", err)
		return
	}
	s.Sync.Logger.Infof("已经遍历了全部目录")
}
