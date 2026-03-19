package scrape

import (
	"Q115-STRM/internal/baidupan"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/openlist"
	"Q115-STRM/internal/tmdb"
	"Q115-STRM/internal/v115open"
	"context"
	"fmt"
	"net/url"
	"path/filepath"
)

type uploadFile struct {
	ID         string
	FileName   string
	SourcePath string
	DestPath   string
	DestPathId string
}

type ScrapeBase struct {
	scrapePath     *models.ScrapePath
	ctx            context.Context
	fileTasks      chan *models.ScrapeMediaFile
	identifyImpl   IdentifyImpl
	categoryImpl   categoryImpl
	renameImpl     renameImpl
	tmdbClient     *tmdb.Client
	v115Client     *v115open.OpenClient
	openlistClient *openlist.Client
	baiduPanClient *baidupan.Client
}

// 下载图片到指定文件
func (s *ScrapeBase) DownloadImages(parentPath, ua string, fileList map[string]string) {
	for fileName, url := range fileList {
		filePath := filepath.Join(parentPath, fileName)
		// helpers.AppLogger.Infof("下载图片 %s 到 %s", url, filePath)
		if url == "" || helpers.PathExists(filePath) || url == models.GlobalScrapeSettings.GetTmdbImageUrl() {
			helpers.AppLogger.Warnf("图片 %s 已存在，或者 %s 为空，跳过下载", filePath, url)
			continue
		}
		err := helpers.DownloadFile(url, filePath, ua)
		if err != nil {
			helpers.AppLogger.Errorf("下载图片 %s 失败: %v", filePath, err)
		}
	}
}

func (s *ScrapeBase) GetMovieRealName(sm *models.ScrapeMediaFile, name string, filetype string) string {
	if filetype == "nfo" {
		return fmt.Sprintf("%s.nfo", sm.NewVideoBaseName)
	}
	if sm.ScrapeType == models.ScrapeTypeOnlyRename {
		return fmt.Sprintf("%s-%s", sm.NewVideoBaseName, name)
	} else {
		return name
	}
}

func (s *ScrapeBase) FFprobe(mediaFile *models.ScrapeMediaFile) error {
	// 如果是strm则不提取
	if filepath.Ext(mediaFile.VideoFilename) == ".strm" {
		helpers.AppLogger.Infof("strm文件或网盘文件，跳过ffprobe阶段, 文件名: %s", mediaFile.VideoFilename)
		return nil
	}
	// 解析视频文件包含的视频、音频、字幕流
	videoPathOrUrl := s.GetDownloadUrl(mediaFile)
	if videoPathOrUrl == "" {
		helpers.AppLogger.Errorf("获取视频的下载连接失败, PickCode: %s", mediaFile.VideoPickCode)
		return fmt.Errorf("获取视频的下载连接失败, PickCode: %s", mediaFile.VideoPickCode)
	}
	if mediaFile.SourceType == models.SourceType115 {
		// 代理访问
		videoPathOrUrl = fmt.Sprintf("http://127.0.0.1:12333/proxy-115?url=%s", url.QueryEscape(videoPathOrUrl))
	}
	// 如果有下载连接，则提取视频信息
	s.GetFFprobeInfoFromFileOrUrl(mediaFile, videoPathOrUrl)
	return nil
}

func (sm *ScrapeBase) GetFFprobeInfoFromFileOrUrl(mediaFile *models.ScrapeMediaFile, videoPathOrUrl string) {
	var ffprobeJson *helpers.FFprobeJson
	if mediaFile.VideoCodecJson != "" && mediaFile.AudioCodecJson != "" && mediaFile.Resolution != "" {
		return
	}
	var err error
	ffprobeJson, err = helpers.GetFFprobeJson(videoPathOrUrl)
	if err != nil {
		helpers.AppLogger.Errorf("获取ffprobe数据文件失败: %v", err)
		return
	}
	// }
	mediaFile.AudioCodec = make([]*models.AudioCodec, 0)
	mediaFile.SubtitleCodec = make([]*models.Subtitle, 0)
	if ffprobeJson == nil {
		helpers.AppLogger.Error("获取ffprobe数据失败，数据为空")
		return
	}
	for index, stream := range ffprobeJson.Streams {
		// 解析视频编码
		if stream.CodecType == "video" {
			mediaFile.VideoCodec = &models.VideoCodec{}
			mediaFile.VideoCodec.StreamIndex = index
			mediaFile.VideoCodec.Codec = stream.CodecName
			mediaFile.VideoCodec.Micodec = stream.CodecName
			mediaFile.VideoCodec.Width = stream.Width
			mediaFile.VideoCodec.Height = stream.Height
			mediaFile.VideoCodec.Duration = ffprobeJson.Format.Duration
			mediaFile.VideoCodec.AspectRatio = helpers.CalculateAspectRatio(stream.Width, stream.Height)
			mediaFile.VideoCodec.Aspect = stream.DisplayAspectRatio
			mediaFile.VideoCodec.PixelFormat = stream.PixelFormat
			bitrate, err := helpers.CalculateBitrate(&stream, &ffprobeJson.Format)
			if err == nil {
				mediaFile.VideoCodec.Bitrate = bitrate
			} else {
				mediaFile.VideoCodec.Bitrate = 0
			}
			mediaFile.VideoCodec.Framerate = stream.AvgFrameRate
			mediaFile.VideoCodec.DurationInSeconds, _ = helpers.ParseDurationToSeconds(ffprobeJson.Format.Duration)
			if mediaFile.VideoCodec.DurationInSeconds > 0 {
				mediaFile.VideoCodec.DurationInMinutes = mediaFile.VideoCodec.DurationInSeconds / 60
			} else {
				mediaFile.VideoCodec.DurationInMinutes = 0
			}
		}
		// 解析音频编码
		if stream.CodecType == "audio" {
			au := &models.AudioCodec{
				StreamIndex:  index,
				Codec:        stream.CodecName,
				Micodec:      stream.CodecName,
				Channels:     stream.Channels,
				Language:     stream.Tags["language"],
				SamplingRate: helpers.StringToInt64(stream.SampleRate),
			}
			bitrate, err := helpers.CalculateBitrate(&stream, &ffprobeJson.Format)
			if err == nil {
				au.Bitrate = bitrate
			} else {
				au.Bitrate = 0
			}
			mediaFile.AudioCodec = append(mediaFile.AudioCodec, au)
		}
		// 解析字幕流
		if stream.CodecType == "subtitle" {
			sub := &models.Subtitle{
				StreamIndex: index,
				Codec:       stream.CodecName,
				Micodec:     stream.CodecName,
				Title:       stream.Tags["title"],
				Language:    stream.Tags["language"],
			}
			mediaFile.SubtitleCodec = append(mediaFile.SubtitleCodec, sub)
		}
	}
	if mediaFile.VideoCodec != nil {
		resolution := helpers.GetResolutionLevel(mediaFile.VideoCodec.Width, mediaFile.VideoCodec.Height)
		mediaFile.ResolutionLevel = resolution.CommonName
		mediaFile.Resolution = resolution.StandardName
		// 计算是否HDR视频
		mediaFile.IsHDR = helpers.IsHDRFormat(mediaFile.VideoCodec.PixelFormat)
	}
	// 检查文件名中的DDP标识，修复audioCodec识别错误
	if len(mediaFile.AudioCodec) > 0 {
		filename := mediaFile.VideoFilename
		// 检查文件名中是否包含DDP或Dolby Digital Plus标识
		if helpers.ContainsIgnoreCase(filename, "DDP") || helpers.ContainsIgnoreCase(filename, "Dolby Digital Plus") {
			// 将第一个音频流的Micodec设置为DDP
			mediaFile.AudioCodec[0].Micodec = "DDP"
		}
	}
}

func (s *ScrapeBase) GetDownloadUrl(mediaFile *models.ScrapeMediaFile) string {
	if mediaFile.SourceType == models.SourceTypeLocal {
		return mediaFile.VideoPickCode
	}
	videoPathOrUrl := mediaFile.VideoPickCode
	switch mediaFile.SourceType {
	case models.SourceType115:
		videoPathOrUrl = s.v115Client.GetDownloadUrl(context.Background(), mediaFile.VideoPickCode, v115open.DEFAULTUA, false)
	case models.SourceTypeOpenList:
		videoPathOrUrl = s.openlistClient.GetRawUrl(mediaFile.VideoPickCode)
	case models.SourceType123:
	}
	return videoPathOrUrl
}

// 将本地临时文件移动到本地目标路径
func (m *ScrapeBase) MoveLocalTempFileToDest(mediaFile *models.ScrapeMediaFile, files []uploadFile) (bool, error) {
	if mediaFile.SourceType != models.SourceTypeLocal {
		helpers.AppLogger.Warnf("非本地文件刮削，无法移动到目标位置")
		return true, fmt.Errorf("非本地文件刮削，无法移动到目标位置")
	}
	for _, file := range files {
		tempPath := file.SourcePath
		if !helpers.PathExists(tempPath) {
			helpers.AppLogger.Warnf("刮削临时文件 %s 不存在，跳过移动", tempPath)
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
