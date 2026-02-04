package syncstrm

import (
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/v115open"

	"context"
	"path/filepath"
	"strings"

	"golang.org/x/sync/errgroup"
)

// 115文件路径处理器
// 从临时表中查询Path=""的记录，调用115接口获取路径信息，然后更新回数据库，然后处理这个路径下所有文件的逻辑
func (s *SyncStrm) Start115PathDispathcer() error {
	// 使用 errgroup 管理并发
	eg, ctx := errgroup.WithContext(s.Context)
	eg.SetLimit(int(s.PathWorkerMax))
	offset := 0
	limit := 1000
	// 先找到所有路径为空的目录ID，去重
	parentIds := make(map[string]bool)
	for {

		c := s.memSyncCache.Count()
		if c == 0 {
			break
		}
		fileItems := s.memSyncCache.GetAllFile()
		for _, item := range fileItems {
			if item.FileType == v115open.TypeDir || item.Path != "" {
				continue
			}
			parentIds[item.ParentId] = true
		}
		if len(fileItems) < limit {
			break
		}
		offset += limit
	}
	// 将路径ID加入任务队列
	s.Sync.Logger.Infof("开始路径补全任务，共有 %d 个需要补全路径的目录", len(parentIds))
	for pathId := range parentIds {
		currentPathId := pathId // 捕获循环变量
		s.Sync.Logger.Infof("加入目录ID %s 到路径补全队列", currentPathId)
		eg.Go(func() error {
			return s.process115Path(ctx, currentPathId)
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}
	parentIds = nil // 释放内存
	s.Sync.Logger.Infof("结束路径补全任务")
	return nil
}

func (s *SyncStrm) process115Path(ctx context.Context, pathId string) error {
	select {
	case <-ctx.Done():
		// 上下文取消，退出循环
		return ctx.Err()
	default:
	}
	var pathStr string
	var pathName string
	var detail *v115open.FileDetail
	var fsErr error
	// 处理路径ID
	s.Sync.Logger.Infof("处理路径ID %s", pathId)
	detail, fsErr = s.SyncDriver.DetailByFileId(ctx, pathId)
	if fsErr != nil {
		s.Sync.Logger.Errorf("获取路径 %s 详情失败: %v", pathId, fsErr)
		// 如果失败，则中断所有任务
		return fsErr
	}
	if strings.Contains(detail.FileName, "***") {
		s.Sync.Logger.Infof("目录ID %s 名称：%s 包含 *** 号，跳过", pathId, detail.FileName)
		return nil
	}
	// 把当前目录加入路径数组
	detail.Paths = append(detail.Paths, v115open.FileDetailPath{
		FileId: detail.FileId,
		Name:   detail.FileName,
	})
	pathStr = ""
	foundBase := false
	lastRemotePathPart := filepath.Base(s.SourcePath)
	isExclude := false
	lastPathId := ""
pathloop:
	for _, p := range detail.Paths {
		if p.FileId == "" || p.FileId == "0" {
			continue
		}
		if pathStr == "" {
			pathStr = p.Name
		} else {
			pathStr = filepath.Join(pathStr, p.Name)
			pathStr = filepath.ToSlash(pathStr)
		}
		if p.FileId == s.SourcePathId {
			foundBase = true
			continue
		}
		if !foundBase || p.Name == lastRemotePathPart {
			continue
		}
		if s.IsExcludeName(p.Name) {
			s.Sync.Logger.Infof("路径 %s 名称：%s 被排除", p.FileId, p.Name)
			isExclude = true
			break
		}
		// 检查是否已经存在
		if _, ok := s.sync115.existsPathes.Load(p.FileId); ok {
			lastPathId = p.FileId
			continue pathloop
		}
		// 入库，如果入库成功则加入缓存（因为有唯一索引，如果入库报重复代表已经入库了，则跳过）
		insertPath := filepath.ToSlash(filepath.Dir(pathStr))
		pathSyncFile := &SyncFileCache{
			FileId:     p.FileId,
			FileName:   p.Name,
			FileType:   v115open.TypeDir,
			ParentId:   lastPathId,
			Path:       insertPath,
			SourceType: models.SourceType115,
			IsVideo:    false,
			IsMeta:     false,
		}
		lastPathId = p.FileId
		pathSyncFile.GetLocalFilePath(s.TargetPath, s.SourcePath)
		s.Sync.Logger.Infof("目录ID %s 名称：%s 路径：%s 本地路径：%s", pathId, detail.FileName, pathSyncFile.Path, pathSyncFile.LocalFilePath)
		// 判断缓存中是否存在
		if _, ok := s.sync115.existsPathes.Load(p.FileId); !ok {
			s.memSyncCache.Insert(pathSyncFile)
			s.sync115.existsPathes.Store(p.FileId, insertPath)
			s.Sync.Logger.Infof("目录ID %s 名称：%s 路径：%s 放入同步缓存成功", p.FileId, p.Name, insertPath)
		}
	}
	// 检查是否被排除，如果排除需要从临时表删除所有该目录下的文件
	if isExclude {
		// 从临时表中删除所有该目录下的文件
		s.Sync.Logger.Infof("目录ID %s 名称：%s 被排除，从同步缓存中删除所有该目录下的文件", pathId, detail.FileName)

		if err := s.memSyncCache.DeleteByParentId(pathId); err != nil {
			s.Sync.Logger.Errorf("删除同步缓存中记录失败: parent_id=%s, %v", pathId, err)
		}
		return nil
	}
	pathName = detail.FileName
	// 将完整路径更新到所有文件记录中
	if err := s.memSyncCache.UpdatePathByParentId(pathId, pathStr, s.TargetPath, s.SourcePath); err != nil {
		s.Sync.Logger.Errorf("更新临时表路径失败: parent_id=%s, path=%s, %v", pathId, pathStr, err)
	} else {
		s.Sync.Logger.Infof("目录ID %s 名称：%s 路径：%s 更新所有该目录下的文件路径成功", pathId, pathName, pathStr)
	}
	// 处理路径下所有文件
	if updateErr := s.handelTempFileByPathId(pathId); updateErr != nil {
		return updateErr
	}
	return nil
}

// 更新路径下的所有文件并处理他们
func (s *SyncStrm) handelTempFileByPathId(pathId string) error {
	// 加锁
	files, err := s.memSyncCache.GetByParentId(pathId)
	if err != nil {
		s.Sync.Logger.Errorf("查询临时表文件失败: parent_id=%s, %v", pathId, err.Error)
		return err
	}
	s.Sync.Logger.Infof("开始处理路径ID %s 下的所有文件，共 %d 个", pathId, len(files))
	if len(files) == 0 {
		// 如果没有更多文件，退出
		return nil
	}
	for _, file := range files {
		if file.Processed {
			s.Sync.Logger.Infof("文件ID %s %s 路径 %s 已处理，跳过", file.FileId, file.FileName, file.Path)
			continue
		}
		// 更新文件路径
		file.GetLocalFilePath(s.TargetPath, s.SourcePath)
		s.Sync.Logger.Infof("文件ID %s 路径 %s 本地路径 %s 路径已补全，开始处理文件", file.FileId, file.Path, file.LocalFilePath)
		// 开始处理文件
		s.processNetFile(file)
	}
	return nil
}
