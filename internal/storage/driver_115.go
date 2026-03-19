package storage

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/v115open"
)

// Driver115 115网盘驱动
type Driver115 struct {
	client *v115open.OpenClient
}

// New115Driver 创建115网盘驱动
func New115Driver(config DriverConfig) (CloudStorageDriver, error) {
	if config.Client115 == nil {
		return nil, fmt.Errorf("115客户端不能为空")
	}

	client, ok := config.Client115.(*v115open.OpenClient)
	if !ok {
		return nil, fmt.Errorf("115客户端类型错误")
	}

	return &Driver115{
		client: client,
	}, nil
}

// GetNetFileFiles 获取文件列表（支持分页）
func (d *Driver115) GetNetFileFiles(ctx context.Context, parentPath, parentPathId string, offset, limit int) ([]File, error) {
	const maxLimit = 1150 // 115 API限制

	// 限制limit最大值
	if limit > maxLimit {
		limit = maxLimit
	}
	if limit <= 0 {
		limit = 1000
	}

	resp, err := d.client.GetFsList(ctx, parentPathId, true, false, true, offset, limit)
	if err != nil {
		if err.Error() == "访问频率过高" {
			// 访问频率过高，等待30秒后重试
			time.Sleep(30 * time.Second)
			resp, err = d.client.GetFsList(ctx, parentPathId, true, false, true, offset, limit)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	if len(resp.Data) == 0 {
		return []File{}, nil
	}

	files := make([]File, 0, len(resp.Data))
	for _, file := range resp.Data {
		if file.Aid != "1" {
			// 文件已删除或放入回收站，跳过
			continue
		}

		files = append(files, File{
			FileId:   file.FileId,
			Path:     parentPath,
			FileName: file.FileName,
			PickCode: file.PickCode,
			FileType: string(file.FileCategory),
			FileSize: file.FileSize,
			MTime:    file.Ptime,
			Sha1:     file.Sha1,
			ThumbUrl: file.Thumbnail,
		})
	}

	return files, nil
}

// GetDirsByPathId 获取子目录列表
func (d *Driver115) GetDirsByPathId(ctx context.Context, pathId string) ([]Dir, error) {
	offset := 0
	const limit = 1150
	dirs := make([]Dir, 0)

	for {
		resp, err := d.client.GetFsList(ctx, pathId, true, true, true, offset, limit)
		if err != nil {
			if err.Error() == "访问频率过高" {
				// 访问频率过高，等待30秒后重试
				time.Sleep(30 * time.Second)
				continue
			}
			return nil, err
		}

		if len(resp.Data) == 0 {
			break
		}

		for _, file := range resp.Data {
			if file.Aid != "1" {
				continue
			}
			if file.FileCategory != v115open.TypeDir {
				continue
			}

			path := filepath.Join(resp.PathStr, file.FileName)
			path = filepath.ToSlash(path)
			dirs = append(dirs, Dir{
				Path:   path,
				PathId: file.FileId,
				Mtime:  file.Ptime,
			})
		}

		if resp.Count < limit {
			break
		}

		offset += limit
	}

	return dirs, nil
}

// DetailByFileId 根据 fileId 获取文件详情
func (d *Driver115) DetailByFileId(ctx context.Context, fileId string) (*FileDetail, error) {
	resp, err := d.client.GetFsDetailByCid(ctx, fileId)
	if err != nil {
		return nil, err
	}

	parentId := ""
	if len(resp.Paths) > 0 {
		parentId = resp.Paths[len(resp.Paths)-1].FileId
	}

	detail := &FileDetail{
		FileId:   resp.FileId,
		FileName: resp.FileName,
		FileType: string(resp.FileCategory),
		FileSize: helpers.StringToInt64(resp.FileSize),
		MTime:    helpers.StringToInt64(resp.Utime),
		Path:     resp.Path,
		ParentId: parentId,
		PickCode: resp.PickCode,
		Sha1:     resp.Sha1,
	}

	// 转换路径信息
	detail.Paths = make([]PathInfo, len(resp.Paths))
	for i, p := range resp.Paths {
		detail.Paths[i] = PathInfo{
			FileId: p.FileId,
			Path:   p.Name,
		}
	}

	return detail, nil
}

// GetPathIdByPath 根据 path 获取 pathId
func (d *Driver115) GetPathIdByPath(ctx context.Context, path string) (string, error) {
	fsDetail, err := d.client.GetFsDetailByPath(ctx, path)
	if err != nil {
		return "", err
	}
	return fsDetail.FileId, nil
}

// CreateDirRecursively 递归创建目录
func (d *Driver115) CreateDirRecursively(ctx context.Context, path string) (pathId, remotePath string, err error) {
	// 确保路径以 / 开头
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// 分隔路径
	pathParts := strings.Split(path, "/")

	// 反向检查，找到哪一集不存在，再正向创建
	notExistIndex := -1
	lastExistsPathId := ""
	for i := len(pathParts) - 1; i >= 0; i-- {
		if i == 0 {
			// 根目录总是存在的
			lastExistsPathId = "0"
			break
		}

		dir := strings.Join(pathParts[:i+1], "/")
		fsDetail, err := d.client.GetFsDetailByPath(ctx, dir)
		if err != nil || fsDetail == nil || fsDetail.FileId == "" {
			notExistIndex = i
			continue
		}
		// 一旦发现存在的，就退出
		lastExistsPathId = fsDetail.FileId
		break
	}

	// 从notExistIndex开始，正向创建目录
	for i := notExistIndex + 1; i <= len(pathParts); i++ {
		dir := strings.Join(pathParts[:i+1], "/")
		var currentFileId string
		currentFileId, err = d.client.MkDir(ctx, lastExistsPathId, filepath.Base(dir))
		if err != nil {
			return "", "", fmt.Errorf("创建目录失败: %s 错误：%v", dir, err)
		}
		lastExistsPathId = currentFileId
	}

	return lastExistsPathId, path, nil
}

// DeleteDir 删除目录
func (d *Driver115) DeleteDir(ctx context.Context, path, pathId string) error {
	_, err := d.client.Del(ctx, []string{pathId}, "")
	return err
}

// DeleteFile 删除文件
func (d *Driver115) DeleteFile(ctx context.Context, parentId string, fileIds []string) error {
	_, err := d.client.Del(ctx, fileIds, "")
	return err
}

// RenameFile 重命名文件
func (d *Driver115) RenameFile(ctx context.Context, fileId, newName string) error {
	// TODO: 115 API 可能不支持直接重命名，需要先复制后删除
	return fmt.Errorf("115网盘暂不支持重命名操作")
}

// MoveFile 移动文件
func (d *Driver115) MoveFile(ctx context.Context, fileId, newParentId, newPath string) error {
	// TODO: 115 API 可能需要特殊处理
	return fmt.Errorf("115网盘暂不支持移动操作")
}

// ReadFileContent 读取文件内容
func (d *Driver115) ReadFileContent(ctx context.Context, fileId string) ([]byte, error) {
	// 115网盘不支持直接读取文件内容
	// 需要先获取下载地址，然后下载
	return nil, fmt.Errorf("115网盘暂不支持读取文件内容")
}

// WriteFileContent 写入文件内容
func (d *Driver115) WriteFileContent(ctx context.Context, path, pathId string, content []byte) error {
	// 115网盘不支持直接写入文件内容
	// 需要创建临时文件后上传
	return fmt.Errorf("115网盘暂不支持写入文件内容")
}

// UploadFile 上传文件
func (d *Driver115) UploadFile(ctx context.Context, localPath, remotePath, remotePathId string) error {
	// TODO: 使用115的上传API
	return fmt.Errorf("115网盘上传功能待实现")
}

// GetTotalFileCount 获取文件总数
func (d *Driver115) GetTotalFileCount(ctx context.Context, path, pathId string) (int64, string, error) {
	resp, err := d.client.GetFsList(ctx, pathId, false, false, false, 0, 1)
	if err != nil || len(resp.Data) == 0 {
		return 0, "", err
	}
	return int64(resp.Count), resp.Data[0].FileId, nil
}

// GetFilesByMtime 根据修改时间获取文件列表
func (d *Driver115) GetFilesByMtime(ctx context.Context, rootPathId string, offset, limit int, mtime int64) ([]File, error) {
	// 115 API 不支持按时间过滤，需要获取全部后过滤
	files, err := d.GetNetFileFiles(ctx, "", rootPathId, offset, limit)
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
func (d *Driver115) MakeStrmContent(file *File) (string, error) {
	// TODO: 115网盘需要根据配置生成STRM内容
	// 这里需要访问配置信息，但当前驱动结构体中没有保存配置
	// 可能需要在 DriverConfig 中添加 STRM 相关配置
	return "", fmt.Errorf("115网盘 STRM 内容生成功能待实现")
}

// CheckPathExists 检查路径是否存在
func (d *Driver115) CheckPathExists(ctx context.Context, path, pathId string) error {
	_, err := d.client.GetFsDetailByPath(ctx, path)
	return err
}
