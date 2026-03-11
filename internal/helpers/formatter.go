package helpers

import (
	"fmt"
	"path/filepath"
	"strings"
)

// FormatMoviePath 格式化电影路径
// 参考 MoviePilot 的命名规则: {电影名称} ({年份})/{电影名称} ({年份}).{扩展名}
func FormatMoviePath(title string, year int, originalPath string) string {
	name := title
	if year > 0 {
		name += fmt.Sprintf(" (%d)", year)
	}
	ext := filepath.Ext(originalPath)
	return filepath.Join(name, name+ext)
}

// FormatTVPath 格式化电视剧路径
// 参考 MoviePilot 的命名规则: {剧集名称}/Season {季号}/{剧集名称} - S{季号}E{集号} - {集名}.{扩展名}
func FormatTVPath(title string, season, episode int, episodeTitle, originalPath string) string {
	showName := title
	seasonDir := fmt.Sprintf("Season %02d", season)
	episodeName := fmt.Sprintf("%s - S%02dE%02d", showName, season, episode)
	if episodeTitle != "" {
		episodeName += " - " + episodeTitle
	}
	ext := filepath.Ext(originalPath)
	return filepath.Join(showName, seasonDir, episodeName+ext)
}

// FormatSeasonPath 格式化季目录路径
// 返回: {剧集名称}/Season {季号}
func FormatSeasonPath(title string, season int) string {
	return filepath.Join(title, fmt.Sprintf("Season %02d", season))
}

// FormatEpisodeName 格式化集名称
// 返回: {剧集名称} - S{季号}E{集号} - {集名}
func FormatEpisodeName(title string, season, episode int, episodeTitle string) string {
	episodeName := fmt.Sprintf("%s - S%02dE%02d", title, season, episode)
	if episodeTitle != "" {
		episodeName += " - " + episodeTitle
	}
	return episodeName
}

// CleanTitle 清理标题，移除不合法字符
func CleanTitle(title string) string {
	// 移除 Windows 文件名不允许的字符
	illegalChars := []string{"<", ">", ":", "\"", "/", "\\", "|", "?", "*"}
	result := title
	for _, char := range illegalChars {
		result = strings.ReplaceAll(result, char, "")
	}
	// 移除首尾空格
	result = strings.TrimSpace(result)
	// 替换多个空格为单个空格
	for strings.Contains(result, "  ") {
		result = strings.ReplaceAll(result, "  ", " ")
	}
	return result
}

// SanitizePath 清理路径，确保路径合法
func SanitizePath(path string) string {
	// 分割路径
	parts := strings.Split(path, string(filepath.Separator))
	// 清理每一部分
	cleanedParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			cleanedParts = append(cleanedParts, CleanTitle(part))
		}
	}
	// 重新组合
	return filepath.Join(cleanedParts...)
}
