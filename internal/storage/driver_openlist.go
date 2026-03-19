package storage

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"Q115-STRM/internal/openlist"
)

// DriverOpenList OpenList驱动
type DriverOpenList struct {
	client *openlist.Client
}

// NewOpenListDriver 创建OpenList驱动
func NewOpenListDriver(config DriverConfig) (CloudStorageDriver, error) {
	if config.ClientOpenList == nil {
		return nil, fmt.Errorf("OpenList客户端不能为空")
	}

	client, ok := config.ClientOpenList.(*openlist.Client)
	if !ok {
		return nil, fmt.Errorf("OpenList客户端类型错误")
	}

	return &DriverOpenList{
		client: client,
	}, nil
}

// GetNetFileFiles 获取文件列表（支持分页）
func (d *DriverOpenList) GetNetFileFiles(ctx context.Context, parentPath, parentPathId string, offset, limit int) ([]File, error) {
	page := offset/limit + 1
	if page < 1 {
		page = 1
	}

	pageSize := 50 // OpenList默认每页50条
	if limit > 0 && limit < pageSize {
		pageSize = limit
	}

	files := make([]File, 0)
	totalFetched := 0

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			resp, err := d.client.FileList(ctx, parentPath, page, pageSize)
			if err != nil {
				return nil, err
			}

			if resp.Total == 0 {
				break
			}

			for _, file := range resp.Content {
				if totalFetched < offset {
					totalFetched++
					continue
				}

				// 将ISO 8601格式的日期字符串转换为时间戳
				t, err := time.Parse(time.RFC3339, file.Modified)
				var mtime int64
				if err != nil {
					mtime = 0
				} else {
					mtime = t.Unix()
				}

				fileItem := File{
					Path:     parentPath,
					FileName: file.Name,
					FileSize: file.Size,
					MTime:    mtime,
				}

				if file.IsDir {
					fileItem.FileType = "dir"
				} else {
					fileItem.FileType = "file"
					fileItem.PickCode = file.Sign
				}

				files = append(files, fileItem)
				totalFetched++

				if limit > 0 && len(files) >= limit {
					return files, nil
				}
			}

			if resp.Total <= int64(page*pageSize) {
				break
			}

			page++
		}
	}

	return files, nil
}

// GetDirsByPathId 获取子目录列表
func (d *DriverOpenList) GetDirsByPathId(ctx context.Context, pathId string) ([]Dir, error) {
	files, err := d.GetNetFileFiles(ctx, pathId, pathId, 0, 1000)
	if err != nil {
		return nil, err
	}

	dirs := make([]Dir, 0)
	for _, file := range files {
		if file.FileType == "dir" {
			path := filepath.Join(pathId, file.FileName)
			path = filepath.ToSlash(path)
			dirs = append(dirs, Dir{
				Path:   path,
				PathId: path,
				Mtime:  file.MTime,
			})
		}
	}

	return dirs, nil
}

// DetailByFileId 根据 fileId 获取文件详情
func (d *DriverOpenList) DetailByFileId(ctx context.Context, fileId string) (*FileDetail, error) {
	fsDetail, err := d.client.FileDetail(fileId)
	if err != nil || (fsDetail != nil && fsDetail.Name == "") {
		return nil, fmt.Errorf("文件ID %s 不存在: %v", fileId, err)
	}

	parentId := filepath.ToSlash(filepath.Dir(fileId))

	// 将ISO 8601格式的日期字符串转换为时间戳
	t, err := time.Parse(time.RFC3339, fsDetail.Modified)
	var mtime int64
	if err != nil {
		mtime = 0
	} else {
		mtime = t.Unix()
	}

	detail := &FileDetail{
		FileId:   fileId,
		FileName: fsDetail.Name,
		FileType: "file",
		FileSize: int64(fsDetail.Size),
		MTime:    mtime,
		Path:     fileId,
		ParentId: parentId,
	}

	if fsDetail.IsDir {
		detail.FileType = "dir"
	} else {
		detail.PickCode = fsDetail.Sign
	}

	return detail, nil
}

// GetPathIdByPath 根据 path 获取 pathId
func (d *DriverOpenList) GetPathIdByPath(ctx context.Context, path string) (string, error) {
	fsDetail, err := d.client.FileDetail(path)
	if err != nil || (fsDetail != nil && fsDetail.Name == "") {
		return "", fmt.Errorf("路径 %s 不存在: %v", path, err)
	}
	return path, nil
}

// CreateDirRecursively 递归创建目录
func (d *DriverOpenList) CreateDirRecursively(ctx context.Context, path string) (pathId, remotePath string, err error) {
	// 确保路径以 / 开头
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// 分隔路径
	pathParts := strings.Split(path, "/")

	// 反向检查，找到哪一集不存在，再正向创建
	notExistIndex := -1
	for i := len(pathParts) - 1; i >= 0; i-- {
		if i == 0 {
			// 根目录
			return path, path, nil
		}

		dir := strings.Join(pathParts[:i+1], "/")
		fsDetail, err := d.client.FileDetail(dir)
		if err != nil || (fsDetail != nil && fsDetail.Name == "") {
			notExistIndex = i
			continue
		}
		// 一旦发现存在的，就退出
		break
	}

	if notExistIndex == -1 {
		return path, path, nil
	}

	// 从notExistIndex开始，正向创建目录
	for i := notExistIndex + 1; i <= len(pathParts); i++ {
		dir := strings.Join(pathParts[:i+1], "/")
		err := d.client.Mkdir(dir)
		if err != nil {
			return "", "", fmt.Errorf("创建目录失败: %s 错误：%v", dir, err)
		}
	}

	return path, path, nil
}

// DeleteDir 删除目录
func (d *DriverOpenList) DeleteDir(ctx context.Context, path, pathId string) error {
	// OpenList删除目录与删除文件相同
	names := []string{filepath.Base(path)}
	return d.client.Del(pathId, names)
}

// DeleteFile 删除文件
func (d *DriverOpenList) DeleteFile(ctx context.Context, parentId string, fileIds []string) error {
	names := make([]string, len(fileIds))
	for i, fileId := range fileIds {
		names[i] = filepath.Base(fileId)
	}
	return d.client.Del(parentId, names)
}

// RenameFile 重命名文件
func (d *DriverOpenList) RenameFile(ctx context.Context, fileId, newName string) error {
	// TODO: OpenList暂不支持重命名
	return fmt.Errorf("OpenList暂不支持重命名操作")
}

// MoveFile 移动文件
func (d *DriverOpenList) MoveFile(ctx context.Context, fileId, newParentId, newPath string) error {
	// TODO: OpenList暂不支持移动
	return fmt.Errorf("OpenList暂不支持移动操作")
}

// ReadFileContent 读取文件内容
func (d *DriverOpenList) ReadFileContent(ctx context.Context, fileId string) ([]byte, error) {
	// OpenList不支持直接读取文件内容
	return nil, fmt.Errorf("OpenList暂不支持读取文件内容")
}

// WriteFileContent 写入文件内容
func (d *DriverOpenList) WriteFileContent(ctx context.Context, path, pathId string, content []byte) error {
	// OpenList不支持直接写入文件内容
	return fmt.Errorf("OpenList暂不支持写入文件内容")
}

// UploadFile 上传文件
func (d *DriverOpenList) UploadFile(ctx context.Context, localPath, remotePath, remotePathId string) error {
	// TODO: OpenList上传功能待实现
	return fmt.Errorf("OpenList上传功能待实现")
}

// GetTotalFileCount 获取文件总数
func (d *DriverOpenList) GetTotalFileCount(ctx context.Context, path, pathId string) (int64, string, error) {
	// OpenList需要递归获取文件数，这里先返回0
	return 0, "", nil
}

// GetFilesByMtime 根据修改时间获取文件列表
func (d *DriverOpenList) GetFilesByMtime(ctx context.Context, rootPathId string, offset, limit int, mtime int64) ([]File, error) {
	// OpenList不支持按时间过滤，需要获取全部后过滤
	files, err := d.GetNetFileFiles(ctx, rootPathId, rootPathId, offset, limit)
	if err != nil {
		return nil, err
	}

	// 过滤修改时间大于指定值的文件
	result := make([]File, 0)
	for _, file := range files {
		if file.MTime > mtime {
			result = append(result, file)
		}
	}

	return result, nil
}

// MakeStrmContent 生成 STRM 文件内容
func (d *DriverOpenList) MakeStrmContent(file *File) (string, error) {
	// TODO: OpenList需要根据配置生成STRM内容
	// 这里需要访问配置信息，但当前驱动结构体中没有保存配置
	// 可能需要在 DriverConfig 中添加 STRM 相关配置
	return "", fmt.Errorf("OpenList STRM 内容生成功能待实现")
}

// CheckPathExists 检查路径是否存在
func (d *DriverOpenList) CheckPathExists(ctx context.Context, path, pathId string) error {
	fsDetail, err := d.client.FileDetail(path)
	if err != nil || (fsDetail != nil && fsDetail.Name == "") {
		return fmt.Errorf("路径不存在: %s", path)
	}
	return nil
}
