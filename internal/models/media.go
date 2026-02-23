package models

import (
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/tmdb"
	"encoding/json"
	"fmt"
	"strings"
)

type MediaStatus string

const (
	MediaStatusScraped   MediaStatus = "scraped" // 已刮削
	MediaStatusUnScraped MediaStatus = "scanned" // 待刮削
	MediaStatusRenamed   MediaStatus = "renamed" // 已重命名
)

// 电影或电视剧，刮削后的信息
// 每一个刮削过的视频都对应一条Media纪录
type Media struct {
	BaseModel
	ScrapePathId        uint               `json:"scrape_path_id" gorm:"index:scrapepathid"` // 刮削路径ID
	TmdbId              int64              `json:"tmdb_id" gorm:"index:tmdbid"`              // TMDB ID
	ImdbId              string             `json:"imdb_id"`                                  // IMDB ID
	Name                string             `json:"name" gorm:"index:nameyear"`               // TMDB名称
	Year                int                `json:"year" gorm:"index:nameyear"`               // 年份
	OriginalName        string             `json:"original_title"`                           // 原始标题
	MediaType           MediaType          `json:"media_type"`                               // 媒体类型: movie-电影 tvshow-电视剧
	ReleaseDate         string             `json:"release_date"`                             // 上映时间，剧集为首播时间
	Actors              []helpers.Actor    `json:"actors" gorm:"-"`                          // 演员列表
	ActorsJson          string             `json:"-" gorm:"type:text"`                       // 演员列表JSON字符串
	Director            []helpers.Director `json:"director" gorm:"-"`                        // 导演列表
	DirectorJson        string             `json:"-" gorm:"type:text"`                       // 导演列表JSON字符串
	Overview            string             `json:"overview"`                                 // 媒体描述
	Tagline             string             `json:"tagline"`                                  // 媒体标语
	PosterPath          string             `json:"poster_path"`                              // 海报路径, poster.jpg 竖版，推荐 1000x1500 像素 比例: 2:3
	BackdropPath        string             `json:"backdrop_path"`                            // 背景路径，生成三个文件：fanart.jpg backdrop.jpg background.jpg 横版，推荐1920x1080 像素 比例: 16:9
	LogoPath            string             `json:"logo_path"`                                // logo路径，clearlogo.jpg
	ThumbPath           string             `json:"thumb_path"`                               // 缩略图路径, thumb.jpg 推荐 400x300 像素 比例: 4:3 // 暂时不用
	LandscapePath       string             `json:"landscape_path"`                           // 横屏海报路径，landscape.jpg 尺寸: 约 1000x562 比例: 16:9 // 暂时不用
	BannerPath          string             `json:"banner_path"`                              // 超宽横幅路径，banner.jpg 尺寸: 约 1000x185 像素 5.4:1 (超宽) 暂时不用
	VoteAverage         float64            `json:"vote_average"`                             // 投票平均分
	VoteCount           int64              `json:"vote_count"`                               // 投票数
	OriginalLanguage    string             `json:"original_language"`                        // 原始语言
	OriginCountry       []string           `json:"origin_country" gorm:"-"`                  // 原始国家
	OriginalCountryJson string             `json:"-"`                                        // 原始国家JSON字符串
	Genres              []tmdb.Genre       `json:"genres" gorm:"-"`                          // 流派
	GenresJson          string             `json:"-"`                                        // 流派JSON字符串
	Runtime             int64              `json:"runtime"`                                  // 运行时间，单位：分钟
	LastAirDate         string             `json:"last_air_date"`                            // 最后一集播出时间
	NumberOfEpisodes    int                `json:"number_of_episodes"`                       // 剧集总数
	NumberOfSeasons     int                `json:"number_of_seasons"`                        // 季数
	Num                 string             `json:"num"`                                      // 番号
	MpaaRating          string             `json:"mpaa_rating"`                              // MPAA分级
	Path                string             `json:"path"`                                     // 刮削整理后的电影或者电视剧的路径
	PathId              string             `json:"path_id"`                                  // 刮削整理后的电影或者电视剧的路径ID
	VideoFileName       string             `json:"video_file_name"`                          // 刮削整理后的电影或者电视剧的视频文件名
	VideoFileId         string             `json:"video_file_id"`                            // 刮削整理后的电影或者电视剧的视频文件ID，完整路径或者网盘文件ID
	VideoPickCode       string             `json:"video_pick_code"`                          // 115 pickcode 或者 百度网盘 fsid
	VideoOpenListSign   string             `json:"video_open_list_sign"`                     // openlist签名
	Status              MediaStatus        `gorm:"index" json:"status"`                      // 状态
	SubtitleFiles       []*MediaMetaFiles  `json:"subtitle_files" gorm:"-"`                  // 整理后的字幕文件列表
	SubtitleFileJson    string             `json:"-"`                                        // SubtitleFiles的JSON字符串
}

// 刮削好数据的集
type MediaEpisode struct {
	BaseModel
	ScrapePathId      uint              `json:"scrape_path_id" gorm:"index:scrapepathid"` // 刮削路径ID
	MediaId           uint              `gorm:"index" json:"media_id"`                    // 媒体ID
	MediaSeasonId     uint              `gorm:"index" json:"media_season_id"`             // 季ID
	EpisodeName       string            `json:"episode_name"`                             // 集名称
	Overview          string            `json:"overview"`                                 // 集描述
	PosterPath        string            `json:"poster_path"`                              // 海报路径 // TMDB链接
	SeasonNumber      int               `gorm:"index" json:"season_number"`               // 季编号，例如：S01中的1
	EpisodeNumber     int               `gorm:"index" json:"episode_number"`              // 集编号，例如：S01E01中的1
	ReleaseDate       string            `json:"release_date"`                             // 发布日期
	VoteAverage       float64           `json:"vote_average"`                             // 投票平均分
	VoteCount         int64             `json:"vote_count"`                               // 投票数
	Actors            []helpers.Actor   `json:"actors" gorm:"-"`                          // 演员列表
	ActorsJson        string            `json:"-"`                                        // 演员列表JSON字符串
	Year              int               `json:"year"`                                     // 年份
	VideoFileName     string            `json:"video_file_name"`                          // 刮削整理后的电影或者电视剧的视频文件名
	VideoFileId       string            `json:"video_file_id"`                            // 刮削整理后的电影或者电视剧的视频文件ID，完整路径或者网盘文件ID
	VideoPickCode     string            `json:"video_pick_code"`                          // 115 pickcode 或者 百度网盘 fsid
	VideoOpenListSign string            `json:"video_open_list_sign"`                     // openlist签名
	Status            MediaStatus       `gorm:"index" json:"status"`                      // 状态
	SubtitleFiles     []*MediaMetaFiles `json:"subtitle_files" gorm:"-"`                  // 整理后的字幕文件列表
	SubtitleFileJson  string            `json:"-"`                                        // SubtitleFiles的JSON字符串
}

// 刮削好数据的季
type MediaSeason struct {
	BaseModel
	ScrapePathId     uint        `json:"scrape_path_id" gorm:"index:scrapepathid"` // 刮削路径ID
	MediaId          uint        `gorm:"index" json:"media_id"`                    // 媒体ID
	SeasonNumber     int         `gorm:"index" json:"season_number"`               // 季编号，例如：S01中的S1
	SeasonName       string      `json:"season_name"`                              // 季名称，可能为空
	Overview         string      `json:"overview"`                                 // 季描述
	NumberOfEpisodes int         `json:"number_of_episodes"`                       // 集总数
	ReleaseDate      string      `json:"release_date"`                             // 发布日期
	Path             string      `json:"path"`                                     // 刮削整理后的季文件夹路径，固定值：Season + SeasonNumber
	PathId           string      `json:"path_id"`                                  // 刮削整理后的季文件夹路径ID，完整路径或者网盘文件ID
	PosterPath       string      `json:"poster_path"`                              // 海报路径 // TMDB链接
	VoteAverage      float64     `json:"vote_average"`                             // 投票平均分
	Year             int         `json:"year"`                                     // 年份
	Status           MediaStatus `gorm:"index" json:"status"`                      // 状态
}

func (m *Media) Save() error {
	// 将所有字段转换为JSON字符串
	m.ActorsJson = helpers.JsonString(m.Actors)
	m.DirectorJson = helpers.JsonString(m.Director)
	m.OriginalCountryJson = helpers.JsonString(m.OriginCountry)
	m.GenresJson = helpers.JsonString(m.Genres)
	m.SubtitleFileJson = helpers.JsonString(m.SubtitleFiles)
	// 保存到数据库
	err := db.Db.Save(m).Error
	if err != nil {
		helpers.AppLogger.Errorf("保存Media到数据库失败: %v", err)
		return err
	}
	return nil
}

func (m *Media) DecodeJson() {
	// 解码JSON字符串
	if err := json.Unmarshal([]byte(m.ActorsJson), &m.Actors); err != nil {
		helpers.AppLogger.Errorf("解码ActorsJson失败: %v", err)
	}
	if err := json.Unmarshal([]byte(m.DirectorJson), &m.Director); err != nil {
		helpers.AppLogger.Errorf("解码DirectorJson失败: %v", err)
	}
	if err := json.Unmarshal([]byte(m.OriginalCountryJson), &m.OriginCountry); err != nil {
		helpers.AppLogger.Errorf("解码OriginalCountryJson失败: %v", err)
	}
	if err := json.Unmarshal([]byte(m.GenresJson), &m.Genres); err != nil {
		helpers.AppLogger.Errorf("解码GenresJson失败: %v", err)
	}
	if err := json.Unmarshal([]byte(m.SubtitleFileJson), &m.SubtitleFiles); err != nil {
		helpers.AppLogger.Errorf("解码SubtitleFileJson失败: %v", err)
	}
}

func (m *Media) UpdateSeasonCount(i int) {
	m.NumberOfSeasons += i
	// 保存
	updateData := map[string]interface{}{
		"number_of_seasons": m.NumberOfSeasons,
	}
	// 保存到数据库
	err := db.Db.Model(&Media{}).Where("id = ?", m.ID).Updates(updateData).Error
	if err != nil {
		helpers.AppLogger.Errorf("更新电视剧的总季数失败: %v", err)
	}
}

func (m *Media) UpdateEpisodeCount(i int) {
	m.NumberOfEpisodes += i
	// 保存
	updateData := map[string]interface{}{
		"number_of_episodes": m.NumberOfEpisodes,
	}
	// 保存到数据库
	err := db.Db.Model(&Media{}).Where("id = ?", m.ID).Updates(updateData).Error
	if err != nil {
		helpers.AppLogger.Errorf("更新电视剧的总剧集数失败: %v", err)
	}
}

func (ms *MediaSeason) Save() {
	// 保存到数据库
	err := db.Db.Save(ms).Error
	if err != nil {
		helpers.AppLogger.Errorf("保存季到数据库失败: %v", err)
	}
}

func (me *MediaEpisode) Save() {
	me.ActorsJson = helpers.JsonString(me.Actors)
	// 保存到数据库
	err := db.Db.Save(me).Error
	if err != nil {
		helpers.AppLogger.Errorf("保存剧集到数据库失败: %v", err)
	}
}

func (me *MediaEpisode) DecodeJson() {
	// 解码JSON字符串
	if err := json.Unmarshal([]byte(me.ActorsJson), &me.Actors); err != nil {
		helpers.AppLogger.Errorf("解码ActorsJson失败: %v", err)
	}
}

func (m *Media) FillInfoByTmdbInfo(tmdbInfo *TmdbInfo) {
	if m.MediaType == MediaTypeTvShow {
		m.TmdbId = tmdbInfo.TvShowDetail.ID
		m.Name = tmdbInfo.TvShowDetail.Name
		m.OriginalName = tmdbInfo.TvShowDetail.OriginalName
		m.ReleaseDate = tmdbInfo.TvShowDetail.FirstAirDate
		m.Overview = strings.TrimSpace(tmdbInfo.TvShowDetail.Overview)
		m.OriginCountry = tmdbInfo.TvShowDetail.OriginCountry
		m.Genres = tmdbInfo.TvShowDetail.Genres
		m.VoteAverage = tmdbInfo.TvShowDetail.VoteAverage
		m.VoteCount = tmdbInfo.TvShowDetail.VoteCount
		m.NumberOfSeasons = tmdbInfo.TvShowDetail.NumberOfSeasons
		m.NumberOfEpisodes = tmdbInfo.TvShowDetail.NumberOfEpisodes
		m.OriginalLanguage = tmdbInfo.TvShowDetail.OriginalLanguage
	} else {
		m.TmdbId = tmdbInfo.MovieDetail.ID
		m.Name = tmdbInfo.MovieDetail.Title
		m.OriginalName = tmdbInfo.MovieDetail.OriginalTitle
		m.ReleaseDate = tmdbInfo.MovieDetail.ReleaseDate
		m.Overview = tmdbInfo.MovieDetail.Overview
		m.OriginCountry = make([]string, 0)
		m.Genres = tmdbInfo.MovieDetail.Genres
		m.VoteAverage = tmdbInfo.MovieDetail.VoteAverage
		m.VoteCount = tmdbInfo.MovieDetail.VoteCount
		m.OriginalLanguage = tmdbInfo.MovieDetail.OriginalLanguage
		m.ImdbId = tmdbInfo.MovieDetail.ImdbID
		// 提取分级信息
		for _, releaseDate := range tmdbInfo.ReleasesDate {
			if releaseDate.ISO_3166_1 == "US" {
				for _, date := range releaseDate.ReleaseDates {
					if date.Type == 3 {
						m.MpaaRating = date.Certification
						break
					}
				}
			}
		}
	}
	// 解析演员
	actors := make([]helpers.Actor, 0)
	if tmdbInfo.Credits != nil && tmdbInfo.Credits.Cast != nil {
		for _, actor := range tmdbInfo.Credits.Cast {
			act := helpers.Actor{
				Name:    actor.Name,
				Role:    actor.Character,
				TmdbId:  actor.ID,
				Order:   actor.Order,
				Thumb:   "",
				Profile: fmt.Sprintf("https://www.themoviedb.org/person/%d", actor.ID),
			}
			if actor.ProfilePath != "" {
				act.Thumb = fmt.Sprintf("%s/t/p/original%s", GlobalScrapeSettings.GetTmdbImageUrl(), actor.ProfilePath)
			}
			actors = append(actors, act)
		}
	}
	m.Actors = actors
	// 解析导演
	directors := make([]helpers.Director, 0)
	if tmdbInfo.Credits != nil && tmdbInfo.Credits.Crew != nil {
		for _, director := range tmdbInfo.Credits.Crew {
			if director.Job == "Director" {
				directors = append(directors, helpers.Director{
					Name:   director.Name,
					TmdbId: director.ID,
				})
			}
		}
	}
	m.Director = directors
	// 解析图片
	if tmdbInfo.Images != nil && len(tmdbInfo.Images.Posters) > 0 {
		for _, poster := range tmdbInfo.Images.Posters {
			if poster.FilePath != "" {
				m.PosterPath = fmt.Sprintf("%s/t/p/original%s", GlobalScrapeSettings.GetTmdbImageUrl(), poster.FilePath)
				break
			}
		}
	}
	if tmdbInfo.Images != nil && len(tmdbInfo.Images.Backdrops) > 0 {
		for _, backdrop := range tmdbInfo.Images.Backdrops {
			if backdrop.FilePath != "" {
				m.BackdropPath = fmt.Sprintf("%s/t/p/original%s", GlobalScrapeSettings.GetTmdbImageUrl(), backdrop.FilePath)
				break
			}
		}
	}
	if tmdbInfo.Images != nil && len(tmdbInfo.Images.Logos) > 0 {
		for _, logo := range tmdbInfo.Images.Logos {
			if logo.FilePath != "" {
				m.LogoPath = fmt.Sprintf("%s/t/p/original%s", GlobalScrapeSettings.GetTmdbImageUrl(), logo.FilePath)
				break
			}
		}
	}
	m.Status = MediaStatusScraped
	m.Save()
}

func (ms *MediaSeason) FillInfoByTmdbInfo(seasonDetail *tmdb.SeasonDetail) {
	if seasonDetail == nil {
		ms.Save()
		return
	}
	ms.SeasonName = seasonDetail.Name
	ms.Overview = seasonDetail.Overview
	ms.PosterPath = fmt.Sprintf("%s/t/p/original%s", GlobalScrapeSettings.GetTmdbImageUrl(), seasonDetail.PosterPath)
	ms.ReleaseDate = seasonDetail.AirDate
	ms.VoteAverage = seasonDetail.VoteAverage
	ms.Year = helpers.ParseYearFromDate(ms.ReleaseDate)
	ms.Status = MediaStatusScraped
	ms.Save()
}

func (me *MediaEpisode) FillInfoByTmdbInfo(episodeDetail *tmdb.Episode) {
	if episodeDetail == nil {
		return
	}
	me.EpisodeName = episodeDetail.Name
	me.Overview = episodeDetail.Overview
	me.PosterPath = fmt.Sprintf("%s/t/p/original%s", GlobalScrapeSettings.GetTmdbImageUrl(), episodeDetail.StillPath)
	me.ReleaseDate = episodeDetail.AirDate
	me.VoteAverage = episodeDetail.VoteAverage
	me.VoteCount = episodeDetail.VoteCount
	me.Year = helpers.ParseYearFromDate(me.ReleaseDate)
	// 解析演员
	actors := make([]helpers.Actor, 0)
	if len(episodeDetail.Cast) > 0 {
		for _, actor := range episodeDetail.Cast {
			act := helpers.Actor{
				Name:    actor.Name,
				Role:    actor.Character,
				TmdbId:  actor.ID,
				Order:   actor.Order,
				Thumb:   "",
				Profile: fmt.Sprintf("https://www.themoviedb.org/person/%d", actor.ID),
			}
			if actor.ProfilePath != "" {
				act.Thumb = fmt.Sprintf("%s/t/p/original%s", GlobalScrapeSettings.GetTmdbImageUrl(), actor.ProfilePath)
			}
			actors = append(actors, act)
		}
		// 编码为JSON字符串
		actorsJson, err := json.Marshal(actors)
		if err != nil {
			helpers.AppLogger.Errorf("编码演员列表为JSON字符串失败:%v", err)
		}
		me.ActorsJson = string(actorsJson)
	}
	me.Actors = actors
	me.Status = MediaStatusScraped
}

func GetMediaById(id uint) (*Media, error) {
	var media Media
	if err := db.Db.Where("id = ?", id).First(&media).Error; err != nil {
		return nil, err
	}
	// 解码JSON字符串
	media.DecodeJson()
	return &media, nil
}

func GetMediaSeasonById(id uint) (*MediaSeason, error) {
	var mediaSeason MediaSeason
	if err := db.Db.Where("id = ?", id).First(&mediaSeason).Error; err != nil {
		return nil, err
	}
	return &mediaSeason, nil
}

func GetMediaEpisodeById(id uint) (*MediaEpisode, error) {
	var mediaEpisode MediaEpisode
	if err := db.Db.Where("id = ?", id).First(&mediaEpisode).Error; err != nil {
		return nil, err
	}
	mediaEpisode.DecodeJson()
	return &mediaEpisode, nil
}

func GetMediaByTmdbId(tmdbId int64) (*Media, error) {
	var media Media
	if err := db.Db.Where("tmdb_id = ?", tmdbId).First(&media).Error; err != nil {
		return nil, err
	}
	// 解码JSON字符串
	media.DecodeJson()
	return &media, nil
}

// 如果year = 0 则只用name查询
func GetMediaByName(mediaType MediaType, name string, year int) (*Media, error) {
	var media Media
	name = helpers.TitleCase(name)
	if year == 0 {
		if err := db.Db.Where("(name = ? OR original_name = ?) AND media_type = ?", name, name, mediaType).First(&media).Error; err != nil {
			return nil, err
		}
	} else {
		if err := db.Db.Where("(name = ? OR original_name = ?) AND year = ? AND media_type = ?", name, name, year, mediaType).First(&media).Error; err != nil {
			return nil, err
		}
	}
	// 解码JSON字符串
	media.DecodeJson()
	return &media, nil
}

// 使用TMDB信息创建Media
func MakeMediaFromTMDB(mediaType MediaType, tmdbInfo *TmdbInfo) (*Media, error) {
	media := &Media{
		MediaType: mediaType,
	}
	media.FillInfoByTmdbInfo(tmdbInfo)
	return media, nil
}

func MakeMovieMediaFromNfo(movie *helpers.Movie) (*Media, error) {
	var media Media
	media.MediaType = MediaTypeMovie
	media.Name = movie.Title
	// 从nfo中提取信息
	if movie.Title != "" {
		media.Name = movie.Title
	}
	if movie.Year != 0 {
		media.Year = movie.Year
	}
	// 获取tmdbid
	for _, uid := range movie.Uniqueid {
		if uid.Type == "tmdb" {
			media.TmdbId = helpers.StringToInt64(uid.Id)
		}
		// 获取imdbid
		if uid.Type == "imdb" {
			media.ImdbId = uid.Id
		}
	}
	if movie.Num != "" {
		media.Num = movie.Num
	}
	media.Runtime = movie.Runtime
	media.Num = movie.Num
	// 提取演员
	if movie.Actor != nil {
		media.Actors = movie.Actor
	}
	// 提取导演
	if movie.Director != nil {
		media.Director = movie.Director
	}
	if movie.MPAA != "" {
		media.MpaaRating = movie.MPAA
	}
	media.ReleaseDate = movie.Premiered
	media.Status = MediaStatusUnScraped
	return &media, nil
}

func GetSeasonByMediaIdAndSeasonNumber(mediaId uint, seasonNumber int) *MediaSeason {
	var season MediaSeason
	if err := db.Db.Where("media_id = ? AND season_number = ?", mediaId, seasonNumber).First(&season).Error; err != nil {
		return nil
	}
	return &season
}

func GetEpisodeByMediaIdAndSeasonNumber(mediaId uint, seasonNumber int, episodeNumber int) *MediaEpisode {
	var episode MediaEpisode
	err := db.Db.Where("media_id = ? AND season_number = ? AND episode_number = ?", mediaId, seasonNumber, episodeNumber).First(&episode).Error
	if err != nil {
		return nil
	}
	// 解码JSON字符串
	episode.DecodeJson()
	return &episode
}

func SetUnScrappedByMediaId(mediaId uint) {
	if mediaId == 0 {
		return
	}
	// 更新MediaSeason中对应media_id的状态为未刮削
	if err := db.Db.Model(&MediaSeason{}).Where("media_id = ?", mediaId).Update("status", MediaStatusUnScraped).Error; err != nil {
		return
	}
	// 更新MediaEpisode中对应media_id的状态为未刮削
	if err := db.Db.Model(&MediaEpisode{}).Where("media_id = ?", mediaId).Update("status", MediaStatusUnScraped).Error; err != nil {
		return
	}
}
