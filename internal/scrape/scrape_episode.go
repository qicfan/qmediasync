package scrape

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/notificationmanager"
	"Q115-STRM/internal/syncstrm"
	"Q115-STRM/internal/tmdb"
	"Q115-STRM/internal/v115open"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

// 处理集的刮削任务，启动N个协程
func (t *tvShowScrapeImpl) scrapeEpisode(taskIndex int, wg *sync.WaitGroup) {
mainloop:
	for {
		select {
		case <-t.ctx.Done():
			// 停止任务
			helpers.AppLogger.Infof("刮削整理任务队列 %d 收到停止信号，退出", taskIndex)
			return
		case mediaFileId, ok := <-t.episodeTasks:
			if !ok {
				helpers.AppLogger.Infof("刮削整理任务队列 %d 已关闭", taskIndex)
				return
			}
			// 查询集详情
			mediaFile := models.GetScrapeMediaFileById(mediaFileId)
			if mediaFile == nil {
				wg.Done() // 处理完成后，计数-1
				helpers.AppLogger.Errorf("根据ID查待刮削文件记录失败，ID: %d", mediaFileId)
				continue mainloop
			}
			helpers.AppLogger.Infof("集刮削整理任务队列 %d 开始处理电视剧 %s 季 %d 集 %d", taskIndex, mediaFile.Name, mediaFile.SeasonNumber, mediaFile.EpisodeNumber)
			err := t.Process(mediaFile)
			if err != nil {
				helpers.AppLogger.Errorf("集刮削整理任务队列 %d 刮削电视剧 %s 季 %d 集 %d 失败: %v", taskIndex, mediaFile.Name, mediaFile.SeasonNumber, mediaFile.EpisodeNumber, err)
			} else {
				helpers.AppLogger.Infof("集刮削整理任务队列 %d 处理电视剧 %s 季 %d 集 %d 成功", taskIndex, mediaFile.Name, mediaFile.SeasonNumber, mediaFile.EpisodeNumber)
			}
			wg.Done() // 处理完成后，计数-1
		}
	}
}

func (t *tvShowScrapeImpl) ScrapeEpisodeMedia(mediaFile *models.ScrapeMediaFile) error {
	// 查询集详情
	episodeDetail, err := t.tmdbClient.GetTvEpisodeDetail(mediaFile.TmdbId, mediaFile.SeasonNumber, mediaFile.EpisodeNumber, models.GlobalScrapeSettings.GetTmdbLanguage())
	if err != nil {
		helpers.AppLogger.Errorf("查询tmdb电视剧集详情失败,下次重试, 失败原因: %v", err)
		return err
	}
	// 查询集演员
	credits, err := t.tmdbClient.GetTvEpisodeCredits(mediaFile.TmdbId, mediaFile.SeasonNumber, mediaFile.EpisodeNumber, models.GlobalScrapeSettings.GetTmdbLanguage())
	if err != nil {
		helpers.AppLogger.Errorf("查询tmdb电视剧集演员失败,下次重试, 失败原因: %v", err)
	} else {
		episodeDetail.Cast = credits.Cast
		episodeDetail.Crew = credits.Crew
	}
	t.MakeMediaEpisodeFromTMDB(mediaFile, episodeDetail)
	return nil
}

func (t *tvShowScrapeImpl) Scrape(mediaFile *models.ScrapeMediaFile) error {
	// 改为刮削中...
	mediaFile.Scraping()
	// 跳过已刮削的集
	epError := t.ScrapeEpisodeMedia(mediaFile)
	if epError != nil {
		helpers.AppLogger.Errorf("电视剧 %s 季 %d 集 %d 刮削失败: %v", mediaFile.Name, mediaFile.SeasonNumber, mediaFile.EpisodeNumber, epError)
		return epError
	}
	// 下载视频文件解析视频信息
	t.FFprobe(mediaFile)
	t.GenerateNewEpisodeName(mediaFile)
	episodePath := mediaFile.GetTmpFullSeasonPath()
	if !helpers.PathExists(episodePath) {
		// 没有目录就创建
		merr := os.MkdirAll(episodePath, 0777)
		if merr != nil {
			helpers.AppLogger.Errorf("创建季目录 %s 失败, 失败原因: %v", episodePath, merr)
			return merr
		}
	}
	if mediaFile.ScrapeType != models.ScrapeTypeOnlyRename {
		// 生成nfo
		t.GenerateEpisodeNfo(mediaFile)
		// 下载集的图片
		episodeImageList := make(map[string]string)
		episodeImageList[mediaFile.GetEpisodePosterName()] = mediaFile.MediaEpisode.PosterPath
		t.DownloadImages(episodePath, v115open.DEFAULTUA, episodeImageList)
		helpers.AppLogger.Infof("电视剧 %s 季 %d 集 %d 生成nfo和下载图片成功，路径：%s", mediaFile.Name, mediaFile.SeasonNumber, mediaFile.EpisodeNumber, episodePath)
	}
	mediaFile.ScrapeFinish()
	return nil
}

func (t *tvShowScrapeImpl) Process(mediaFile *models.ScrapeMediaFile) error {
	mediaFile.ScrapeRootPath = filepath.Join(helpers.ConfigDir, "tmp", "刮削临时文件", fmt.Sprintf("%d", mediaFile.ScrapePathId), "电视剧")
	if err := os.MkdirAll(mediaFile.ScrapeRootPath, 0777); err != nil {
		helpers.AppLogger.Errorf("创建临时目录失败: %v", err)
		return err
	}
	if mediaFile.MediaEpisode == nil || (mediaFile.MediaEpisode != nil && mediaFile.MediaEpisode.Status == models.MediaStatusUnScraped) {
		// 刮削
		err := t.Scrape(mediaFile)
		if err != nil {
			mediaFile.Failed(err.Error())
			return err
		}
	}
	// 改为整理中
	mediaFile.Renaming()
	// 非仅刮削，先移动视频文件到新目录
	if mediaFile.ScrapeType != models.ScrapeTypeOnly {
		if err := t.renameImpl.RenameAndMove(mediaFile, "", "", ""); err != nil {
			// 整理失败
			mediaFile.RenameFailed(err.Error())
			return err
		}
		mediaFile.MediaEpisode.Status = models.MediaStatusRenamed
		mediaFile.MediaEpisode.Save()
	}
	// 上传所有刮削好的元数据
	if mediaFile.ScrapeType != models.ScrapeTypeOnlyRename {
		if uerr := t.UploadEpisodeScrapeFile(mediaFile); uerr != nil {
			// 标记为整理失败
			mediaFile.RenameFailed(uerr.Error())
			return uerr
		}
	}
	// 将自己标记为完成，状态立即完成，网盘的临时文件等网盘上传完成删除
	t.FinishEpisode(mediaFile)
	// 查询是否还有未完成整理的集，如果全部完成则发送通知
	return nil
}

// 删除集的临时文件
func (t *tvShowScrapeImpl) RemoveEpisodeTmpFiles(mediaFile *models.ScrapeMediaFile) {
	episodeUploadFiles := t.GetEpisodeUploadFiles(mediaFile)
	for _, f := range episodeUploadFiles {
		os.Remove(f.SourcePath)
		helpers.AppLogger.Infof("删除集 %s 的刮削临时文件 %s 成功", mediaFile.Name, f.SourcePath)
	}
}

func (t *tvShowScrapeImpl) FinishEpisode(mediaFile *models.ScrapeMediaFile) {
	mediaFile.StatusFinish()
	// 检查是否全部完成
	if models.GetUnFinishEpisodeCount(mediaFile) != 0 {
		return
	}
	// 电视剧所有的集已经全部完成，发送通知，删除来源
	// 检查同批次的所有集是否都完成
	// 检查是否已整理完成
	sameBatchMediaFiles := models.GetAllEpisodeByTvshow(mediaFile.MediaId, mediaFile.BatchNo)
	s := true
	eList := make(map[int][]int, 0)
	for _, f := range sameBatchMediaFiles {
		if slices.Contains([]models.ScrapeMediaStatus{models.ScrapeMediaStatusScanned, models.ScrapeMediaStatusScraped, models.ScrapeMediaStatusScraping, models.ScrapeMediaStatusRenaming}, f.Status) {
			// 有未完成的记录，不删除目录
			s = false
			continue
		}
		// 检查季是否存在eList
		if _, ok := eList[f.SeasonNumber]; !ok {
			eList[f.SeasonNumber] = make([]int, 0)
		}
		eList[f.SeasonNumber] = append(eList[f.SeasonNumber], f.EpisodeNumber)
	}
	// 是否可以删除来源目录
	if !s {
		helpers.AppLogger.Infof("电视剧 %s 季 %d 集 %d 完成,但有未完成的记录，不能删除来源目录", mediaFile.Name, mediaFile.SeasonNumber, mediaFile.EpisodeNumber)
		return
	}
	seasonStrArray := make([]string, 0)
	for sn, se := range eList {
		if len(se) == 0 {
			continue
		}
		// 对se进行排序，由小到大
		sort.Ints(se)
		min := se[0]
		max := se[len(se)-1]
		if min == max {
			seasonStrArray = append(seasonStrArray, fmt.Sprintf("S%02dE%02d", sn, min))
		} else {
			seasonStrArray = append(seasonStrArray, fmt.Sprintf("S%02dE%02d-%02d", sn, min, max))
		}
	}
	seasonStr := strings.Join(seasonStrArray, ", ")
	// 发送通知
	helpers.AppLogger.Infof("电视剧 %s 刮削整理完成， 新路径：%s  季集：%s", mediaFile.Name, mediaFile.NewPathName, seasonStr)
	if mediaFile.Media != nil {
		ctx := context.Background()
		notif := &models.Notification{
			Type:      models.ScrapeFinished,
			Title:     fmt.Sprintf("✅ %s 刮削整理完成", mediaFile.Name),
			Content:   fmt.Sprintf("📊 类型: 电视剧, 类别: %s, 分辨率: %s\n📺 季集: %s\n⏰ 时间: %s", mediaFile.CategoryName, mediaFile.Resolution, seasonStr, time.Now().Format("2006-01-02 15:04:05")),
			Image:     mediaFile.Media.PosterPath,
			Timestamp: time.Now(),
			Priority:  models.NormalPriority,
		}
		if notificationmanager.GlobalEnhancedNotificationManager != nil {
			if err := notificationmanager.GlobalEnhancedNotificationManager.SendNotification(ctx, notif); err != nil {
				helpers.AppLogger.Errorf("发送电视剧刮削完成通知失败: %v", err)
			}
		}
	}
	// 删除临时目录
	if mediaFile.SourceType == models.SourceTypeLocal {
		t.RemoveEpisodeTmpFiles(mediaFile)
	}
	if mediaFile.ScrapeType == models.ScrapeTypeOnly || mediaFile.RenameType != models.RenameTypeMove {
		// 如果仅刮削，跳过
		// 如果不是移动模式，跳过
		// 如果不强制删除来源目录，跳过
		// 如果视频在来源根目录，跳过
		helpers.AppLogger.Infof("视频 %s 存在不符合删除来源目录的条件，跳过删除来源目录: %s", mediaFile.Name, mediaFile.Path)
		return
	}
	err := t.renameImpl.RemoveMediaSourcePath(mediaFile, t.scrapePath)
	if err != nil {
		helpers.AppLogger.Errorf("删除来源路径 %s 失败: %v", mediaFile.TvshowPath, err)
	}
}

func (t *tvShowScrapeImpl) MakeMediaEpisodeFromTMDB(mediaFile *models.ScrapeMediaFile, episodeDetail *tmdb.Episode) {
	if mediaFile.MediaEpisodeId != 0 {
		// 检查是否存在
		mediaEpisode := models.GetEpisodeByMediaIdAndSeasonNumber(mediaFile.MediaId, mediaFile.SeasonNumber, mediaFile.EpisodeNumber)
		if mediaEpisode != nil {
			mediaFile.MediaEpisode = mediaEpisode
		}
	}
	if mediaFile.MediaEpisode == nil {
		mediaFile.MediaEpisode = &models.MediaEpisode{
			MediaId:       mediaFile.MediaId,
			MediaSeasonId: mediaFile.MediaSeasonId,
			ScrapePathId:  mediaFile.ScrapePathId,
			SeasonNumber:  mediaFile.SeasonNumber,
			EpisodeNumber: mediaFile.EpisodeNumber,
		}
	}
	mediaFile.MediaEpisode.FillInfoByTmdbInfo(episodeDetail)
	mediaFile.MediaEpisode.Save()
	mediaFile.MediaEpisodeId = mediaFile.MediaEpisode.ID
	mediaFile.Save()
}

func (t *tvShowScrapeImpl) GenerateNewEpisodeName(mediaFile *models.ScrapeMediaFile) {
	// 生成去掉扩展名的文件名
	ext := filepath.Ext(mediaFile.VideoFilename)
	baseName := strings.TrimSuffix(filepath.Base(mediaFile.VideoFilename), ext)
	mediaFile.NewVideoBaseName = baseName
	mediaFile.VideoExt = ext
	if mediaFile.ScrapeType == models.ScrapeTypeOnly {
		helpers.AppLogger.Infof("仅刮削模式下，不改变文件名，生成去掉扩展名的文件名: %s, 扩展名: %s", baseName, ext)
		return
	}
	if t.scrapePath.FileNameTemplate != "" {
		baseName = mediaFile.GenerateNameByTemplate(t.scrapePath.FileNameTemplate) // 不含扩展名
	}
	mediaFile.NewVideoBaseName = baseName
	helpers.AppLogger.Infof("生成去掉扩展名的文件名: %s, 扩展名: %s", baseName, ext)
}

func (t *tvShowScrapeImpl) GenerateEpisodeNfo(mediaFile *models.ScrapeMediaFile) error {
	has, result := helpers.ChineseToPinyin(mediaFile.MediaEpisode.EpisodeName)
	originalTitle := mediaFile.MediaEpisode.EpisodeName
	SortTitle := mediaFile.MediaEpisode.EpisodeName
	if has {
		originalTitle = fmt.Sprintf("%s #(%s)", mediaFile.MediaEpisode.EpisodeName, result)
		SortTitle = fmt.Sprintf("%s #(%s)", result, mediaFile.MediaEpisode.EpisodeName)
	}
	episode := &helpers.TVShowEpisode{
		Title:         mediaFile.MediaEpisode.EpisodeName,
		OriginalTitle: originalTitle,
		SortTitle:     SortTitle,
		Premiered:     mediaFile.MediaEpisode.ReleaseDate,
		Releasedate:   mediaFile.MediaEpisode.ReleaseDate,
		Year:          mediaFile.MediaEpisode.Year,
		SeasonNumber:  mediaFile.MediaSeason.SeasonNumber,
		EpisodeNumber: mediaFile.MediaEpisode.EpisodeNumber,
		Season:        mediaFile.MediaSeason.SeasonNumber,
		Episode:       mediaFile.MediaEpisode.EpisodeNumber,
		DateAdded:     time.Now().Format("2006-01-02"),
		Director:      mediaFile.Media.Director,
		Outline:       fmt.Sprintf("<![CDATA[%s]]>", mediaFile.MediaEpisode.Overview),
		Plot:          fmt.Sprintf("<![CDATA[%s]]>", mediaFile.MediaEpisode.Overview),
	}
	if t.scrapePath.ExcludeNoImageActor {
		episode.Actor = make([]helpers.Actor, 0)
		for _, actor := range mediaFile.Media.Actors {
			if actor.Thumb != "" {
				episode.Actor = append(episode.Actor, actor)
			}
		}
	} else {
		episode.Actor = mediaFile.Media.Actors
	}
	episodePath := mediaFile.GetTmpFullSeasonPath()
	episodeNfoFile := filepath.Join(episodePath, mediaFile.GetEpisodeNfoName())
	err := helpers.WriteEpisodeNfo(episode, episodeNfoFile)
	if err != nil {
		helpers.AppLogger.Errorf("生成集的nfo文件失败，电视剧 %s 季 %d 集 %d 文件路径：%s 错误： %v", mediaFile.Name, mediaFile.SeasonNumber, mediaFile.EpisodeNumber, episodeNfoFile, err)
		return err
	} else {
		helpers.AppLogger.Infof("生成集的nfo文件成功，电视剧 %s 季 %d 集 %d 文件路径：%s", mediaFile.Name, mediaFile.SeasonNumber, mediaFile.EpisodeNumber, episodeNfoFile)
	}
	return nil
}

func (t *tvShowScrapeImpl) GetEpisodeUploadFiles(mediaFile *models.ScrapeMediaFile) []uploadFile {
	destPath := mediaFile.GetDestFullSeasonPath()
	destPathId := mediaFile.NewSeasonPathId
	if destPathId == "" {
		destPathId = mediaFile.NewPathId
	}
	sourcePath := mediaFile.GetTmpFullSeasonPath()
	fileList := make([]uploadFile, 0)
	nfoName := mediaFile.GetEpisodeNfoName()
	file := uploadFile{
		ID:         fmt.Sprintf("%d", mediaFile.ID),
		DestPathId: destPathId,
		DestPath:   destPath,
		FileName:   nfoName,
		SourcePath: filepath.Join(sourcePath, nfoName),
	}
	fileList = append(fileList, file)
	jpgName := mediaFile.GetEpisodePosterName()
	file = uploadFile{
		ID:         fmt.Sprintf("%d", mediaFile.ID),
		DestPathId: destPathId,
		DestPath:   destPath,
		FileName:   jpgName,
		SourcePath: filepath.Join(sourcePath, jpgName),
	}
	fileList = append(fileList, file)
	return fileList
}

// 先命中一个syncPath，使用newPath
func (t *tvShowScrapeImpl) SyncFilesToSTRMPath(mediaFile *models.ScrapeMediaFile, files []uploadFile) {
	syncPath := t.scrapePath.GetSyncPathByPath(mediaFile.Media.Path)
	if syncPath == nil {
		helpers.AppLogger.Errorf("未命中任何STRM同步目录, 无法将文件同步到STRM目录 %s", mediaFile.Media.Path)
		return
	}
	// 先生成STRM文件
	// 1. 构造STRM文件路径
	syncStrm := syncstrm.NewSyncStrmFromSyncPath(syncPath)
	path := mediaFile.GetDestFullSeasonPath()
	syncStrm.ProcessStrmFile(&syncstrm.SyncFileCache{
		Path:          path,
		ParentId:      path,
		FileType:      v115open.TypeFile,
		FileName:      mediaFile.MediaEpisode.VideoFileName,
		FileId:        mediaFile.MediaEpisode.VideoFileId,
		PickCode:      mediaFile.MediaEpisode.VideoPickCode,
		OpenlistSign:  mediaFile.MediaEpisode.VideoOpenListSign,
		FileSize:      0,
		MTime:         0,
		IsVideo:       true,
		IsMeta:        false,
		LocalFilePath: filepath.Join(syncPath.LocalPath, path, mediaFile.NewVideoBaseName+".strm"),
	})
	models.DeleteSyncRecordById(syncStrm.Sync.ID)
	// 将其他文件放入STRM同步目录内
	for _, file := range files {
		destPath := filepath.Join(syncPath.LocalPath, file.DestPath)
		if !helpers.PathExists(destPath) {
			err := os.MkdirAll(destPath, 0755)
			if err != nil {
				helpers.AppLogger.Errorf("创建目录 %s 失败, 失败原因: %v", destPath, err)
			}
		}
		destFile := filepath.Join(destPath, file.FileName)
		// 复制过去
		err := helpers.CopyFile(file.SourcePath, destFile)
		if err != nil {
			helpers.AppLogger.Errorf("复制文件 %s 到 %s 失败, 失败原因: %v", file.SourcePath, destFile, err)
		}
		helpers.AppLogger.Infof("复制文件 %s 到 %s 成功", file.SourcePath, destFile)
	}
}

func (t *tvShowScrapeImpl) UploadEpisodeScrapeFile(mediaFile *models.ScrapeMediaFile) error {
	// helpers.AppLogger.Infof("开始处理电视剧 %s 季 %d 集 %d 的元数据", mediaFile.Name, mediaFile.SeasonNumber, mediaFile.EpisodeNumber)
	files := t.GetEpisodeUploadFiles(mediaFile)
	// 将文件同步到STRM同步目录内
	t.SyncFilesToSTRMPath(mediaFile, files)
	// 如果是本地文件直接移动到目标位置
	ok, err := t.MoveLocalTempFileToDest(mediaFile, files)
	if err == nil {
		return nil
	}
	if !ok {
		// 标记为失败
		return err
	}
	for _, file := range files {
		err := models.AddUploadTaskFromMediaFile(mediaFile, t.scrapePath, file.FileName, file.SourcePath, filepath.Join(file.DestPath, file.FileName), file.DestPathId, false)
		if err != nil {
			helpers.AppLogger.Errorf("添加上传任务 %s 失败, 失败原因: %v", file.FileName, err)
		}
	}
	// // 将上传文件添加到上传队列
	// helpers.AppLogger.Infof("完成电视剧 %s 季 %d 集 %d 的元数据处理", mediaFile.Name, mediaFile.SeasonNumber, mediaFile.EpisodeNumber)
	return nil
}

func (t *tvShowScrapeImpl) RollbackEpisode(mediaFile *models.ScrapeMediaFile) error {
	return nil
}
