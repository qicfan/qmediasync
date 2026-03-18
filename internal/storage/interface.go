package storage

import (
	"context"
)

// CloudStorageDriver 网盘驱动统一接口
type CloudStorageDriver interface {
	// ========== 文件列表操作 ==========

	// GetNetFileFiles 获取文件列表（支持分页）
	GetNetFileFiles(ctx context.Context, parentPath, parentPathId string, offset, limit int) ([]File, error)

	// GetDirsByPathId 获取子目录列表
	GetDirsByPathId(ctx context.Context, pathId string) ([]Dir, error)

	// ========== 文件详情操作 ==========

	// DetailByFileId 根据 fileId 获取文件详情（含路径）
	DetailByFileId(ctx context.Context, fileId string) (*FileDetail, error)

	// GetPathIdByPath 根据 path 获取 pathId
	GetPathIdByPath(ctx context.Context, path string) (string, error)

	// ========== 目录操作 ==========

	// CreateDirRecursively 递归创建目录
	CreateDirRecursively(ctx context.Context, path string) (pathId, remotePath string, err error)

	// DeleteDir 删除目录
	DeleteDir(ctx context.Context, path, pathId string) error

	// ========== 文件操作 ==========

	// DeleteFile 删除文件
	DeleteFile(ctx context.Context, parentId string, fileIds []string) error

	// RenameFile 重命名文件
	RenameFile(ctx context.Context, fileId, newName string) error

	// MoveFile 移动文件
	MoveFile(ctx context.Context, fileId, newParentId, newPath string) error

	// ========== 文件内容操作 ==========

	// ReadFileContent 读取文件内容
	ReadFileContent(ctx context.Context, fileId string) ([]byte, error)

	// WriteFileContent 写入文件内容
	WriteFileContent(ctx context.Context, path, pathId string, content []byte) error

	// UploadFile 上传文件
	UploadFile(ctx context.Context, localPath, remotePath, remotePathId string) error

	// ========== 统计操作 ==========

	// GetTotalFileCount 获取文件总数
	GetTotalFileCount(ctx context.Context, path, pathId string) (int64, string, error)

	// ========== 增量同步操作 ==========

	// GetFilesByMtime 根据修改时间获取文件列表（增量同步）
	GetFilesByMtime(ctx context.Context, rootPathId string, offset, limit int, mtime int64) ([]File, error)

	// ========== 特殊操作 ==========

	// MakeStrmContent 生成 STRM 文件内容（网盘特有）
	MakeStrmContent(file *File) (string, error)

	// CheckPathExists 检查路径是否存在
	CheckPathExists(ctx context.Context, path, pathId string) error
}

// File 文件基本信息
type File struct {
	FileId   string
	Path     string
	FileName string
	FileType string // dir/file
	FileSize int64
	MTime    int64
	PickCode string // 网盘特有
	Sha1     string
	ThumbUrl string
	// 扩展字段，各驱动可以自定义
	Extra map[string]interface{}
}

// Dir 目录基本信息
type Dir struct {
	Path   string
	PathId string
	Mtime  int64
}

// FileDetail 文件详情
type FileDetail struct {
	FileId     string
	FileName   string
	FileType   string
	FileSize   int64
	MTime      int64
	Path       string
	ParentId   string
	PickCode   string
	Sha1       string
	Paths      []PathInfo
	Extra      map[string]interface{}
}

// PathInfo 路径信息
type PathInfo struct {
	FileId string
	Path   string
}
