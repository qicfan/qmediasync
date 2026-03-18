package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalDriver 本地文件系统驱动
type LocalDriver struct {
	rootPath string
}

// NewLocalDriver 创建本地驱动
func NewLocalDriver(config DriverConfig) (CloudStorageDriver, error) {
	return &LocalDriver{
		rootPath: "",
	}, nil
}

// GetNetFileFiles 获取文件列表
func (d *LocalDriver) GetNetFileFiles(ctx context.Context, parentPath, parentPathId string, offset, limit int) ([]File, error) {
	var files []File

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		entries, err := os.ReadDir(parentPath)
		if err != nil {
			return nil, err
		}

		files = make([]File, 0, len(entries))
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				continue
			}

			file := File{
				Path:     filepath.Join(parentPath, entry.Name()),
				FileName: entry.Name(),
				FileSize: info.Size(),
				MTime:    info.ModTime().Unix(),
			}

			if entry.IsDir() {
				file.FileType = "dir"
			} else {
				file.FileType = "file"
			}

			files = append(files, file)
		}
	}

	return files, nil
}

// GetDirsByPathId 获取子目录列表
func (d *LocalDriver) GetDirsByPathId(ctx context.Context, pathId string) ([]Dir, error) {
	var dirs []Dir

	entries, err := os.ReadDir(pathId)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			dirs = append(dirs, Dir{
				Path:   filepath.Join(pathId, entry.Name()),
				PathId: filepath.Join(pathId, entry.Name()),
				Mtime:  info.ModTime().Unix(),
			})
		}
	}

	return dirs, nil
}

// DetailByFileId 根据 fileId 获取文件详情
func (d *LocalDriver) DetailByFileId(ctx context.Context, fileId string) (*FileDetail, error) {
	info, err := os.Stat(fileId)
	if err != nil {
		return nil, err
	}

	absPath, err := filepath.Abs(fileId)
	if err != nil {
		absPath = fileId
	}

	detail := &FileDetail{
		FileId:   absPath,
		FileName: filepath.Base(fileId),
		FileSize: info.Size(),
		MTime:    info.ModTime().Unix(),
		Path:     absPath,
		ParentId: filepath.Dir(absPath),
	}

	if info.IsDir() {
		detail.FileType = "dir"
	} else {
		detail.FileType = "file"
	}

	return detail, nil
}

// GetPathIdByPath 根据 path 获取 pathId
func (d *LocalDriver) GetPathIdByPath(ctx context.Context, path string) (string, error) {
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	return path, nil
}

// CreateDirRecursively 递归创建目录
func (d *LocalDriver) CreateDirRecursively(ctx context.Context, path string) (pathId, remotePath string, err error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", "", err
	}
	return path, path, nil
}

// DeleteDir 删除目录
func (d *LocalDriver) DeleteDir(ctx context.Context, path, pathId string) error {
	return os.RemoveAll(path)
}

// DeleteFile 删除文件
func (d *LocalDriver) DeleteFile(ctx context.Context, parentId string, fileIds []string) error {
	for _, fileId := range fileIds {
		if err := os.Remove(fileId); err != nil {
			return err
		}
	}
	return nil
}

// RenameFile 重命名文件
func (d *LocalDriver) RenameFile(ctx context.Context, fileId, newName string) error {
	dir := filepath.Dir(fileId)
	newPath := filepath.Join(dir, newName)
	return os.Rename(fileId, newPath)
}

// MoveFile 移动文件
func (d *LocalDriver) MoveFile(ctx context.Context, fileId, newParentId, newPath string) error {
	return os.Rename(fileId, newPath)
}

// ReadFileContent 读取文件内容
func (d *LocalDriver) ReadFileContent(ctx context.Context, fileId string) ([]byte, error) {
	return os.ReadFile(fileId)
}

// WriteFileContent 写入文件内容
func (d *LocalDriver) WriteFileContent(ctx context.Context, path, pathId string, content []byte) error {
	return os.WriteFile(path, content, 0644)
}

// UploadFile 上传文件
func (d *LocalDriver) UploadFile(ctx context.Context, localPath, remotePath, remotePathId string) error {
	src, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(remotePath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

// GetTotalFileCount 获取文件总数
func (d *LocalDriver) GetTotalFileCount(ctx context.Context, path, pathId string) (int64, string, error) {
	var count int64

	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			count++
		}
		return nil
	})

	if err != nil {
		return 0, "", err
	}

	return count, "", nil
}

// GetFilesByMtime 根据修改时间获取文件列表
func (d *LocalDriver) GetFilesByMtime(ctx context.Context, rootPathId string, offset, limit int, mtime int64) ([]File, error) {
	var files []File
	count := 0
	skipped := 0

	err := filepath.Walk(rootPathId, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if info.IsDir() {
			return nil
		}

		if info.ModTime().Unix() < mtime {
			return nil
		}

		if skipped < offset {
			skipped++
			return nil
		}

		if count >= limit {
			return io.EOF
		}

		files = append(files, File{
			FileId:   path,
			Path:     path,
			FileName: info.Name(),
			FileType: "file",
			FileSize: info.Size(),
			MTime:    info.ModTime().Unix(),
		})
		count++

		return nil
	})

	if err != nil && err != io.EOF {
		return nil, err
	}

	return files, nil
}

// MakeStrmContent 生成 STRM 文件内容
func (d *LocalDriver) MakeStrmContent(file *File) (string, error) {
	content := file.Path
	if strings.Contains(content, "\\") {
		// Windows 路径，确保使用反斜杠
		content = strings.ReplaceAll(content, "/", "\\")
	}
	return content, nil
}

// CheckPathExists 检查路径是否存在
func (d *LocalDriver) CheckPathExists(ctx context.Context, path, pathId string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("路径不存在: %s", path)
		}
		return err
	}
	return nil
}
