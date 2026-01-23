package scrape

import (
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/tmdb"
	"Q115-STRM/internal/v115open"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func (t tvShowScrapeImpl) GetSeasonUploadFiles(seasonMediaFile *models.ScrapeMediaFile) []uploadFile {
	destPath := seasonMediaFile.GetDestFullSeasonPath()
	destPathId := seasonMediaFile.NewSeasonPathId
	if destPathId == "" {
		destPathId = seasonMediaFile.NewPathId
	}
	sourcePath := seasonMediaFile.GetTmpFullSeasonPath()
	tvshowSourcePath := seasonMediaFile.GetTmpFullTvshowPath()
	tvshowPath := seasonMediaFile.GetDestFullTvshowPath()
	tvshowPathId := seasonMediaFile.NewPathId
	fileList := make([]uploadFile, 0)
	nfoName := seasonMediaFile.GetSeasonNfoName()
	nfoFullSourceName := filepath.Join(sourcePath, nfoName)
	file := uploadFile{
		ID:         fmt.Sprintf("%d", seasonMediaFile.ID),
		DestPathId: destPathId,
		DestPath:   destPath,
		FileName:   nfoName,
		SourcePath: nfoFullSourceName,
	}
	fileList = append(fileList, file)
	posterName := fmt.Sprintf("season%02d-poster.jpg", seasonMediaFile.SeasonNumber)
	file = uploadFile{
		ID:         fmt.Sprintf("%d", seasonMediaFile.ID),
		DestPathId: tvshowPathId,
		DestPath:   tvshowPath,
		FileName:   posterName,
		SourcePath: filepath.Join(tvshowSourcePath, posterName),
	}
	fileList = append(fileList, file)
	return fileList
}

func (t *tvShowScrapeImpl) UploadSeasonScrapeFile(seasonMediaFile *models.ScrapeMediaFile) error {
	helpers.AppLogger.Infof("开始处理电视剧 %s 季 %d 的元数据", seasonMediaFile.Name, seasonMediaFile.SeasonNumber)
	files := t.GetSeasonUploadFiles(seasonMediaFile)
	// 如果是本地文件直接移动到目标位置
	ok, err := t.MoveLocalTempFileToDest(seasonMediaFile, files)
	if err == nil {
		return nil
	}
	if !ok {
		// 标记为失败
		return err
	}
	for _, file := range files {
		err := models.AddUploadTaskFromMediaFile(seasonMediaFile, t.scrapePath, file.FileName, file.SourcePath, filepath.Join(file.DestPath, file.FileName), file.DestPathId, true)
		if err != nil {
			helpers.AppLogger.Errorf("添加上传任务 %s 失败, 失败原因: %v", file.FileName, err)
		}
	}
	// 将上传文件添加到上传队列
	helpers.AppLogger.Infof("完成电视剧 %s 季 %d 的元数据处理", seasonMediaFile.Name, seasonMediaFile.SeasonNumber)
	return nil
}

func (t *tvShowScrapeImpl) ScrapeFailedAllEpisodeBySeason(seasonMediaFile *models.ScrapeMediaFile, failedReason string) error {
	// 将所有集标记为失败
	err := db.Db.Table("scrape_media_files").Where("tvshow_path = ? AND batch_no = ? AND season_number = ?", seasonMediaFile.TvshowPathId, seasonMediaFile.BatchNo, seasonMediaFile.SeasonNumber).Updates(map[string]interface{}{
		"status":        models.ScrapeMediaStatusScrapeFailed,
		"failed_reason": failedReason,
	}).Error
	if err != nil {
		helpers.AppLogger.Errorf("批量更新电视剧 %s 季 %d 的所有集为刮削失败状态失败, 失败原因: %v", seasonMediaFile.Name, seasonMediaFile.SeasonNumber, err)
		return err
	}
	// 将当前seasonMediaFile也标记为失败
	seasonMediaFile.Failed(failedReason)
	helpers.AppLogger.Infof("批量更新电视剧 %s 季 %d 的所有集为刮削失败状态成功", seasonMediaFile.Name, seasonMediaFile.SeasonNumber)
	return nil
}

func (t *tvShowScrapeImpl) RenamedFailedAllEdpisodeBySeason(seasonMediaFile *models.ScrapeMediaFile, failedReason string) {
	// 将所有集标记为失败
	err := db.Db.Table("scrape_media_files").Where("tvshow_path = ? AND batch_no = ? AND season_number = ?", seasonMediaFile.TvshowPathId, seasonMediaFile.BatchNo, seasonMediaFile.SeasonNumber).Updates(map[string]interface{}{
		"status":        models.ScrapeMediaStatusScrapeFailed,
		"failed_reason": failedReason,
	}).Error
	if err != nil {
		helpers.AppLogger.Errorf("批量更新电视剧 %s 季 %d 的所有集为刮削失败状态失败, 失败原因: %v", seasonMediaFile.Name, seasonMediaFile.SeasonNumber, err)
		return
	}
	helpers.AppLogger.Infof("批量更新电视剧 %s 季 %d 的所有集为刮削失败状态成功", seasonMediaFile.Name, seasonMediaFile.SeasonNumber)
}

func (t *tvShowScrapeImpl) UpdateSeasonDataToAllEpisodeBySeason(seasonMediaFile *models.ScrapeMediaFile) error {
	// 更新所有集的NewPathId为新创建的文件夹ID
	err := db.Db.Table("scrape_media_files").Where("media_id = ? AND batch_no = ? AND season_number = ?", seasonMediaFile.MediaId, seasonMediaFile.BatchNo, seasonMediaFile.SeasonNumber).Updates(map[string]interface{}{
		"new_season_path_id":   seasonMediaFile.NewSeasonPathId,
		"new_season_path_name": seasonMediaFile.NewSeasonPathName,
		"media_season_id":      seasonMediaFile.MediaSeasonId,
	}).Error
	if err != nil {
		helpers.AppLogger.Errorf("批量更新电视剧 %s 季 %d 的所有集的新目录ID失败, 失败原因: %v", seasonMediaFile.Name, seasonMediaFile.SeasonNumber, err)
		return err
	}
	helpers.AppLogger.Infof("批量更新电视剧 %s 季 %d 的所有集的新目录ID成功", seasonMediaFile.Name, seasonMediaFile.SeasonNumber)
	return nil
}

func (t *tvShowScrapeImpl) MakeSeasonPath(seasonMediaFile *models.ScrapeMediaFile) error {
	destFullPath := seasonMediaFile.GetDestFullSeasonPath()
	helpers.AppLogger.Infof("电视剧 %s 季 %d 文件夹，目标路径：%s", seasonMediaFile.Name, seasonMediaFile.SeasonNumber, destFullPath)
	if seasonMediaFile.ScrapeType == models.ScrapeTypeOnly {
		seasonMediaFile.NewSeasonPathId = seasonMediaFile.PathId
		return nil
	}
	// 非仅刮削需要创建目标目录
	newPathId, err := t.renameImpl.CheckAndMkDir(destFullPath, seasonMediaFile.DestPath, seasonMediaFile.DestPathId)
	if err != nil {
		helpers.AppLogger.Errorf("创建目录 %s 失败, 失败原因: %v", destFullPath, err)
		return err
	}
	seasonMediaFile.NewSeasonPathId = newPathId
	seasonMediaFile.MediaSeason.Path = destFullPath
	seasonMediaFile.MediaSeason.PathId = newPathId
	seasonMediaFile.MediaSeason.Save()
	helpers.AppLogger.Infof("电视剧 %s 季 %d的目标目录 %s 成功", seasonMediaFile.Name, seasonMediaFile.SeasonNumber, destFullPath)
	return nil
}

// 处理季的刮削
func (t *tvShowScrapeImpl) ProcessSeason(seasonMediaFile *models.ScrapeMediaFile) error {
	seasonMediaFile.ScrapeRootPath = filepath.Join(helpers.RootDir, "config", "tmp", "刮削临时文件", fmt.Sprintf("%d", seasonMediaFile.ScrapePathId), "电视剧")
	if err := os.MkdirAll(seasonMediaFile.ScrapeRootPath, 0777); err != nil {
		helpers.AppLogger.Errorf("创建临时目录失败: %v", err)
		return err
	}
	// 如果已经关联了MediaSeason，且MediaSeason是刮削完毕，则跳过
	if seasonMediaFile.MediaSeason == nil || (seasonMediaFile.MediaSeason != nil && seasonMediaFile.MediaSeason.Status == models.MediaStatusUnScraped) {
		serr := t.ScrapeSeasonMedia(seasonMediaFile)
		if serr != nil {
			// 将季下所有集都设置为刮削失败
			t.ScrapeFailedAllEpisodeBySeason(seasonMediaFile, serr.Error())
			return serr
		}
	}
	if seasonMediaFile.MediaSeason != nil && seasonMediaFile.MediaSeason.Status == models.MediaStatusScraped {
		// 上传所有刮削好的电视剧的元数据
		if t.scrapePath.ScrapeType != models.ScrapeTypeOnlyRename {
			if uerr := t.UploadSeasonScrapeFile(seasonMediaFile); uerr != nil {
				// 将季下所有集标记为失败
				t.RenamedFailedAllEdpisodeBySeason(seasonMediaFile, uerr.Error())
				return uerr
			}
			// 将季标记为已整理
			seasonMediaFile.MediaSeason.Status = models.MediaStatusRenamed
			seasonMediaFile.MediaSeason.Save()
		}
	}
	return nil
}

func (t *tvShowScrapeImpl) ScrapeSeasonMedia(mediaFile *models.ScrapeMediaFile) error {
	seasonNumber := mediaFile.SeasonNumber
	// 如果已经关联了MediaSeason，且MediaSeason是刮削完毕，则跳过
	if mediaFile.MediaSeason != nil && mediaFile.MediaSeason.Status != models.MediaStatusUnScraped {
		helpers.AppLogger.Infof("电视剧 %s 季 %d 已刮削完毕，跳过刮削", mediaFile.Name, seasonNumber)
		return nil
	}
	// 查询季详情
	seasonDetail, err := t.tmdbClient.GetTvSeasonDetail(mediaFile.TmdbId, mediaFile.SeasonNumber, models.GlobalScrapeSettings.GetTmdbLanguage())
	if err != nil {
		helpers.AppLogger.Errorf("查询tmdb电视剧季详情失败,下次重试, 失败原因: %v", err)
		return err
	}
	if mediaFile.MediaSeasonId == 0 {
		mediaFile.MediaSeason = &models.MediaSeason{
			MediaId:      mediaFile.MediaId,
			SeasonNumber: mediaFile.SeasonNumber,
		}
	}
	t.MakeMediaSeasonFromTMDB(mediaFile, seasonDetail)
	mediaFile.NewSeasonPathName = mediaFile.GetDestSeasonPath()
	if mediaFile.ScrapeType != models.ScrapeTypeOnlyRename {
		localTempSeasonPath := mediaFile.GetTmpFullSeasonPath()
		if mkdirErr := os.MkdirAll(localTempSeasonPath, 0777); mkdirErr != nil {
			helpers.AppLogger.Errorf("创建临时目录失败, 失败原因: %v", mkdirErr)
			// 将所有其他相同电视剧的集也修改为失败
			// 构造更新数据
			t.ScrapeFailedAllEpisodeBySeason(mediaFile, mkdirErr.Error())
			return mkdirErr
		} else {
			helpers.AppLogger.Infof("季临时刮削文件存储路径创建成功，电视剧 %s 的第 %d 季: %s", mediaFile.Name, mediaFile.SeasonNumber, localTempSeasonPath)
		}
		t.GenerateSeasonNfo(mediaFile)
		// 下载季的图片
		seasonImageList := make(map[string]string)
		seasonPosterFile := fmt.Sprintf("season%02d-poster.jpg", mediaFile.SeasonNumber)
		seasonImageList[seasonPosterFile] = mediaFile.MediaSeason.PosterPath
		localTempTvshowPath := mediaFile.GetTmpFullTvshowPath()
		t.DownloadImages(localTempTvshowPath, v115open.DEFAULTUA, seasonImageList)
		helpers.AppLogger.Infof("电视剧 %s 季 %d 的封面文件已下载，路径: %s", mediaFile.Name, mediaFile.SeasonNumber, filepath.Join(localTempTvshowPath, seasonPosterFile))
	}
	// 生成季目录
	if err := t.MakeSeasonPath(mediaFile); err != nil {
		return err
	}
	// 更新季下所有集的新目录和ID
	if err := t.UpdateSeasonDataToAllEpisodeBySeason(mediaFile); err != nil {
		return err
	}
	return nil
}

func (sm *tvShowScrapeImpl) GenerateSeasonNfo(mediaFile *models.ScrapeMediaFile) error {
	season := &helpers.TVShowSeason{
		Title:         mediaFile.Media.Name,
		OriginalTitle: mediaFile.Media.OriginalName,
		Premiered:     mediaFile.Media.ReleaseDate,
		Releasedate:   mediaFile.Media.ReleaseDate,
		Year:          mediaFile.Media.Year,
		SeasonNumber:  mediaFile.MediaSeason.SeasonNumber,
		DateAdded:     time.Now().Format("2006-01-02"),
	}
	seasonPath := mediaFile.GetTmpFullSeasonPath()
	seasonFileName := mediaFile.GetSeasonNfoName()
	seasonNfoFile := filepath.Join(seasonPath, seasonFileName)
	err := helpers.WriteSeasonNfo(season, seasonNfoFile)
	if err != nil {
		helpers.AppLogger.Errorf("生成电视剧 %s 季 %d 的nfo文件失败: %v", mediaFile.Media.Name, mediaFile.MediaSeason.SeasonNumber, err)
		return err
	}
	helpers.AppLogger.Infof("生成电视剧 %s 季 %d 的nfo文件成功: %s", mediaFile.Media.Name, mediaFile.MediaSeason.SeasonNumber, seasonNfoFile)
	return nil
}

func (t *tvShowScrapeImpl) MakeMediaSeasonFromTMDB(mediaFile *models.ScrapeMediaFile, seasonDetail *tmdb.SeasonDetail) {
	if mediaFile.MediaSeasonId != 0 {
		// 检查是否存在
		mediaSeason := models.GetSeasonByMediaIdAndSeasonNumber(mediaFile.MediaId, mediaFile.SeasonNumber)
		if mediaSeason != nil {
			mediaFile.MediaSeason = mediaSeason
		}
	}
	if mediaFile.MediaSeason == nil {
		mediaFile.MediaSeason = &models.MediaSeason{
			MediaId:      mediaFile.MediaId,
			SeasonNumber: mediaFile.SeasonNumber,
			ScrapePathId: mediaFile.ScrapePathId,
		}
	}
	mediaFile.MediaSeason.FillInfoByTmdbInfo(seasonDetail)
	mediaFile.MediaSeasonId = mediaFile.MediaSeason.ID
	mediaFile.Save()
}

func (t *tvShowScrapeImpl) RollbackTvShowSeason(mediaFile *models.ScrapeMediaFile) error {
	// 仅刮削则删除上传的元数据
	if mediaFile.ScrapeType == models.ScrapeTypeOnly {
		uploadFiles := t.GetSeasonUploadFiles(mediaFile)
		files := make([]models.WillDeleteFile, 0)
		for _, uf := range uploadFiles {
			files = append(files, models.WillDeleteFile{FullFilePath: filepath.Join(uf.DestPath, uf.FileName)})
		}
		// 删除这些文件
		err := t.renameImpl.CheckAndDeleteFiles(mediaFile, files)
		if err != nil {
			helpers.AppLogger.Errorf("删除已上传的元数据文失败: %v", err)
			return err
		}
		helpers.AppLogger.Infof("删除已上传的元数据文件成功: %v", files)
	}
	// 如果是刮削和整理或者仅整理
	if mediaFile.ScrapeType == models.ScrapeTypeScrapeAndRename || mediaFile.ScrapeType == models.ScrapeTypeOnlyRename && mediaFile.Path != "" {
		// 检查目录是否存在，如果存在则改名字，如果不存在则创建
		parentPath := filepath.Dir(mediaFile.Path)
		var newPath string
		var pathId string
		var existsPathId string = ""
		newPath = filepath.Join(parentPath, mediaFile.NewSeasonPathName)
		if mediaFile.RenameType != models.RenameTypeMove {
			// 先检查旧文件夹是否存在
			var eerr error
			existsPathId, eerr = t.renameImpl.ExistsAndRename(mediaFile.PathId, mediaFile.NewSeasonPathName)
			if eerr != nil {
				helpers.AppLogger.Errorf("重命名旧文件夹 %s 失败: %v", mediaFile.Path, eerr)
				return eerr
			}
		}
		if existsPathId == "" {
			var err error
			pathId, err = t.renameImpl.CheckAndMkDir(newPath, mediaFile.TvshowPath, mediaFile.TvshowPathId)
			if err != nil {
				helpers.AppLogger.Errorf("创建父文件夹 %s 失败: %v", newPath, err)
				return err
			}
		} else {
			pathId = existsPathId
		}
		mediaFile.Path = newPath
		mediaFile.PathId = pathId
		// 更新所有集的路径
		err := t.UpdateSeasonPathAndIdToAllEpisode(mediaFile)
		if err != nil {
			helpers.AppLogger.Errorf("更新电视剧 %s 的所有集的路径失败: %v", mediaFile.Name, err)
			return err
		}
	}
	// 将季设置为未刮削
	mediaFile.MediaSeason.Status = models.MediaStatusUnScraped
	mediaFile.MediaSeason.Save()
	helpers.AppLogger.Infof("回滚电视剧 %s 季 %d 成功", mediaFile.Name, mediaFile.SeasonNumber)
	return nil
}

func (t *tvShowScrapeImpl) UpdateSeasonPathAndIdToAllEpisode(mediaFile *models.ScrapeMediaFile) error {
	// 更新所有集的路径
	updateData := map[string]interface{}{
		"path_id": mediaFile.PathId,
		"path":    mediaFile.Path,
	}
	// 批量更新
	affectedRows := db.Db.Table("scrape_media_files").Where("scrape_path_id =? AND media_season_id = ? AND batch_no = ?", mediaFile.ScrapePathId, mediaFile.MediaSeasonId, mediaFile.BatchNo).Updates(updateData).RowsAffected
	if affectedRows == 0 {
		helpers.AppLogger.Errorf("批量更新电视剧 %s 季 %d 的所有集的信息失败, 未更新任何行", mediaFile.Name, mediaFile.SeasonNumber)
		return errors.New("no rows affected")
	}
	helpers.AppLogger.Infof("批量更新电视剧 %s 季 %d 的所有集的信息成功，共更新 %d 行", mediaFile.Name, mediaFile.SeasonNumber, affectedRows)
	return nil
}
