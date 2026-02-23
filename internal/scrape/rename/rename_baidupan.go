package rename

import (
	"Q115-STRM/internal/baidupan"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"context"
	"errors"
	"path/filepath"
	"strings"
)

type RenameBaiduPan struct {
	RenameBase
	client *baidupan.Client
}

func NewRenameBaiduPan(ctx context.Context, scrapePath *models.ScrapePath, client *baidupan.Client) *RenameBaiduPan {
	return &RenameBaiduPan{
		RenameBase: RenameBase{
			scrapePath: scrapePath,
			ctx:        ctx,
		},
		client: client,
	}
}

func (r *RenameBaiduPan) RenameAndMove(mediaFile *models.ScrapeMediaFile, destPath, destPathId, newName string) error {
	oldPath := mediaFile.PathId
	if oldPath == "" && mediaFile.MediaType == models.MediaTypeTvShow {
		oldPath = mediaFile.TvshowPathId
	}
	destName := filepath.ToSlash(filepath.Join(destPath, newName))
	destPath = filepath.ToSlash(destPath)
	destPathId = filepath.ToSlash(destPathId)
	fsDetail, _ := r.client.FileExists(r.ctx, destName)
	if fsDetail != nil && fsDetail.ServerFilename != "" {
		helpers.AppLogger.Infof("文件 %s 已存在，无需移动", destName)
		return nil
	}
	switch mediaFile.RenameType {
	case models.RenameTypeMove:
		err := r.move(mediaFile, newName, destPathId, oldPath)
		if err != nil {
			return err
		}
	case models.RenameTypeCopy:
		err := r.copy(mediaFile, newName, destPathId, oldPath)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *RenameBaiduPan) move(mediaFile *models.ScrapeMediaFile, newName, newPathId, oldPath string) error {
	helpers.AppLogger.Infof("百度网盘 准备将文件 %s 从 %s 移动到新文件夹 %s", newName, oldPath, newPathId)
	// 检查是否存在
	destFullPath := filepath.ToSlash(filepath.Join(newPathId, newName))
	fsDetail, _ := r.client.FileExists(r.ctx, destFullPath)
	fileList := make([]baidupan.MoveOrCopyItem, 0)
	if fsDetail != nil && fsDetail.ServerFilename != "" {
		helpers.AppLogger.Infof("文件 %s 已存在，无需移动", destFullPath)
	} else {
		fileList = append(fileList, baidupan.MoveOrCopyItem{
			Path:    filepath.ToSlash(filepath.Join(oldPath, mediaFile.VideoFilename)),
			Dest:    newPathId,
			NewName: newName,
		})
	}
	mediaFile.Media.VideoFileId = destFullPath
	mediaFile.Media.VideoPickCode = mediaFile.VideoPickCode
	oldBaseName := strings.TrimSuffix(mediaFile.VideoFilename, mediaFile.VideoExt)
	// 移动字幕文件到新目录
	if mediaFile.SubtitleFileJson != "" {
		if mediaFile.MediaType != models.MediaTypeTvShow {
			mediaFile.Media.SubtitleFiles = make([]*models.MediaMetaFiles, 0)
		} else {
			mediaFile.MediaEpisode.SubtitleFiles = make([]*models.MediaMetaFiles, 0)
		}
		for _, sub := range mediaFile.SubtitleFiles {
			newSubName := strings.Replace(sub.FileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
			fileList = append(fileList, baidupan.MoveOrCopyItem{
				Path:    filepath.ToSlash(filepath.Join(oldPath, sub.FileName)),
				Dest:    newPathId,
				NewName: newSubName,
			})
			if mediaFile.MediaType != models.MediaTypeTvShow {
				mediaFile.Media.SubtitleFiles = append(mediaFile.Media.SubtitleFiles, &models.MediaMetaFiles{
					FileName: newSubName,
					FileId:   filepath.ToSlash(filepath.Join(newPathId, newSubName)),
					PickCode: sub.PickCode,
				})
			} else {
				mediaFile.MediaEpisode.SubtitleFiles = append(mediaFile.MediaEpisode.SubtitleFiles, &models.MediaMetaFiles{
					FileName: newSubName,
					FileId:   filepath.ToSlash(filepath.Join(newPathId, newSubName)),
					PickCode: sub.PickCode,
				})
			}
		}
	}

	if mediaFile.MediaType != models.MediaTypeTvShow {
		// 保存
		mediaFile.Media.Save()
	} else {
		// 保存
		mediaFile.MediaEpisode.Save()
	}

	if mediaFile.ScrapeType == models.ScrapeTypeOnlyRename && mediaFile.MediaType == models.MediaTypeOther {
		// 其他类型仅整理要把图片和nfo也转移过去
		if mediaFile.ImageFilesJson != "" {
			for _, imageFile := range mediaFile.ImageFiles {
				newImageName := strings.Replace(imageFile.FileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
				fileList = append(fileList, baidupan.MoveOrCopyItem{
					Path:    filepath.ToSlash(filepath.Join(oldPath, imageFile.FileName)),
					Dest:    newPathId,
					NewName: newImageName,
				})
			}
		}
		// 移动nfo文件
		if mediaFile.NfoFileId != "" {
			newNfoName := strings.Replace(mediaFile.NfoFileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
			fileList = append(fileList, baidupan.MoveOrCopyItem{
				Path:    filepath.ToSlash(filepath.Join(oldPath, mediaFile.NfoFileName)),
				Dest:    newPathId,
				NewName: newNfoName,
			})
		}
	}
	// 移动文件
	err := r.client.MoveBatch(r.ctx, fileList)
	if err != nil {
		helpers.AppLogger.Errorf("百度网盘移动文件失败: %v", err)
		return err
	}
	return nil
}

func (r *RenameBaiduPan) copy(mediaFile *models.ScrapeMediaFile, newName, newPathId, oldPath string) error {
	fileList := make([]baidupan.MoveOrCopyItem, 0)
	fileList = append(fileList, baidupan.MoveOrCopyItem{
		Path:    filepath.ToSlash(filepath.Join(oldPath, mediaFile.VideoFilename)),
		Dest:    newPathId,
		NewName: newName,
	})
	oldBaseName := strings.TrimSuffix(mediaFile.VideoFilename, mediaFile.VideoExt)
	// 复制字幕文件到新目录
	if mediaFile.SubtitleFileJson != "" {
		if mediaFile.MediaType != models.MediaTypeTvShow {
			mediaFile.Media.SubtitleFiles = make([]*models.MediaMetaFiles, 0)
		} else {
			mediaFile.MediaEpisode.SubtitleFiles = make([]*models.MediaMetaFiles, 0)
		}
		for _, sub := range mediaFile.SubtitleFiles {
			newSubName := strings.Replace(sub.FileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
			fileList = append(fileList, baidupan.MoveOrCopyItem{
				Path:    filepath.ToSlash(filepath.Join(oldPath, sub.FileName)),
				Dest:    newPathId,
				NewName: newSubName,
			})
			if mediaFile.MediaType != models.MediaTypeTvShow {
				mediaFile.Media.SubtitleFiles = append(mediaFile.Media.SubtitleFiles, &models.MediaMetaFiles{
					FileName: newSubName,
					FileId:   filepath.ToSlash(filepath.Join(newPathId, newSubName)),
					PickCode: sub.PickCode,
				})
			} else {
				mediaFile.MediaEpisode.SubtitleFiles = append(mediaFile.MediaEpisode.SubtitleFiles, &models.MediaMetaFiles{
					FileName: newSubName,
					FileId:   filepath.ToSlash(filepath.Join(newPathId, newSubName)),
					PickCode: sub.PickCode,
				})
			}
		}
	}
	if mediaFile.ScrapeType == models.ScrapeTypeOnlyRename && mediaFile.MediaType == models.MediaTypeOther {
		// 其他类型仅整理要把图片和nfo也转移过去
		if mediaFile.ImageFilesJson != "" {
			for _, imageFile := range mediaFile.ImageFiles {
				newImageName := strings.Replace(imageFile.FileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
				fileList = append(fileList, baidupan.MoveOrCopyItem{
					Path:    filepath.Join(oldPath, imageFile.FileName),
					Dest:    newPathId,
					NewName: newImageName,
				})
			}
		}
		// 移动nfo文件
		if mediaFile.NfoFileId != "" {
			newNfoName := strings.Replace(mediaFile.NfoFileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
			fileList = append(fileList, baidupan.MoveOrCopyItem{
				Path:    filepath.Join(oldPath, mediaFile.NfoFileName),
				Dest:    newPathId,
				NewName: newNfoName,
			})
		}
	}
	// 复制文件
	err := r.client.CopyBatch(r.ctx, fileList)
	if err != nil {
		helpers.AppLogger.Errorf("百度网盘 复制文件失败: %v", err)
		return err
	}
	start := 0
	for {
		// 查询新的fsid
		fsList, err := r.client.GetFileList(r.ctx, newPathId, 1, 1, int32(start), 1000)
		if err != nil {
			helpers.AppLogger.Errorf("百度网盘 获取文件列表失败: id=%s %v", newPathId, err)
			return err
		}
		for _, file := range fsList {
			if file.ServerFilename == newName {
				if mediaFile.MediaType != models.MediaTypeTvShow {
					mediaFile.Media.VideoPickCode = helpers.Int64ToString(int64(file.FsId))
					mediaFile.Media.VideoFileId = filepath.ToSlash(filepath.Join(newPathId, newName))
				} else {
					mediaFile.MediaEpisode.VideoPickCode = helpers.Int64ToString(int64(file.FsId))
					mediaFile.MediaEpisode.VideoFileId = filepath.ToSlash(filepath.Join(newPathId, newName))
				}
				break
			}
			if mediaFile.MediaType != models.MediaTypeTvShow {
				for _, sub := range mediaFile.Media.SubtitleFiles {
					if sub.FileName == file.ServerFilename {
						sub.PickCode = helpers.Int64ToString(int64(file.FsId))
						break
					}
				}
			} else {
				for _, sub := range mediaFile.MediaEpisode.SubtitleFiles {
					if sub.FileName == file.ServerFilename {
						sub.PickCode = helpers.Int64ToString(int64(file.FsId))
						break
					}
				}
			}
		}
		if len(fsList) < 1000 {
			break
		}
		start += 1000
	}
	if mediaFile.MediaType != models.MediaTypeTvShow {
		// 保存
		mediaFile.Media.Save()
	} else {
		// 保存
		mediaFile.MediaEpisode.Save()
	}
	helpers.AppLogger.Infof("百度网盘 文件 %s 成功复制到 %s", oldPath+"/"+mediaFile.VideoFilename, newPathId+"/"+newName)
	return nil
}

func (r *RenameBaiduPan) CheckAndMkDir(destFullPath string, rootPath, rootPathId string) (string, error) {
	exists, err := r.client.PathExists(r.ctx, destFullPath)
	if err != nil || !exists {
		// 创建文件夹
		err = r.client.Mkdir(r.ctx, destFullPath)
		if err != nil {
			helpers.AppLogger.Errorf("11百度网盘 创建文件夹失败: %s 错误：%v", destFullPath, err)
			return destFullPath, err
		}
	}
	return destFullPath, nil
}

func (r *RenameBaiduPan) RemoveMediaSourcePath(mediaFile *models.ScrapeMediaFile, sp *models.ScrapePath) error {
	sourcePath := mediaFile.PathId
	if sourcePath == "" && mediaFile.MediaType == models.MediaTypeTvShow {
		sourcePath = mediaFile.TvshowPathId
	}
	fsList, err := r.client.GetFileList(r.ctx, sourcePath, 1, 1, 0, 1000)
	if err != nil {
		helpers.AppLogger.Errorf("百度网盘 获取文件列表失败: id=%s %v", mediaFile.PathId, err)
		return err
	}
	if len(fsList) > 0 {
		for _, file := range fsList {
			if sp.IsVideoFile(file.ServerFilename) {
				helpers.AppLogger.Infof("目录 %s 下有其他视频文件，不删除", mediaFile.PathId)
				return nil
			}
		}
	}
	if len(fsList) == 0 || sp.ForceDeleteSourcePath {
		if sourcePath == sp.SourcePath {
			helpers.AppLogger.Info("视频文件的父目录是来源根路径，不删除")
			return nil
		}
		// 删除目录
		err := r.client.Del(r.ctx, []string{sourcePath})
		if err != nil {
			helpers.AppLogger.Errorf("百度网盘 删除文件夹失败: %s %v", sourcePath, err)
			return err
		}
		helpers.AppLogger.Infof("刮削完成，尝试删除百度网盘文件夹成功, 路径：%s", sourcePath)
	}
	// 再删除电视剧文件夹
	if mediaFile.PathId != "" {
		// 检查电视剧文件夹的父目录是否是来源根路径
		tvshowParentId := mediaFile.TvshowPathId
		if tvshowParentId == sp.SourcePath {
			helpers.AppLogger.Info("电视剧的父目录是来源根路径，不删除")
			return nil
		}
		fsList, err := r.client.GetFileList(r.ctx, tvshowParentId, 1, 1, 0, 1000)
		if err != nil {
			helpers.AppLogger.Errorf("删除Openlist文件失败: %s %v", mediaFile.PathId, err)
			return err
		}
		if len(fsList) == 0 || sp.ForceDeleteSourcePath {
			// 删除目录
			err := r.client.Del(r.ctx, []string{tvshowParentId})
			if err != nil {
				helpers.AppLogger.Errorf("删除百度网盘电视剧文件夹失败: %s %v", mediaFile.TvshowPathId, err)
				return err
			}
			helpers.AppLogger.Infof("刮削完成，尝试删除百度网盘电视剧文件夹成功, 路径：%s", mediaFile.TvshowPathId)
		}

	}
	return nil
}

func (r *RenameBaiduPan) ReadFileContent(fileId string) ([]byte, error) {
	fsDetail, err := r.client.GetFileDetail(r.ctx, fileId, 1)
	if err != nil || fsDetail == nil || fsDetail.Dlink == "" {
		helpers.AppLogger.Errorf("获取百度网盘文件下载链接失败: fileId=%s, url为空", fileId)
		return nil, errors.New("获取百度网盘文件下载链接失败, url为空")
	}
	// 读取url的内容
	content, err := helpers.ReadFromUrl(fsDetail.Dlink, "pan.baidu.com")
	if err != nil {
		helpers.AppLogger.Errorf("百度网盘读取文件下载链接内容失败: fileId=%s, url=%s, %v", fileId, fsDetail.Dlink, err)
		return nil, err
	}
	return content, nil
}

func (r *RenameBaiduPan) CheckAndDeleteFiles(mediaFile *models.ScrapeMediaFile, files []models.WillDeleteFile) error {
	for _, f := range files {
		// 检查是否存在
		fsDetail, err := r.client.FileExists(r.ctx, f.FullFilePath)
		if err != nil || (fsDetail != nil && fsDetail.ServerFilename == "") {
			helpers.AppLogger.Infof("百度网盘 文件不存在，无需删除: 路径：%s", f.FullFilePath)
			continue
		}
		err = r.client.Del(r.ctx, []string{filepath.Dir(f.FullFilePath)})
		if err != nil {
			helpers.AppLogger.Errorf("删除百度网盘文件失败: 路径：%s %v", f.FullFilePath, err)
			continue
		}
		helpers.AppLogger.Infof("删除百度网盘文件成功, 路径：%s", f.FullFilePath)
	}
	return nil
}

func (r *RenameBaiduPan) MoveFiles(f models.MoveNewFileToSourceFile) error {
	// 检查旧文件是否存在
	newFileId := filepath.ToSlash(filepath.Join(f.PathId, filepath.Base(f.FileId)))
	fsDetail, err := r.client.FileExists(r.ctx, newFileId)
	if err == nil || (fsDetail != nil && fsDetail.ServerFilename != "") {
		helpers.AppLogger.Infof("百度网盘 文件存在，无需移动: 路径：%s", newFileId)
		return nil
	}
	// 移动文件
	err = r.client.Move(r.ctx, filepath.Dir(f.FileId), f.PathId, filepath.Base(f.FileId))
	if err != nil {
		helpers.AppLogger.Errorf("移动百度网盘文件失败: %s => %s 错误：%v", f.FileId, newFileId, err)
		return err
	}
	helpers.AppLogger.Infof("移动百度网盘文件成功: %s => %s", f.FileId, newFileId)
	return nil
}

func (r *RenameBaiduPan) DeleteDir(path, pathId string) error {
	return r.client.Del(r.ctx, []string{pathId})
}

func (r *RenameBaiduPan) Rename(fileId, newName string) error {
	return r.client.Rename(r.ctx, filepath.Dir(fileId), newName)
}

// 检查是否存在，存在就改名字，然后返回新的fileId
func (r *RenameBaiduPan) ExistsAndRename(fileId, newName string) (string, error) {
	// 检查是否存在
	fsDetail, err := r.client.FileExists(r.ctx, fileId)
	if err != nil || fsDetail == nil || fsDetail.ServerFilename == "" {
		helpers.AppLogger.Infof("百度网盘 文件不存在，无需重命名: 文件ID：%s", fileId)
		return "", nil
	}
	// 如果名字没变则不需要改名字
	if fsDetail.ServerFilename == newName {
		helpers.AppLogger.Infof("百度网盘 文件名字没变，无需重命名: 文件ID：%s", fileId)
		return fileId, nil
	}
	// 重命名文件
	err = r.Rename(fileId, newName)
	if err != nil {
		helpers.AppLogger.Errorf("重命名百度网盘文件失败：%s => %s 错误：%v", fileId, newName, err)
		return "", err
	}
	helpers.AppLogger.Infof("重命名百度网盘文件成功, %s => %s", fileId, newName)
	return filepath.ToSlash(filepath.Join(filepath.Dir(fileId), newName)), nil
}
