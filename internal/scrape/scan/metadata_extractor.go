package scan

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"path/filepath"
)

// ExtractMetadataFromPathEnhanced 增强的元数据提取
// 参考 MoviePilot 的实现，从多层级路径提取元数据并智能合并
func (m *scanBaseImpl) ExtractMetadataFromPathEnhanced(mediaFile *models.ScrapeMediaFile) error {
	// 构建完整路径
	var fullPath string
	if mediaFile.SourceType == models.SourceTypeLocal {
		fullPath = filepath.Join(mediaFile.SourcePath, mediaFile.Path, mediaFile.VideoFilename)
	} else {
		// 网盘类型，使用相对路径
		fullPath = filepath.Join(mediaFile.Path, mediaFile.VideoFilename)
	}

	// 使用新的多层级路径提取功能
	isMovie := mediaFile.MediaType == models.MediaTypeMovie
	info := helpers.ExtractMetadataFromPath(fullPath, isMovie, m.scrapePath.VideoExtList, m.scrapePath.DeleteKeyword...)

	if info == nil {
		// 如果提取失败，回退到原有逻辑
		return m.ExtractSeasonEpisode(mediaFile)
	}

	// 填充提取到的信息
	if mediaFile.Name == "" && info.Name != "" {
		mediaFile.Name = info.Name
	}
	if mediaFile.Year == 0 && info.Year != 0 {
		mediaFile.Year = info.Year
	}
	if mediaFile.TmdbId == 0 && info.TmdbId != 0 {
		mediaFile.TmdbId = info.TmdbId
	}
	if mediaFile.SeasonNumber == -1 && info.Season != -1 {
		mediaFile.SeasonNumber = info.Season
	}
	if mediaFile.EpisodeNumber == -1 && info.Episode != -1 {
		mediaFile.EpisodeNumber = info.Episode
	}

	// 如果季集信息仍然缺失，使用原有逻辑
	if mediaFile.SeasonNumber == -1 || mediaFile.EpisodeNumber == -1 {
		return m.ExtractSeasonEpisode(mediaFile)
	}

	return nil
}
