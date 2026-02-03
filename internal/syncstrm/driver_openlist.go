package syncstrm

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/openlist"
	"Q115-STRM/internal/v115open"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

type openListDriver struct {
	s      *SyncStrm
	client *openlist.Client
}

func NewOpenListDriver(client *openlist.Client) *openListDriver {
	return &openListDriver{
		client: client,
	}
}

func (d *openListDriver) SetSyncStrm(s *SyncStrm) {
	d.s = s
}

func (d *openListDriver) GetNetFileFiles(ctx context.Context, parentPath, parentPathId string) ([]*SyncFileCache, error) {
	page := 1
	pageSize := 50 // 每次取50条
	var fileItems []*SyncFileCache = make([]*SyncFileCache, 0)
mainloop:
	for {
		select {
		case <-ctx.Done():
			d.s.Sync.Logger.Infof("获取openlist文件列表上下文已取消, path=%s, page=%d, pageSize=%d", parentPath, page, pageSize)
			return nil, ctx.Err()
		default:
			resp, err := d.client.FileList(ctx, parentPath, page, pageSize)
			if err != nil {
				d.s.Sync.Logger.Errorf("获取openlist文件列表失败: %v", err)
				return nil, err
			}
			if resp.Total == 0 {
				// 取完了
				break mainloop
			}
			for _, file := range resp.Content {
				atomic.AddInt64(&d.s.TotalFile, 1)
				// 将ISO 8601格式的日期字符串转换为时间戳
				t, err := time.Parse(time.RFC3339, file.Modified)
				var mtime int64
				if err != nil {
					d.s.Sync.Logger.Errorf("解析时间格式失败: %v, 时间字符串: %s", err, file.Modified)
					mtime = 0 // 错误时使用默认值
				} else {
					mtime = t.Unix() // 转换为Unix时间戳（秒）
				}
				fileItem := SyncFileCache{
					ParentId:     parentPathId,
					FileName:     file.Name,
					FileType:     v115open.TypeFile,
					FileSize:     file.Size,
					MTime:        mtime,
					OpenlistSign: file.Sign,
					SourceType:   models.SourceTypeOpenList,
				}
				if file.IsDir {
					fileItem.FileType = v115open.TypeDir
					fileItem.IsVideo = false
					fileItem.IsMeta = false
					fileItem.Processed = true
				}
				fileItems = append(fileItems, &fileItem)

			}
			if resp.Total <= int64(page*pageSize) {
				break mainloop
			}
		}
		page += 1
	}
	return fileItems, nil
}

// 检查每一部分是否存在，不存在就创建
func (d *openListDriver) CreateDirRecursively(ctx context.Context, path string) (pathId, remotePath string, err error) {
	// 检查路径是否存在
	relPath, err := filepath.Rel(d.s.TargetPath, path)
	if err != nil {
		return "", "", fmt.Errorf("计算相对路径失败:%s %w", path, err)
	}
	relPath = filepath.ToSlash(relPath)
	// 如果不以/开头，则加上/
	if !strings.HasPrefix(relPath, "/") {
		relPath = "/" + relPath
	}
	// 分隔
	pathParts := strings.Split(relPath, "/")
	// 反向检查，找到哪一集不存在，再正向创建
	notExistIndex := -1
	for i := len(pathParts) - 1; i >= 0; i-- {
		dir := filepath.Join(pathParts[:i+1]...)
		fsDetail, err := d.client.FileDetail(dir)
		if err != nil || (fsDetail != nil && fsDetail.Name == "") {
			notExistIndex = i
			continue
		}
		// 一旦发现存在的，就退出
		break
	}
	if notExistIndex == -1 {
		return relPath, relPath, nil
	}
	// 正向创建
	for i := notExistIndex; i < len(pathParts); i++ {
		dir := filepath.Join(pathParts[:i+1]...)
		err := d.client.Mkdir(dir)
		if err != nil {
			return "", "", fmt.Errorf("创建目录失败: %s 错误：%v", dir, err)
		}
		// fullLocalPath := filepath.ToSlash(filepath.Join(d.s.TargetPath, dir))
		// 将新添加的目录加入同步缓存
		syncFileCache := &SyncFileCache{
			ParentId:   filepath.Dir(dir),
			FileName:   filepath.Base(dir),
			FileType:   v115open.TypeDir,
			IsVideo:    false,
			IsMeta:     false,
			Processed:  true,
			SourceType: models.SourceTypeOpenList,
		}
		syncFileCache.GetLocalFilePath(d.s.TargetPath, d.s.SourcePath)
		d.s.memSyncCache.Insert(syncFileCache)
		d.s.Sync.Logger.Infof("创建网盘目录: %s", dir)
	}
	return relPath, relPath, nil
}

func (d *openListDriver) GetPathIdByPath(ctx context.Context, path string) (string, error) {
	fsDetail, err := d.client.FileDetail(path)
	if err != nil || (fsDetail != nil && fsDetail.Name == "") {
		return "", fmt.Errorf("路径 %s 不存在: %v", path, err)
	}
	return path, nil
}

func (d *openListDriver) MakeStrmContent(sf *SyncFileCache) string {
	return helpers.MakeOpenListUrl(d.s.Account.BaseUrl, sf.OpenlistSign, sf.GetFileId())
}

func (d *openListDriver) GetTotalFileCount(ctx context.Context) (int64, string, error) {
	return 0, "", nil
}

func (d *openListDriver) GetDirsByPathId(ctx context.Context, pathId string) ([]pathQueueItem, error) {
	return nil, nil
}

func (d *openListDriver) GetFilesByPathId(ctx context.Context, rootPathId string, offset, limit int) ([]v115open.File, error) {
	return nil, nil
}

// 所有文件详情，含路径
func (d *openListDriver) DetailByFileId(ctx context.Context, fileId string) (*v115open.FileDetail, error) {
	return nil, nil
}
