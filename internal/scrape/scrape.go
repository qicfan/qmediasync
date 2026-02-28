package scrape

import (
	"Q115-STRM/internal/baidupan"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/openlist"
	"Q115-STRM/internal/scrape/scan"
	"Q115-STRM/internal/v115open"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// 刮削入口
// 流程：
// 1. 扫描并入库要刮削的文件
// 2. 从数据库查询待刮削的文件
// 3. 识别
// 4. 分类（如果没有开启二级分类则跳过）
// 5. 从tmdb和fanart.tv刮削元数据 （如果是仅整理则跳过）
// 6. 生成nfo（如果是仅整理则跳过）
// 7. ffprobe提取视频流（如果是仅整理则跳过）
// 8. 上传刮削到的元数据（图片+nfo)（如果是仅整理则跳过）
// 9. 重命名视频文件和文件夹（如果是仅刮削则跳过）
// 扫描文件
type scanImpl interface {
	GetNetFileFiles() error
	CheckPathExists() error
}

type IdentifyImpl interface {
	Identify(mediaFile *models.ScrapeMediaFile) error
}

type TmdbImpl interface {
	CheckByNameAndYear(name string, year int, switchYear bool) (string, int64, int, error)
	CheckByTmdbId(tmdbId int64) (string, int, error)
}

type categoryImpl interface {
	DoCategory(mediaFile *models.ScrapeMediaFile) (string, *models.ScrapePathCategory)
}

type renameImpl interface {
	RenameAndMove(mediaFile *models.ScrapeMediaFile, destPath, destPathId, newName string) error
	CheckAndMkDir(destFullPath, rootPath, rootPathId string) (string, error)
	RemoveMediaSourcePath(mediaFile *models.ScrapeMediaFile, sp *models.ScrapePath) error
	ReadFileContent(fileId string) ([]byte, error)
	CheckAndDeleteFiles(mediaFile *models.ScrapeMediaFile, files []models.WillDeleteFile) error
	MoveFiles(f models.MoveNewFileToSourceFile) error
	DeleteDir(path, pathId string) error
	Rename(fileId, newName string) error
	ExistsAndRename(fileId, newName string) (string, error)
}

// 刮削
type scrapeImpl interface {
	Start() error
	Process(*models.ScrapeMediaFile) error
	Scrape(*models.ScrapeMediaFile) error
	Rollback(*models.ScrapeMediaFile) error
}

type Scrape struct {
	// 刮削路径
	scrapePath     *models.ScrapePath
	scanImpl       scanImpl
	scrapeImpl     scrapeImpl
	ctx            context.Context    // 用来控制是否取消任务
	ctxCancel      context.CancelFunc // 用来取消任务
	V115Client     *v115open.OpenClient
	OpenlistClient *openlist.Client
	BaiduPanClient *baidupan.Client
}

// scrapePath 要刮削的目录
// ctx 控制刮削任务是否取消
func NewScrape(scrapePath *models.ScrapePath) *Scrape {
	cancelCtx, ctxCancel := context.WithCancel(context.Background())
	return &Scrape{
		scrapePath: scrapePath,
		ctx:        cancelCtx,
		ctxCancel:  ctxCancel,
	}
}

func (s *Scrape) initOpenClient() error {
	if s.scrapePath.SourceType == models.SourceTypeLocal {
		return nil
	}
	account, err := s.scrapePath.GetAccount()
	if err != nil {
		helpers.AppLogger.Errorf("获取刮削目录 %s 账号失败: %v", s.scrapePath.SourcePath, err)
		return err
	}
	switch s.scrapePath.SourceType {
	case models.SourceType115:
		s.V115Client = account.Get115Client()
	case models.SourceTypeOpenList:
		s.OpenlistClient = account.GetOpenListClient()
	case models.SourceTypeBaiduPan:
		s.BaiduPanClient = account.GetBaiDuPanClient()
	}
	return nil
}

func (s *Scrape) CreateTmpRotDir() {
	// 创建临时目录
	s.scrapePath.ScrapeRootPath = filepath.Join(helpers.ConfigDir, "tmp", "刮削临时文件", fmt.Sprintf("%d", s.scrapePath.ID), "电影或其他")
	if s.scrapePath.MediaType == models.MediaTypeTvShow {
		s.scrapePath.ScrapeRootPath = filepath.Join(helpers.ConfigDir, "tmp", "刮削临时文件", fmt.Sprintf("%d", s.scrapePath.ID), "电视剧")
	}
	if err := os.MkdirAll(s.scrapePath.ScrapeRootPath, 0777); err != nil {
		helpers.AppLogger.Errorf("创建临时目录失败: %v", err)
		return
	}
}

func (s *Scrape) init() {
	// 1. 准备115或者Openlist客户端
	err := s.initOpenClient()
	if err != nil {
		helpers.AppLogger.Errorf("初始化刮削目录 %s 失败: %v", s.scrapePath.SourcePath, err)
		return
	}
	// 2. 记录开始状态
	s.scrapePath.SetRunning()
	// 3. 创建刮削临时目录
	s.CreateTmpRotDir()
	switch s.scrapePath.SourceType {
	case models.SourceTypeLocal:
		s.scanImpl = scan.NewLocalScanImpl(s.scrapePath, s.ctx)
	case models.SourceType115:
		s.scanImpl = scan.New115ScanImpl(s.scrapePath, s.V115Client, s.ctx)
	case models.SourceTypeOpenList:
		s.scanImpl = scan.NewOpenlistScanImpl(s.scrapePath, s.OpenlistClient, s.ctx)
	case models.SourceTypeBaiduPan:
		s.scanImpl = scan.NewBaiduPanScanImpl(s.scrapePath, s.BaiduPanClient, s.ctx)
	}
	// 确定扫描接口，识别接口，刮削接口，重命名接口
	if s.scrapePath.MediaType == models.MediaTypeTvShow {
		s.scrapeImpl = NewTvShowScrapeImpl(s.scrapePath, s.ctx, s.V115Client, s.OpenlistClient, s.BaiduPanClient)
	} else {
		s.scrapeImpl = NewMovieScrapeImpl(s.scrapePath, s.ctx, s.V115Client, s.OpenlistClient, s.BaiduPanClient)
	}
}

// 开始扫描识别和刮削
func (s *Scrape) Start() bool {
	// 启动一个协程定时监控是否需要退出
	helpers.AppLogger.Infof("开始刮削目录 %s", s.scrapePath.SourcePath)
	s.scrapePath.SetRunning()
	defer func() {
		s.scrapePath.SetNotRunning()
		select {
		case <-s.ctx.Done():
			return
		default:
			s.ctxCancel() // 通知其他相关协程都退出
		}
	}()
	// 把刮削中的全部改成待刮削
	models.UpdateScrapeMediaStatus(models.ScrapeMediaStatusScraping, models.ScrapeMediaStatusScanned, s.scrapePath.ID)
	s.init()
	// 检查来源目录和目标目录是否存在
	err := s.scanImpl.CheckPathExists()
	if err != nil {
		helpers.AppLogger.Errorf("检查来源目录 %s 或者目标目录 %s 是否异常: %v", s.scrapePath.SourcePathId, s.scrapePath.DestPathId, err)
		return false
	}
	// 先生成所有二级分类
	s.scrapePath.V115Client = s.V115Client
	s.scrapePath.OpenListClient = s.OpenlistClient
	s.scrapePath.BaiduPanClient = s.BaiduPanClient
	s.scrapePath.GenerateCategory()
	// 获取视频文件列表并从文件名中提取媒体信息用来刮削
	eerr := s.scanImpl.GetNetFileFiles()
	if eerr != nil {
		helpers.AppLogger.Errorf("获取目录 %s 视频文件列表失败: %v", s.scrapePath.SourcePath, eerr)
		return false
	}
	helpers.AppLogger.Infof("获取目录 %s 视频文件列表成功", s.scrapePath.SourcePath)
	err = s.scrapeImpl.Start()
	if err != nil {
		helpers.AppLogger.Errorf("启动刮削 %s 失败: %v", s.scrapePath.SourcePath, err)
		return false
	}
	helpers.AppLogger.Infof("刮削整理 #%d %s 成功", s.scrapePath.ID, s.scrapePath.SourcePath)
	s.ctxCancel() // 通知其他相关协程都退出
	return true
}

func (s *Scrape) Stop() {
	helpers.AppLogger.Infof("停止刮削目录 %s", s.scrapePath.SourcePath)
	// 取消上下文，通知其他协程取消任务
	s.ctxCancel()
	helpers.AppLogger.Infof("刮削目录 %s 停止信号已发送", s.scrapePath.SourcePath)
	s.scrapePath.SetNotRunning()
}

// 识别错误就回滚所有刮削好的元数据
func (s *Scrape) Rollback(mediaFile *models.ScrapeMediaFile) error {
	s.init()
	if s.scrapeImpl == nil {
		helpers.AppLogger.Errorf("刮削实现为空")
		return errors.New("刮削实现为空")
	}
	if mediaFile == nil {
		helpers.AppLogger.Errorf("刮削文件为空")
		return errors.New("刮削文件为空")
	}
	return s.scrapeImpl.Rollback(mediaFile)
}
