package storage

import (
	"context"
	"fmt"
	"path/filepath"

	"Q115-STRM/internal/baidupan"
)

// DriverBaiduPan 百度网盘驱动
type DriverBaiduPan struct {
	client *baidupan.Client
}

// NewBaiduPanDriver 创建百度网盘驱动
func NewBaiduPanDriver(config DriverConfig) (CloudStorageDriver, error) {
	if config.ClientBaidu == nil {
		return nil, fmt.Errorf("百度网盘客户端不能为空")
	}

	client, ok := config.ClientBaidu.(*baidupan.Client)
	if !ok {
		return nil, fmt.Errorf("百度网盘客户端类型错误")
	}

	return &DriverBaiduPan{
		client: client,
	}, nil
}

// GetNetFileFiles 获取文件列表（支持分页）
func (d *DriverBaiduPan) GetNetFileFiles(ctx context.Context, parentPath, parentPathId string, offset, limit int) ([]File, error) {
	pageSize := 50 // 百度网盘默认每页50条
	if limit > 0 && limit < pageSize {
		pageSize = limit
	}

	page := offset/pageSize + 1
	var files []File

	for {
		resp, err := d.client.GetFileList(ctx, parentPath, 0, 1, int32((page-1)*pageSize), int32(pageSize))
		if err != nil {
			return nil, err
		}

		if len(resp) == 0 {
			break
		}

		for _, file := range resp {
			files = append(files, File{
				FileId:   file.Path,
				Path:     filepath.ToSlash(filepath.Dir(file.Path)),
				FileName: filepath.Base(file.Path),
				FileType: "file",
				FileSize: int64(file.Size),
				MTime:    int64(file.ServerMtime),
				Sha1:     file.Md5,
			})

			if file.IsDir == 1 {
				files[len(files)-1].FileType = "dir"
			}
		}

		if len(resp) < pageSize {
			break
		}

		// 如果已经获取了足够的文件，停止
		if limit > 0 && len(files) >= limit {
			files = files[:limit]
			break
		}

		page++
	}

	return files, nil
}

// GetDirsByPathId 获取子目录列表
func (d *DriverBaiduPan) GetDirsByPathId(ctx context.Context, pathId string) ([]Dir, error) {
	// 百度网盘使用 path 作为 pathId
	resp, err := d.client.GetFileList(ctx, pathId, 0, 1, 0, 1000)
	if err != nil {
		return nil, err
	}

	dirs := make([]Dir, 0)
	for _, file := range resp {
		if file.IsDir == 1 {
			dirs = append(dirs, Dir{
				Path:   file.Path,
				PathId: file.Path,
				Mtime:  int64(file.ServerMtime),
			})
		}
	}

	return dirs, nil
}

// DetailByFileId 根据 fileId 获取文件详情
func (d *DriverBaiduPan) DetailByFileId(ctx context.Context, fileId string) (*FileDetail, error) {
	// 百度网盘使用 path 作为 fileId
	resp, err := d.client.FileExists(ctx, fileId)
	if err != nil {
		return nil, err
	}

	parentId := filepath.ToSlash(filepath.Dir(fileId))

	detail := &FileDetail{
		FileId:   fileId,
		FileName: resp.ServerFilename,
		FileType: "file",
		FileSize: int64(resp.Size),
		MTime:    int64(resp.ServerMtime),
		Path:     fileId,
		ParentId: parentId,
	}

	if resp.IsDir == 1 {
		detail.FileType = "dir"
	} else {
		detail.PickCode = fmt.Sprintf("%d", resp.FsId)
		detail.Sha1 = resp.Md5
	}

	return detail, nil
}

// GetPathIdByPath 根据 path 获取 pathId
func (d *DriverBaiduPan) GetPathIdByPath(ctx context.Context, path string) (string, error) {
	// 百度网盘直接使用 path 作为 pathId
	_, err := d.client.GetFileList(ctx, path, 0, 1, 0, 1)
	if err != nil {
		return "", fmt.Errorf("路径 %s 不存在: %v", path, err)
	}
	return path, nil
}

// CreateDirRecursively 递归创建目录
func (d *DriverBaiduPan) CreateDirRecursively(ctx context.Context, path string) (pathId, remotePath string, err error) {
	// 百度网盘直接根据完整路径创建
	relPath := filepath.ToSlash(filepath.Clean(path))
	err = d.client.Mkdir(ctx, relPath)
	if err != nil {
		return "", "", fmt.Errorf("创建目录 %s 失败: %v", relPath, err)
	}
	return relPath, relPath, nil
}

// DeleteDir 删除目录
func (d *DriverBaiduPan) DeleteDir(ctx context.Context, path, pathId string) error {
	// 百度网盘删除目录与删除文件相同
	return d.client.Del(ctx, []string{pathId})
}

// DeleteFile 删除文件
func (d *DriverBaiduPan) DeleteFile(ctx context.Context, parentId string, fileIds []string) error {
	return d.client.Del(ctx, fileIds)
}

// RenameFile 重命名文件
func (d *DriverBaiduPan) RenameFile(ctx context.Context, fileId, newName string) error {
	// TODO: 百度网盘暂不支持重命名
	return fmt.Errorf("百度网盘暂不支持重命名操作")
}

// MoveFile 移动文件
func (d *DriverBaiduPan) MoveFile(ctx context.Context, fileId, newParentId, newPath string) error {
	// TODO: 百度网盘暂不支持移动
	return fmt.Errorf("百度网盘暂不支持移动操作")
}

// ReadFileContent 读取文件内容
func (d *DriverBaiduPan) ReadFileContent(ctx context.Context, fileId string) ([]byte, error) {
	// 百度网盘不支持直接读取文件内容
	return nil, fmt.Errorf("百度网盘暂不支持读取文件内容")
}

// WriteFileContent 写入文件内容
func (d *DriverBaiduPan) WriteFileContent(ctx context.Context, path, pathId string, content []byte) error {
	// 百度网盘不支持直接写入文件内容
	return fmt.Errorf("百度网盘暂不支持写入文件内容")
}

// UploadFile 上传文件
func (d *DriverBaiduPan) UploadFile(ctx context.Context, localPath, remotePath, remotePathId string) error {
	// TODO: 百度网盘上传功能待实现
	return fmt.Errorf("百度网盘上传功能待实现")
}

// GetTotalFileCount 获取文件总数
func (d *DriverBaiduPan) GetTotalFileCount(ctx context.Context, path, pathId string) (int64, string, error) {
	// 百度网盘需要递归获取文件数
	return 0, "", nil
}

// GetFilesByMtime 根据修改时间获取文件列表
func (d *DriverBaiduPan) GetFilesByMtime(ctx context.Context, rootPathId string, offset, limit int, mtime int64) ([]File, error) {
	// 使用百度网盘的增量同步API
	resp, err := d.client.GetAllFiles(ctx, rootPathId, offset, limit, mtime)
	if err != nil {
		return nil, err
	}

	files := make([]File, 0, len(resp.List))
	for _, file := range resp.List {
		files = append(files, File{
			FileId:   file.Path,
			Path:     filepath.ToSlash(filepath.Dir(file.Path)),
			FileName: file.ServerFilename,
			FileType: "file",
			FileSize: int64(file.Size),
			MTime:    int64(file.ServerMtime),
			Sha1:     file.Md5,
		})

		if file.IsDir == 1 {
			files[len(files)-1].FileType = "dir"
		}
	}

	return files, nil
}

// MakeStrmContent 生成 STRM 文件内容
func (d *DriverBaiduPan) MakeStrmContent(file *File) (string, error) {
	// TODO: 百度网盘需要根据配置生成STRM内容
	return "", fmt.Errorf("百度网盘 STRM 内容生成功能待实现")
}

// CheckPathExists 检查路径是否存在
func (d *DriverBaiduPan) CheckPathExists(ctx context.Context, path, pathId string) error {
	_, err := d.client.FileExists(ctx, path)
	if err != nil {
		return fmt.Errorf("路径不存在: %s", path)
	}
	return nil
}
