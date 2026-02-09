package models

import (
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/tmdb"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

type ScrapeMediaStatus string

const (
	ScrapeMediaStatusUnscanned    ScrapeMediaStatus = "unscanned"     // 未识别
	ScrapeMediaStatusScanned      ScrapeMediaStatus = "scanned"       // 已识别，未刮削
	ScrapeMediaStatusScraping     ScrapeMediaStatus = "scraping"      // 正在刮削
	ScrapeMediaStatusScraped      ScrapeMediaStatus = "scraped"       // 已刮削，未重命名
	ScrapeMediaStatusRenaming     ScrapeMediaStatus = "renaming"      // 正在重命名
	ScrapeMediaStatusRenamed      ScrapeMediaStatus = "renamed"       // 已重命名
	ScrapeMediaStatusRenameFailed ScrapeMediaStatus = "rename_failed" // 重命名失败
	ScrapeMediaStatusIgnore       ScrapeMediaStatus = "ignore"        // 忽略
	ScrapeMediaStatusScrapeFailed ScrapeMediaStatus = "scrape_failed" // 刮削失败
	ScrapeMediaStatusRollbacking  ScrapeMediaStatus = "rollbacking"   // 回滚中
)

type TmdbGender int

const (
	TmdbGenderUnknown TmdbGender = 0 // 未知
	TmdbGenderFemale  TmdbGender = 1 // 女性
	TmdbGenderMale    TmdbGender = 2 // 男性
	TmdbGenderOther   TmdbGender = 3 // 其他
)

// ResolutionLevel 分辨率等级常量
const (
	ResolutionSD   = "SD"   // 标清
	ResolutionHD   = "HD"   // 720p
	ResolutionFHD  = "FHD"  // 1080p
	ResolutionQHD  = "QHD"  // 1440p
	ResolutionUHD  = "UHD"  // 2160p
	ResolutionFUHD = "FUHD" // 4320p
)

// 分辨率阈值
const (
	Height720p  = 720
	Height1080p = 1080
	Height1440p = 1440
	Height2160p = 2160
	Height4320p = 4320
)

type VideoCodec struct {
	StreamIndex       int     `json:"stream_index"`        // 视频流索引
	Codec             string  `json:"codec"`               // 视频编码
	Micodec           string  `json:"micodec"`             // 视频编码，例如: hevc
	Bitrate           int64   `json:"bitrate"`             // 视频码率，单位：bps
	Width             int64   `json:"width"`               // 视频宽度
	Height            int64   `json:"height"`              // 视频高度
	Duration          string  `json:"duration"`            // 视频时长，格式: 00:00:00.000
	DurationInSeconds int64   `json:"duration_in_seconds"` // 视频时长，单位：秒
	DurationInMinutes int64   `json:"duration_in_minutes"` // 视频时长，单位：分
	AspectRatio       float64 `json:"aspect_ratio"`        // 视频宽高比
	Aspect            string  `json:"aspect"`              // 视频宽高比，格式: 16:9
	Framerate         string  `json:"framerate"`           // 视频帧率，格式: 25.000
	Scantype          string  `json:"scantype"`            // 视频扫描类型
	PixelFormat       string  `json:"pixel_format"`        // 视频像素格式
	Default           string  `json:"default"`             // 默认视频
	Forced            string  `json:"forced"`              // 强制视频
}

type AudioCodec struct {
	StreamIndex  int    `json:"stream_index"`  // 视频流索引
	Codec        string `json:"codec"`         // 音频编码
	Micodec      string `json:"micodec"`       // 音频编码，例如: aac
	Bitrate      int64  `json:"bitrate"`       // 音频码率，单位：bps
	SamplingRate int64  `json:"sampling_rate"` // 音频采样率，单位：kHz
	Channels     int64  `json:"channels"`      // 音频通道数
	Language     string `json:"language"`      // 音频语言
	Scantype     string `json:"scantype"`      // 音频扫描类型
	Default      string `json:"default"`       // 默认音频
	Forced       string `json:"forced"`        // 强制音频
}

type Subtitle struct {
	StreamIndex int    `json:"stream_index"` // 视频流索引
	Title       string `json:"title"`        // 字幕标题
	Language    string `json:"language"`     // 字幕语言
	Codec       string `json:"codec"`        // 字幕编码
	Micodec     string `json:"micodec"`      // 字幕编码，例如: srt
	Scantype    string `json:"scantype"`     // 字幕扫描类型
	Default     string `json:"default"`      // 默认字幕
	Forced      string `json:"forced"`       // 强制字幕
}

type MediaFiles struct {
	NfoPath          string            `json:"nfo_path"`                // nfo信息文件路径，相对于SyncPath.LocalPath + SyncPath.RemotePath的相对路径
	NfoFileName      string            `json:"nfo_file_name"`           // nfo信息文件名
	NfoFileId        string            `json:"nfo_file_id"`             // nfo信息文件ID
	NfoPickCode      string            `json:"nfo_pick_code"`           // nfo信息文件选择码
	ImageFiles       []*MediaMetaFiles `json:"image_files" gorm:"-"`    // 图片文件列表，json数组
	ImageFilesJson   string            `json:"-"`                       // 图片文件列表 json字符串
	SubtitleFiles    []*MediaMetaFiles `json:"subtitle_files" gorm:"-"` // 字幕文件列表
	SubtitleFileJson string            `json:"-"`                       // SubtitleFiles的JSON字符串
}

type TmdbInfo struct {
	MovieDetail  *tmdb.MovieDetail         `json:"movie_detail"`  // 电影详细信息
	TvShowDetail *tmdb.TvDetail            `json:"tvshow_detail"` // 剧集详细信息
	Credits      *tmdb.PepolesRes          `json:"credits"`       // 演职员信息
	Images       *tmdb.Images              `json:"images"`        // 图片信息
	ReleasesDate []tmdb.ReleasesDateResult `json:"releases_date"` // 发布日期信息
}

type MediaMetaFiles struct {
	FileName string `json:"file_name"` // 文件名
	FileId   string `json:"file_id"`   // 文件ID
	PickCode string `json:"pick_code"` // 识别码
}

type WillDeleteFile struct {
	FullFilePath string
}

type MoveNewFileToSourceFile struct {
	FileId       string `json:"file_id"`        // 文件ID
	PathId       string `json:"path_id"`        // 路径ID
	FileFullPath string `json:"file_full_path"` // 文件完整路径
}

// 待刮削的视频文件列表
// 会收集已存在nfo，图片，字幕文件，以及视频文件的ffprobe信息
type ScrapeMediaFile struct {
	BaseModel
	MediaFiles
	ScrapePathId         uint              `gorm:"index" json:"scrape_path_id"`                     // 同步路径ID
	MediaType            MediaType         `gorm:"index" json:"media_type"`                         // 媒体类型
	SourceType           SourceType        `gorm:"index" json:"source_type"`                        // 来源类型
	ScrapeType           ScrapeType        `gorm:"index" json:"scrape_type"`                        // 刮削类型
	RenameType           RenameType        `gorm:"index" json:"rename_type"`                        // 重命名类型
	EnableCategory       bool              `gorm:"index" json:"enable_category"`                    // 是否启用二级分类
	SourcePath           string            `gorm:"index" json:"source_path"`                        // 来源路径
	SourcePathId         string            `gorm:"index" json:"source_path_id"`                     // 来源路径ID
	DestPath             string            `gorm:"index" json:"dest_path"`                          // 目标路径
	DestPathId           string            `gorm:"index" json:"dest_path_id"`                       // 目标路径ID
	MediaId              uint              `gorm:"index" json:"media_id"`                           // 媒体ID
	MediaSeasonId        uint              `gorm:"index" json:"media_season_id"`                    // 季ID
	MediaEpisodeId       uint              `gorm:"index" json:"media_episode_id"`                   // 集ID
	Name                 string            `json:"name"`                                            // TMDB名称，如果没有Media数据则使用该字段
	Year                 int               `json:"year"`                                            // TMDB年份，如果没有Media数据则使用该字段
	TmdbId               int64             `json:"tmdb_id"`                                         // TMDB ID，如果没有Media数据则使用该字段
	SeasonNumber         int               `json:"season_number"`                                   // 季编号，例如：S01E01中的S01
	EpisodeNumber        int               `json:"episode_number"`                                  // 集编号，例如：S01E01中的E01
	Path                 string            `json:"path"`                                            // 媒体文件夹路径，相对ScrapePath.SourcePath的路径
	PathId               string            `json:"path_id"`                                         // 媒体文件夹路径ID，local类型是绝对路径，网盘类型是文件ID
	TvshowPath           string            `json:"tvshow_path"`                                     // 电视剧路径，相对ScrapePath.SourcePath的路径
	TvshowPathId         string            `json:"tvshow_path_id" gorm:"index:idx_batch_no_tvshow"` // 电视剧路径ID，local类型是绝对路径，网盘类型是文件ID
	VideoFilename        string            `json:"video_filename"`                                  // 视频文件名，相对于SyncPath.LocalPath + SyncPath.RemotePath的相对路径
	VideoFileId          string            `json:"video_file_id"`                                   // 视频文件ID，local类型是绝对路径，网盘类型是文件ID
	VideoPickCode        string            `json:"video_pick_code"`                                 // 视频文件PickCode
	TvshowFiles          []*MediaMetaFiles `json:"tvshow_files" gorm:"-"`                           // 剧集文件列表
	TvshowFilesJson      string            `json:"-"`                                               // 剧集文件列表json字符串
	SeasonFiles          []*MediaMetaFiles `json:"season_files" gorm:"-"`                           // 季文件列表
	SeasonFilesJson      string            `json:"-"`                                               // 季文件列表json字符串
	Resolution           string            `json:"resolution"`                                      // 分辨率
	ResolutionLevel      string            `json:"resolution_level"`                                // 分辨率等级
	IsHDR                bool              `json:"is_hdr"`                                          // 是否HDR
	VideoCodec           *VideoCodec       `json:"video_codec" gorm:"-"`                            // 视频编码，使用ffprobe提取
	AudioCodec           []*AudioCodec     `json:"audio_codec" gorm:"-"`                            // 音频编码，使用ffprobe提取
	SubtitleCodec        []*Subtitle       `json:"subtitle_codec" gorm:"-"`                         // 内封字幕流，使用ffprobe提取
	VideoCodecJson       string            `json:"-"`                                               // 视频编码json字符串
	AudioCodecJson       string            `json:"-"`                                               // 音频编码json字符串
	SubtitleCodecJson    string            `json:"-"`                                               // 内封字幕流json字符串
	Status               ScrapeMediaStatus `json:"status"`                                          // 媒体状态
	FailedReason         string            `json:"failed_reason"`                                   // 刮削失败原因
	ScanTime             int64             `json:"scan_time"`                                       // 识别时间
	ScrapeTime           int64             `json:"scrape_time"`                                     // 刮削时间
	RenameTime           int64             `json:"rename_time"`                                     // 重命名时间
	CategoryName         string            `json:"category_name"`                                   // 分类名称
	ScrapePathCategoryId uint              `json:"scrape_path_category_id"`                         // 分类目录ID
	NewPathName          string            `json:"new_path_name"`                                   // 新路径名称，不含二级分类
	NewSeasonPathName    string            `json:"new_season_path_name"`                            // 新季路径名称
	NewPathId            string            `json:"new_path_id"`                                     // 新路径ID
	NewSeasonPathId      string            `json:"new_season_path_id"`                              // 新季路径ID
	NewVideoBaseName     string            `json:"new_video_base_name"`                             // 新视频文件名（不含扩展名）
	VideoExt             string            `json:"video_ext"`                                       // 视频文件扩展名
	ReScrapeTime         int64             `json:"re_scrape_time"`                                  // 重新刮削时间
	IsReScrape           bool              `json:"is_re_scrape"`                                    // 是否重新刮削        // 已上传文件数量
	BatchNo              string            `json:"batch_no" gorm:"index:idx_batch_no_tvshow"`       // 批次号(每次扫描都是同一个批次)
	TvIsRename           bool              `json:"tv_is_rename"`                                    // 是否重命名剧集
	SeasonIsRename       bool              `json:"season_is_rename"`                                // 是否重命名季
	Media                *Media            `json:"-" gorm:"-"`                                      // 影视剧信息
	MediaSeason          *MediaSeason      `json:"-" gorm:"-"`                                      // 季信息
	MediaEpisode         *MediaEpisode     `json:"-" gorm:"-"`                                      // 集信息
	ScrapeRootPath       string            `json:"scrape_root_path" gorm:"-"`                       // 刮削根目录
}

func (sm *ScrapeMediaFile) Save() error {
	// 转换字幕文件列表为json字符串
	if len(sm.SubtitleFiles) > 0 {
		sm.SubtitleFileJson = helpers.JsonString(sm.SubtitleFiles)
	}
	// 转换图片文件列表为json字符串
	if len(sm.ImageFiles) > 0 {
		sm.ImageFilesJson = helpers.JsonString(sm.ImageFiles)
	}
	// 编码视频编码
	if sm.VideoCodec != nil {
		sm.VideoCodecJson = helpers.JsonString(sm.VideoCodec)
	}
	// 编码音频编码
	if len(sm.AudioCodec) > 0 {
		sm.AudioCodecJson = helpers.JsonString(sm.AudioCodec)
	}
	// 编码内封字幕流
	if len(sm.SubtitleCodec) > 0 {
		sm.SubtitleCodecJson = helpers.JsonString(sm.SubtitleCodec)
	}
	// 编码剧集文件列表
	if len(sm.TvshowFiles) > 0 {
		sm.TvshowFilesJson = helpers.JsonString(sm.TvshowFiles)
	}
	// 编码季文件列表
	if len(sm.SeasonFiles) > 0 {
		sm.SeasonFilesJson = helpers.JsonString(sm.SeasonFiles)
	}
	// 提交写入请求（同步）
	err := db.Db.Save(sm).Error
	if err != nil {
		helpers.AppLogger.Errorf("保存待刮削的视频文件出错：%v", err)
	}
	return err
}

func (sm *ScrapeMediaFile) DecodeJson() {
	// 解码剧集文件列表json字符串
	if sm.TvshowFilesJson != "" {
		tvshowFiles, err := helpers.StringJson[[]*MediaMetaFiles](sm.TvshowFilesJson)
		if err != nil {
			helpers.AppLogger.Errorf("解码剧集文件列表失败: %v", err)
		}
		sm.TvshowFiles = tvshowFiles
	}
	// 解码季文件列表json字符串
	if sm.SeasonFilesJson != "" {
		seasonFiles, err := helpers.StringJson[[]*MediaMetaFiles](sm.SeasonFilesJson)
		if err != nil {
			helpers.AppLogger.Errorf("解码季文件列表失败: %v", err)
		}
		sm.SeasonFiles = seasonFiles
	}
	// 解码视频编码json字符串
	if sm.VideoCodecJson != "" {
		videoCodec, err := helpers.StringJson[*VideoCodec](sm.VideoCodecJson)
		if err != nil {
			helpers.AppLogger.Errorf("解码视频编码失败: %v", err)
		}
		sm.VideoCodec = videoCodec
	}
	// 解码音频编码json字符串
	if sm.AudioCodecJson != "" {
		audioCodec, err := helpers.StringJson[[]*AudioCodec](sm.AudioCodecJson)
		if err != nil {
			helpers.AppLogger.Errorf("解码音频编码失败: %v", err)
		}
		sm.AudioCodec = audioCodec
	}
	// 解码内封字幕流json字符串
	if sm.SubtitleCodecJson != "" {
		subtitleCodec, err := helpers.StringJson[[]*Subtitle](sm.SubtitleCodecJson)
		if err != nil {
			helpers.AppLogger.Errorf("解码内封字幕流失败: %v", err)
		}
		sm.SubtitleCodec = subtitleCodec
	}
	// 解码图片文件列表json字符串
	if sm.ImageFilesJson != "" {
		imageFiles, err := helpers.StringJson[[]*MediaMetaFiles](sm.ImageFilesJson)
		if err != nil {
			helpers.AppLogger.Errorf("解码图片文件列表失败: %v", err)
		}
		sm.ImageFiles = imageFiles
	}
	// 解码外挂字幕文件列表json字符串
	if sm.SubtitleFileJson != "" {
		subtitleFiles, err := helpers.StringJson[[]*MediaMetaFiles](sm.SubtitleFileJson)
		if err != nil {
			helpers.AppLogger.Errorf("解码外挂字幕文件列表失败: %v", err)
		}
		sm.SubtitleFiles = subtitleFiles
	}
	if sm.TvshowFilesJson != "" {
		tvshowFiles, err := helpers.StringJson[[]*MediaMetaFiles](sm.TvshowFilesJson)
		if err != nil {
			helpers.AppLogger.Errorf("解码剧集文件列表失败: %v", err)
		}
		sm.TvshowFiles = tvshowFiles
	}
	// 解码季文件列表json字符串
	if sm.SeasonFilesJson != "" {
		seasonFiles, err := helpers.StringJson[[]*MediaMetaFiles](sm.SeasonFilesJson)
		if err != nil {
			helpers.AppLogger.Errorf("解码季文件列表失败: %v", err)
		}
		sm.SeasonFiles = seasonFiles
	}
}

// 查询关联的数据
func (sm *ScrapeMediaFile) QueryRelation() {
	// 查询Media表
	if sm.MediaId > 0 {
		media, err := GetMediaById(sm.MediaId)
		if err != nil {
			helpers.AppLogger.Errorf("查询Media表失败: media_id=%d %v", sm.MediaId, err)
		}
		sm.Media = media
	}
	// 如果是剧集则查询季和集的表
	if sm.MediaType == MediaTypeTvShow {
		if sm.MediaSeasonId > 0 {
			// 查询季表
			season, err := GetMediaSeasonById(sm.MediaSeasonId)
			if err != nil {
				helpers.AppLogger.Errorf("查询季表失败: season_id=%d %v", sm.MediaSeasonId, err)
			}
			sm.MediaSeason = season
		}
		if sm.MediaEpisodeId > 0 {
			// 查询集表
			episode, err := GetMediaEpisodeById(sm.MediaEpisodeId)
			if err != nil {
				helpers.AppLogger.Errorf("查询集表失败: episode_id=%d %v", sm.MediaEpisodeId, err)
			}
			sm.MediaEpisode = episode
		}
	}

}

// 状态改为失败
func (sm *ScrapeMediaFile) Failed(reason string) {
	sm.Status = ScrapeMediaStatusScrapeFailed
	sm.FailedReason = reason
	sm.ScrapeTime = time.Now().Unix()
	// 保存到数据库
	updateData := make(map[string]interface{})
	updateData["status"] = sm.Status
	updateData["failed_reason"] = sm.FailedReason
	updateData["scrape_time"] = sm.ScrapeTime
	// 提交写入请求（同步）
	if err := db.Db.Model(&ScrapeMediaFile{}).Where("id = ?", sm.ID).Updates(updateData).Error; err != nil {
		helpers.AppLogger.Errorf("更新刮削媒体失败: id=%d %v", sm.ID, err)
	}
}

// 状态改为已识别
func (sm *ScrapeMediaFile) Scanned() {
	sm.Status = ScrapeMediaStatusScraped
	sm.ScanTime = time.Now().Unix()
	updateData := make(map[string]interface{})
	updateData["status"] = sm.Status
	updateData["failed_reason"] = ""
	updateData["scan_time"] = sm.ScanTime
	// 提交写入请求（同步）
	if err := db.Db.Model(&ScrapeMediaFile{}).Where("id = ?", sm.ID).Updates(updateData).Error; err != nil {
		helpers.AppLogger.Errorf("更新刮削媒体失败: id=%d %v", sm.ID, err)
	}
}

// 状态改为正在刮削
func (sm *ScrapeMediaFile) Scraping() {
	sm.Status = ScrapeMediaStatusScraping
	// 保存到数据库
	updateData := make(map[string]interface{})
	updateData["status"] = sm.Status
	// 提交写入请求（同步）
	if err := db.Db.Model(&ScrapeMediaFile{}).Where("id = ?", sm.ID).Updates(updateData).Error; err != nil {
		helpers.AppLogger.Errorf("更新刮削媒体失败: id=%d %v", sm.ID, err)
	}
}

// 状态改为刮削完成
func (sm *ScrapeMediaFile) ScrapeFinish() {
	sm.Status = ScrapeMediaStatusScraped
	sm.ScrapeTime = time.Now().Unix()
	sm.FailedReason = ""
	sm.IsReScrape = false
	// 保存到数据库
	updateData := make(map[string]interface{})
	updateData["status"] = sm.Status
	updateData["scrape_time"] = sm.ScrapeTime
	updateData["failed_reason"] = sm.FailedReason
	updateData["new_path_name"] = sm.NewPathName
	updateData["new_video_base_name"] = sm.NewVideoBaseName
	updateData["video_ext"] = sm.VideoExt
	updateData["new_season_path_name"] = sm.NewSeasonPathName
	updateData["is_re_scrape"] = sm.IsReScrape
	if err := db.Db.Model(&ScrapeMediaFile{}).Where("id = ?", sm.ID).Updates(updateData).Error; err != nil {
		helpers.AppLogger.Errorf("更新刮削媒体失败: id=%d %v", sm.ID, err)
	}
	sm.Media.Status = MediaStatusScraped
	sm.Media.Save()
}

func (sm *ScrapeMediaFile) StatusScrapeFinish() {
	sm.Status = ScrapeMediaStatusScraped
	sm.RenameTime = time.Now().Unix()
	// 保存到数据库
	updateData := make(map[string]interface{})
	updateData["status"] = sm.Status
	// 提交写入请求（同步）
	if err := db.Db.Model(&ScrapeMediaFile{}).Where("id = ?", sm.ID).Updates(updateData).Error; err != nil {
		helpers.AppLogger.Errorf("更新刮削媒体失败: id=%d %v", sm.ID, err)
	}
}

func (sm *ScrapeMediaFile) Renaming() {
	sm.Status = ScrapeMediaStatusRenaming
	// 保存到数据库
	updateData := make(map[string]interface{})
	updateData["status"] = sm.Status
	if err := db.Db.Model(&ScrapeMediaFile{}).Where("id = ?", sm.ID).Updates(updateData).Error; err != nil {
		helpers.AppLogger.Errorf("更新刮削媒体失败: id=%d %v", sm.ID, err)
	}
}

func (sm *ScrapeMediaFile) StatusFinish() {
	sm.Status = ScrapeMediaStatusRenamed
	sm.RenameTime = time.Now().Unix()
	// 保存到数据库
	updateData := make(map[string]interface{})
	updateData["status"] = sm.Status
	updateData["rename_time"] = sm.RenameTime
	if err := db.Db.Model(&ScrapeMediaFile{}).Where("id = ?", sm.ID).Updates(updateData).Error; err != nil {
		helpers.AppLogger.Errorf("更新刮削媒体失败: id=%d %v", sm.ID, err)
	}
}

func (sm *ScrapeMediaFile) RenameFailed(reason string) {
	sm.Status = ScrapeMediaStatusRenameFailed
	sm.FailedReason = reason
	sm.RenameTime = time.Now().Unix()
	// 保存到数据库
	updateData := make(map[string]interface{})
	updateData["status"] = sm.Status
	updateData["failed_reason"] = sm.FailedReason
	updateData["rename_time"] = sm.RenameTime
	if err := db.Db.Model(&ScrapeMediaFile{}).Where("id = ?", sm.ID).Updates(updateData).Error; err != nil {
		helpers.AppLogger.Errorf("更新刮削媒体失败: id=%d %v", sm.ID, err)
	}
}

func (sm *ScrapeMediaFile) RemoveMovieTmpFile(sp *ScrapePath, task *DbUploadTask) {
	if sp.ScrapeType == ScrapeTypeOnlyRename {
		return
	}
	sourcepath := sm.GetTmpFullMoviePath()
	e := 0
	if sp.ScrapeType == ScrapeTypeOnly {
		// 仅刮削模式下，检查文件使用文件名
		imageList := []string{"poster.jpg", "clearlogo.jpg", "clearart.jpg", "square.jpg", "logo.jpg", "fanart.jpg", "backdrop.jpg", "background.jpg", "4kbackground.jpg", "thumb.jpg", "banner.jpg", "disc.jpg"}
		for _, image := range imageList {
			n := sp.GetMovieRealName(sm, image, "image")
			imageFile := filepath.Join(sourcepath, n)
			// 判断文件是否存在
			if helpers.PathExists(imageFile) {
				e++
			}
		}
		// 判断nfo文件是否存在
		nfo := sp.GetMovieRealName(sm, "nfo", "nfo")
		nfoFile := filepath.Join(sourcepath, nfo)
		if helpers.PathExists(nfoFile) {
			e++
		}
	} else {
		// 检查目录是否为空
		dirEntries, _ := os.ReadDir(sourcepath)
		if len(dirEntries) > 0 {
			e++
		}
	}
	if e == 0 {
		if sourcepath != sp.SourcePathId {
			if helpers.PathExists(sourcepath) {
				// 删除目录
				os.Remove(sourcepath)
				helpers.AppLogger.Infof("删除电影 %s 的刮削临时目录 %s 成功", sm.Name, sourcepath)
			}
		}
	} else {
		helpers.AppLogger.Warnf("电影 %s 的刮削临时目录下还有其他文件，不能删除整个目录 %s", sm.Name, sourcepath)
		return
	}
}

func (sm *ScrapeMediaFile) RemoveTvShowTmpFile(sp *ScrapePath, task *DbUploadTask) {
	// 查询电视剧下的所有集
	// 检查是否有未完成整理的集
	// 检查是否全部上传完成
	episodeMediaFiles := GetAllEpisodeByTvshow(sm.MediaId, sm.BatchNo)
	helpers.AppLogger.Infof("电视剧 %s 共包含 %d 集", sm.Name, len(episodeMediaFiles))
	// 使用集ID查询上传任务是否完成
	allUploaded := true
	for _, episodeMedia := range episodeMediaFiles {
		if slices.Contains([]ScrapeMediaStatus{ScrapeMediaStatusScanned, ScrapeMediaStatusScraped, ScrapeMediaStatusScraping, ScrapeMediaStatusRenaming}, episodeMedia.Status) {
			allUploaded = false
			helpers.AppLogger.Infof("电视剧 %s 季 %d 集 %d 的状态：%s，未完成整理", episodeMedia.Name, episodeMedia.SeasonNumber, episodeMedia.EpisodeNumber, episodeMedia.Status)
			break
		} else {
			// helpers.AppLogger.Infof("电视剧 %s 季 %d 集 %d 的状态：%s，已整理或整理失败", episodeMedia.Name, episodeMedia.SeasonNumber, episodeMedia.EpisodeNumber, episodeMedia.Status)
		}
		count := GetUnFinishUploadTaskCountByScrapeMediaId(episodeMedia.ID)
		// helpers.AppLogger.Infof("电视剧 %s 季 %d 集 %d 的状态：%s，剩余未上传完成的文件数量：%d", episodeMedia.Name, episodeMedia.SeasonNumber, episodeMedia.EpisodeNumber, episodeMedia.Status, count)
		if count > 0 {
			allUploaded = false
			break
		}
	}
	if !allUploaded {
		helpers.AppLogger.Infof("电视剧 %s 还有未完成上传的集，不能删除刮削临时目录", sm.Name)
		return
	}
	// 检查电视剧目录是否可以删除
	tvShowPath := sm.GetTmpFullTvshowPath()
	// 检查电视剧目录是否为空
	dirEntries, _ := os.ReadDir(tvShowPath)
	if len(dirEntries) == 0 {
		os.RemoveAll(tvShowPath)
		helpers.AppLogger.Infof("删除电视剧 %s 的刮削临时目录 %s 成功", sm.Name, tvShowPath)
	} else {
		helpers.AppLogger.Warnf("电视剧 %s 的刮削临时目录 %s 下还有其他文件，不能删除整个目录", sm.Name, tvShowPath)
		return
	}
}

// 状态改为完成
// 上传完成以后，只处理自己
func (sm *ScrapeMediaFile) RemoveTmpFiles(task *DbUploadTask) {
	if task != nil {
		os.Remove(task.LocalFullPath)
		helpers.AppLogger.Infof("删除影视剧 %s 的刮削临时文件 %s 成功", sm.Name, task.LocalFullPath)
		if task.IsSeasonOrTvshowFile {
			return
		}
	}
	// 检查来源目录是否为空，如果是空则删除
	sp := GetScrapePathByID(sm.ScrapePathId)
	if sp == nil {
		helpers.AppLogger.Errorf("获取刮削路径失败: id=%d", sm.ScrapePathId)
		return
	}
	sp.Init()
	sm.ScrapeRootPath = sp.ScrapeRootPath
	// 清理临时文件
	if sm.MediaType != MediaTypeTvShow {
		sm.RemoveMovieTmpFile(sp, task)
	} else {
		sm.RemoveTvShowTmpFile(sp, task)
	}
}

// 源文件不存在则标记为成功
// 目标文件已存在则标记为成功
func (sm *ScrapeMediaFile) FinishFromRenaming() {
	if sm.Status != ScrapeMediaStatusRenaming && sm.Status != ScrapeMediaStatusScraped {
		return
	}
	sm.Status = ScrapeMediaStatusRenamed
	sm.RenameTime = time.Now().Unix()
	sm.Save()
}

func (sm *ScrapeMediaFile) GenerateNameByTemplate(template string) string {
	if template == "" {
		// 替换占位符
		template = "{title} ({year})"
	}
	newName := strings.ReplaceAll(template, "{title}", sm.Name)
	newName = strings.ReplaceAll(newName, "{year}", strconv.Itoa(sm.Year))
	if sm.Resolution != "" {
		newName = strings.ReplaceAll(newName, "{resolution}", sm.Resolution)
	} else {
		newName = strings.ReplaceAll(newName, "{resolution}", "")
	}
	if sm.ResolutionLevel != "" {
		newName = strings.ReplaceAll(newName, "{resolution_level}", sm.ResolutionLevel)
	} else {
		newName = strings.ReplaceAll(newName, "{resolution_level}", "")
	}
	if sm.VideoCodec != nil && sm.VideoCodec.Bitrate != 0 {
		newName = strings.ReplaceAll(newName, "{bitrate}", fmt.Sprintf("%dMbps", sm.VideoCodec.Bitrate/1000000))
	} else {
		newName = strings.ReplaceAll(newName, "{bitrate}", "")
	}
	if sm.TmdbId != 0 {
		newName = strings.ReplaceAll(newName, "{tmdb_id}", fmt.Sprintf("{tmdbid-%d}", sm.TmdbId))
	} else {
		newName = strings.ReplaceAll(newName, "{tmdb_id}", "")
	}
	// 处理演员
	actorName := ""
	if sm.Media != nil && len(sm.Media.Actors) > 0 {
		actorCount := len(sm.Media.Actors)
		if actorCount >= 3 {
			actorName = "多人演员"
		} else if actorCount > 1 {
			actorNames := make([]string, 0)
			for _, actor := range sm.Media.Actors {
				actorNames = append(actorNames, actor.Name)
			}
			newName = strings.ReplaceAll(newName, "{actors}", strings.Join(actorNames, ", "))
		} else if actorCount == 1 {
			actorName = sm.Media.Actors[0].Name
		}
	}
	if actorName != "" {
		newName = strings.ReplaceAll(newName, "{actors}", actorName)
	} else {
		newName = strings.ReplaceAll(newName, "{actors}", "")
	}
	if sm.Media != nil && sm.Media.Num != "" {
		newName = strings.ReplaceAll(newName, "{num}", sm.Media.Num)
	} else {
		newName = strings.ReplaceAll(newName, "{num}", "")
	}
	if sm.MediaType == MediaTypeTvShow {
		if sm.SeasonNumber >= 0 {
			// 季
			newName = strings.ReplaceAll(newName, "{season_number}", fmt.Sprintf("%d", sm.SeasonNumber))
		} else {
			newName = strings.ReplaceAll(newName, "{season_number}", "")
		}
		// 集
		if sm.EpisodeNumber > 0 {
			newName = strings.ReplaceAll(newName, "{episode_number}", fmt.Sprintf("%d", sm.EpisodeNumber))
		} else {
			newName = strings.ReplaceAll(newName, "{episode_number}", "")
		}
		if sm.SeasonNumber >= 0 && sm.EpisodeNumber > 0 {
			newName = strings.ReplaceAll(newName, "{season_episode}", fmt.Sprintf("S%02dE%02d", sm.SeasonNumber, sm.EpisodeNumber))
		} else {
			newName = strings.ReplaceAll(newName, "{season_episode}", "")
		}
		if sm.MediaEpisode != nil && sm.MediaEpisode.EpisodeName != "" {
			newName = strings.ReplaceAll(newName, "{episode_name}", sm.MediaEpisode.EpisodeName)
		} else {
			newName = strings.ReplaceAll(newName, "{episode_name}", "")
		}
	}
	return newName
}

// 使用指定的名字和年份重新刮削
func (sm *ScrapeMediaFile) ReScrape(name string, year int, tmdbId int64, season int, episode int) error {
	sm.TmdbId = tmdbId
	sm.Name = name
	sm.Year = year
	// 使用该参数在tmdb搜索, 如果能搜到则更新tmdb_id
	tmdbClient := GlobalScrapeSettings.GetTmdbClient()
	if sm.MediaType == MediaTypeTvShow {
		if tmdbId > 0 {
			// 使用tmdb直接查询详情
			tvDetail, err := tmdbClient.GetTvDetail(tmdbId, GlobalScrapeSettings.GetTmdbLanguage())
			if err != nil || tvDetail == nil {
				helpers.AppLogger.Errorf("使用ID查询tmdb剧集详情失败: %v", err)
				return err
			}
			sm.Name = tvDetail.Name
			sm.Year = helpers.ParseYearFromDate(tvDetail.FirstAirDate)
		} else {
			// 剧集
			tvSearch, err := tmdbClient.SearchTv(sm.Name, sm.Year, GlobalScrapeSettings.GetTmdbLanguage(), true)
			if err != nil {
				helpers.AppLogger.Errorf("使用名称和年份查询tmdb剧集失败: %v", err)
				return err
			}
			if len(tvSearch.Results) > 1 {
				helpers.AppLogger.Errorf("通过名称 %s，年份 %d 查询到多条记录，请输入tmdbid重新识别", sm.Name, sm.Year)
				return fmt.Errorf("通过名称 %s，年份 %d 查询到多条记录，请输入tmdbid重新识别", sm.Name, sm.Year)
			}
			// 查询第一个
			if len(tvSearch.Results) > 0 {
				sm.TmdbId = tvSearch.Results[0].ID
			} else {
				helpers.AppLogger.Errorf("查询tmdb剧集失败，未找到匹配的剧集，名称 %s，年份 %d", sm.Name, sm.Year)
				return fmt.Errorf("查询tmdb剧集失败，未找到匹配的剧集，名称 %s，年份 %d", sm.Name, sm.Year)
			}
		}
	} else {
		if sm.TmdbId > 0 {
			movieDetail, merror := tmdbClient.GetMovieDetail(sm.TmdbId, GlobalScrapeSettings.GetTmdbLanguage())
			if merror != nil || movieDetail == nil {
				helpers.AppLogger.Errorf("使用ID查询tmdb电影详情失败: %v", merror)
				return merror
			}
			sm.Name = movieDetail.Title
			sm.Year = helpers.ParseYearFromDate(movieDetail.ReleaseDate)
		} else {
			// 电影
			movieSearch, err := tmdbClient.SearchMovie(sm.Name, sm.Year, GlobalScrapeSettings.GetTmdbLanguage(), true, false)
			if err != nil {
				helpers.AppLogger.Errorf("查询tmdb电影失败: %v", err)
				return err
			}
			if len(movieSearch.Results) > 1 {
				helpers.AppLogger.Errorf("通过名称 %s，年份 %d 查询到多条记录，请输入tmdbid重新识别", sm.Name, sm.Year)
				return fmt.Errorf("通过名称 %s，年份 %d 查询到多条记录，请输入tmdbid重新识别", sm.Name, sm.Year)
			}
			// 查询第一个
			if len(movieSearch.Results) > 0 {
				sm.TmdbId = movieSearch.Results[0].ID
			} else {
				helpers.AppLogger.Errorf("查询tmdb电影失败，未找到匹配的电影，名称 %s，年份 %d", sm.Name, sm.Year)
				return fmt.Errorf("查询tmdb电影失败，未找到匹配的电影，名称 %s，年份 %d", sm.Name, sm.Year)
			}
		}
	}
	oldStatus := sm.Status

	if oldStatus == ScrapeMediaStatusScrapeFailed || oldStatus == ScrapeMediaStatusScanned {
		mediaId := sm.MediaId
		if sm.MediaType == MediaTypeTvShow {
			// 将所有关联的ScrapeMediaFile设置为未刮削
			updateData := make(map[string]any)
			updateData["is_re_scrape"] = true
			updateData["re_scrape_time"] = time.Now().Unix()
			updateData["name"] = sm.Name
			updateData["year"] = sm.Year
			updateData["tmdb_id"] = sm.TmdbId
			updateData["status"] = ScrapeMediaStatusScanned
			updateData["media_season_id"] = 0
			updateData["media_episode_id"] = 0
			updateData["failed_reason"] = ""
			updateData["media_id"] = 0
			db.Db.Where("id = ?", mediaId).Delete(&Media{})
			db.Db.Where("media_id = ?", mediaId).Delete(&MediaSeason{})
			db.Db.Where("media_id = ?", mediaId).Delete(&MediaEpisode{})
			// if sm.PathId == "" {
			if err := db.Db.Model(&ScrapeMediaFile{}).Where("tvshow_path_id = ? and batch_no = ?", sm.TvshowPathId, sm.BatchNo).Updates(updateData).Error; err != nil {
				helpers.AppLogger.Errorf("重新刮削时更新电视剧内所有剧集失败: %v", err)
				return err
			}
			if err := db.Db.Where("id = ?", sm.ID).Updates(updateData).Error; err != nil {
				helpers.AppLogger.Errorf("重新刮削时更新剧集失败: %v", err)
				return err
			}
			// } else {
			// 	if err := db.Db.Model(&ScrapeMediaFile{}).Where("path_id = ? and batch_no = ?", sm.PathId, sm.BatchNo).Updates(updateData).Error; err != nil {
			// 		helpers.AppLogger.Errorf("重新刮削时更新电视剧内其他剧集失败2: %v", err)
			// 		return err
			// 	}
			// 	db.Db.Where("id = ?", sm.ID).Updates(updateData)
			// }

			hasEdit := false
			// 检查输入的季是否存在
			if season > 0 {
				tvSeason, err := tmdbClient.GetTvSeasonDetail(sm.TmdbId, season, GlobalScrapeSettings.GetTmdbLanguage())
				if err != nil || tvSeason == nil {
					helpers.AppLogger.Errorf("查询tmdb剧集 季 %d 查询失败: %v", season, err)
					return err
				} else {
					sm.SeasonNumber = season
					hasEdit = true
				}
				// 检查集是否存在
				if episode > 0 {
					tvEpisode, err := tmdbClient.GetTvEpisodeDetail(sm.TmdbId, season, episode, GlobalScrapeSettings.GetTmdbLanguage())
					if err != nil || tvEpisode == nil {
						helpers.AppLogger.Errorf("查询tmdb剧集 季 %d 集 %d 查询失败: %v", season, episode, err)
						return err
					} else {
						sm.EpisodeNumber = episode
						hasEdit = true
					}
				}
				// 保存
				if hasEdit {
					sm.Save()
				}
			}
		} else {
			sm.IsReScrape = true
			sm.ReScrapeTime = time.Now().Unix()
			sm.Status = ScrapeMediaStatusScanned
			sm.FailedReason = ""
			sm.MediaId = 0
			db.Db.Where("id = ?", mediaId).Delete(&Media{})
			sm.Save()
		}
	}
	if oldStatus == ScrapeMediaStatusRenamed {
		sm.Status = ScrapeMediaStatusRollbacking
		sm.Save()
		if sm.MediaType == MediaTypeTvShow {
			// 将所有关联的ScrapeMediaFile设置为回滚中
			updateData := make(map[string]any)
			updateData["name"] = sm.Name
			updateData["year"] = sm.Year
			updateData["tmdb_id"] = sm.TmdbId
			updateData["status"] = ScrapeMediaStatusRollbacking
			updateData["failed_reason"] = ""
			if sm.TvshowPathId != "" {
				if err := db.Db.Model(&ScrapeMediaFile{}).Where("tvshow_path_id = ? and batch_no = ?", sm.TvshowPathId, sm.BatchNo).Updates(updateData).Error; err != nil {
					helpers.AppLogger.Errorf("重新刮削时更新电视剧内其他剧集失败1: %v", err)
					return err
				}
			} else {
				if err := db.Db.Model(&ScrapeMediaFile{}).Where("path_id = ? and batch_no = ?", sm.PathId, sm.BatchNo).Updates(updateData).Error; err != nil {
					helpers.AppLogger.Errorf("重新刮削时更新电视剧内其他剧集失败2: %v", err)
					return err
				}
			}
		}
	}
	return nil
}

func (sm *ScrapeMediaFile) ExtractSeasonEpisode(sp *ScrapePath) error {
	if sm.EpisodeNumber == -1 {
		// 先识别季集
		info := helpers.ExtractMediaInfoRe(sm.VideoFilename, false, true, sp.VideoExtList, sp.DeleteKeyword...)
		if info == nil {
			helpers.AppLogger.Errorf("使用正则从文件名中提取媒体信息失败，文件名 %s", sm.VideoFilename)
			return errors.New("使用正则从文件名中提取媒体信息失败")
		}
		sm.EpisodeNumber = info.Episode
		sm.SeasonNumber = info.Season
		helpers.AppLogger.Infof("从文件名中提取到季集: %s %d, %d", sm.VideoFilename, sm.SeasonNumber, sm.EpisodeNumber)
	}
	// 提取季相对目录（相对来源目录）
	relSeasonPath, _ := filepath.Rel(sm.SourcePath, sm.TvshowPath)
	// 分割路径
	pathPartCount := strings.Count(relSeasonPath, string(os.PathSeparator))
	seasonFolderNumber := helpers.ExtractSeasonsFromSeasonPath(filepath.Base(sm.TvshowPath))
	helpers.AppLogger.Infof("季相对目录: %s", relSeasonPath)
	helpers.AppLogger.Infof("识别到的季编号：%d, 路径切片数量：%d", seasonFolderNumber, pathPartCount)
	if seasonFolderNumber >= 0 && pathPartCount >= 1 {
		// 有季文件夹
		sm.Path = sm.TvshowPath
		sm.PathId = sm.TvshowPathId
		sm.TvshowPath = filepath.Dir(sm.Path)
		if sm.SourceType != SourceType115 {
			sm.TvshowPathId = filepath.Dir(sm.PathId)
		} else {
			sm.TvshowPathId = ""
		}
		helpers.AppLogger.Infof("识别到有季文件夹，电视剧路径: %s， 季路径：%s", sm.TvshowPath, sm.Path)
		if sm.SeasonNumber == -1 {
			// 从季文件夹中提取
			sm.SeasonNumber = seasonFolderNumber
			helpers.AppLogger.Infof("从季文件夹中提取到季数: %d", sm.SeasonNumber)
		}
	} else {
		// 无季文件夹
		helpers.AppLogger.Infof("识别到无季文件夹，电视剧路径: %s", sm.TvshowPath)
		seasonNumber := helpers.ExtractSeasonFromTvshowPath(sm.TvshowPath)
		if seasonNumber != -1 {
			sm.SeasonNumber = seasonNumber
			helpers.AppLogger.Infof("从电视剧文件夹中提取到季数: %d", sm.SeasonNumber)
		}
	}
	if sm.SeasonNumber == -1 {
		sm.SeasonNumber = 1
	}
	sm.Save()
	return nil
}

// 返回电影或电视剧的目标目录路径
func (sm *ScrapeMediaFile) GetMovieOrTvshowDestPath() (string, string) {
	newPathId := sm.NewPathId
	if sm.ScrapeType == ScrapeTypeOnly {
		if sm.TvshowPath != "" {
			newPathId = sm.TvshowPathId
		} else {
			newPathId = sm.PathId
		}
	}
	return sm.GetDestFullMoviePath(), newPathId
}

// 返回来源端的电影路径，会去掉sp.SourcePath
// 例如 电影/我是一个中国人 (2025)
func (sm *ScrapeMediaFile) GetRemoteMoviePath() string {
	return strings.Replace(sm.Path, sm.SourcePath, "", 1)
}

// 来源端是否有季文件夹
func (sm *ScrapeMediaFile) HasRemoteSeasonPath() bool {
	return sm.Path != ""
}

// 返回完整的来源端电影路径
func (sm *ScrapeMediaFile) GetRemoteFullMoviePath() string {
	return filepath.Join(sm.SourcePath, sm.GetRemoteMoviePath())
}

// 返回来源端的电视剧路径，会去掉sp.SourcePath
// 例如 电视剧/我是一个中国人 (2025)
func (sm *ScrapeMediaFile) GetRemoteTvshowPath() string {
	return strings.Replace(sm.TvshowPath, sm.SourcePath, "", 1)
}

// 返回完整的来源端电视剧路径
func (sm *ScrapeMediaFile) GetRemoteFullTvshowPath() string {
	return sm.TvshowPath
}

// 返回来源端的季路径，会去掉sp.SourcePath
// 例如：电视剧/我是一个中国人 (2025)/Season 1
func (sm *ScrapeMediaFile) GetRemoteSeasonPath() string {
	if sm.Path != "" {
		return strings.Replace(sm.Path, sm.TvshowPath, "", 1)
	}
	return ""
}

// 返回完整的来源端季路径
func (sm *ScrapeMediaFile) GetRemoteFullSeasonPath() string {
	if sm.Path == "" {
		return sm.TvshowPath
	}
	return sm.Path
}

// 返回目标端电影路径
// 例如：华语电影/毕正明的证明 (2025)
func (sm *ScrapeMediaFile) GetDestMoviePath() string {
	remotePath := sm.GetRemoteMoviePath()
	if sm.ScrapeType == ScrapeTypeOnly {
		return remotePath
	}
	if sm.EnableCategory && sm.CategoryName != "" {
		return filepath.Join(sm.CategoryName, sm.NewPathName)
	}
	return sm.NewPathName
}

// 返回目标端电视剧路径
// 例如：国产剧剧/我是一个中国人 (2025)
func (sm *ScrapeMediaFile) GetDestTvshowPath() string {
	remotePath := sm.GetRemoteTvshowPath()
	if sm.ScrapeType == ScrapeTypeOnly {
		return remotePath
	}
	if sm.EnableCategory && sm.CategoryName != "" {
		return filepath.Join(sm.CategoryName, sm.NewPathName)
	}
	return sm.NewPathName
}

// 返回目标端季路径
func (sm *ScrapeMediaFile) GetDestSeasonPath() string {
	remoteSeasonPath := sm.GetRemoteSeasonPath()
	if sm.ScrapeType == ScrapeTypeOnly {
		return remoteSeasonPath
	}
	return fmt.Sprintf("Season %d", sm.SeasonNumber)
}

// 返回电影的临时刮削目录完整路径
func (sm *ScrapeMediaFile) GetTmpFullMoviePath() string {
	return filepath.Join(sm.ScrapeRootPath, sm.GetDestMoviePath())
}

func (sm *ScrapeMediaFile) GetTmpFullTvshowPath() string {
	return filepath.Join(sm.ScrapeRootPath, sm.GetDestTvshowPath())
}

func (sm *ScrapeMediaFile) GetTmpFullSeasonPath() string {
	return filepath.Join(sm.GetTmpFullTvshowPath(), sm.GetDestSeasonPath())
}

// 返回电影的目标端完整路径
func (sm *ScrapeMediaFile) GetDestFullMoviePath() string {
	path := filepath.Join(sm.DestPath, sm.GetDestMoviePath())
	if sm.ScrapeType == ScrapeTypeOnly {
		path = filepath.Join(sm.SourcePath, sm.GetDestMoviePath())
	}

	// helpers.AppLogger.Debugf("电影目标端完整路径: %s", path)
	return path
}

// 返回电视剧的目标端完整路径
func (sm *ScrapeMediaFile) GetDestFullTvshowPath() string {
	path := filepath.Join(sm.DestPath, sm.GetDestTvshowPath())
	if sm.ScrapeType == ScrapeTypeOnly {
		path = filepath.Join(sm.SourcePath, sm.GetDestTvshowPath())
	}
	return path
}

// 返回目标端季路径完整路径
func (sm *ScrapeMediaFile) GetDestFullSeasonPath() string {
	return filepath.Join(sm.GetDestFullTvshowPath(), sm.GetDestSeasonPath())
}

// 生成季的nfo文件的名字
// 如果有单独的季目录，则命名为season.nfo
// 如果没有单独的季目录，则命名为season01.nfo
func (sm *ScrapeMediaFile) GetSeasonNfoName() string {
	hasSeasonPath := sm.HasRemoteSeasonPath()
	if sm.ScrapeType == ScrapeTypeOnly {
		if !hasSeasonPath {
			return fmt.Sprintf("season%02d.nfo", sm.MediaSeason.SeasonNumber)
		} else {
			return "season.nfo"
		}
	}
	return "season.nfo"
}

// 返回集的nfo文件名
func (sm *ScrapeMediaFile) GetEpisodeNfoName() string {
	return fmt.Sprintf("%s.nfo", sm.NewVideoBaseName)
}

// 返回集的封面文件名
func (sm *ScrapeMediaFile) GetEpisodePosterName() string {
	return fmt.Sprintf("%s.jpg", sm.NewVideoBaseName)
}

func GetScrapeMediaByVideoFileId(videoFileId string) (*ScrapeMediaFile, error) {
	var media ScrapeMediaFile
	err := db.Db.Where("video_file_id = ?", videoFileId).First(&media).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &media, nil
}

// 从数据库中查询所有scrapemedia
// id倒序
// 先查询总数
// 再查询列表
func GetScrapeMediaFiles(page int, pageSize int, mediaType string, status string, name string) (int64, []*ScrapeMediaFile) {
	offset := (page - 1) * pageSize
	var scrapeMediaFiles []*ScrapeMediaFile
	tx := db.Db.Order("id desc").Offset(offset).Limit(pageSize).Order("id DESC")
	txc := db.Db.Model(&ScrapeMediaFile{})
	condition := make(map[string]interface{})
	if mediaType != "" {
		condition["media_type"] = mediaType
	}
	if status != "" {
		condition["status"] = status
	}
	if len(condition) > 0 {
		tx.Where(condition)
		txc.Where(condition)
	}
	if name != "" {
		tx.Where("(path LIKE ? OR video_filename LIKE ? OR name LIKE ?)", fmt.Sprintf("%%%s%%", name), fmt.Sprintf("%%%s%%", name), fmt.Sprintf("%%%s%%", name))
		txc.Where("(path LIKE ? OR video_filename LIKE ? OR name LIKE ?)", fmt.Sprintf("%%%s%%", name), fmt.Sprintf("%%%s%%", name), fmt.Sprintf("%%%s%%", name))
	}
	err := tx.Find(&scrapeMediaFiles).Error
	if err != nil {
		helpers.AppLogger.Errorf("查询scrapemedia失败: %v", err)
		return 0, nil
	}
	var total int64
	err = txc.Count(&total).Error
	if err != nil {
		helpers.AppLogger.Errorf("查询scrapemedia总数失败: %v", err)
		return 0, nil
	}
	// helpers.AppLogger.Infof("查询scrapemedia总数: %d, 当前页 %d offset %d 数量: %d", total, page, offset, len(scrapeMediaFiles))
	scrapeMediaFiles = DecodeScrapeMediaFile(scrapeMediaFiles)
	return total, scrapeMediaFiles
}

// 查询所有待刮削或者待整理的记录，不分页
func GetAllScannedScrapeMediaFiles(scrapePathId uint, mediaType MediaType) []*ScrapeMediaFile {
	var scrapeMediaFiles []*ScrapeMediaFile
	if err := db.Db.Where("scrape_path_id = ? AND status IN ? AND media_type = ?", scrapePathId, []ScrapeMediaStatus{ScrapeMediaStatusScanned, ScrapeMediaStatusScraped}, mediaType).Order("id desc").Find(&scrapeMediaFiles).Error; err != nil {
		helpers.AppLogger.Errorf("查询待刮削文件失败: %v", err)
		return nil
	}
	return DecodeScrapeMediaFile(scrapeMediaFiles)
}

// 查询所有待刮削或者待整理的记录，只取limit条
func GetScannedScrapeMediaFiles(scrapePathId uint, mediaType MediaType, limit int) []*ScrapeMediaFile {
	var scrapeMediaFiles []*ScrapeMediaFile
	if err := db.Db.Where("scrape_path_id = ? AND status IN ? AND media_type = ?", scrapePathId, []ScrapeMediaStatus{ScrapeMediaStatusScanned, ScrapeMediaStatusScraped}, mediaType).Order("id desc").Limit(limit).Order("id asc").Find(&scrapeMediaFiles).Error; err != nil {
		helpers.AppLogger.Errorf("查询待刮削文件失败: %v", err)
		return nil
	}
	return DecodeScrapeMediaFile(scrapeMediaFiles)
}

// 查询所有待刮削或者待整理的记录总数
func GetScannedScrapeMediaFilesTotal(scrapePathId uint, mediaType MediaType) int64 {
	var total int64
	if err := db.Db.Model(&ScrapeMediaFile{}).Where("scrape_path_id = ? AND status IN ? AND media_type = ?", scrapePathId, []ScrapeMediaStatus{ScrapeMediaStatusScanned, ScrapeMediaStatusScraped}, mediaType).Count(&total).Error; err != nil {
		helpers.AppLogger.Errorf("查询待刮削文件失败: %v", err)
		return 0
	}
	return total
}

// 电视剧刮削使用，按照path_id分组返回（分页查询优化版）
func GetScannedScrapeMediaFilesGroupByTvshowPathId(scrapePathId uint, limit int) []*ScrapeMediaFile {
	// 使用分页查询，每次处理1000条记录，避免内存溢出
	const pageSize = 1000
	var page = 1
	var result []*ScrapeMediaFile
	groupedFiles := make(map[string]*ScrapeMediaFile)
	// 分页查询，直到达到limit或没有更多数据
	for {
		var pageFiles []*ScrapeMediaFile
		currentOffset := (page - 1) * pageSize
		// 查询当前页的数据
		if err := db.Db.Where("scrape_path_id = ? AND status IN ?", scrapePathId, []ScrapeMediaStatus{ScrapeMediaStatusScanned, ScrapeMediaStatusScraped}).
			Order("id asc").
			Limit(pageSize).
			Offset(currentOffset).
			Find(&pageFiles).Error; err != nil {
			helpers.AppLogger.Errorf("查询待刮削文件失败: %v", err)
			break
		}
		// 如果没有更多数据，退出循环
		if len(pageFiles) == 0 {
			break
		}

		// 处理当前页的数据
		for _, file := range pageFiles {
			key := fmt.Sprintf("%s-%d", file.TvshowPath, file.SeasonNumber)
			if _, exists := groupedFiles[key]; !exists {
				groupedFiles[key] = file
				result = append(result, file)

				// 如果达到限制数量，立即返回
				if len(result) >= limit {
					return DecodeScrapeMediaFile(result)
				}
			}
		}
		if len(pageFiles) < pageSize {
			break
		}
		page++
	}
	if len(result) == 0 {
		return nil
	}
	return DecodeScrapeMediaFile(result)
}

// 查询所有已刮削的记录，不分页
func GetAllScrappedMedialFiles(scrapePathId uint, mediaType MediaType) []*ScrapeMediaFile {
	var scrapeMediaFiles []*ScrapeMediaFile
	if err := db.Db.Where("scrape_path_id = ? AND status = ? AND media_type = ?", scrapePathId, ScrapeMediaStatusScraped, mediaType).Order("id desc").Find(&scrapeMediaFiles).Error; err != nil {
		helpers.AppLogger.Errorf("查询已刮削文件失败: %v", err)
		return nil
	}
	return DecodeScrapeMediaFile(scrapeMediaFiles)
}

// 查询某一季的所有集
func GetScrapeMediaFileIdBySeasonId(seasonMediaId uint) []uint {
	var episodeIds []uint
	if err := db.Db.Model(&ScrapeMediaFile{}).Where("media_season_id = ?", seasonMediaId).Pluck("id", &episodeIds).Error; err != nil {
		helpers.AppLogger.Errorf("查询集ID失败: %v", err)
		return nil
	}
	return episodeIds
}

func GetScrapeMediaFilesByIds(ids []uint) []*ScrapeMediaFile {
	var scrapeMediaFiles []*ScrapeMediaFile
	if err := db.Db.Where("id IN ?", ids).Order("id desc").Find(&scrapeMediaFiles).Error; err != nil {
		helpers.AppLogger.Errorf("查询scrapemedia失败: %v", err)
		return nil
	}
	scrapeMediaFiles = DecodeScrapeMediaFile(scrapeMediaFiles)
	return scrapeMediaFiles
}

func DecodeScrapeMediaFile(scrapeMediaFiles []*ScrapeMediaFile) []*ScrapeMediaFile {
	mediaIdList := make([]uint, 0)
	mediaSeasonIdList := make([]uint, 0)
	mediaEpisodeIdList := make([]uint, 0)
	// 解码JSON字符串
	for _, sm := range scrapeMediaFiles {
		sm.DecodeJson()
		if sm.MediaId > 0 {
			mediaIdList = append(mediaIdList, sm.MediaId)
		}
		if sm.MediaSeasonId > 0 {
			mediaSeasonIdList = append(mediaSeasonIdList, sm.MediaSeasonId)
		}
		if sm.MediaEpisodeId > 0 {
			mediaEpisodeIdList = append(mediaEpisodeIdList, sm.MediaEpisodeId)
		}
	}
	// 根据IdList查询数据，然后赋值给mediaFiles
	var medias []*Media
	var mediaSeasons []*MediaSeason
	var mediaEpisodes []*MediaEpisode
	if len(mediaIdList) > 0 {
		if err := db.Db.Where("id IN ?", mediaIdList).Find(&medias).Error; err != nil {
			helpers.AppLogger.Errorf("查询Media失败: %v", err)
		}
	}
	if len(mediaSeasonIdList) > 0 {
		if err := db.Db.Where("id IN ?", mediaSeasonIdList).Find(&mediaSeasons).Error; err != nil {
			helpers.AppLogger.Errorf("查询MediaSeason失败: %v", err)
		}
	}
	if len(mediaEpisodeIdList) > 0 {
		if err := db.Db.Where("id IN ?", mediaEpisodeIdList).Find(&mediaEpisodes).Error; err != nil {
			helpers.AppLogger.Errorf("查询MediaEpisode失败: %v", err)
		}
	}
	// 赋值给mediaFiles
	for _, sm := range scrapeMediaFiles {
		for _, m := range medias {
			if m.ID == sm.MediaId {
				m.DecodeJson()
				sm.Media = m
				break
			}
		}
		if sm.MediaSeasonId > 0 {
			for _, ms := range mediaSeasons {
				if ms.ID == sm.MediaSeasonId {
					sm.MediaSeason = ms
					break
				}
			}
		}
		if sm.MediaEpisodeId > 0 {
			for _, me := range mediaEpisodes {
				if me.ID == sm.MediaEpisodeId {
					me.DecodeJson()
					sm.MediaEpisode = me
					break
				}
			}
		}
	}
	return scrapeMediaFiles
}

// 根据ID查询ScrapeMediaFile
func GetScrapeMediaFileById(id uint) *ScrapeMediaFile {
	var scrapeMediaFile ScrapeMediaFile
	if err := db.Db.Where("id = ?", id).First(&scrapeMediaFile).Error; err != nil {
		helpers.AppLogger.Errorf("查询scrapemedia失败: %v", err)
		return nil
	}
	scrapeMediaFile.DecodeJson()
	// 查询关联的数据
	scrapeMediaFile.QueryRelation()
	return &scrapeMediaFile
}

func CheckExistsFileIdAndName(fileId string, scrapePathId uint) bool {
	// helpers.AppLogger.Infof("检查文件是否存在: %s, %s, %d", fileId, fileName, syncPathId)
	var total int64
	err := db.Db.Model(&ScrapeMediaFile{}).Where("video_file_id = ? AND scrape_path_id = ?", fileId, scrapePathId).Count(&total).Error
	if err != nil {
		helpers.AppLogger.Errorf("检查文件是否存在失败: %v", err)
		return false
	}
	return total > 0
}

func CheckScrapeMediaFileExists(fileId string) bool {
	var file ScrapeMediaFile
	db.Db.Where("file_id = ?", fileId).First(&file)
	return file.ID > 0
}

// 将所有old改为new
func UpdateScrapeMediaStatus(old, new ScrapeMediaStatus, scrapePathId uint) {
	var err error
	if scrapePathId > 0 {
		err = db.Db.Model(&ScrapeMediaFile{}).Where("scrape_path_id = ? AND status = ?", scrapePathId, old).Update("status", new).Error
	} else {
		err = db.Db.Model(&ScrapeMediaFile{}).Where("status = ?", old).Update("status", new).Error
	}
	if err != nil {
		helpers.AppLogger.Errorf("将全部整理中改为待整理状态失败: %v", err)
	} else {
		helpers.AppLogger.Infof("将全部整理中改为待整理状态成功")
	}
}

// 清除所有刮削失败的记录，包括subtitlefiles
func ClearFailedScrapeRecords(ids []uint) error {
	// 查询所有失败的记录
	var failedScrapeMediaFiles []*ScrapeMediaFile
	if len(ids) == 0 {
		if err := db.Db.Where("status = ?", ScrapeMediaStatusScrapeFailed).Find(&failedScrapeMediaFiles).Error; err != nil {
			helpers.AppLogger.Errorf("查询所有失败的记录失败: %v", err)
			return err
		}
	} else {
		if err := db.Db.Where("id IN ?", ids).Find(&failedScrapeMediaFiles).Error; err != nil {
			return err
		}
	}
	idArray := make([]uint, 0)
	mediaIdArray := make([]uint, 0)
	// 找出所有保存下来的fileid
	for _, sm := range failedScrapeMediaFiles {
		idArray = append(idArray, sm.ID)
		if !slices.Contains(mediaIdArray, sm.MediaId) {
			mediaIdArray = append(mediaIdArray, sm.MediaId)
		}
	}
	err := db.Db.Where("id IN ?", idArray).Delete(&ScrapeMediaFile{}).Error
	if err != nil {
		helpers.AppLogger.Errorf("删除刮削记录失败: %v", err)
		return err
	}
	// 删除所有Media
	if len(mediaIdArray) > 0 {
		if err := db.Db.Where("id IN ?", mediaIdArray).Delete(&Media{}).Error; err != nil {
			helpers.AppLogger.Errorf("删除Media失败: %v", err)
			return err
		}
		// 删除所有mediaSeason和mediaEpisode
		if err := db.Db.Where("media_id IN ?", mediaIdArray).Delete(&MediaSeason{}).Error; err != nil {
			helpers.AppLogger.Errorf("删除MediaSeason失败: %v", err)
			return err
		}
		if err := db.Db.Where("media_id IN ?", mediaIdArray).Delete(&MediaEpisode{}).Error; err != nil {
			helpers.AppLogger.Errorf("删除MediaEpisode失败: %v", err)
			return err
		}
	}
	helpers.AppLogger.Infof("删除刮削记录成功")
	return nil
}

func RenameFailedScrapeRecords(ids []uint) error {
	updateData := make(map[string]interface{}, 0)
	updateData["status"] = ScrapeMediaStatusScanned
	updateData["failed_reason"] = ""
	err := db.Db.Model(&ScrapeMediaFile{}).Where("id IN ?", ids).Updates(updateData).Error
	if err != nil {
		helpers.AppLogger.Errorf("将所有失败的记录标记为待整理状态失败: %v", err)
	}
	// 查询所有scrape_media_file
	var scrapeMediaFiles []*ScrapeMediaFile
	if err := db.Db.Where("id IN ?", ids).Find(&scrapeMediaFiles).Error; err != nil {
		helpers.AppLogger.Errorf("查询所有失败的记录失败: %v", err)
		return err
	}
	// 将所选记录对应的Media或者MediaEpisode标记为待整理
	for _, sm := range scrapeMediaFiles {
		if sm.MediaEpisodeId > 0 {
			db.Db.Model(&MediaEpisode{}).Where("id = ?", sm.MediaEpisodeId).Update("status", MediaStatusUnScraped)
			continue
		}
		if sm.MediaId > 0 && sm.MediaType != MediaTypeTvShow {
			db.Db.Model(&Media{}).Where("id = ?", sm.MediaId).Update("status", MediaStatusUnScraped)
		}
	}
	helpers.AppLogger.Infof("将所有失败的记录标记为待整理状态成功")
	return nil
}

func GetUnFinishEpisodeCount(mediaFile *ScrapeMediaFile) int64 {
	var total int64
	db.Db.Model(&ScrapeMediaFile{}).Where("media_id = ? AND batch_no = ? AND status NOT IN ?", mediaFile.MediaId, mediaFile.BatchNo, []ScrapeMediaStatus{ScrapeMediaStatusRenamed, ScrapeMediaStatusRenameFailed, ScrapeMediaStatusScrapeFailed}).Count(&total)
	return total
}

func GetAllEpisodeByTvshow(mediaId uint, batchNo string) []*ScrapeMediaFile {
	var scrapeMediaFiles []*ScrapeMediaFile
	db.Db.Model(&ScrapeMediaFile{}).Where("media_id = ? AND batch_no = ?", mediaId, batchNo).Find(&scrapeMediaFiles)
	return scrapeMediaFiles
}

// TruncateAllScrapeRecords 清空所有刮削记录
// 使用DELETE命令清空ScrapeMediaFile、Media、MediaSeason、MediaEpisode四张表
func TruncateAllScrapeRecords() error {
	// 按顺序删除表数据，注意外键依赖关系
	// 先清空子表（MediaEpisode, MediaSeason），再清空父表（Media），最后清空ScrapeMediaFile
	tables := []string{
		"media_episodes",
		"media_seasons",
		"media",
		"scrape_media_files",
	}

	for _, tableName := range tables {
		if err := db.Db.Exec(fmt.Sprintf("DELETE FROM %s", tableName)).Error; err != nil {
			helpers.AppLogger.Errorf("清空表 %s 失败: %v", tableName, err)
			return fmt.Errorf("清空表 %s 失败: %v", tableName, err)
		}
		helpers.AppLogger.Infof("清空表 %s 成功", tableName)
	}

	helpers.AppLogger.Info("清空所有刮削记录成功")
	return nil
}
