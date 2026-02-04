package syncstrm

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (s *SyncStrm) checkPathExists(path string) bool {
	return helpers.PathExists(path)
}

// 获取本地根目录
func (s *SyncStrm) GetLocalBaseDir() string {
	fullPath := filepath.Join(s.TargetPath, s.SourcePath)
	fullPath = filepath.ToSlash(fullPath)
	if s.Account.SourceType == models.SourceTypeLocal {
		fullPath = s.TargetPath
	}
	return fullPath
}

func (s *SyncStrm) MakeFullLocalPath(file *models.SyncFile) string {
	// 视频文件要转成strm扩展名
	fileName := file.FileName
	if file.IsVideo {
		ext := filepath.Ext(fileName)
		baseName := strings.TrimSuffix(fileName, ext)
		fileName = baseName + ".strm"
	}
	fullPath := filepath.Join(s.TargetPath, file.Path, fileName)
	if s.Account.SourceType == models.SourceTypeLocal {
		// 本地不能拼接完整的file.Path
		relPath, err := filepath.Rel(s.SourcePath, file.Path)
		if err != nil {
			return ""
		}
		fullPath = filepath.Join(s.TargetPath, relPath, fileName)
	}
	// 将windows路径转换为unix路径
	fullPath = filepath.ToSlash(fullPath)
	return fullPath
}

func (s *SyncStrm) RemoveFileAndCheckDirEmtry(filePath string) error {
	// 删除文件
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("删除文件失败: %w", err)
	} else {
		s.Sync.Logger.Infof("删除文件成功: %s", filePath)
	}
	if !s.Config.DelEmptyLocalDir {
		return nil
	}
	// 检查目录是否为空
	dir := filepath.Dir(filePath)
	if entries, err := os.ReadDir(dir); err == nil && len(entries) == 0 {
		if err := os.Remove(dir); err != nil {
			return fmt.Errorf("删除空目录失败: %w", err)
		} else {
			s.Sync.Logger.Infof("删除空目录成功: %s", dir)
			// 删除网盘目录
			file, err := s.memSyncCache.GetByLocalPath(dir)
			if err != nil {
				s.Sync.Logger.Warnf("查询空目录对应的网盘记录失败:  %s %s", filePath, err.Error())
				return nil
			}
			// 从同步缓存中删除
			err = s.memSyncCache.DeleteByFileId(file.GetFileId())
			if err != nil {
				s.Sync.Logger.Warnf("删除空目录对应的网盘记录失败:  %s %s", file.GetFileId(), err.Error())
				return nil
			}
		}
	}
	return nil
}
