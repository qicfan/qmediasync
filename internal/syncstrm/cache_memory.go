package syncstrm

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/v115open"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

// SyncFileCache 完整缓存结构（主要用于115网盘）
// 包含基础字段 + 115网盘特有字段
// 内存占用约: 32 (BaseSyncFileCache) + 1 (FileType) + 24 (3 strings) = ~57 bytes
type SyncFileCache struct {
	// 核心标识字段
	FileId   string            `json:"file_id"`
	ParentId string            `json:"parent_id"`
	FileType v115open.FileType `json:"file_type"`

	// 文件信息
	FileName      string `json:"file_name"`
	Path          string `json:"path"`            // 绝对路径，不包含FileName
	LocalFilePath string `json:"local_file_path"` // 本地完整路径（TargetPath + Path + FileName）
	FileSize      int64  `json:"file_size"`
	MTime         int64  `json:"mtime"` // 最后修改时间
	PickCode      string `json:"pick_code"`

	// 类型标识
	IsVideo bool `json:"is_video"`
	IsMeta  bool `json:"is_meta"`

	NeedDownload bool `json:"need_download"` // 标记需要下载

	// 115特有字段

	Sha1     string `json:"sha1"`
	ThumbUrl string `json:"thumb_url"`

	// openlist特有字段
	OpenlistSign string `json:"openlist_sign"`

	SourceType models.SourceType `json:"source_path"`
}

func (sfc *SyncFileCache) GetPath() string {
	switch sfc.SourceType {
	case models.SourceType115:
		return sfc.Path
	case models.SourceTypeOpenList:
		return sfc.ParentId
	case models.SourceTypeLocal:
		return sfc.ParentId
	case models.SourceType123:
		return sfc.Path
	}
	return sfc.Path
}

func (sfc *SyncFileCache) GetFileId() string {
	filePath := filepath.ToSlash(filepath.Join(sfc.ParentId, sfc.FileName))
	switch sfc.SourceType {
	case models.SourceType115:
		return sfc.FileId
	case models.SourceTypeOpenList:
		return filePath
	case models.SourceTypeLocal:
		return filePath
	case models.SourceType123:
		return sfc.FileId
	}
	return sfc.FileId
}

func (sfc *SyncFileCache) GetPickCode(openlistBaseUrl string) string {
	switch sfc.SourceType {
	case models.SourceType115:
		return sfc.PickCode
	case models.SourceTypeOpenList:
		// 计算出完整的下载链接
		return helpers.MakeOpenListUrl(openlistBaseUrl, sfc.OpenlistSign, sfc.GetFileId())
	case models.SourceTypeLocal:
		return sfc.GetFileId()
	case models.SourceType123:
		return sfc.PickCode
	}
	return sfc.PickCode
}

func (sfc *SyncFileCache) GetFullRemotePath() string {
	switch sfc.SourceType {
	case models.SourceType115:
		return filepath.ToSlash(filepath.Join(sfc.Path, sfc.FileName))
	case models.SourceTypeOpenList:
		return filepath.ToSlash(filepath.Join(sfc.ParentId, sfc.FileName))
	case models.SourceTypeLocal:
		return filepath.ToSlash(filepath.Join(sfc.ParentId, sfc.FileName))
	case models.SourceType123:
		return filepath.ToSlash(filepath.Join(sfc.Path, sfc.FileName))
	}
	return filepath.ToSlash(filepath.Join(sfc.Path, sfc.FileName))
}

// GetLocalFilePath 实时生成本地文件完整路径（Path + FileName）
// 避免存储冗余数据，节省内存约 100 bytes/file
func (b *SyncFileCache) GetLocalFilePath(targetPath, sourcePath string) string {
	if b.LocalFilePath != "" {
		return b.LocalFilePath
	}
	// 视频文件要转成strm扩展名
	fileName := b.FileName
	if b.IsVideo {
		ext := filepath.Ext(fileName)
		baseName := strings.TrimSuffix(fileName, ext)
		fileName = baseName + ".strm"
	}
	fullPath := filepath.Join(targetPath, b.GetPath(), fileName)
	if b.SourceType == models.SourceTypeLocal {
		// 本地不能拼接完整的file.Path
		relPath, err := filepath.Rel(sourcePath, b.GetPath())
		if err != nil {
			return ""
		}
		fullPath = filepath.Join(targetPath, relPath, fileName)
	}
	// 将windows路径转换为unix路径
	fullPath = filepath.ToSlash(fullPath)
	b.LocalFilePath = fullPath
	return b.LocalFilePath
}

// 将SyncFileCache转换为models.SyncFile
func (d *SyncFileCache) GetSyncFile(s *SyncStrm, openlistBaseUrl string) *models.SyncFile {
	syncFile := &models.SyncFile{
		AccountId:     s.Account.ID,
		SyncPathId:    s.SyncPathId,
		SourceType:    d.SourceType,
		FileId:        d.GetFileId(),
		ParentId:      d.ParentId,
		Path:          d.GetPath(),
		FileName:      d.FileName,
		FileSize:      d.FileSize,
		FileType:      d.FileType,
		MTime:         d.MTime,
		PickCode:      d.GetPickCode(openlistBaseUrl),
		OpenlistSign:  d.OpenlistSign,
		ThumbUrl:      d.ThumbUrl,
		Sha1:          d.Sha1,
		IsVideo:       d.IsVideo,
		IsMeta:        d.IsMeta,
		Processed:     d.Processed,
		LocalFilePath: d.GetLocalFilePath(s.TargetPath, s.SourcePath),
	}
	return syncFile
}

// MemorySyncCache 内存同步缓存
type MemorySyncCache struct {
	mu sync.RWMutex

	// 主索引：file_id -> SyncFileCache
	fileIndex map[string]*SyncFileCache

	// 辅助索引：需要下载的文件 pick_code -> SyncFileCache
	downloadIndex map[string]*SyncFileCache

	// 辅助索引：local_file_path -> file_id（用于快速查找）
	localPathIndex map[string]*SyncFileCache

	// 辅助索引：parent_id -> []file_id（用于按父目录查询）
	parentIndex map[string][]*SyncFileCache

	// // 按顺序存储的 file_id 列表（用于分页查询）
	// orderedFiles []string

	// 同步路径ID（用于过滤）
	syncPathId uint
}

// NewMemorySyncCache 创建内存同步缓存
func NewMemorySyncCache(syncPathId uint) *MemorySyncCache {
	return &MemorySyncCache{
		fileIndex:      make(map[string]*SyncFileCache),
		downloadIndex:  make(map[string]*SyncFileCache),
		localPathIndex: make(map[string]*SyncFileCache),
		parentIndex:    make(map[string][]*SyncFileCache),
		// orderedFiles:   make([]string, 0),
		syncPathId: syncPathId,
	}
}

// Insert 插入单条记录
func (c *MemorySyncCache) Insert(file *SyncFileCache) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if file.GetFileId() == "" {
		return fmt.Errorf("file_id不能为空")
	}

	// 主索引
	c.fileIndex[file.GetFileId()] = file

	// 本地路径索引(有Path一定有LocalFilePath)
	// helpers.AppLogger.Infof("缓存文件: %s 路径：%s 本地路径: %s", file.GetFileId(), file.GetPath(), file.LocalFilePath)
	if file.GetPath() != "" {
		// helpers.AppLogger.Infof("加入本地文件索引: %s 路径：%s 本地路径: %s", file.GetFileId(), file.GetPath(), file.LocalFilePath)
		c.localPathIndex[file.LocalFilePath] = file
	}

	// 父目录索引
	if file.ParentId != "" {
		c.parentIndex[file.ParentId] = append(c.parentIndex[file.ParentId], file)
	}

	// 顺序列表
	// c.orderedFiles = append(c.orderedFiles, file.GetFileId())

	return nil
}

// 放入待下载索引
func (c *MemorySyncCache) InsertDownloadIndex(file *SyncFileCache) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if file.GetFileId() == "" {
		return nil
	}
	c.downloadIndex[file.GetFileId()] = file
	return nil
}

// BatchInsert 批量插入
func (c *MemorySyncCache) BatchInsert(files []*SyncFileCache) error {
	for _, file := range files {
		if err := c.Insert(file); err != nil {
			return err
		}
	}
	return nil
}

// Query 查询，不分页
func (c *MemorySyncCache) Query() {
	// c.mu.RLock()
	// defer c.mu.RUnlock()

	// total := len(c.orderedFiles)
	// if offset >= total {
	// 	return []*SyncFileCache{}, nil
	// }

	// end := offset + limit
	// if limit <= 0 || end > total {
	// 	end = total
	// }

	// result := make([]*SyncFileCache, 0, end-offset)
	// for i := offset; i < end; i++ {
	// 	fileId := c.orderedFiles[i]
	// 	if file, ok := c.fileIndex[fileId]; ok {
	// 		result = append(result, file)
	// 	}
	// }

	// return result, nil
}

// GetByFileId 根据 file_id 查询
func (c *MemorySyncCache) GetByFileId(fileId string) (*SyncFileCache, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	file, ok := c.fileIndex[fileId]
	if !ok {
		return nil, fmt.Errorf("未找到记录: file_id=%s", fileId)
	}
	return file, nil
}

// GetByLocalPath 根据本地路径查询
func (c *MemorySyncCache) GetByLocalPath(localFilePath string) (*SyncFileCache, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	file, ok := c.localPathIndex[localFilePath]
	if !ok {
		return nil, fmt.Errorf("未找到记录: local_file_path=%s", localFilePath)
	}

	return file, nil
}

// GetByParentId 根据 parent_id 查询
func (c *MemorySyncCache) GetByParentId(parentId string) ([]*SyncFileCache, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	files, ok := c.parentIndex[parentId]
	if !ok {
		return nil, fmt.Errorf("未找到记录: parent_id=%s", parentId)
	}
	return files, nil
}

// ExistsByLocalPath 检查本地路径是否存在
func (c *MemorySyncCache) ExistsByLocalPath(localFilePath string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, ok := c.localPathIndex[localFilePath]
	return ok
}

// DeleteByFileId 根据 file_id 删除
func (c *MemorySyncCache) DeleteByFileId(fileId string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	file, ok := c.fileIndex[fileId]
	if !ok {
		return nil // 不存在就当作删除成功
	}

	// 删除主索引
	delete(c.fileIndex, fileId)

	// 删除本地路径索引（使用实时生成的 GetLocalFilePath()）
	localPath := file.LocalFilePath
	if localPath != "" {
		delete(c.localPathIndex, localPath)
	}

	// 删除父目录索引中的引用
	if file.ParentId != "" {
		children := c.parentIndex[file.ParentId]
		for i, child := range children {
			if child.GetFileId() == fileId {
				c.parentIndex[file.ParentId] = append(children[:i], children[i+1:]...)
				break
			}
		}
	}
	return nil
}

// DeleteByParentId 根据 parent_id 删除所有子项
func (c *MemorySyncCache) DeleteByParentId(parentId string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	files, ok := c.parentIndex[parentId]
	if !ok {
		return nil // 没有子项
	}

	// 删除所有子项
	for _, file := range files {
		if file, exists := c.fileIndex[file.GetFileId()]; exists {
			// 删除主索引
			delete(c.fileIndex, file.GetFileId())

			// 删除本地路径索引（使用实时生成的 GetLocalFilePath()）
			localPath := file.LocalFilePath
			if localPath != "" {
				delete(c.localPathIndex, localPath)
			}
		}
	}

	// 删除父目录索引
	delete(c.parentIndex, parentId)

	return nil
}

// UpdatePathByParentId 更新指定父目录下所有文件的路径
func (c *MemorySyncCache) UpdatePathByParentId(parentId string, newPath string, targetPath, sourcePath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	files, ok := c.parentIndex[parentId]
	if !ok {
		return nil // 没有子项
	}

	for _, file := range files {
		if file, exists := c.fileIndex[file.GetFileId()]; exists {
			file.Path = newPath
			// 更新完整本地路径
			file.GetLocalFilePath(targetPath, sourcePath)
			// 加入本地路径索引
			c.localPathIndex[file.LocalFilePath] = file
		}
	}

	return nil
}

// Count 统计记录数
func (c *MemorySyncCache) Count() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return int64(len(c.fileIndex))
}

// Clear 清空所有数据
func (c *MemorySyncCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.fileIndex = make(map[string]*SyncFileCache)
	c.localPathIndex = make(map[string]*SyncFileCache)
	c.parentIndex = make(map[string][]*SyncFileCache)
}

func (c *MemorySyncCache) GetAllFile() map[string]*SyncFileCache {
	return c.fileIndex
}
