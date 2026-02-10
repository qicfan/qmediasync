package scan

import (
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

type localFile struct {
	Id       string
	PickCode string
	Name     string
	Size     int64
	Path     string
}
type scanBaseImpl struct {
	ctx        context.Context
	scrapePath *models.ScrapePath
	BatchNo    string
	pathBuffer []string     // 如果pathTasks满了，先放入这里
	mu         sync.RWMutex // 保护缓冲区的锁
	wg         sync.WaitGroup
	pathTasks  chan string
}

func (s *scanBaseImpl) CheckIsRunning() bool {
	select {
	case <-s.ctx.Done():
		return false
	default:
		return true
	}
}

func (s *scanBaseImpl) addPathToTasks(path string) {
	select {
	case <-s.ctx.Done():
		return
	case s.pathTasks <- path:
		s.wg.Add(1)
		helpers.AppLogger.Infof("任务 %s 已添加到channel", path)
	default:
		// 超时，尝试添加到缓冲区
		s.addToBuffer(path)
	}
}

// bufferMonitor 监控缓冲区，尝试将缓冲区任务移入channel
func (s *scanBaseImpl) bufferMonitor(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			// 清空缓存
			s.mu.Lock()
			// 根据buffer长度操作wg
			s.wg.Add(-len(s.pathBuffer))
			s.pathBuffer = nil
			s.mu.Unlock()
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 定期尝试处理缓冲区
			s.tryDrainBuffer()
		}
	}
}

// tryDrainBuffer 尝试从缓冲区取出任务放入channel
func (s *scanBaseImpl) tryDrainBuffer() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.pathBuffer) == 0 {
		return
	}

	// 尝试将缓冲区任务移入channel
	for len(s.pathBuffer) > 0 {
		select {
		case <-s.ctx.Done():
			return
		case s.pathTasks <- s.pathBuffer[0]:
			helpers.AppLogger.Infof("从缓冲区移出任务 %s 到任务处理队列", s.pathBuffer[0])
			s.pathBuffer = s.pathBuffer[1:]
		default:
			// channel已满，停止尝试
			return
		}
	}
}

// addToBuffer 添加任务到缓冲区
func (s *scanBaseImpl) addToBuffer(task string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.CheckIsRunning() {
		return
	}
	s.wg.Add(1)
	s.pathBuffer = append(s.pathBuffer, task)
	helpers.AppLogger.Infof("任务 %s 已添加到缓冲区，当前缓冲区大小: %d", task, len(s.pathBuffer))
}

func (s *scanBaseImpl) processVideoFile(parentPath, pathId string, videoFiles, picFiles, nfoFiles, subFiles []*localFile) error {
	waitSaveFiles := make([]*models.ScrapeMediaFile, 0)
	// 记录下图片、nfo、字幕文件
	// 如果发现了视频文件，则寻找有没有视频文件对应的图片、nfo、字幕文件
	// 如果没发现视频文件，则清空
videoloop:
	// 处理videofiles以及对应的字幕等
	for _, videoFile := range videoFiles {
		// 检查是否停止任务
		if !s.CheckIsRunning() {
			return errors.New("任务被停止")
		}
		ext := filepath.Ext(videoFile.Name)
		baseName := strings.TrimSuffix(videoFile.Name, ext)
		imageList := make([]*models.MediaMetaFiles, 0)
		var nfoMetaFile *models.MediaMetaFiles
		subMetaFiles := make([]*models.MediaMetaFiles, 0)
		// 只有其他仅整理需要查找图片和nfo
		if s.scrapePath.MediaType == models.MediaTypeOther && s.scrapePath.ScrapeType == models.ScrapeTypeOnlyRename {
			imageAllowdList := []string{
				"poster",
				"fanart",
				"backdrop",
				"clearlogo",
				"thumb",
				"logo",
				"background",
				"4kbackground",
				baseName + "-poster",
				baseName + "-fanart",
				baseName + "-backdrop",
				baseName + "-clearlogo",
				baseName + "-thumb",
				baseName + "-logo",
				baseName + "-background",
				baseName + "-4kbackground",
			}
			nfoAllowdList := []string{
				"movie.nfo",
				"tvshow.nfo",
				"season.nfo",
			}
			// 查找对应的图片
		picloop:
			for _, picFile := range picFiles {
				imageBaseName := strings.TrimSuffix(picFile.Name, filepath.Ext(picFile.Name))
				if strings.HasPrefix(picFile.Name, baseName) || slices.Contains(imageAllowdList, imageBaseName) {
					imageList = append(imageList, &models.MediaMetaFiles{
						FileName: picFile.Name,
						FileId:   picFile.Id,
						PickCode: picFile.PickCode,
					})
					continue picloop
				}
			}
			// 查找对应的nfo
		nfoloop:
			for _, nfoFile := range nfoFiles {
				if strings.HasPrefix(nfoFile.Name, baseName) || slices.Contains(nfoAllowdList, nfoFile.Name) {
					nfoMetaFile = &models.MediaMetaFiles{
						FileName: nfoFile.Name,
						FileId:   nfoFile.Id,
						PickCode: nfoFile.PickCode,
					}
					break nfoloop
				}
				// 是否又sesson开头的nfo文件
				if strings.HasPrefix(nfoFile.Name, "season") {
					nfoMetaFile = &models.MediaMetaFiles{
						FileName: nfoFile.Name,
						FileId:   nfoFile.Id,
						PickCode: nfoFile.PickCode,
					}
					break nfoloop
				}
			}
		}
		// 查找是否有字幕文件
		for _, subFile := range subFiles {
			helpers.AppLogger.Infof("检查字幕文件 %s 是否匹配视频文件 %s", subFile.Name, videoFile.Name)
			if strings.HasPrefix(subFile.Name, baseName) {
				helpers.AppLogger.Infof("字幕文件 %s 匹配视频文件 %s", subFile.Name, videoFile.Name)
				subMetaFiles = append(subMetaFiles, &models.MediaMetaFiles{
					FileName: subFile.Name,
					FileId:   subFile.Id,
					PickCode: subFile.PickCode,
				})
			} else {
				helpers.AppLogger.Infof("字幕文件 %s 不匹配视频文件 %s", subFile.Name, videoFile.Name)
			}
		}
		// 生成ScrapeMediaFiles，并加入待保存列表
		if s.scrapePath.MediaType == models.MediaTypeOther && s.scrapePath.ScrapeType == models.ScrapeTypeOnlyRename && nfoMetaFile == nil {
			// 其他类型仅整理必须有nfo
			helpers.AppLogger.Infof("其他类型仅整理模式下，文件 %s 没有对应的nfo文件，跳过", videoFile.Name)
			continue videoloop
		}
		mediaFile := s.scrapePath.MakeScrapeMediaFile(parentPath, pathId, videoFile.Name, videoFile.Id, videoFile.PickCode)
		if nfoMetaFile != nil {
			mediaFile.NfoFileId = nfoMetaFile.FileId
			mediaFile.NfoPickCode = nfoMetaFile.PickCode
			mediaFile.NfoFileName = nfoMetaFile.FileName
		}
		if len(subMetaFiles) > 0 {
			jsonStr, _ := json.Marshal(subMetaFiles)
			mediaFile.SubtitleFileJson = string(jsonStr)
		}
		if len(imageList) > 0 {
			mediaFile.ImageFiles = imageList
			// 格式化成json字符串
			jsonStr, _ := json.Marshal(imageList)
			mediaFile.ImageFilesJson = string(jsonStr)
		}
		// 如果时电视剧，加入批次号
		if s.scrapePath.MediaType == models.MediaTypeTvShow {
			mediaFile.BatchNo = s.BatchNo
			// 识别季和集序号
			// 提取季和集
			// 填充电视剧和季目录（如果有季的话）
			if err := mediaFile.ExtractSeasonEpisode(s.scrapePath); err != nil {
				helpers.AppLogger.Errorf("提取季和集序号失败, 文件名: %s, 季 %d 集 %d 失败原因: %v", mediaFile.VideoFilename, mediaFile.SeasonNumber, mediaFile.EpisodeNumber, err)
				continue
			}
		}
		mediaFile.Status = models.ScrapeMediaStatusScanned
		mediaFile.ScanTime = time.Now().Unix()
		waitSaveFiles = append(waitSaveFiles, mediaFile)
		if len(waitSaveFiles) > 100 {
			// 批量入库
			db.Db.Create(waitSaveFiles)
			waitSaveFiles = []*models.ScrapeMediaFile{}
		}
	}
	if len(waitSaveFiles) > 0 {
		// 批量入库
		db.Db.Create(waitSaveFiles)
	}
	return nil
}

func (m *scanBaseImpl) ExtractSeasonEpisode(mediaFile *models.ScrapeMediaFile) error {
	if mediaFile.EpisodeNumber == -1 {
		// 先识别季集
		info := helpers.ExtractMediaInfoRe(mediaFile.VideoFilename, false, true, m.scrapePath.VideoExtList, m.scrapePath.DeleteKeyword...)
		if info == nil {
			helpers.AppLogger.Errorf("使用正则从文件名中提取媒体信息失败，文件名 %s", mediaFile.VideoFilename)
			return errors.New("使用正则从文件名中提取媒体信息失败")
		}
		mediaFile.EpisodeNumber = info.Episode
		mediaFile.SeasonNumber = info.Season
		helpers.AppLogger.Infof("从文件名中提取到季集: %s %d, %d", mediaFile.VideoFilename, mediaFile.SeasonNumber, mediaFile.EpisodeNumber)
	}
	if mediaFile.SeasonNumber == -1 {
		// 从父目录中提取季数
		seasonNumber := helpers.ExtractSeasonFromTvshowPath(mediaFile.TvshowPath)
		if seasonNumber != -1 {
			mediaFile.SeasonNumber = seasonNumber
			helpers.AppLogger.Infof("从电视剧文件夹中提取到季数: %d", mediaFile.SeasonNumber)
		}
	}
	if mediaFile.SeasonNumber == -1 {
		mediaFile.SeasonNumber = 1
	}
	return nil
}
