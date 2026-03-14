package syncstrm

import (
	"Q115-STRM/internal/baidupan"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/v115open"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
)

type localDriver struct {
	s *SyncStrm
}

func NewLocalDriver() *localDriver {
	return &localDriver{}
}

func (d *localDriver) SetSyncStrm(s *SyncStrm) {
	d.s = s
}

func (d *localDriver) GetNetFileFiles(ctx context.Context, parentPath, parentPathId string) ([]*SyncFileCache, error) {
	var fileItems []*SyncFileCache
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		// 查询文件列表
		fileList, err := os.ReadDir(parentPath)
		if err != nil {
			return nil, err
		}
		if len(fileList) == 0 {
			return fileItems, nil
		}
		fileItems = make([]*SyncFileCache, 0, len(fileList))
	fileloop:
		for _, file := range fileList {
			atomic.AddInt64(&d.s.TotalFile, 1)
			filePath := filepath.ToSlash(filepath.Join(parentPath, file.Name()))
			// 检查视频大小是否合规
			stat, err := os.Stat(filePath)
			if err != nil {
				d.s.Sync.Logger.Errorf("获取文件 %s 信息失败，跳过，错误: %v", filePath, err)
				continue fileloop
			}
			atomic.AddInt64(&d.s.TotalFile, 1)
			fileItem := SyncFileCache{
				ParentId:   parentPathId,
				FileName:   file.Name(),
				FileType:   v115open.TypeFile,
				FileSize:   stat.Size(),
				MTime:      stat.ModTime().Unix(),
				SourceType: models.SourceTypeLocal,
			}
			if file.IsDir() {
				fileItem.FileType = v115open.TypeDir
				fileItem.IsVideo = false
				fileItem.IsMeta = false
			}
			fileItems = append(fileItems, &fileItem)
		}
	}
	return fileItems, nil
}

func (d *localDriver) CreateDirRecursively(ctx context.Context, path string) (pathId, remotePath string, err error) {
	relPath, err := filepath.Rel(d.s.TargetPath, path)
	if err != nil {
		return "", "", fmt.Errorf("计算相对路径失败: %s 错误：%v", path, err)
	}
	targetPath := filepath.Join(d.s.SourcePath, relPath)
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return "", "", fmt.Errorf("创建目录失败: %s 错误：%v", targetPath, err)
	}
	// 将新添加的目录加入同步缓存
	syncFileCache := &SyncFileCache{
		ParentId:   filepath.Dir(relPath),
		FileName:   filepath.Base(relPath),
		FileType:   v115open.TypeDir,
		IsVideo:    false,
		IsMeta:     false,
		SourceType: models.SourceTypeLocal,
	}
	syncFileCache.GetLocalFilePath(d.s.TargetPath, d.s.SourcePath)
	d.s.memSyncCache.Insert(syncFileCache)
	return targetPath, relPath, nil
}

func (d *localDriver) GetPathIdByPath(ctx context.Context, path string) (string, error) {
	if !helpers.PathExists(path) {
		return "", fmt.Errorf("路径 %s 不存在", path)
	}
	return path, nil
}

func (d *localDriver) MakeStrmContent(sf *SyncFileCache) string {
	fullPath := sf.GetFileId()
	if runtime.GOOS == "windows" {
		// windows要将分隔换成\
		fullPath = strings.ReplaceAll(fullPath, "/", "\\")
	}
	return fullPath
}

func (d *localDriver) GetTotalFileCount(ctx context.Context) (int64, string, error) {
	return 0, "", nil
}

func (d *localDriver) GetDirsByPathId(ctx context.Context, pathId string) ([]pathQueueItem, error) {
	return nil, nil
}

func (d *localDriver) GetFilesByPathId(ctx context.Context, rootPathId string, offset, limit int) ([]v115open.File, error) {
	return nil, nil
}

// 所有文件详情，含路径
func (d *localDriver) DetailByFileId(ctx context.Context, fileId string) (*SyncFileCache, error) {
	return nil, nil
}

// 删除目录下的某些文件
func (d *localDriver) DeleteFile(ctx context.Context, parentId string, fileIds []string) error {
	for _, fileId := range fileIds {
		if err := os.Remove(fileId); err != nil {
			d.s.Sync.Logger.Errorf("删除文件 %s 失败，错误: %v", fileId, err)
			continue
		}
	}
	return nil
}

func (d *localDriver) GetFilesByPathMtime(ctx context.Context, rootPathId string, offset, limit int, mtime int64) (*baidupan.FileListAllResponse, error) {
	return nil, nil
}
