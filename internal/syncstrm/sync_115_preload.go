package syncstrm

import (
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/v115open"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sync/errgroup"
)

// 预取两层115目录，入库(写入 SyncFile表），写入缓存s.sync115.existsPathes，如果缓存不存在且写入了，s.sync115.existsPathesCount++
func (s *SyncStrm) Preload115Dirs(firstFileId string) error {
	// 查询第一个文件的详情，拿到路径
	firstFile, detailErr := s.SyncDriver.DetailByFileId(s.Context, firstFileId)
	if detailErr != nil {
		s.Sync.Logger.Errorf("查询第一个文件详情失败: file_id=%s, %v", firstFileId, detailErr)
		return detailErr
	}

	depth := s.GetDepth(firstFile.Paths)
	if depth < 1 || depth > 2 {
		s.Sync.Logger.Warnf("计算预取目录深度小于1或者大于2，无需预取目录")
		return nil
	}

	// 使用 errgroup 管理并发生命周期
	eg, ctx := errgroup.WithContext(s.Context)
	// SetLimit 自动限制并发数（替代 Worker Pool）
	eg.SetLimit(int(s.PathWorkerMax) + 2) // 增加2个协程，避免死锁，115 openapi客户端会限制并发

	// 递归函数，优雅地处理树状目录结构
	var processPath func(*pathQueueItem) error
	processPath = func(item *pathQueueItem) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		pathItems, err := s.SyncDriver.GetDirsByPathId(ctx, item.PathId)
		if err != nil {
			reason := "查询路径下的子目录失败"
			s.Sync.Logger.Warnf("%s: %v", reason, err)
			s.Sync.Failed(fmt.Sprintf("%s: %v", reason, err))
			return err
		}

		if len(pathItems) == 0 {
			s.Sync.Logger.Infof("路径: %s 下没有子目录", item.Path)
			return nil // 递归终止条件
		}

		for _, pathItem := range pathItems {
			s.Sync.Logger.Infof("查询路径下的子目录: %s", pathItem.Path)
			//检查是否被排除
			if s.IsExcludeName(filepath.Base(pathItem.Path)) {
				s.Sync.Logger.Infof("路径: %s 名称被排除，跳过", pathItem.Path)
				s.sync115.excludePathId.Store(pathItem.PathId, true)
				continue
			}
			fileItem := &SyncFileCache{
				FileId:     pathItem.PathId,
				FileName:   filepath.Base(pathItem.Path),
				FileType:   v115open.TypeDir,
				SourceType: models.SourceType115,
				Path:       filepath.ToSlash(filepath.Dir(pathItem.Path)),
				ParentId:   item.PathId,
				MTime:      pathItem.Mtime,
				IsVideo:    false,
				IsMeta:     false,
				Processed:  true,
			}
			fileItem.GetLocalFilePath(s.TargetPath, s.SourcePath) // 生成本地路径缓存
			s.memSyncCache.Insert(fileItem)
			// 更新缓存
			if _, ok := s.sync115.existsPathes.Load(pathItem.PathId); !ok {
				s.sync115.existsPathes.Store(pathItem.PathId, pathItem.Path)
			}

			// 递归处理子目录，errgroup 自动管理并发
			if item.Depth < depth-1 {
				subItem := &pathQueueItem{
					Path:   pathItem.Path,
					PathId: pathItem.PathId,
					Depth:  item.Depth + 1,
				}
				// 每个子目录启动一个 goroutine，SetLimit 自动控制并发
				s.Sync.Logger.Infof("预取115目录，递归处理子目录: %s", subItem.Path)
				eg.Go(func() error {
					return processPath(subItem)
				})
			}
		}

		return nil
	}

	// 启动根目录处理
	eg.Go(func() error {
		return processPath(&pathQueueItem{
			Path:   s.SourcePath,
			PathId: s.SourcePathId,
			Depth:  0,
		})
	})

	// 统一等待所有任务并处理错误
	return eg.Wait()
}

func (s *SyncStrm) GetDepth(pathes []v115open.FileDetailPath) uint {
	pathArr := []string{}
	for _, p := range pathes {
		if p.FileId == "0" {
			continue
		}
		pathArr = append(pathArr, p.Name)
	}
	firstFilePath := filepath.Join(pathArr...)
	depth := uint(1)
	relPath, relErr := filepath.Rel(s.SourcePath, firstFilePath)
	if relErr != nil {
		s.Sync.Logger.Warnf("计算相对路径失败: %v", relErr)
	}
	// 用分隔符分割relPath，计算深度
	parts := strings.Split(relPath, string(os.PathSeparator))
	depth = uint(len(parts))
	s.Sync.Logger.Infof("计算预取目录深度为 %s => %s, %d", firstFilePath, relPath, depth)
	return depth
}
