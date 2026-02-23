package rename

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"context"
	"os"
	"path/filepath"
	"strings"
)

type RenameLocal struct {
	RenameBase
}

func NewRenameLocal(ctx context.Context, scrapePath *models.ScrapePath) *RenameLocal {
	return &RenameLocal{
		RenameBase: RenameBase{
			scrapePath: scrapePath,
			ctx:        ctx,
		},
	}
}

func (r *RenameLocal) RenameAndMove(mediaFile *models.ScrapeMediaFile, destPath, destPathId, newName string) error {
	if helpers.PathExists(destPathId + "/" + newName) {
		helpers.AppLogger.Infof("文件 %s 已存在，无需移动", newName)
		return nil
	}
	switch mediaFile.RenameType {
	case models.RenameTypeCopy:
		r.copy(mediaFile, destPathId, newName)
	case models.RenameTypeMove:
		r.move(mediaFile, destPathId, newName)
	case models.RenameTypeHardSymlink:
		r.symlink(mediaFile, destPathId, newName, true)
	case models.RenameTypeSoftSymlink:
		r.symlink(mediaFile, destPathId, newName, false)
	}
	return nil
}

func (r *RenameLocal) move(mediaFile *models.ScrapeMediaFile, destPathId, newName string) error {
	// 将视频文件复制到目标位置
	sourcePath := mediaFile.PathId
	if sourcePath == "" && mediaFile.MediaType == models.MediaTypeTvShow {
		sourcePath = mediaFile.TvshowPathId
	}
	sourceFullPath := filepath.Join(sourcePath, mediaFile.VideoFilename)
	destFullPath := filepath.Join(destPathId, newName)
	if helpers.PathExists(destFullPath) {
		helpers.AppLogger.Infof("文件 %s 已存在，无需移动", destFullPath)
	} else {
		err := helpers.MoveFile(sourceFullPath, destFullPath, false)
		if err != nil {
			helpers.AppLogger.Errorf("移动文件失败: %v", err)
			return err
		} else {
			helpers.AppLogger.Infof("文件 %s 成功移动到 %s", sourcePath+"/"+mediaFile.VideoFilename, destPathId+"/"+newName)
		}
	}
	if mediaFile.MediaType != models.MediaTypeTvShow {
		mediaFile.Media.VideoFileId = destFullPath
		mediaFile.Media.VideoPickCode = destFullPath
	} else {
		mediaFile.MediaEpisode.VideoFileId = destFullPath
		mediaFile.MediaEpisode.VideoPickCode = destFullPath
	}
	oldBaseName := strings.TrimSuffix(mediaFile.VideoFilename, mediaFile.VideoExt)
	// 移动字幕文件到新目录
	if mediaFile.SubtitleFileJson != "" {
		if mediaFile.MediaType != models.MediaTypeTvShow {
			mediaFile.Media.SubtitleFiles = make([]*models.MediaMetaFiles, 0)
		} else {
			mediaFile.MediaEpisode.SubtitleFiles = make([]*models.MediaMetaFiles, 0)
		}
		for _, sub := range mediaFile.SubtitleFiles {
			// 改名+移动
			newSubName := strings.Replace(sub.FileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
			newSubFullPath := filepath.Join(destPathId, newSubName)
			err := helpers.MoveFile(sub.FileId, newSubFullPath, false)
			if err != nil {
				helpers.AppLogger.Errorf("移动字幕文件 %s 到 %s 失败: %v", sub.FileName, destPathId+"/"+newSubName, err)
			} else {
				helpers.AppLogger.Infof("字幕文件 %s 成功移动到 %s", sub.FileId, destPathId+"/"+newSubName)
				newSub := &models.MediaMetaFiles{
					FileName: newSubName,
					FileId:   newSubFullPath,
					PickCode: newSubFullPath,
				}
				if mediaFile.MediaType != models.MediaTypeTvShow {
					mediaFile.Media.SubtitleFiles = append(mediaFile.Media.SubtitleFiles, newSub)
				} else {
					mediaFile.MediaEpisode.SubtitleFiles = append(mediaFile.MediaEpisode.SubtitleFiles, newSub)
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
			for _, imageFile := range mediaFile.ImageFiles {
				// 改名+移动
				newImageName := strings.Replace(imageFile.FileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
				err := helpers.MoveFile(imageFile.FileId, filepath.Join(destPathId, newImageName), false)
				if err != nil {
					helpers.AppLogger.Errorf("移动图片文件 %s 到 %s 失败: %v", imageFile.FileName, destPathId+"/"+newImageName, err)
				} else {
					helpers.AppLogger.Infof("图片文件 %s 成功移动到 %s", imageFile.FileId, destPathId+"/"+newImageName)
				}
			}
		}
		// 移动nfo文件
		if mediaFile.NfoFileId != "" {
			newNfoName := strings.Replace(mediaFile.NfoFileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
			// 移动nfo文件到新目录
			err := helpers.MoveFile(mediaFile.NfoFileId, filepath.Join(destPathId, newNfoName), false)
			if err != nil {
				helpers.AppLogger.Errorf("移动nfo文件 %s 到 %s 失败: %v", mediaFile.NfoFileName, destPathId+"/"+newNfoName, err)
			} else {
				helpers.AppLogger.Infof("nfo文件 %s 成功移动到 %s", mediaFile.NfoFileId, destPathId+"/"+newNfoName)
			}
		}
	}
	return nil
}

func (r *RenameLocal) copy(mediaFile *models.ScrapeMediaFile, destPathId, newName string) error {
	sourcePath := mediaFile.PathId
	if sourcePath == "" && mediaFile.MediaType == models.MediaTypeTvShow {
		sourcePath = mediaFile.TvshowPathId
	}
	sourceFullPath := filepath.Join(sourcePath, mediaFile.VideoFilename)
	destFullPath := filepath.Join(destPathId, newName)
	if helpers.PathExists(destFullPath) {
		helpers.AppLogger.Infof("文件 %s 已存在，无需移动", destFullPath)
	} else {
		err := helpers.CopyFile(sourceFullPath, destFullPath)
		if err != nil {
			helpers.AppLogger.Errorf("移动文件失败: %v", err)
			return err
		} else {
			helpers.AppLogger.Infof("文件 %s 成功移动到 %s", sourcePath+"/"+mediaFile.VideoFilename, destPathId+"/"+newName)
		}
	}
	if mediaFile.MediaType != models.MediaTypeTvShow {
		mediaFile.Media.VideoFileId = destFullPath
		mediaFile.Media.VideoPickCode = destFullPath
	} else {
		mediaFile.MediaEpisode.VideoFileId = destFullPath
		mediaFile.MediaEpisode.VideoPickCode = destFullPath
	}
	oldBaseName := strings.TrimSuffix(mediaFile.VideoFilename, mediaFile.VideoExt)
	// 移动字幕文件到新目录
	if mediaFile.SubtitleFileJson != "" {
		if mediaFile.MediaType != models.MediaTypeTvShow {
			mediaFile.Media.SubtitleFiles = make([]*models.MediaMetaFiles, 0)
		} else {
			mediaFile.MediaEpisode.SubtitleFiles = make([]*models.MediaMetaFiles, 0)
		}
		for _, sub := range mediaFile.SubtitleFiles {
			// 改名+移动
			newSubName := strings.Replace(sub.FileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
			newSubFullPath := filepath.Join(destPathId, newSubName)
			err := helpers.CopyFile(sub.FileId, newSubFullPath)
			if err != nil {
				helpers.AppLogger.Errorf("移动字幕文件 %s 到 %s 失败: %v", sub.FileName, destPathId+"/"+newSubName, err)
			} else {
				helpers.AppLogger.Infof("字幕文件 %s 成功移动到 %s", sub.FileId, destPathId+"/"+newSubName)
				newSub := &models.MediaMetaFiles{
					FileName: newSubName,
					FileId:   newSubFullPath,
					PickCode: newSubFullPath,
				}
				if mediaFile.MediaType != models.MediaTypeTvShow {
					mediaFile.Media.SubtitleFiles = append(mediaFile.Media.SubtitleFiles, newSub)
				} else {
					mediaFile.MediaEpisode.SubtitleFiles = append(mediaFile.MediaEpisode.SubtitleFiles, newSub)
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
			for _, imageFile := range mediaFile.ImageFiles {
				// 改名+移动
				newImageName := strings.Replace(imageFile.FileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
				err := helpers.CopyFile(imageFile.FileId, filepath.Join(destPathId, newImageName))
				if err != nil {
					helpers.AppLogger.Errorf("移动图片文件 %s 到 %s 失败: %v", imageFile.FileName, destPathId+"/"+newImageName, err)
				} else {
					helpers.AppLogger.Infof("图片文件 %s 成功移动到 %s", imageFile.FileId, destPathId+"/"+newImageName)
				}
			}
		}
		// 移动nfo文件
		if mediaFile.NfoFileId != "" {
			newNfoName := strings.Replace(mediaFile.NfoFileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
			// 移动nfo文件到新目录
			err := helpers.CopyFile(mediaFile.NfoFileId, filepath.Join(destPathId, newNfoName))
			if err != nil {
				helpers.AppLogger.Errorf("移动nfo文件 %s 到 %s 失败: %v", mediaFile.NfoFileName, destPathId+"/"+newNfoName, err)
			} else {
				helpers.AppLogger.Infof("nfo文件 %s 成功移动到 %s", mediaFile.NfoFileId, destPathId+"/"+newNfoName)
			}
		}
	}
	return nil
}

func (r *RenameLocal) symlink(mediaFile *models.ScrapeMediaFile, destPathId, newName string, isHard bool) error {
	sourcePath := mediaFile.PathId
	if sourcePath == "" && mediaFile.MediaType == models.MediaTypeTvShow {
		sourcePath = mediaFile.TvshowPathId
	}
	sourceFullPath := filepath.Join(sourcePath, mediaFile.VideoFilename)
	destFullPath := filepath.Join(destPathId, newName)
	if helpers.PathExists(destFullPath) {
		helpers.AppLogger.Infof("文件 %s 已存在，无需硬链接", destFullPath)
	} else {
		var err error
		if isHard {
			err = os.Link(sourceFullPath, destFullPath)
		} else {
			err = os.Symlink(sourceFullPath, destFullPath)
		}
		if err != nil {
			helpers.AppLogger.Errorf("创建硬链接失败: %v", err)
			return err
		} else {
			helpers.AppLogger.Infof("文件 %s 成功链接到 %s", sourceFullPath, destFullPath)
		}
	}
	if mediaFile.MediaType != models.MediaTypeTvShow {
		mediaFile.Media.VideoFileId = destFullPath
		mediaFile.Media.VideoPickCode = destFullPath
	} else {
		mediaFile.MediaEpisode.VideoFileId = destFullPath
		mediaFile.MediaEpisode.VideoPickCode = destFullPath
	}
	oldBaseName := strings.TrimSuffix(mediaFile.VideoFilename, mediaFile.VideoExt)
	// 移动字幕文件到新目录
	if mediaFile.SubtitleFileJson != "" {
		if mediaFile.MediaType != models.MediaTypeTvShow {
			mediaFile.Media.SubtitleFiles = make([]*models.MediaMetaFiles, 0)
		} else {
			mediaFile.MediaEpisode.SubtitleFiles = make([]*models.MediaMetaFiles, 0)
		}
		for _, sub := range mediaFile.SubtitleFiles {
			// 改名+移动
			newSubName := strings.Replace(sub.FileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
			var err error
			if isHard {
				err = os.Link(sub.FileId, filepath.Join(destPathId, newSubName))
			} else {
				err = os.Symlink(sub.FileId, filepath.Join(destPathId, newSubName))
			}
			if err != nil {
				helpers.AppLogger.Errorf("创建硬链接字幕文件 %s 到 %s 失败: %v", sub.FileName, destPathId+"/"+newSubName, err)
			} else {
				helpers.AppLogger.Infof("字幕文件 %s 成功链接到 %s", sub.FileId, destPathId+"/"+newSubName)
				newSub := &models.MediaMetaFiles{
					FileName: newSubName,
					FileId:   filepath.Join(destPathId, newSubName),
					PickCode: filepath.Join(destPathId, newSubName),
				}
				if mediaFile.MediaType != models.MediaTypeTvShow {
					mediaFile.Media.SubtitleFiles = append(mediaFile.Media.SubtitleFiles, newSub)
				} else {
					mediaFile.MediaEpisode.SubtitleFiles = append(mediaFile.MediaEpisode.SubtitleFiles, newSub)
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
			for _, imageFile := range mediaFile.ImageFiles {
				// 改名+移动
				newImageName := strings.Replace(imageFile.FileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
				var err error
				if isHard {
					err = os.Link(imageFile.FileId, filepath.Join(destPathId, newImageName))
				} else {
					err = os.Symlink(imageFile.FileId, filepath.Join(destPathId, newImageName))
				}
				if err != nil {
					helpers.AppLogger.Errorf("创建硬链接图片文件 %s 到 %s 失败: %v", imageFile.FileName, destPathId+"/"+newImageName, err)
				} else {
					helpers.AppLogger.Infof("图片文件 %s 成功硬链接到 %s", imageFile.FileId, destPathId+"/"+newImageName)
				}
			}
		}
		// 移动nfo文件
		if mediaFile.NfoFileId != "" {
			newNfoName := strings.Replace(mediaFile.NfoFileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
			// 移动nfo文件到新目录
			var err error
			if isHard {
				err = os.Link(mediaFile.NfoFileId, filepath.Join(destPathId, newNfoName))
			} else {
				err = os.Symlink(mediaFile.NfoFileId, filepath.Join(destPathId, newNfoName))
			}
			if err != nil {
				helpers.AppLogger.Errorf("创建硬链接nfo文件 %s 到 %s 失败: %v", mediaFile.NfoFileName, destPathId+"/"+newNfoName, err)
			} else {
				helpers.AppLogger.Infof("nfo文件 %s 成功硬链接到 %s", mediaFile.NfoFileId, destPathId+"/"+newNfoName)
			}
		}
	}
	return nil
}

func (r *RenameLocal) CheckAndMkDir(destFullPath string, rootPath, rootPathId string) (string, error) {
	if !helpers.PathExists(destFullPath) {
		err := os.MkdirAll(destFullPath, 0777)
		if err != nil {
			helpers.AppLogger.Errorf("创建父文件夹失败: %v", err)
			return "", err
		}
	}
	return destFullPath, nil
}

func (r *RenameLocal) RemoveMediaSourcePath(mediaFile *models.ScrapeMediaFile, sp *models.ScrapePath) error {
	sourcePath := mediaFile.PathId
	if sourcePath == "" && mediaFile.MediaType == models.MediaTypeTvShow {
		sourcePath = mediaFile.TvshowPathId
	}
	if sourcePath == sp.SourcePath {
		helpers.AppLogger.Info("视频文件的父目录是来源根路径，不删除")
		return nil
	}
	// 判断这个目录下是否没有任何其他文件或目录
	dirEntries, _ := os.ReadDir(sourcePath)
	if len(dirEntries) > 0 {
		// 检查目录下是否有其他视频文件，如果有则跳过
		for _, entry := range dirEntries {
			if sp.IsVideoFile(entry.Name()) {
				helpers.AppLogger.Infof("目录 %s 下有其他视频文件，不删除", sourcePath)
				return nil
			}
		}
	}
	if len(dirEntries) == 0 || sp.ForceDeleteSourcePath {
		// 删除本地目录
		err := os.RemoveAll(sourcePath)
		if err != nil {
			helpers.AppLogger.Errorf("删除本地目录失败: %s %v", sourcePath, err)
			return err
		}
		helpers.AppLogger.Infof("刮削完成，尝试删除本地目录成功, 路径：%s", sourcePath)
	}
	// 如果有电视剧文件夹，则删除
	if mediaFile.PathId != "" {
		// 检查电视剧文件夹的父目录是否是来源根路径
		tvshowParentId := mediaFile.TvshowPathId
		if tvshowParentId == sp.SourcePathId {
			helpers.AppLogger.Info("电视剧的父目录是来源根路径，不删除")
			return nil
		}
		// 判断这个目录下是否没有任何其他文件或目录
		dirEntries, _ := os.ReadDir(tvshowParentId)
		if len(dirEntries) == 0 || sp.ForceDeleteSourcePath {
			err := os.RemoveAll(mediaFile.TvshowPathId)
			if err != nil {
				helpers.AppLogger.Errorf("删除电视剧文件夹失败: %s %v", mediaFile.TvshowPathId, err)
				return err
			}
			helpers.AppLogger.Infof("刮削完成，尝试删除本地电视剧文件夹成功, 路径：%s", mediaFile.TvshowPathId)
		}
	}
	return nil
}

func (r *RenameLocal) ReadFileContent(fileId string) ([]byte, error) {
	// 读取文件内容并返回
	content, err := os.ReadFile(fileId)
	if err != nil {
		helpers.AppLogger.Errorf("读取文件内容失败: %s %v", fileId, err)
		return nil, err
	}
	return content, nil
}

func (r *RenameLocal) CheckAndDeleteFiles(mediaFile *models.ScrapeMediaFile, files []models.WillDeleteFile) error {
	for _, f := range files {
		// 检查是否存在
		if !helpers.PathExists(f.FullFilePath) {
			helpers.AppLogger.Infof("本地文件不存在，无需删除: 路径：%s", f.FullFilePath)
			continue
		}
		err := os.Remove(f.FullFilePath)
		if err != nil {
			helpers.AppLogger.Errorf("删除本地文件失败: 路径：%s %v", f.FullFilePath, err)
			continue
		}
		helpers.AppLogger.Infof("删除本地文件成功, 路径：%s", f.FullFilePath)
	}
	return nil
}

func (r *RenameLocal) MoveFiles(f models.MoveNewFileToSourceFile) error {
	newFileId := filepath.Join(f.PathId, filepath.Base(f.FileId))
	if helpers.PathExists(newFileId) {
		helpers.AppLogger.Infof("目标文件已存在，无需移动: 路径：%s", newFileId)
		return nil
	}
	// 移动文件到旧目录
	err := helpers.MoveFile(f.FileId, newFileId, true)
	if err != nil {
		helpers.AppLogger.Errorf("移动本地文件失败: %s => %s 错误:%v", f.FileId, newFileId, err)
		return err
	}
	helpers.AppLogger.Infof("移动本地文件成功: %s => %s", f.FileId, newFileId)
	return nil
}

func (r *RenameLocal) DeleteDir(path, pathId string) error {
	return os.RemoveAll(pathId)
}

func (r *RenameLocal) Rename(fileId, newName string) error {
	return helpers.MoveFile(fileId, filepath.Join(filepath.Dir(fileId), newName), false)
}

// 检查是否存在，存在就改名字，然后返回新的fileId
func (r *RenameLocal) ExistsAndRename(fileId, newName string) (string, error) {
	// 检查是否存在
	if !helpers.PathExists(fileId) {
		helpers.AppLogger.Infof("本地文件不存在，无需重命名: 路径：%s", fileId)
		return "", nil
	}
	// 如果名字没变则不需要改名字
	if filepath.Base(fileId) == newName {
		helpers.AppLogger.Infof("本地文件名字没变，无需重命名: 路径：%s", fileId)
		return fileId, nil
	}
	// 重命名文件
	err := r.Rename(fileId, newName)
	if err != nil {
		helpers.AppLogger.Errorf("重命名本地文件失败：%s => %s 错误：%v", fileId, newName, err)
		return "", err
	}
	helpers.AppLogger.Infof("重命名本地文件成功, %s => %s", fileId, newName)
	return filepath.Join(filepath.Dir(fileId), newName), nil
}
