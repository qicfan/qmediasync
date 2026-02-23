package rename

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/v115open"
	"context"
	"errors"
	"path/filepath"
	"strings"
)

type Rename115 struct {
	RenameBase
	client *v115open.OpenClient
}

func NewRename115(ctx context.Context, scrapePath *models.ScrapePath, client *v115open.OpenClient) *Rename115 {
	return &Rename115{
		RenameBase: RenameBase{
			scrapePath: scrapePath,
			ctx:        ctx,
		},
		client: client,
	}
}

func (r *Rename115) RenameAndMove(mediaFile *models.ScrapeMediaFile, destPath, destPathId, newName string) error {
	switch mediaFile.RenameType {
	case models.RenameTypeMove:
		err := r.move(mediaFile, destPathId, destPath, newName)
		if err != nil {
			return err
		}
	case models.RenameTypeCopy:
		var err error
		err = r.copy(mediaFile, destPathId, destPath, newName)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Rename115) move(mediaFile *models.ScrapeMediaFile, destPathId, destPath, newName string) error {
	// helpers.AppLogger.Infof("115整理文件：%s 到 %s", mediaFile.Path+"/"+mediaFile.VideoFilename, destPath+"/"+newName)
	// 先检查是否已存在，如果已存在，就不移动了
	detail, detailErr := r.client.GetFsDetailByPath(r.ctx, filepath.Join(destPath, newName))
	if detail == nil || detailErr != nil || detail.FileId == "" {
		_, err := r.client.Move(r.ctx, []string{mediaFile.VideoFileId}, destPathId)
		if err != nil {
			helpers.AppLogger.Errorf("115移动文件失败: %v", err)
			return err
		} else {
			helpers.AppLogger.Infof("文件 %s 成功移动到 %s", mediaFile.Path+"/"+mediaFile.VideoFilename, destPath+"/"+newName)
		}
		if mediaFile.VideoFilename != newName {
			// 改名
			_, err := r.client.ReName(r.ctx, mediaFile.VideoFileId, newName)
			if err != nil {
				helpers.AppLogger.Errorf("115改名文件失败: %v", err)
				return err
			} else {
				helpers.AppLogger.Infof("文件 %s 成功重命名为 %s", mediaFile.VideoFilename, newName)
			}
		}
		if mediaFile.MediaType != models.MediaTypeTvShow {
			mediaFile.Media.VideoFileId = mediaFile.VideoFileId
			mediaFile.Media.VideoPickCode = mediaFile.VideoPickCode
		} else {
			mediaFile.MediaEpisode.VideoFileId = mediaFile.VideoFileId
			mediaFile.MediaEpisode.VideoPickCode = mediaFile.VideoPickCode
		}
	} else {
		helpers.AppLogger.Infof("文件 %s 已存在, 无需移动", destPath+"/"+newName)
		if mediaFile.MediaType != models.MediaTypeTvShow {
			mediaFile.Media.VideoFileId = detail.FileId
			mediaFile.Media.VideoPickCode = detail.PickCode
		} else {
			mediaFile.MediaEpisode.VideoFileId = detail.FileId
			mediaFile.MediaEpisode.VideoPickCode = detail.PickCode
		}
	}
	oldBaseName := strings.TrimSuffix(mediaFile.VideoFilename, mediaFile.VideoExt)
	// 移动字幕文件到新目录
	if mediaFile.SubtitleFileJson != "" {
		if mediaFile.MediaType != models.MediaTypeTvShow {
			mediaFile.Media.SubtitleFiles = make([]*models.MediaMetaFiles, 0)
		} else {
			mediaFile.MediaEpisode.SubtitleFiles = make([]*models.MediaMetaFiles, 0)
		}
		// files := []string{}
		for _, sub := range mediaFile.SubtitleFiles {
			newSub := &models.MediaMetaFiles{
				FileName: sub.FileName,
				FileId:   sub.FileId,
				PickCode: sub.PickCode,
			}
			if mediaFile.MediaType != models.MediaTypeTvShow {
				mediaFile.Media.SubtitleFiles = append(mediaFile.Media.SubtitleFiles, newSub)
			} else {
				mediaFile.MediaEpisode.SubtitleFiles = append(mediaFile.MediaEpisode.SubtitleFiles, newSub)
			}
			// 移动字幕文件到新目录
			_, err := r.client.Move(r.ctx, []string{sub.FileId}, destPathId)
			if err != nil {
				helpers.AppLogger.Errorf("115移动字幕文件 %s 失败: %v", sub.FileName, err)
				continue
			}
			// 检查是否需要改名
			if mediaFile.VideoFilename != newName {
				// 改名
				newSubName := strings.Replace(sub.FileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
				if newSubName != sub.FileName {
					// 改名
					_, err := r.client.ReName(r.ctx, sub.FileId, newSubName)
					if err != nil {
						helpers.AppLogger.Errorf("115改名字幕文件 %s 失败: %v", sub.FileName, err)
						continue
					} else {
						helpers.AppLogger.Infof("字幕文件 %s 成功重命名为 %s", sub.FileName, newSubName)
					}
					newSub.FileName = newSubName
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
				// 移动图片文件到新目录
				_, err := r.client.Move(r.ctx, []string{imageFile.FileId}, destPathId)
				if err != nil {
					helpers.AppLogger.Errorf("115移动图片文件 %s 失败: %v", imageFile.FileName, err)
					continue
				}
				newSubName := strings.Replace(imageFile.FileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
				// 检查是否需要改名
				if newSubName != imageFile.FileName {
					// 改名
					_, err := r.client.ReName(r.ctx, imageFile.FileId, newSubName)
					if err != nil {
						helpers.AppLogger.Errorf("115改名图片文件 %s 失败: %v", imageFile.FileName, err)
						continue
					} else {
						helpers.AppLogger.Infof("图片文件 %s 成功重命名为 %s", imageFile.FileName, newSubName)
					}
				}
			}
		}
		// 移动nfo文件
		if mediaFile.NfoFileId != "" {
			newNfoName := strings.Replace(mediaFile.NfoFileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
			// 移动nfo文件到新目录
			_, err := r.client.Move(r.ctx, []string{mediaFile.NfoFileId}, destPathId)
			if err != nil {
				helpers.AppLogger.Errorf("115移动nfo文件 %s 失败: %v", mediaFile.NfoFileName, err)
			}
			// 检查是否需要改名
			if newNfoName != mediaFile.NfoFileName {
				// 改名
				_, err := r.client.ReName(r.ctx, mediaFile.NfoFileId, newNfoName)
				if err != nil {
					helpers.AppLogger.Errorf("115改名nfo文件 %s 失败: %v", mediaFile.NfoFileName, err)
				} else {
					helpers.AppLogger.Infof("nfo文件 %s 成功重命名为 %s", mediaFile.NfoFileName, newNfoName)
				}

			}
		}
	}
	return nil
}

func (r *Rename115) copy(mediaFile *models.ScrapeMediaFile, destPathId, destPath, newName string) error {
	// helpers.AppLogger.Infof("115整理文件：%s 到 %s", filepath.Join(mediaFile.Path, mediaFile.VideoFilename), filepath.Join(destPath, newName))
	var err error
	var videoFileId string = mediaFile.VideoFileId
	var pickcode string = mediaFile.VideoPickCode
	// 先检查是否已存在，如果已存在，就不移动了
	detail, detailErr := r.client.GetFsDetailByPath(r.ctx, filepath.Join(destPath, newName))
	if detail == nil || detailErr != nil || detail.FileId == "" {
		_, err = r.client.Copy(r.ctx, []string{mediaFile.VideoFileId}, destPathId, false)
		if err != nil {
			helpers.AppLogger.Errorf("115复制文件失败: %v", err)
			return err
		} else {
			helpers.AppLogger.Infof("文件 %s 成功复制到 %s", filepath.Join(mediaFile.Path, mediaFile.VideoFilename), filepath.Join(destPath, newName))
			// 查询新文件id
			newDetail, newDetailErr := r.client.GetFsDetailByPath(r.ctx, filepath.Join(destPath, mediaFile.VideoFilename))
			if newDetailErr != nil || newDetail.FileId == "" {
				helpers.AppLogger.Errorf("复制文件 %s 到 %s 后，查询新文件ID失败: %v", mediaFile.VideoFileId, filepath.Join(destPath, newName), newDetailErr)
				return newDetailErr
			}
			helpers.AppLogger.Infof("复制文件 %s 到 %s 后，新文件ID为 %s", mediaFile.VideoFilename, filepath.Join(destPath, mediaFile.VideoFilename), newDetail.FileId)
			videoFileId = newDetail.FileId
			pickcode = newDetail.PickCode
		}
		if mediaFile.VideoFilename != newName {
			// 改名
			_, err := r.client.ReName(r.ctx, videoFileId, newName)
			if err != nil {
				helpers.AppLogger.Errorf("115改名文件失败: %s => %s %v", videoFileId, newName, err)
				return err
			} else {
				helpers.AppLogger.Infof("文件 %s 成功重命名为 %s", filepath.Join(mediaFile.Path, mediaFile.VideoFilename), filepath.Join(destPath, newName))
			}
		}
		if mediaFile.MediaType != models.MediaTypeTvShow {
			mediaFile.Media.VideoFileId = videoFileId
			mediaFile.Media.VideoPickCode = pickcode
		} else {
			mediaFile.MediaEpisode.VideoFileId = videoFileId
			mediaFile.MediaEpisode.VideoPickCode = pickcode
		}
	} else {
		helpers.AppLogger.Infof("文件 %s 已存在, 无需复制", destPath+"/"+newName)
		if mediaFile.MediaType != models.MediaTypeTvShow {
			mediaFile.Media.VideoFileId = detail.FileId
			mediaFile.Media.VideoPickCode = detail.PickCode
		} else {
			mediaFile.MediaEpisode.VideoFileId = detail.FileId
			mediaFile.MediaEpisode.VideoPickCode = detail.PickCode
		}
	}
	oldBaseName := strings.TrimSuffix(mediaFile.VideoFilename, mediaFile.VideoExt)
	// 复制字幕文件到新目录
	if mediaFile.SubtitleFileJson != "" {
		if mediaFile.MediaType != models.MediaTypeTvShow {
			mediaFile.Media.SubtitleFiles = make([]*models.MediaMetaFiles, 0)
		} else {
			mediaFile.MediaEpisode.SubtitleFiles = make([]*models.MediaMetaFiles, 0)
		}
		for _, sub := range mediaFile.SubtitleFiles {
			// 改名
			newSubName := strings.Replace(sub.FileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
			newSub := &models.MediaMetaFiles{
				FileName: newSubName,
				FileId:   sub.FileId,
				PickCode: sub.PickCode,
			}
			if mediaFile.MediaType != models.MediaTypeTvShow {
				mediaFile.Media.SubtitleFiles = append(mediaFile.Media.SubtitleFiles, newSub)
			} else {
				mediaFile.MediaEpisode.SubtitleFiles = append(mediaFile.MediaEpisode.SubtitleFiles, newSub)
			}
			// 先检查是否存在
			// 查询新文件ID
			newSubDetail, newSubDetailErr := r.client.GetFsDetailByPath(r.ctx, filepath.Join(destPath, newSubName))
			if newSubDetailErr == nil && newSubDetail.FileId != "" {
				// 字幕文件已存在，无需复制
				newSub.FileId = newSubDetail.FileId
				newSub.PickCode = newSubDetail.PickCode
				continue
			}
			// 复制字幕文件到新目录
			_, err := r.client.Copy(r.ctx, []string{sub.FileId}, destPathId, false)
			if err != nil {
				helpers.AppLogger.Errorf("115复制字幕文件 %s 失败: %v", sub.FileName, err)
				continue
			}
			// 查询新文件ID
			newSubDetail, newSubDetailErr = r.client.GetFsDetailByPath(r.ctx, filepath.Join(destPath, sub.FileName))
			if newSubDetailErr != nil {
				helpers.AppLogger.Errorf("复制字幕文件 %s 到 %s 后，查询新文件ID失败: %v", sub.FileId, filepath.Join(destPath, sub.FileName), newSubDetailErr)
				continue
			}
			newSub.FileId = newSubDetail.FileId
			newSub.PickCode = newSubDetail.PickCode
			mediaFile.Media.SubtitleFiles = append(mediaFile.Media.SubtitleFiles, newSub)
			// 检查是否需要改名
			if newSubName != sub.FileName {
				// 改名
				_, err := r.client.ReName(r.ctx, newSub.FileId, newSubName)
				if err != nil {
					helpers.AppLogger.Errorf("115改名字幕文件 %s 失败: %v", sub.FileName, err)
					continue
				} else {
					helpers.AppLogger.Infof("字幕文件 %s 成功重命名为 %s", sub.FileName, newSubName)
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
				// 移动图片文件到新目录
				_, err := r.client.Copy(r.ctx, []string{imageFile.FileId}, destPathId, false)
				if err != nil {
					helpers.AppLogger.Errorf("115移动图片文件 %s 失败: %v", imageFile.FileName, err)
					continue
				}
				// 查询新文件ID
				newImageDetail, newImageDetailErr := r.client.GetFsDetailByPath(r.ctx, filepath.Join(destPath, imageFile.FileName))
				if newImageDetailErr != nil {
					helpers.AppLogger.Errorf("复制图片文件 %s 到 %s 后，查询新文件ID失败: %v", imageFile.FileId, filepath.Join(destPath, imageFile.FileName), newImageDetailErr)
					continue
				}
				imageFile.FileId = newImageDetail.FileId
				newSubName := strings.Replace(imageFile.FileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
				// 检查是否需要改名
				if newSubName != imageFile.FileName {
					// 改名
					_, err := r.client.ReName(r.ctx, imageFile.FileId, newSubName)
					if err != nil {
						helpers.AppLogger.Errorf("115改名图片文件 %s 失败: %v", imageFile.FileName, err)
						continue
					} else {
						helpers.AppLogger.Infof("图片文件 %s 成功重命名为 %s", imageFile.FileName, newSubName)
					}
				}
			}
		}
		// 移动nfo文件
		if mediaFile.NfoFileId != "" {
			newNfoName := strings.Replace(mediaFile.NfoFileName, oldBaseName, mediaFile.NewVideoBaseName, 1)
			// 移动nfo文件到新目录
			_, err := r.client.Copy(r.ctx, []string{mediaFile.NfoFileId}, destPathId, false)
			if err != nil {
				helpers.AppLogger.Errorf("115复制nfo文件 %s 失败: %v", mediaFile.NfoFileName, err)
			}
			// 查询新文件ID
			newNfoDetail, newNfoDetailErr := r.client.GetFsDetailByPath(r.ctx, filepath.Join(destPath, newNfoName))
			if newNfoDetailErr != nil {
				helpers.AppLogger.Errorf("复制nfo文件 %s 到 %s 后，查询新文件ID失败: %v", mediaFile.NfoFileId, filepath.Join(destPath, newNfoName), newNfoDetailErr)
			} else {
				mediaFile.NfoFileId = newNfoDetail.FileId
				// 检查是否需要改名
				if newNfoName != mediaFile.NfoFileName {
					// 改名
					_, err := r.client.ReName(r.ctx, mediaFile.NfoFileId, newNfoName)
					if err != nil {
						helpers.AppLogger.Errorf("115改名nfo文件 %s 失败: %v", mediaFile.NfoFileName, err)
					} else {
						helpers.AppLogger.Infof("nfo文件 %s 成功重命名为 %s", mediaFile.NfoFileName, newNfoName)
					}
				}
			}
		}
	}
	return nil
}

func (r *Rename115) CheckAndMkDir(destFullPath string, rootPath, rootPathId string) (string, error) {
	fsDetail, err := r.client.GetFsDetailByPath(r.ctx, destFullPath)
	if err == nil && fsDetail.FileId != "" {
		return fsDetail.FileId, nil
	}
	relPath, err := filepath.Rel(rootPath, destFullPath)
	if err != nil {
		helpers.AppLogger.Errorf("获取相对路径失败: %v", err)
		return "", err
	}
	// 将newPath中的\替换为/
	relPath = strings.ReplaceAll(relPath, "\\", "/")
	pathParts := strings.SplitSeq(relPath, "/")
	currentParentPath := rootPath
	currentParentId := rootPathId
	for p := range pathParts {
		currentCheckPath := filepath.Join(currentParentPath, p)
		// 先检查是否存在
		pDetail, pErr := r.client.GetFsDetailByPath(r.ctx, currentCheckPath)
		if pErr == nil && pDetail.FileId != "" {
			currentParentPath = currentCheckPath
			currentParentId = pDetail.FileId
			continue
		}
		// 分段创建目录
		cpId, mErr := r.client.MkDir(r.ctx, currentParentId, p)
		if mErr != nil {
			helpers.AppLogger.Errorf("创建父文件夹 %s 失败: %v", currentCheckPath, mErr)
			return "", mErr
		} else {
			helpers.AppLogger.Infof("父文件夹创建成功，路径：%s，目录ID：%s", currentCheckPath, cpId)
		}
		currentParentPath = currentCheckPath
		currentParentId = cpId
	}
	return currentParentId, nil
}

func (r *Rename115) RemoveMediaSourcePath(mediaFile *models.ScrapeMediaFile, sp *models.ScrapePath) error {
	hasSeason := true
	sourcePathId := mediaFile.PathId
	sourcePath := mediaFile.Path
	if sourcePathId == "" {
		sourcePathId = mediaFile.TvshowPathId
		sourcePath = mediaFile.TvshowPath
		hasSeason = false
	}
	if sourcePathId == mediaFile.SourcePathId {
		helpers.AppLogger.Warnf("视频文件 %s 所在目录 %s 是来源根路径，不删除", mediaFile.Path, sourcePath)
		return nil
	}
	// 查询mediaFile.PathId下面是否为空
	fsList, err := r.client.GetFsList(r.ctx, sourcePathId, true, false, true, 1, 10)
	if err != nil {
		helpers.AppLogger.Errorf("获取115文件详情失败:路径：%s 文件夹ID=%s %v", sourcePath, sourcePathId, err)
		return err
	}
	if fsList.Count > 0 {
		for _, file := range fsList.Data {
			if sp.IsVideoFile(file.FileName) {
				helpers.AppLogger.Infof("目录 %s 下有其他视频文件，不删除", sourcePathId)
				return nil
			}
		}
	}
	if fsList.Count == 0 || sp.ForceDeleteSourcePath {
		// 取父文件夹，从fsDetail.Paths中取最后一个
		parentId := ""
		// 查询父目录
		parentPath := filepath.Dir(sourcePath)
		fsDetail, perr := r.client.GetFsDetailByPath(r.ctx, parentPath)
		if perr != nil {
			return perr
		}
		parentId = fsDetail.FileId
		// 删除季文件夹
		_, err = r.client.Del(r.ctx, []string{sourcePathId}, parentId)
		if err != nil {
			helpers.AppLogger.Errorf("删除115文件失败: 路径：%s 文件夹ID=%s %v", mediaFile.Path, mediaFile.PathId, err)
			return err
		}
		helpers.AppLogger.Infof("刮削完成，尝试删除115中的文件夹成功, 路径：%s 文件夹ID=%s", sourcePath, sourcePathId)
	}
	// 再删除电视剧文件夹
	if mediaFile.PathId != "" {
		if mediaFile.TvshowPathId == sp.SourcePathId {
			helpers.AppLogger.Infof("电视剧目录 %s 是来源根路径，不删除", mediaFile.TvshowPath)
			return nil
		}
		fsDetail, err := r.client.GetFsDetailByCid(r.ctx, mediaFile.TvshowPathId)
		if err != nil {
			return err
		}
		if hasSeason {
			// 如果含有季目录，需要检查电视剧目录下是否以己经没有季目录了
			tvshowFileList, err := r.client.GetFsList(r.ctx, mediaFile.TvshowPathId, true, false, true, 1, 10)
			if err != nil {
				helpers.AppLogger.Errorf("获取115文件详情失败:路径：%s 文件夹ID=%s %v", mediaFile.TvshowPath, mediaFile.TvshowPathId, err)
				return err
			}
			for _, tvshowFile := range tvshowFileList.Data {
				if tvshowFile.FileCategory == v115open.TypeDir {
					helpers.AppLogger.Infof("电视剧目录 %s 下有其他目录 %s，不删除", mediaFile.TvshowPath, tvshowFile.FileName)
					return nil
				}
			}
		}
		if fsDetail.Count == "0" || sp.ForceDeleteSourcePath {
			// 删除115文件
			// 查询mediaFile.TvshowPathId的详情
			tvshowDetail, err := r.client.GetFsDetailByCid(r.ctx, mediaFile.TvshowPathId)
			if err != nil {
				return err
			}
			tvshowParentId := tvshowDetail.Paths[len(fsDetail.Paths)-1].FileId
			_, err = r.client.Del(r.ctx, []string{mediaFile.TvshowPathId}, tvshowParentId)
			if err != nil {
				helpers.AppLogger.Errorf("删除115文件失败: 路径：%s 文件夹ID=%s %v", mediaFile.TvshowPath, mediaFile.TvshowPathId, err)
				return err
			}
			helpers.AppLogger.Infof("刮削完成，删除115中的电视剧文件夹成功, 路径：%s 文件夹ID=%s", mediaFile.TvshowPath, mediaFile.TvshowPathId)
		}
	}
	return nil
}

func (r *Rename115) ReadFileContent(fileId string) ([]byte, error) {
	ctx := context.Background()
	url := r.client.GetDownloadUrl(ctx, fileId, v115open.DEFAULTUA, false)
	if url == "" {
		helpers.AppLogger.Errorf("获取115文件下载链接失败: pickcode=%s, url为空", fileId)
		return nil, errors.New("获取115文件下载链接失败, url为空")
	}
	// 读取url的内容
	content, err := helpers.ReadFromUrl(url, v115open.DEFAULTUA)
	if err != nil {
		helpers.AppLogger.Errorf("115读取文件下载链接内容失败: pickcode=%s, url=%s, %v", fileId, url, err)
		return nil, err
	}
	return content, nil
}

func (r *Rename115) CheckAndDeleteFiles(mediaFile *models.ScrapeMediaFile, files []models.WillDeleteFile) error {
	for _, f := range files {
		// 检查是否存在
		fsDetail, err := r.client.GetFsDetailByPath(r.ctx, f.FullFilePath)
		if err != nil || (fsDetail != nil && fsDetail.FileId == "") {
			helpers.AppLogger.Infof("115文件不存在，无需删除: 路径：%s", f.FullFilePath)
			continue
		}
		// 查询父目录
		parentPath := filepath.Dir(f.FullFilePath)
		parentDetail, err := r.client.GetFsDetailByPath(r.ctx, parentPath)
		if err != nil || (parentDetail != nil && parentDetail.FileId == "") {
			helpers.AppLogger.Errorf("获取115父目录ID失败: 路径：%s %v", parentPath, err)
			continue
		}
		_, err = r.client.Del(r.ctx, []string{fsDetail.FileId}, parentDetail.FileId)
		if err != nil {
			helpers.AppLogger.Errorf("删除115文件失败: 路径：%s %v", f.FullFilePath, err)
			continue
		}
		helpers.AppLogger.Infof("删除115文件成功, 路径：%s", f.FullFilePath)
	}
	return nil
}

func (r *Rename115) MoveFiles(f models.MoveNewFileToSourceFile) error {
	// 检查是否存在
	fsDetail, err := r.client.GetFsDetailByPath(r.ctx, f.FileFullPath)
	if err == nil && (fsDetail != nil && fsDetail.FileId != "") {
		helpers.AppLogger.Infof("115文件存在，无需移动: 路径：%s", f.FileFullPath)
		return nil
	}
	// 移动文件
	_, err = r.client.Move(r.ctx, []string{f.FileId}, f.PathId)
	if err != nil {
		helpers.AppLogger.Errorf("移动115文件失败: 新路径：%s %v", f.FileFullPath, err)
		return err
	}
	helpers.AppLogger.Infof("移动115文件成功, %s => %s", f.FileId, f.FileFullPath)
	return nil
}

func (r *Rename115) DeleteDir(path, pathId string) error {
	parentPath := filepath.Dir(path)
	parentDetail, err := r.client.GetFsDetailByPath(r.ctx, parentPath)
	if err != nil || (parentDetail != nil && parentDetail.FileId == "") {
		helpers.AppLogger.Errorf("获取115父目录ID失败: 路径：%s %v", parentPath, err)
		return err
	}
	// 删除目录
	_, err = r.client.Del(r.ctx, []string{pathId}, parentDetail.FileId)
	if err != nil {
		helpers.AppLogger.Errorf("删除115目录失败: 路径：%s %v", pathId, err)
		return err
	}
	helpers.AppLogger.Infof("删除115目录成功, 路径：%s", pathId)
	return nil
}

func (r *Rename115) Rename(fileId, newName string) error {
	// 重命名文件
	_, err := r.client.ReName(r.ctx, fileId, newName)
	if err != nil {
		helpers.AppLogger.Errorf("重命名115文件失败：%s => %s 错误：%v", fileId, newName, err)
		return err
	}
	helpers.AppLogger.Infof("重命名115文件成功, %s => %s", fileId, newName)
	return nil
}

// 检查是否存在，存在就改名字，然后返回新的fileId
func (r *Rename115) ExistsAndRename(fileId, newName string) (string, error) {
	// 检查是否存在
	fsDetail, err := r.client.GetFsDetailByCid(r.ctx, fileId)
	if err != nil && (fsDetail != nil && fsDetail.FileId == "") {
		helpers.AppLogger.Infof("115文件不存在，无需重命名: 文件ID：%s", fileId)
		return "", nil
	}
	// 如果名字没变则不需要改名字
	if fsDetail.FileName == newName {
		helpers.AppLogger.Infof("115文件名字没变，无需重命名: 文件ID：%s", fileId)
		return fileId, nil
	}
	// 重命名文件
	_, err = r.client.ReName(r.ctx, fileId, newName)
	if err != nil {
		helpers.AppLogger.Errorf("重命名115文件失败：%s => %s 错误：%v", fileId, newName, err)
		return "", err
	}
	helpers.AppLogger.Infof("重命名115文件成功, %s => %s", fileId, newName)
	return fileId, nil
}
