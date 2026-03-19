package scrape

import (
	"Q115-STRM/internal/baidupan"
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/notificationmanager"
	"Q115-STRM/internal/openlist"
	"Q115-STRM/internal/syncstrm"
	"Q115-STRM/internal/tmdb"
	"Q115-STRM/internal/v115open"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type movieScrapeImpl struct {
	ScrapeBase
}

func NewMovieScrapeImpl(scrapePath *models.ScrapePath, ctx context.Context, v115Client *v115open.OpenClient, openlistClient *openlist.Client, baiduPanClient *baidupan.Client) scrapeImpl {
	tmdbImpl := NewTmdbMovieImpl(scrapePath, ctx)
	return &movieScrapeImpl{
		ScrapeBase: ScrapeBase{
			scrapePath:     scrapePath,
			ctx:            ctx,
			identifyImpl:   NewIdMovieImpl(scrapePath, ctx, tmdbImpl),
			tmdbClient:     tmdbImpl.Client,
			categoryImpl:   NewCategoryMovieImpl(scrapePath),
			renameImpl:     NewRenameMovieImpl(scrapePath, ctx, v115Client, openlistClient, baiduPanClient),
			v115Client:     v115Client,
			openlistClient: openlistClient,
			baiduPanClient: baiduPanClient,
		},
	}
}

func (m *movieScrapeImpl) Start() error {
	m.fileTasks = make(chan *models.ScrapeMediaFile, m.scrapePath.GetMaxThreads())
	// 每次从数据库中查询maxthreads个任务加入队列，等待处理完成后继续下一次查询直到无法查询到数据
	// 启动N个协程协程，由m.ctx控制是否取消
	wg := &sync.WaitGroup{}
	max := m.scrapePath.GetMaxThreads()
	// 查询数据库中所有待刮削和待整理的记录总数来决定要启动的工作协程数量
	total := models.GetScannedScrapeMediaFilesTotal(m.scrapePath.ID, m.scrapePath.MediaType)
	if total == 0 {
		helpers.AppLogger.Infof("没有待刮削和待整理的记录，无需启动刮削任务")
		return nil
	}
	threads := min(max, int(total))
	for i := 0; i < threads; i++ {
		go m.scrapeWorker(i+1, wg)
	}
mainloop:
	for {
		select {
		case <-m.ctx.Done():
			helpers.AppLogger.Infof("电影主循环检测到停止信号，退出")
			break mainloop
		default:
			// 从数据库取数据
			mediaFiles := models.GetScannedScrapeMediaFiles(m.scrapePath.ID, m.scrapePath.MediaType, m.scrapePath.GetMaxThreads()*2)
			if len(mediaFiles) == 0 {
				helpers.AppLogger.Infof("所有待刮削和待整理记录都已加入处理队列，关闭队列通道，等待执行完成")
				close(m.fileTasks)
				break mainloop
			}
			for _, mediaFile := range mediaFiles {
				m.fileTasks <- mediaFile
				wg.Add(1) // 加进去之后，计数+1
				helpers.AppLogger.Infof("文件 %s 已加入刮削处理队列", mediaFile.VideoFilename)
			}
			wg.Wait()
		}
	}
	helpers.AppLogger.Infof("所有刮削整理任务都已完成，本次任务结束")
	return nil
}

func (m *movieScrapeImpl) scrapeWorker(taskIndex int, wg *sync.WaitGroup) {
mainloop:
	for {
		select {
		case <-m.ctx.Done():
			helpers.AppLogger.Infof("电影工作线程 %d 检测到停止信号，退出", taskIndex)
			return
		case mediaFile, ok := <-m.fileTasks:
			if !ok {
				helpers.AppLogger.Infof("刮削整理任务队列 %d 已关闭", taskIndex)
				return
			}
			err := m.Process(mediaFile)
			wg.Done() // 处理完成后，计数-1
			if err != nil {
				helpers.AppLogger.Errorf("任务队列 %d 刮削文件 %s 失败: %v", taskIndex, mediaFile.VideoFilename, err)
			}
			continue mainloop
		case <-time.After(5 * time.Minute):
			return // 5分钟没响应自动退出
		}
	}
}

func (m *movieScrapeImpl) Process(mediaFile *models.ScrapeMediaFile) error {
	// 创建临时目录
	mediaFile.ScrapeRootPath = filepath.Join(helpers.ConfigDir, "tmp", "刮削临时文件", fmt.Sprintf("%d", mediaFile.ScrapePathId), "电影或其他")
	if err := os.MkdirAll(mediaFile.ScrapeRootPath, 0777); err != nil {
		helpers.AppLogger.Errorf("创建临时目录失败: %v", err)
		return err
	}
	// 先从文件名或文件夹名字中提取影片名字+年份或tmdbid
	if mediaFile.Status == models.ScrapeMediaStatusScanned {
		// 待刮削，启动刮削流程
		err := m.Scrape(mediaFile)
		if err != nil {
			mediaFile.Failed(err.Error())
			return err
		}
	}
	// 改为整理中
	mediaFile.Renaming()
	m.MakeParentPath(mediaFile, m.scrapePath.CategoryMap)
	// 非仅刮削，先移动视频文件到新目录，如果是其他仅整理，也移动图片、nfo到新目录
	if mediaFile.ScrapeType != models.ScrapeTypeOnly {
		if err := m.renameImpl.RenameAndMove(mediaFile, "", "", ""); err != nil {
			// 整理失败
			mediaFile.RenameFailed(err.Error())
			return err
		}
		mediaFile.Media.Status = models.MediaStatusRenamed
		mediaFile.Media.Save()
	}
	// 上传所有刮削好的元数据
	if mediaFile.ScrapeType != models.ScrapeTypeOnlyRename {
		if uerr := m.UploadMovieScrapeFile(mediaFile); uerr != nil {
			// 标记为整理失败
			mediaFile.RenameFailed(uerr.Error())
			return uerr
		}
	} else {
		// 将文件同步到STRM同步目录内
		m.SyncFilesToSTRMPath(mediaFile, nil)
	}
	// 将自己标记为完成，状态立即完成，网盘的临时文件等网盘上传完成删除
	m.FinishMovie(mediaFile)
	return nil
}

func (m *movieScrapeImpl) Scrape(mediaFile *models.ScrapeMediaFile) error {
	// 改为刮削中...
	mediaFile.Scraping()
	// 识别
	if err := m.identifyImpl.Identify(mediaFile); err != nil {
		return err
	}
	if scrapeErr := m.ScrapeMovieMedia(mediaFile); scrapeErr != nil {
		return scrapeErr
	}
	// 提取分辨率等信息
	if err := m.FFprobe(mediaFile); err != nil {
		helpers.AppLogger.Errorf("提取视频信息失败, 文件名: %s, 错误: %v", mediaFile.VideoFilename, err)
	}
	// 确定二级分类
	if cerr := m.GenrateCategory(mediaFile); cerr != nil {
		return cerr
	}
	m.GenerateNewName(mediaFile)
	if mediaFile.ScrapeType != models.ScrapeTypeOnlyRename {
		// 下载图片，生成nfo文件
		// 生成本地临时路径
		localTempPath := mediaFile.GetTmpFullMoviePath()
		if err := os.MkdirAll(localTempPath, 0777); err != nil {
			helpers.AppLogger.Errorf("创建临时目录 %s 失败，下次重试，错误: %v", localTempPath, err)
			mediaFile.Scanned()
			return err
		} else {
			helpers.AppLogger.Infof("临时目录 %s 创建成功", localTempPath)
		}
		nfoName := m.GetMovieRealName(mediaFile, "", "nfo")
		// 生成nfo
		m.GenerateMovieNfo(mediaFile, localTempPath, nfoName, m.scrapePath.ExcludeNoImageActor)
		fileList := map[string]string{}
		posterExt := filepath.Ext(mediaFile.Media.PosterPath)
		fileList[m.GetMovieRealName(mediaFile, fmt.Sprintf("poster%s", posterExt), "image")] = mediaFile.Media.PosterPath
		logoExt := filepath.Ext(mediaFile.Media.LogoPath)
		fileList[m.GetMovieRealName(mediaFile, fmt.Sprintf("clearlogo%s", logoExt), "image")] = mediaFile.Media.LogoPath
		fanartExt := filepath.Ext(mediaFile.Media.BackdropPath)
		fileList[m.GetMovieRealName(mediaFile, fmt.Sprintf("fanart%s", fanartExt), "image")] = mediaFile.Media.BackdropPath
		m.DownloadImages(localTempPath, v115open.DEFAULTUA, fileList)
		// 从fanart.tv查询图片并下载
		if m.scrapePath.EnableFanartTv {
			fileList = m.DownloadMovieImagesFromFanart(mediaFile)
			if fileList != nil {
				m.DownloadImages(localTempPath, v115open.DEFAULTUA, fileList)
			}
		}
	}
	mediaFile.ScrapeFinish()
	return nil
}

// 从tmdb刮削元数据和图片信息（不下载，不创建目录）
func (m *movieScrapeImpl) ScrapeMovieMedia(mediaFile *models.ScrapeMediaFile) error {
	// 如果是其他类型，需要读取nfo文件
	if mediaFile.MediaType == models.MediaTypeOther {
		return m.CreateMediaFromNfo(mediaFile)
	}
	tmdbInfo := &models.TmdbInfo{}
	// 查询详情
	movieDetail, err := m.tmdbClient.GetMovieDetail(mediaFile.TmdbId, models.GlobalScrapeSettings.GetTmdbLanguage())
	if err != nil {
		helpers.AppLogger.Errorf("查询tmdb电影详情失败, 下次重试, 失败原因: %v", err)
		return err
	}
	tmdbInfo.MovieDetail = movieDetail
	if mediaFile.ScrapeType != models.ScrapeTypeOnlyRename {
		// 查询演职人员
		cast, _ := m.tmdbClient.GetMoviePepoles(mediaFile.TmdbId, models.GlobalScrapeSettings.GetTmdbLanguage())
		tmdbInfo.Credits = cast
		// 查询图片
		images, _ := m.tmdbClient.GetMovieImages(mediaFile.TmdbId, models.GlobalScrapeSettings.GetTmdbImageLanguage())
		if images != nil {
			helpers.AppLogger.Infof("查询tmdb电影图片成功, tmdbId: %d, 语言: %s", mediaFile.TmdbId, models.GlobalScrapeSettings.GetTmdbImageLanguage())
			// 如果图片为空,则使用详情中的图片
			if len(images.Posters) == 0 && movieDetail.PosterPath != "" {
				images.Posters = append(images.Posters, tmdb.Image{
					FilePath: movieDetail.PosterPath,
				})
			}
			if len(images.Backdrops) == 0 && movieDetail.BackdropPath != "" {
				images.Backdrops = append(images.Backdrops, tmdb.Image{
					FilePath: movieDetail.BackdropPath,
				})
			}
		}
		tmdbInfo.Images = images
		// 查询分级信息
		releasesDate, err := m.tmdbClient.GetReleasesDate(mediaFile.TmdbId)
		if err != nil {
			helpers.AppLogger.Errorf("查询tmdb电影分级信息失败, 下次重试, 失败原因: %v", err)
		}
		tmdbInfo.ReleasesDate = releasesDate.Results
	}
	m.MakeMediaFromTMDB(mediaFile, tmdbInfo)
	return nil
}

func (m *movieScrapeImpl) GenrateCategory(mediaFile *models.ScrapeMediaFile) error {
	// 处理二级分类, 关闭或者其他类型不计算二级分类
	if !mediaFile.EnableCategory || mediaFile.MediaType == models.MediaTypeOther {
		return nil
	}
	categoryName, scrapePathCategory := m.categoryImpl.DoCategory(mediaFile)
	if categoryName == "" && scrapePathCategory == nil {
		// 无法确定二级分类则停止刮削
		helpers.AppLogger.Errorf("根据流派ID和语言确定电影的二级分类失败, 文件名: %s", mediaFile.Name)
		mediaFile.Failed("根据流派ID和语言确定电影的二级分类失败，停止刮削")
		// 释放信号量
		return errors.New("根据流派ID和语言确定电影的二级分类失败")
	}
	mediaFile.CategoryName = categoryName
	mediaFile.ScrapePathCategoryId = scrapePathCategory.ID
	// 保存
	mediaFile.Save()
	helpers.AppLogger.Infof("根据流派ID和语言确定二级分类: %s, 分类目录ID:%s", categoryName, scrapePathCategory.FileId)
	return nil
}

// 生成新文件名和新文件夹名
// 如果仅刮削，则返回原始名字
// 否则根据文件名模板来生成文件名
func (m *movieScrapeImpl) GenerateNewName(mediaFile *models.ScrapeMediaFile) {
	remotePath := mediaFile.GetRemoteMoviePath()
	mediaFile.VideoExt = filepath.Ext(mediaFile.VideoFilename)
	oldPathName := filepath.Base(remotePath)
	baseName := strings.TrimSuffix(filepath.Base(mediaFile.VideoFilename), mediaFile.VideoExt)
	if mediaFile.ScrapeType == models.ScrapeTypeOnly {
		mediaFile.NewPathName = oldPathName
		mediaFile.NewVideoBaseName = baseName
		mediaFile.Media.Path = oldPathName
		mediaFile.Media.PathId = mediaFile.PathId
		return
	}
	folderTemplate := m.scrapePath.FolderNameTemplate
	if m.scrapePath.FolderNameTemplate == "" && remotePath == "" {
		folderTemplate = "{title} ({year})"
	}
	// 根据命名规则生成文件夹名称
	if m.scrapePath.FolderNameTemplate == "" {
		if remotePath == "" {
			mediaFile.NewPathName = mediaFile.GenerateNameByTemplate(folderTemplate)
		} else {
			mediaFile.NewPathName = oldPathName
		}
	} else {
		// 检查文件夹模板是否有效
		if !models.CheckFolderTemplateValid(m.scrapePath.FolderNameTemplate) {
			helpers.AppLogger.Errorf("文件夹模板包含不允许的变量，使用默认模板 {title} ({year})")
			mediaFile.NewPathName = mediaFile.GenerateNameByTemplate("{title} ({year})")
		} else {
			mediaFile.NewPathName = mediaFile.GenerateNameByTemplate(m.scrapePath.FolderNameTemplate)
		}
	}
	if m.scrapePath.FileNameTemplate == "" {
		mediaFile.NewVideoBaseName = baseName
	} else {
		mediaFile.NewVideoBaseName = mediaFile.GenerateNameByTemplate(m.scrapePath.FileNameTemplate) // 不含扩展名
	}
	mediaFile.Media.Path = filepath.Join(mediaFile.DestPath, mediaFile.CategoryName, mediaFile.NewPathName)
	mediaFile.Media.VideoFileName = mediaFile.NewVideoBaseName + mediaFile.VideoExt
	// 保存
	mediaFile.Save()
	mediaFile.Media.Save()
}

// 先命中一个syncPath，使用newPath
func (m *movieScrapeImpl) SyncFilesToSTRMPath(mediaFile *models.ScrapeMediaFile, files []uploadFile) {
	syncPath := m.scrapePath.GetSyncPathByPath(mediaFile.Media.Path)
	if syncPath == nil {
		helpers.AppLogger.Errorf("未命中任何STRM同步目录, 无法将文件同步到STRM目录 %s", mediaFile.Media.Path)
		return
	}
	// 先生成STRM文件
	// 1. 构造STRM文件路径
	syncStrm := syncstrm.NewSyncStrmFromSyncPath(syncPath)
	syncStrm.ProcessStrmFile(&syncstrm.SyncFileCache{
		Path:          mediaFile.Media.Path,
		ParentId:      mediaFile.Media.PathId,
		FileType:      v115open.TypeFile,
		FileName:      mediaFile.Media.VideoFileName,
		FileId:        mediaFile.Media.VideoFileId,
		PickCode:      mediaFile.Media.VideoPickCode,
		OpenlistSign:  mediaFile.Media.VideoOpenListSign,
		FileSize:      0,
		MTime:         0,
		IsVideo:       true,
		IsMeta:        false,
		LocalFilePath: filepath.Join(syncPath.LocalPath, mediaFile.Media.Path, mediaFile.NewVideoBaseName+".strm"),
	})
	models.DeleteSyncRecordById(syncStrm.Sync.ID)
	if files == nil {
		return
	}
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

func (m *movieScrapeImpl) UploadMovieScrapeFile(mediaFile *models.ScrapeMediaFile) error {
	if mediaFile.NewPathId == "" {
		helpers.AppLogger.Errorf("父文件夹不存在，无法上传文件元数据 %s", mediaFile.NewPathName)
		return fmt.Errorf("父文件夹不存在")
	}
	helpers.AppLogger.Infof("开始上传文件元数据 %s", mediaFile.NewPathName)
	// 整理要上传的文件
	files := m.GetMovieUploadFiles(mediaFile)
	// 将文件同步到STRM同步目录内
	m.SyncFilesToSTRMPath(mediaFile, files)
	// 如果是本地文件直接移动到目标位置
	ok, err := m.MoveLocalTempFileToDest(mediaFile, files)
	if err == nil {
		return nil
	}
	if !ok {
		// 标记为失败
		return err
	}
	for _, file := range files {
		err := models.AddUploadTaskFromMediaFile(mediaFile, m.scrapePath, file.FileName, file.SourcePath, filepath.Join(file.DestPath, file.FileName), file.DestPathId, false)
		if err != nil {
			helpers.AppLogger.Errorf("添加上传任务 %s 失败, 失败原因: %v", file.FileName, err)
		}
	}
	return nil
}

// 收集要上传的文件
// 视频文件对应的nfo，图片
// {PathId: string, FileName: string}
func (m *movieScrapeImpl) GetMovieUploadFiles(mediaFile *models.ScrapeMediaFile) []uploadFile {
	destPath := mediaFile.GetDestFullMoviePath()
	destPathId := mediaFile.NewPathId
	movieSourcePath := mediaFile.GetTmpFullMoviePath()
	// 将movieSourcePath目录下所有文件全部上传
	files, err := os.ReadDir(movieSourcePath)
	if err != nil {
		helpers.AppLogger.Errorf("读取目录 %s 失败: %v", movieSourcePath, err)
		return nil
	}
	fileList := make([]uploadFile, 0)
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fileList = append(fileList, uploadFile{
			ID:         fmt.Sprintf("%d", mediaFile.ID),
			FileName:   file.Name(),
			SourcePath: filepath.ToSlash(filepath.Join(movieSourcePath, file.Name())),
			DestPath:   destPath,
			DestPathId: destPathId,
		})
	}
	// nfoName := m.GetMovieRealName(mediaFile, "", "nfo")
	// nfoPath := filepath.Join(movieSourcePath, nfoName)
	// if helpers.PathExists(nfoPath) {
	// 	file := uploadFile{
	// 		ID:         fmt.Sprintf("%d", mediaFile.ID),
	// 		FileName:   nfoName,
	// 		SourcePath: nfoPath,
	// 		DestPath:   destPath,
	// 		DestPathId: destPathId,
	// 	}

	// 	fileList = append(fileList, file)
	// }
	// imageList := []string{"poster.jpg", "clearlogo.jpg", "clearart.jpg", "square.jpg", "logo.jpg", "fanart.jpg", "backdrop.jpg", "background.jpg", "4kbackground.jpg", "thumb.jpg", "banner.jpg", "disc.jpg"}
	// for _, im := range imageList {
	// 	name := m.GetMovieRealName(mediaFile, im, "image")
	// 	sPath := filepath.Join(movieSourcePath, name)
	// 	if helpers.PathExists(sPath) {
	// 		file := uploadFile{
	// 			ID:         fmt.Sprintf("%d", mediaFile.ID),
	// 			FileName:   name,
	// 			SourcePath: sPath,
	// 			DestPath:   destPath,
	// 			DestPathId: destPathId,
	// 		}
	// 		fileList = append(fileList, file)
	// 	}
	// }
	return fileList
}

// 将本地临时文件移动到本地目标路径
func (m *movieScrapeImpl) MoveLocalTempFileToDest(mediaFile *models.ScrapeMediaFile, files []uploadFile) (bool, error) {
	if mediaFile.SourceType != models.SourceTypeLocal {
		return true, fmt.Errorf("非本地文件刮削，无法移动到目标位置")
	}
	for _, file := range files {
		tempPath := file.SourcePath
		if !helpers.PathExists(tempPath) {
			continue
		}
		destPath := filepath.Join(file.DestPath, file.FileName)
		err := helpers.MoveFile(tempPath, destPath, true)
		if err != nil {
			helpers.AppLogger.Errorf("移动刮削临时文件 %s 到整理目标位置 %s 失败: %v", tempPath, destPath, err)
			return false, err
		}
		helpers.AppLogger.Infof("移动刮削临时文件 %s 到整理目标位置 %s 成功", tempPath, destPath)
	}
	return true, nil
}

// 创建父文件夹，电影是电影目录
func (m *movieScrapeImpl) MakeParentPath(mediaFile *models.ScrapeMediaFile, categoryMap map[uint]string) error {
	if mediaFile.ScrapeType == models.ScrapeTypeOnly {
		mediaFile.NewPathId = mediaFile.PathId
		mediaFile.Save()
		helpers.AppLogger.Infof("仅刮削模式下，使用旧目录存放元数据：%s，目录ID：%s", mediaFile.Path, mediaFile.PathId)
		return nil
	}
	parentId := mediaFile.DestPathId
	if mediaFile.ScrapePathCategoryId > 0 {
		if category, ok := categoryMap[mediaFile.ScrapePathCategoryId]; ok {
			parentId = category
		}
	}
	destFullPath := mediaFile.GetDestFullMoviePath()
	helpers.AppLogger.Infof("影视剧文件夹，目标路径：%s，根目录ID：%s", destFullPath, parentId)
	newPathId, err := m.renameImpl.CheckAndMkDir(destFullPath, mediaFile.DestPath, mediaFile.DestPathId)
	if err != nil {
		helpers.AppLogger.Errorf("创建父文件夹失败: %v", err)
		return err
	}
	mediaFile.NewPathId = newPathId
	mediaFile.Media.PathId = newPathId
	mediaFile.Save()
	mediaFile.Media.Save()
	return nil
}

// 检查是否完成，不用管上传（上传负责删除自己产生的临时文件）
// 发送通知
// 删除来源路径
func (m *movieScrapeImpl) FinishMovie(mediaFile *models.ScrapeMediaFile) {
	mediaFile.StatusFinish()
	if mediaFile.SourceType == models.SourceTypeLocal {
		mediaFile.RemoveTmpFiles(nil)
	}
	// 发送通知
	if mediaFile.Media != nil {
		ctx := context.Background()
		notif := &models.Notification{
			Type:      models.ScrapeFinished,
			Title:     fmt.Sprintf("✅ %s 刮削整理完成", mediaFile.Name),
			Content:   fmt.Sprintf("📊 类型: 电影, 类别: %s, 分辨率: %s\n⏰ 时间: %s", mediaFile.CategoryName, mediaFile.Resolution, time.Now().Format("2006-01-02 15:04:05")),
			Image:     mediaFile.Media.PosterPath,
			Timestamp: time.Now(),
			Priority:  models.NormalPriority,
		}
		if notificationmanager.GlobalEnhancedNotificationManager != nil {
			if err := notificationmanager.GlobalEnhancedNotificationManager.SendNotification(ctx, notif); err != nil {
				helpers.AppLogger.Errorf("发送电影刮削完成通知失败: %v", err)
			}
		}
	}
	if mediaFile.ScrapeType == models.ScrapeTypeOnly || mediaFile.RenameType != models.RenameTypeMove {
		// 如果仅刮削，跳过
		// 如果不是移动模式，跳过
		// 如果不强制删除来源目录，跳过
		// 如果视频在来源根目录，跳过
		helpers.AppLogger.Infof("视频 %s 存在不符合删除来源目录的条件，跳过删除来源目录: %s", mediaFile.Name, mediaFile.Path)
		return
	}
	err := m.renameImpl.RemoveMediaSourcePath(mediaFile, m.scrapePath)
	if err != nil {
		helpers.AppLogger.Errorf("删除来源路径 %s 失败: %v", mediaFile.PathId, err)
	}
}

func (m *movieScrapeImpl) CreateMediaFromNfo(mediaFile *models.ScrapeMediaFile) error {
	if mediaFile.NfoFileId == "" {
		return fmt.Errorf("其他类型必须有nfo文件")
	}
	// 读取nfo文件内容
	nfoContent, err := m.renameImpl.ReadFileContent(mediaFile.NfoPickCode)
	if err != nil {
		return err
	}
	// 解析nfo文件
	movie, err := helpers.ReadMovieNfo(nfoContent)
	if err != nil {
		helpers.AppLogger.Errorf("解析nfo文件 %s 路径 %s 失败: %v", mediaFile.NfoPath, mediaFile.Path, err)
		return err
	}
	helpers.AppLogger.Infof("已从nfo文件中读取到媒体信息，名称：%s, 年份：%d, 番号: %s, TmdbID: %d", movie.Title, movie.Year, movie.Num, movie.TmdbId)
	var media *models.Media
	existsMedia, _ := models.GetMediaByName(models.MediaTypeMovie, movie.Title, movie.Year)
	if existsMedia != nil {
		media = existsMedia
	} else {
		media, _ = models.MakeMovieMediaFromNfo(movie)
		err := media.Save()
		if err != nil {
			return err
		}
		helpers.AppLogger.Infof("使用nfo文件中的内容创建刮削信息，ID：%d, 名称：%s, 年份：%d, 番号: %s, TmdbID: %d", media.ID, movie.Title, movie.Year, movie.Num, movie.TmdbId)
	}
	mediaFile.MediaId = media.ID
	mediaFile.Media = media
	mediaFile.Name = media.Name
	mediaFile.Year = media.Year
	mediaFile.TmdbId = media.TmdbId
	helpers.AppLogger.Infof("使用nfo中的信息补全刮削视频文件的信息，名称：%s, 年份：%d, 番号: %s, TmdbID: %d", media.Name, media.Year, movie.Num, media.TmdbId)
	fileErr := mediaFile.Save()
	if fileErr != nil {
		return fileErr
	}
	return nil
}

func (sm *movieScrapeImpl) GenerateMovieNfo(mediaFile *models.ScrapeMediaFile, localTempPath string, nfoName string, excludeNoImageActor bool) error {
	// 生成nfo文件
	nfoPath := filepath.Join(localTempPath, nfoName)
	rates := []helpers.Rating{
		{
			Name:  "tmdb",
			Max:   10,
			Value: mediaFile.Media.VoteAverage,
			Votes: mediaFile.Media.VoteCount,
		},
	}
	// 解析tmdb genre
	genres := make([]string, 0)
	for _, genre := range mediaFile.Media.Genres {
		genres = append(genres, genre.Name)
	}
	// 解析视频流
	videoStreams := make([]helpers.StreamVideo, 0)
	if mediaFile.VideoCodecJson != "" {
		videoStreams = append(videoStreams, helpers.StreamVideo{
			Codec:             mediaFile.VideoCodec.Codec,
			Micodec:           mediaFile.VideoCodec.Micodec,
			Bitrate:           mediaFile.VideoCodec.Bitrate,
			Aspect:            mediaFile.VideoCodec.Aspect,
			AspectRatio:       fmt.Sprintf("%.3f", mediaFile.VideoCodec.AspectRatio),
			Width:             mediaFile.VideoCodec.Width,
			Height:            mediaFile.VideoCodec.Height,
			DurationInSeconds: mediaFile.VideoCodec.DurationInSeconds,
			Duration:          mediaFile.VideoCodec.DurationInMinutes,
			FrameRate:         mediaFile.VideoCodec.Framerate,
		})
	}
	// 解析音频流
	audioStreams := make([]helpers.StreamAudio, 0)
	if len(mediaFile.AudioCodec) > 0 {
		for _, au := range mediaFile.AudioCodec {
			audioStreams = append(audioStreams, helpers.StreamAudio{
				Codec:        au.Codec,
				Micodec:      au.Micodec,
				Bitrate:      au.Bitrate,
				SamplingRate: au.SamplingRate,
				Channels:     au.Channels,
				Language:     au.Language,
			})
		}
	}
	subtitleStreams := make([]helpers.StreamSubtitle, 0)
	if len(mediaFile.SubtitleCodec) > 0 {
		// 解析字幕流
		for _, sub := range mediaFile.SubtitleCodec {
			subtitleStreams = append(subtitleStreams, helpers.StreamSubtitle{
				Language: sub.Language,
				Codec:    sub.Codec,
				Micodec:  sub.Micodec,
			})
		}
	}
	// 取第一张poster
	// 取第一张backdrop
	poster := mediaFile.Media.PosterPath
	backdrop := mediaFile.Media.BackdropPath
	thumbs := make([]helpers.Thumb, 0)
	thumbs = append(thumbs, helpers.Thumb{
		Aspect: "poster",
		Link:   poster,
	})
	thumbs = append(thumbs, helpers.Thumb{
		Aspect: "backdrop",
		Link:   backdrop,
	})
	// 包含中文的情况
	has, result := helpers.ChineseToPinyin(mediaFile.Media.Name)
	originalTitle := mediaFile.Media.OriginalName
	SortTitle := mediaFile.Media.Name
	if has {
		originalTitle = fmt.Sprintf("%s #(%s)", mediaFile.Media.Name, result)
		SortTitle = fmt.Sprintf("%s #(%s)", result, mediaFile.Media.Name)
	}
	m := &helpers.Movie{
		Title:         mediaFile.Media.Name,
		OriginalTitle: originalTitle,
		SortTitle:     SortTitle,
		Ratings: struct {
			Rating []helpers.Rating `xml:"rating,omitempty"`
		}{
			Rating: rates,
		},
		UserRating: mediaFile.Media.VoteAverage,
		Outline:    fmt.Sprintf("<![CDATA[%s]]>", mediaFile.Media.Overview),
		Plot:       fmt.Sprintf("<![CDATA[%s]]>", mediaFile.Media.Overview),
		Tagline:    mediaFile.Media.Tagline,
		Runtime:    mediaFile.Media.Runtime,
		Id:         mediaFile.Media.ImdbId,
		TmdbId:     mediaFile.Media.TmdbId,
		ImdbId:     mediaFile.Media.ImdbId,
		Uniqueid: []helpers.UniqueId{
			{
				Id:      mediaFile.Media.ImdbId,
				Type:    "imdb",
				Default: true,
			},
			{
				Id:      fmt.Sprintf("%d", mediaFile.Media.TmdbId),
				Type:    "tmdb",
				Default: false,
			},
		},
		Genre:     genres,
		Director:  mediaFile.Media.Director,
		Premiered: mediaFile.Media.ReleaseDate,
		Year:      mediaFile.Media.Year,
		DateAdded: time.Now().Format("2006-01-02"),
		FileInfo: struct {
			StreamDetails struct {
				Video    []helpers.StreamVideo    `xml:"video,omitempty"`
				Audio    []helpers.StreamAudio    `xml:"audio,omitempty"`
				Subtitle []helpers.StreamSubtitle `xml:"subtitle,omitempty"`
			} `xml:"streamdetails,omitempty"`
		}{
			StreamDetails: struct {
				Video    []helpers.StreamVideo    `xml:"video,omitempty"`
				Audio    []helpers.StreamAudio    `xml:"audio,omitempty"`
				Subtitle []helpers.StreamSubtitle `xml:"subtitle,omitempty"`
			}{
				Video:    videoStreams,
				Audio:    audioStreams,
				Subtitle: subtitleStreams,
			},
		},
		Thumb: thumbs,
		Fanart: &helpers.Fanart{
			Thumb: []helpers.Thumb{
				{
					Aspect: "fanart",
					Link:   backdrop,
				},
			},
		},
	}
	if excludeNoImageActor {
		m.Actor = make([]helpers.Actor, 0)
		for _, actor := range mediaFile.Media.Actors {
			if actor.Thumb != "" {
				m.Actor = append(m.Actor, actor)
			}
		}
	} else {
		m.Actor = mediaFile.Media.Actors
	}
	err := helpers.WriteMovieNfo(m, nfoPath)
	if err != nil {
		helpers.AppLogger.Errorf("生成电影nfo文件失败，文件路径：%s 错误： %v", nfoPath, err)
		return err
	}
	helpers.AppLogger.Infof("生成电影nfo文件成功，文件路径：%s", nfoPath)
	return nil
}

func (m *movieScrapeImpl) MakeMediaFromTMDB(mediaFile *models.ScrapeMediaFile, tmdbInfo *models.TmdbInfo) {
	if mediaFile.MediaId == 0 {
		mediaFile.Media = &models.Media{
			ScrapePathId: mediaFile.ScrapePathId,
			MediaType:    mediaFile.MediaType,
			Name:         mediaFile.Name,
			Year:         mediaFile.Year,
			TmdbId:       mediaFile.TmdbId,
			Status:       models.MediaStatusUnScraped,
		}
		helpers.AppLogger.Infof("创建新的Media对象: %s, TMDBID=%d, 类型=%s", mediaFile.Media.Name, mediaFile.Media.TmdbId, mediaFile.Media.MediaType)
	} else {
		mediaFile.QueryRelation()
	}
	mediaFile.Media.FillInfoByTmdbInfo(tmdbInfo)
	mediaFile.Media.Save()
	mediaFile.MediaId = mediaFile.Media.ID
	mediaFile.Name = mediaFile.Media.Name
	mediaFile.Year = mediaFile.Media.Year
	mediaFile.Save()
}

func (m *movieScrapeImpl) GetMovieRealName(sm *models.ScrapeMediaFile, name string, filetype string) string {
	if filetype == "nfo" {
		return fmt.Sprintf("%s.nfo", sm.NewVideoBaseName)
	}
	if sm.ScrapeType == models.ScrapeTypeOnly {
		return fmt.Sprintf("%s-%s", sm.NewVideoBaseName, name)
	} else {
		return name
	}
}

// 仅刮削的重新刮削逻辑：将对应刮削记录修改为待刮削
// 刮削和整理的重新刮削逻辑：
//   - 移动：将文件移动回源目录，如果源目录已删除，则新建同名目录并修改path、pathid等
//   - 复制：检查源目录和源视频文件是否依然存在，如果存在则删除目录目录，如果不存在则将目标文件移动回源目录（源目录不存在则新建），并修改videofileid, videofilename, videopickcode,pathid, pathname等值
//   - 软链接、硬链接：同复制
//
// 其他类型不支持重新刮削
func (m *movieScrapeImpl) Rollback(mediaFile *models.ScrapeMediaFile) error {
	if mediaFile.MediaType == models.MediaTypeOther {
		return nil
	}
	mediaFile.QueryRelation()
	newBaseName := fmt.Sprintf("%s (%d) {tmdbid-%d}", mediaFile.Name, mediaFile.Year, mediaFile.TmdbId)
	if mediaFile.ScrapeType == models.ScrapeTypeOnly {
		files := make([]models.WillDeleteFile, 0)
		// 删除所有上传的元数据
		destPath := mediaFile.GetDestFullMoviePath()
		nfoName := m.GetMovieRealName(mediaFile, "", "nfo")
		files = append(files, models.WillDeleteFile{FullFilePath: filepath.Join(destPath, nfoName)})
		imageList := []string{"poster.jpg", "clearlogo.jpg", "clearart.jpg", "square.jpg", "logo.jpg", "fanart.jpg", "backdrop.jpg", "background.jpg", "4kbackground.jpg", "thumb.jpg", "banner.jpg", "disc.jpg"}
		for _, im := range imageList {
			imageName := m.GetMovieRealName(mediaFile, im, "image")
			files = append(files, models.WillDeleteFile{FullFilePath: filepath.Join(destPath, imageName)})
		}
		// 删除这些文件
		err := m.renameImpl.CheckAndDeleteFiles(mediaFile, files)
		if err != nil {
			helpers.AppLogger.Errorf("删除已上传的元数据文失败: %v", err)
			return err
		}
		helpers.AppLogger.Infof("删除已上传的元数据文件成功: %v", files)
		// 字幕改名
		if mediaFile.Media.SubtitleFiles != nil {
			for _, sub := range mediaFile.Media.SubtitleFiles {
				m.renameImpl.Rename(sub.FileId, newBaseName+filepath.Ext(sub.FileName))
			}
		}
		// 视频文件改名
		m.renameImpl.Rename(mediaFile.Media.VideoFileId, newBaseName+mediaFile.VideoExt)
		// 文件夹改名
		m.renameImpl.Rename(mediaFile.PathId, newBaseName)
	}
	// 如果是移动则使用现在的处理方式
	if mediaFile.ScrapeType == models.ScrapeTypeScrapeAndRename || mediaFile.ScrapeType == models.ScrapeTypeOnlyRename {
		// 检查目录是否存在，如果存在则改名字，如果不存在则创建
		parentPath := filepath.Dir(mediaFile.Path)
		var newPath string
		var pathId string
		var existsPathId string = ""
		if mediaFile.Path == mediaFile.SourcePath {
			parentPath = mediaFile.SourcePath
			newPath = mediaFile.SourcePath
		} else {
			newPath = filepath.Join(parentPath, newBaseName)
		}
		if mediaFile.RenameType != models.RenameTypeMove && parentPath != mediaFile.SourcePath {
			// 先检查旧文件夹是否存在
			var eerr error
			existsPathId, eerr = m.renameImpl.ExistsAndRename(mediaFile.PathId, newBaseName)
			if eerr != nil {
				helpers.AppLogger.Errorf("重命名旧文件夹 %s 失败: %v", mediaFile.PathId, eerr)
				return eerr
			}
		}
		if existsPathId == "" {
			if parentPath != mediaFile.SourcePath {
				var err error
				pathId, err = m.renameImpl.CheckAndMkDir(newPath, mediaFile.SourcePath, mediaFile.SourcePathId)
				if err != nil {
					helpers.AppLogger.Errorf("创建父文件夹 %s 失败: %v", newPath, err)
					return err
				}
			} else {
				pathId = mediaFile.SourcePathId
				newPath = mediaFile.SourcePath
			}
		} else {
			pathId = existsPathId
		}
		// 将文件移动回pathId
		// 先移动字幕文件
		if mediaFile.Media.SubtitleFiles != nil {
			for _, sub := range mediaFile.Media.SubtitleFiles {
				exists := false
				if mediaFile.SourceType != models.SourceType115 {
					sub.FileId = strings.Replace(sub.FileId, mediaFile.Media.PathId, pathId, 1)
				}
				if mediaFile.RenameType != models.RenameTypeMove {
					// 检查文件是否存在，存在就改名，不存在就移动
					newSubId, _ := m.renameImpl.ExistsAndRename(sub.FileId, newBaseName+filepath.Ext(sub.FileName))
					if newSubId != "" {
						exists = true
					}
				}
				if exists {
					continue
				}
				moveFile := models.MoveNewFileToSourceFile{
					FileId:       sub.FileId,
					FileFullPath: filepath.Join(newPath, newBaseName, filepath.Ext(sub.FileName)),
					PathId:       pathId,
				}
				merr := m.renameImpl.MoveFiles(moveFile)
				if merr != nil {
					continue
				}
				// 改名
				m.renameImpl.Rename(moveFile.FileId, newBaseName+filepath.Ext(sub.FileName))
			}
		}
		exists := false
		if mediaFile.RenameType != models.RenameTypeMove {
			videoFileId := mediaFile.VideoFileId
			if mediaFile.SourceType != models.SourceType115 {
				videoFileId = strings.Replace(videoFileId, mediaFile.PathId, pathId, 1)
			}
			// 检查文件是否存在，存在就改名，不存在就移动
			newVideoId, _ := m.renameImpl.ExistsAndRename(videoFileId, newBaseName+mediaFile.VideoExt)
			if newVideoId != "" {
				exists = true
			}
		}
		if !exists {
			// 再移动视频文件
			moveFile := models.MoveNewFileToSourceFile{
				FileId: mediaFile.Media.VideoFileId,
				PathId: pathId,
			}
			merr := m.renameImpl.MoveFiles(moveFile)
			if merr != nil {
				helpers.AppLogger.Errorf("移动视频文件失败: %v", merr)
				return merr
			}
			if mediaFile.SourceType != models.SourceType115 {
				moveFile.FileId = strings.Replace(moveFile.FileId, mediaFile.Media.PathId, pathId, 1)
			}
			// 改名
			m.renameImpl.Rename(moveFile.FileId, newBaseName+mediaFile.VideoExt)
		}
		// 删除目标目录
		derr := m.renameImpl.DeleteDir(mediaFile.Media.Path, mediaFile.Media.PathId)
		if derr != nil {
			helpers.AppLogger.Errorf("删除目标目录失败: %v", derr)
			return derr
		}
	}
	// 删除media表的记录
	db.Db.Delete(&models.Media{}, mediaFile.MediaId)
	// 删除scrape_media_file表的记录
	db.Db.Delete(&models.ScrapeMediaFile{}, mediaFile.ID)
	return nil
}
