package helpers

import (
	"path/filepath"
	"strings"
)

// MediaType 媒体类型常量
const (
	MediaTypeMovie = "movie"
	MediaTypeTV    = "tv"
	MediaTypeAuto  = "auto"
)

// AutoDetectMediaType 自动识别媒体类型
// 参考文件路径特征、季集信息等自动判断是电影还是电视剧
func AutoDetectMediaType(filePath string, meta *MediaInfo) string {
	// 1. 如果有明确的类型标识，使用标识
	// 这里可以扩展，从文件名中提取类型标识

	// 2. 根据季/集信息判断
	if meta != nil {
		if meta.Season > 0 || meta.Episode > 0 {
			return MediaTypeTV
		}
	}

	// 3. 根据文件路径特征判断
	path := strings.ToLower(filePath)

	// 常见的电视剧目录特征
	tvPathPatterns := []string{
		"season",
		"s0",
		"s1",
		"s2",
		"s3",
		"s4",
		"s5",
		"s6",
		"s7",
		"s8",
		"s9",
		"episode",
		"ep",
		"剧集",
		"电视剧",
		"连续剧",
	}

	for _, pattern := range tvPathPatterns {
		if strings.Contains(path, pattern) {
			return MediaTypeTV
		}
	}

	// 4. 检查父目录名称
	parentDir := strings.ToLower(filepath.Base(filepath.Dir(filePath)))
	for _, pattern := range tvPathPatterns {
		if strings.Contains(parentDir, pattern) {
			return MediaTypeTV
		}
	}

	// 5. 默认为电影
	return MediaTypeMovie
}

// DetectMediaTypeFromPath 从完整路径自动识别媒体类型
func DetectMediaTypeFromPath(filePath string, videoExt []string, excludePatterns ...string) (string, *MediaInfo) {
	// 先提取元数据
	meta := ExtractMetadataFromPath(filePath, false, videoExt, excludePatterns...)

	// 自动识别类型
	mediaType := AutoDetectMediaType(filePath, meta)

	return mediaType, meta
}
