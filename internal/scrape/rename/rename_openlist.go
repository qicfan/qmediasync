package rename

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/openlist"
	"Q115-STRM/internal/v115open"
	"context"
	"errors"
	"path/filepath"
	"strings"
)

type RenameOpenList struct {
	RenameBase
	client *openlist.Client
}

func NewRenameOpenList(ctx context.Context, scrapePath *models.ScrapePath, client *openlist.Client) *RenameOpenList {
	return &RenameOpenList{
		RenameBase: RenameBase{
			scrapePath: scrapePath,
			ctx:        ctx,
		},
		client: client,
	}
}

func (r *RenameOpenList) RenameAndMove(mediaFile *models.ScrapeMediaFile, destPath, destPathId, newName string) error {
	oldPath := mediaFile.PathId
	if oldPath == "" && mediaFile.MediaType == models.MediaTypeTvShow {
		oldPath = mediaFile.TvshowPathId
	}
	destName := filepath.Join(destPath, newName)
	detail, _ := r.client.FileDetail(destName)
	if detail != nil && detail.Name != "" {
		helpers.AppLogger.Infof("文件 %s 已存在，无需移动", destName)
		return nil
	}
	if mediaFile.VideoFilename != newName {
		// 改名
		err := r.client.Rename(oldPath, mediaFile.VideoFilename, newName)
		if err != nil {
			helpers.AppLogger.Errorf("OpenList改名文件失败: %v", err)
			return err
		} else {
			helpers.AppLogger.Infof("文件 %s 重命名成功：%s", oldPath+"/"+mediaFile.VideoFilename, newName)
		}
	}
	// 先改名，后移动或复制
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

func (r *RenameOpenList) move(mediaFile *models.ScrapeMediaFile, newName, newPathId, oldPath string) error {
	helpers.AppLogger.Infof("OpenList 准备将文件 %s 从 %s 移动到新文件夹 %s", newName, oldPath, newPathId)
	// 检查是否存在
	destFullPath := filepath.Join(newPathId, newName)
	detail, _ := r.client.FileDetail(destFullPath)
	if detail != nil && detail.Name != "" {
		helpers.AppLogger.Infof("文件 %s 已存在，无需移动", destFullPath)
	} else {
		err := r.client.Move(oldPath, newPathId, []string{newName})
		if err != nil {
			helpers.AppLogger.Errorf("OpenList移动文件失败: %v", err)
			return err
		} else {
			helpers.AppLogger.Infof("OpenList 文件 %s 成功从 %s 移动到新文件夹 %s", newName, oldPath, newPathId)
		}
	}
	// 查询一下详情
	detail, _ = r.client.FileDetail(destFullPath)
	if detail == nil || detail.Name == "" {
		helpers.AppLogger.Errorf("OpenList 移动文件 %s 后，查询详情失败", destFullPath)
		return errors.New("移动文件后，查询详情失败")
	}
	if mediaFile.MediaType != models.MediaTypeTvShow {
		mediaFile.Media.VideoFileId = destFullPath
		mediaFile.Media.VideoPickCode = destFullPath
		mediaFile.Media.VideoOpenListSign = detail.Sign
	} else {
		mediaFile.MediaEpisode.VideoFileId = destFullPath
		mediaFile.MediaEpisode.VideoPickCode = destFullPath
		mediaFile.MediaEpisode.VideoOpenListSign = detail.Sign
	}
	oldBaseName := strings.TrimSuffix(mediaFile.VideoFilename, mediaFile.VideoExt)
	// 移动字幕文件到新目录
	if mediaFile.SubtitleFileJson != "" {
		if mediaFile.MediaType != models.MediaTypeTvShow {
			mediaFile.Media.SubtitleFiles = make([]*models.MediaMetaFiles, 0)
		} else {
			mediaFile.MediaEpisode.SubtitleFiles = make([]*models.MediaMetaFiles, 0)
		}
		files := []string{}
		for _, sub := range mediaFile.SubtitleFiles {
			files = append(files, sub.FileName)
			if mediaFile.MediaType != models.MediaTypeTvShow {
				mediaFile.Media.SubtitleFiles = append(mediaFile.Media.SubtitleFiles, &models.MediaMetaFiles{
					FileName: sub.FileName,
					FileId:   filepath.Join(newPathId, sub.FileName),
					PickCode: filepath.Join(newPathId, sub.FileName),
				})
			} else {
				mediaFile.MediaEpisode.SubtitleFiles = append(mediaFile.MediaEpisode.SubtitleFiles, &models.MediaMetaFiles{
					FileName: sub.FileName,
					FileId:   filepath.Join(newPathId, sub.FileName),
					PickCode: filepath.Join(newPathId, sub.FileName),
				})
			}
		}
		// 移动字幕文件到新目录
		err := r.client.Move(oldPath, newPathId, files)
		if err != nil {
			helpers.AppLogger.Errorf("OpenList移动字幕文件失败: %v", err)
		}
		// 改名
		for idx, sub := range mediaFile.SubtitleFiles {
			// 改名
			newSubName := strings.Replace(sub.FileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
			if newSubName != sub.FileName {
				// 改名
				err := r.client.Rename(newPathId, sub.FileName, newSubName)
				if err != nil {
					helpers.AppLogger.Errorf("OpenList改名字幕文件失败: %v", err)
				} else {
					helpers.AppLogger.Infof("字幕文件 %s 重命名成功：%s", newPathId+"/"+sub.FileName, newSubName)
					mediaFile.Media.SubtitleFiles[idx].FileName = newSubName
					mediaFile.Media.SubtitleFiles[idx].FileId = filepath.Join(newPathId, newSubName)
					mediaFile.Media.SubtitleFiles[idx].PickCode = filepath.Join(newPathId, newSubName)
				}
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
			files := []string{}
			for _, imageFile := range mediaFile.ImageFiles {
				files = append(files, imageFile.FileName)
			}
			// 移动图片文件到新目录
			err := r.client.Move(oldPath, newPathId, files)
			if err != nil {
				helpers.AppLogger.Errorf("OpenList移动图片文件失败: %v", err)
				return err
			}
			// 改名
			for _, imageFile := range mediaFile.ImageFiles {
				// 改名
				newImageName := strings.Replace(imageFile.FileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
				if newImageName != imageFile.FileName {
					// 改名
					err := r.client.Rename(newPathId, imageFile.FileName, newImageName)
					if err != nil {
						helpers.AppLogger.Errorf("OpenList改图片文件名失败: %v", err)
						return err
					} else {
						helpers.AppLogger.Infof("图片文件 %s 重命名成功：%s", newPathId+"/"+imageFile.FileName, newImageName)
					}
				}
			}
		}
		// 移动nfo文件
		if mediaFile.NfoFileId != "" {
			newNfoName := strings.Replace(mediaFile.NfoFileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
			// 移动nfo文件到新目录
			err := r.client.Move(oldPath, newPathId, []string{mediaFile.NfoFileName})
			if err != nil {
				helpers.AppLogger.Errorf("OpenList移动nfo文件 %s 失败: %v", mediaFile.NfoFileName, err)
			}
			// 检查是否需要改名
			if newNfoName != mediaFile.NfoFileName {
				// 改名
				err := r.client.Rename(newPathId, mediaFile.NfoFileName, newNfoName)
				if err != nil {
					helpers.AppLogger.Errorf("OpenList改名nfo文件 %s 失败: %v", mediaFile.NfoFileName, err)
				} else {
					helpers.AppLogger.Infof("nfo文件 %s 成功重命名为 %s", mediaFile.NfoFileName, newNfoName)
				}
			}
		}
	}
	return nil
}

func (r *RenameOpenList) copy(mediaFile *models.ScrapeMediaFile, newName, newPathId, oldPath string) error {
	err := r.client.Copy(oldPath, newPathId, []string{newName})
	if err != nil {
		helpers.AppLogger.Errorf("OpenList复制文件失败: %v", err)
		return err
	} else {
		helpers.AppLogger.Infof("Openlist 文件 %s 成功复制到 %s", oldPath+"/"+newName, newPathId+"/"+newName)
	}
	destFullPath := filepath.ToSlash(filepath.Join(newPathId, newName))
	// 查询一下详情
	detail, _ := r.client.FileDetail(filepath.ToSlash(filepath.Join(newPathId, newName)))
	if detail == nil || detail.Name == "" {
		helpers.AppLogger.Errorf("OpenList 复制文件 %s 后，查询详情失败", destFullPath)
		return errors.New("复制文件后，查询详情失败")
	}
	if mediaFile.MediaType != models.MediaTypeTvShow {
		mediaFile.Media.VideoFileId = destFullPath
		mediaFile.Media.VideoPickCode = destFullPath
		mediaFile.Media.VideoOpenListSign = detail.Sign
	} else {
		mediaFile.MediaEpisode.VideoFileId = destFullPath
		mediaFile.MediaEpisode.VideoPickCode = destFullPath
		mediaFile.MediaEpisode.VideoOpenListSign = detail.Sign
	}
	oldBaseName := strings.TrimSuffix(mediaFile.VideoFilename, mediaFile.VideoExt)
	// 复制字幕文件到新目录
	if mediaFile.SubtitleFileJson != "" {
		if mediaFile.MediaType != models.MediaTypeTvShow {
			mediaFile.Media.SubtitleFiles = make([]*models.MediaMetaFiles, 0)
		} else {
			mediaFile.MediaEpisode.SubtitleFiles = make([]*models.MediaMetaFiles, 0)
		}
		files := []string{}
		for _, sub := range mediaFile.SubtitleFiles {
			files = append(files, sub.FileName)
			if mediaFile.MediaType != models.MediaTypeTvShow {
				mediaFile.Media.SubtitleFiles = append(mediaFile.Media.SubtitleFiles, &models.MediaMetaFiles{
					FileName: sub.FileName,
					FileId:   filepath.Join(newPathId, sub.FileName),
					PickCode: filepath.Join(newPathId, sub.FileName),
				})
			} else {
				mediaFile.MediaEpisode.SubtitleFiles = append(mediaFile.MediaEpisode.SubtitleFiles, &models.MediaMetaFiles{
					FileName: sub.FileName,
					FileId:   filepath.Join(newPathId, sub.FileName),
					PickCode: filepath.Join(newPathId, sub.FileName),
				})
			}
		}
		// 移动字幕文件到新目录
		err := r.client.Copy(oldPath, newPathId, files)
		if err != nil {
			helpers.AppLogger.Errorf("OpenList复制字幕文件失败: %v", err)
		}
		// 改名
		for _, sub := range mediaFile.SubtitleFiles {
			// 改名
			newSubName := strings.Replace(sub.FileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
			if newSubName != sub.FileName {
				// 改名
				err := r.client.Rename(newPathId, sub.FileName, newSubName)
				if err != nil {
					helpers.AppLogger.Errorf("OpenList改名字幕文件失败: %v", err)
				} else {
					helpers.AppLogger.Infof("字幕文件 %s 重命名成功：%s", newPathId+"/"+sub.FileName, newSubName)
				}
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
			files := []string{}
			for _, imageFile := range mediaFile.ImageFiles {
				files = append(files, imageFile.FileName)
			}
			// 移动图片文件到新目录
			err := r.client.Copy(oldPath, newPathId, files)
			if err != nil {
				helpers.AppLogger.Errorf("OpenList复制图片文件失败: %v", err)
				return err
			}
			// 改名
			for _, imageFile := range mediaFile.ImageFiles {
				// 改名
				newImageName := strings.Replace(imageFile.FileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
				if newImageName != imageFile.FileName {
					// 改名
					err := r.client.Rename(newPathId, imageFile.FileName, newImageName)
					if err != nil {
						helpers.AppLogger.Errorf("OpenList改图片文件名失败: %v", err)
						return err
					} else {
						helpers.AppLogger.Infof("图片文件 %s 重命名成功：%s", newPathId+"/"+imageFile.FileName, newImageName)
					}
				}
			}
		}
		// 移动nfo文件
		if mediaFile.NfoFileId != "" {
			newNfoName := strings.Replace(mediaFile.NfoFileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
			// 移动nfo文件到新目录
			err := r.client.Copy(oldPath, newPathId, []string{mediaFile.NfoFileName})
			if err != nil {
				helpers.AppLogger.Errorf("OpenList复制nfo文件 %s 失败: %v", mediaFile.NfoFileName, err)
			}
			// 检查是否需要改名
			if newNfoName != mediaFile.NfoFileName {
				// 改名
				err := r.client.Rename(newPathId, mediaFile.NfoFileName, newNfoName)
				if err != nil {
					helpers.AppLogger.Errorf("OpenList改名nfo文件 %s 失败: %v", mediaFile.NfoFileName, err)
				} else {
					helpers.AppLogger.Infof("nfo文件 %s 成功重命名为 %s", mediaFile.NfoFileName, newNfoName)
				}
			}
		}
	}
	return nil
}

func (r *RenameOpenList) CheckAndMkDir(destFullPath string, rootPath, rootPathId string) (string, error) {
	fsDetail, err := r.client.FileDetail(destFullPath)
	if err != nil || (fsDetail != nil && fsDetail.Name == "") {
		// 创建文件夹
		err = r.client.Mkdir(destFullPath)
		if err != nil {
			helpers.AppLogger.Errorf("创建文件夹失败: %s 错误：%v", destFullPath, err)
			return destFullPath, err
		}
	}
	return destFullPath, nil
}

func (r *RenameOpenList) RemoveMediaSourcePath(mediaFile *models.ScrapeMediaFile, sp *models.ScrapePath) error {
	sourcePath := mediaFile.PathId
	if sourcePath == "" && mediaFile.MediaType == models.MediaTypeTvShow {
		sourcePath = mediaFile.TvshowPathId
	}
	fsDetail, err := r.client.FileList(r.ctx, sourcePath, 1, 10)
	if err != nil {
		helpers.AppLogger.Errorf("获取OpenList文件列表失败: id=%s %v", mediaFile.PathId, err)
		return err
	}
	if fsDetail.Total > 0 {
		for _, file := range fsDetail.Content {
			if sp.IsVideoFile(file.Name) {
				helpers.AppLogger.Infof("目录 %s 下有其他视频文件，不删除", mediaFile.PathId)
				return nil
			}
		}
	}
	if fsDetail.Total == 0 || sp.ForceDeleteSourcePath {
		if sourcePath == sp.SourcePath {
			helpers.AppLogger.Info("视频文件的父目录是来源根路径，不删除")
			return nil
		}
		// 删除目录
		err := r.client.Del(filepath.Dir(sourcePath), []string{filepath.Base(sourcePath)})
		if err != nil {
			helpers.AppLogger.Errorf("删除Openlist文件失败: %s %v", sourcePath, err)
			return err
		}
		helpers.AppLogger.Infof("刮削完成，尝试删除Openlist文件夹成功, 路径：%s", sourcePath)
	}
	// 再删除电视剧文件夹
	if mediaFile.PathId != "" {
		// 检查电视剧文件夹的父目录是否是来源根路径
		tvshowParentId := mediaFile.TvshowPathId
		if tvshowParentId == sp.SourcePath {
			helpers.AppLogger.Info("电视剧的父目录是来源根路径，不删除")
			return nil
		}
		fsDetail, err := r.client.FileList(r.ctx, tvshowParentId, 1, 10)
		if err != nil {
			helpers.AppLogger.Errorf("删除Openlist文件失败: %s %v", mediaFile.PathId, err)
			return err
		}
		if fsDetail.Total == 0 || sp.ForceDeleteSourcePath {
			// 删除目录
			err := r.client.Del(tvshowParentId, []string{filepath.Base(tvshowParentId)})
			if err != nil {
				helpers.AppLogger.Errorf("删除Openlist文件失败: %s %v", mediaFile.TvshowPathId, err)
				return err
			}
			helpers.AppLogger.Infof("刮削完成，尝试删除Openlist中的电视剧文件夹成功, 路径：%s", mediaFile.TvshowPathId)
		}

	}
	return nil
}

func (r *RenameOpenList) ReadFileContent(fileId string) ([]byte, error) {
	url := r.client.GetRawUrl(fileId)
	if url == "" {
		helpers.AppLogger.Errorf("获取openlist文件下载链接失败: fileId=%s, url为空", fileId)
		return nil, errors.New("获取openlist文件下载链接失败, url为空")
	}
	// 读取url的内容
	content, err := helpers.ReadFromUrl(url, v115open.DEFAULTUA)
	if err != nil {
		helpers.AppLogger.Errorf("openlist读取文件下载链接内容失败: fileId=%s, url=%s, %v", fileId, url, err)
		return nil, err
	}
	return content, nil
}

func (r *RenameOpenList) CheckAndDeleteFiles(mediaFile *models.ScrapeMediaFile, files []models.WillDeleteFile) error {
	for _, f := range files {
		// 检查是否存在
		fsDetail, err := r.client.FileDetail(f.FullFilePath)
		if err != nil || (fsDetail != nil && fsDetail.Name == "") {
			helpers.AppLogger.Infof("OpenList文件不存在，无需删除: 路径：%s", f.FullFilePath)
			continue
		}
		err = r.client.Del(filepath.Dir(f.FullFilePath), []string{filepath.Base(f.FullFilePath)})
		if err != nil {
			helpers.AppLogger.Errorf("删除OpenList文件失败: 路径：%s %v", f.FullFilePath, err)
			continue
		}
		helpers.AppLogger.Infof("删除OpenList文件成功, 路径：%s", f.FullFilePath)
	}
	return nil
}

func (r *RenameOpenList) MoveFiles(f models.MoveNewFileToSourceFile) error {
	// 检查旧文件是否存在
	newFileId := filepath.Join(f.PathId, filepath.Base(f.FileId))
	fsDetail, err := r.client.FileDetail(newFileId)
	if err == nil || (fsDetail != nil && fsDetail.Name != "") {
		helpers.AppLogger.Infof("OpenList文件存在，无需移动: 路径：%s", newFileId)
		return nil
	}
	// 移动文件
	err = r.client.Move(filepath.Dir(f.FileId), f.PathId, []string{filepath.Base(f.FileId)})
	if err != nil {
		helpers.AppLogger.Errorf("移动OpenList文件失败: %s => %s 错误：%v", f.FileId, newFileId, err)
		return err
	}
	helpers.AppLogger.Infof("移动OpenList文件成功: %s => %s", f.FileId, newFileId)
	return nil
}

func (r *RenameOpenList) DeleteDir(path, pathId string) error {
	return r.client.Del(filepath.Dir(pathId), []string{filepath.Base(pathId)})
}

func (r *RenameOpenList) Rename(fileId, newName string) error {
	return r.client.Rename(filepath.Dir(fileId), filepath.Base(fileId), newName)
}

// 检查是否存在，存在就改名字，然后返回新的fileId
func (r *RenameOpenList) ExistsAndRename(fileId, newName string) (string, error) {
	// 检查是否存在
	fsDetail, err := r.client.FileDetail(fileId)
	if err != nil || (fsDetail != nil && fsDetail.Name == "") {
		helpers.AppLogger.Infof("OpenList文件不存在，无需重命名: 文件ID：%s", fileId)
		return "", nil
	}
	// 如果名字没变则不需要改名字
	if fsDetail.Name == newName {
		helpers.AppLogger.Infof("OpenList文件名字没变，无需重命名: 文件ID：%s", fileId)
		return fileId, nil
	}
	// 重命名文件
	err = r.Rename(fileId, newName)
	if err != nil {
		helpers.AppLogger.Errorf("重命名OpenList文件失败：%s => %s 错误：%v", fileId, newName, err)
		return "", err
	}
	helpers.AppLogger.Infof("重命名OpenList文件成功, %s => %s", fileId, newName)
	return filepath.Join(filepath.Dir(fileId), newName), nil
}
