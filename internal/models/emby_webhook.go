package models

import (
	"fmt"
	"time"
)

// EmbyPlaybackWebhook Emby 播放事件 Webhook 消息结构
type EmbyPlaybackWebhook struct {
	Event      string        `json:"Event"`      // Playback.Start/Playback.Pause/Playback.Stop
	Item       EmbyPlaybackItem `json:"Item"`
	UserID     string        `json:"UserId"`
	UserName   string        `json:"UserName"`
	ServerID   string        `json:"ServerId"`
	DeviceName string        `json:"DeviceName"`
	ClientName string        `json:"ClientName"`
	DeviceID   string        `json:"DeviceId"`
	PlaybackDuration int64  `json:"PlaybackDuration,omitempty"` // 播放时长（毫秒，仅 Stop 事件有）
}

// EmbyPlaybackItem Emby 播放媒体项信息
type EmbyPlaybackItem struct {
	Name           string `json:"Name"`
	Type           string `json:"Type"` // Movie/Episode
	OriginalTitle  string `json:"OriginalTitle,omitempty"`
	ProductionYear int    `json:"ProductionYear,omitempty"`
	PremiereDate   string `json:"PremiereDate,omitempty"`
	SeriesName     string `json:"SeriesName,omitempty"`     // 剧集名称
	SeasonNumber   int    `json:"SeasonNumber,omitempty"`   // 季号（剧集）
	EpisodeNumber  int    `json:"EpisodeNumber,omitempty"`  // 集号（剧集）
}

// GetSeasonEpisodeString 获取季集信息字符串（如 "S01E06"）
func (i *EmbyPlaybackItem) GetSeasonEpisodeString() string {
	if i.Type != "Episode" {
		return ""
	}
	return FormatSeasonEpisode(i.SeasonNumber, i.EpisodeNumber)
}

// FormatSeasonEpisode 格式化季集信息（如 1, 6 -> "S01E06"）
func FormatSeasonEpisode(season, episode int) string {
	if season == 0 && episode == 0 {
		return ""
	}
	return fmt.Sprintf("S%02dE%02d", season, episode)
}

// FormatPlaybackDuration 格式化播放时长（毫秒转可读格式）
func FormatPlaybackDuration(durationMs int64) string {
	if durationMs == 0 {
		return "0秒"
	}

	duration := time.Duration(durationMs) * time.Millisecond

	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%d小时%d分钟", hours, minutes)
	} else if minutes > 0 {
		return fmt.Sprintf("%d分钟", minutes)
	} else {
		return fmt.Sprintf("%d秒", seconds)
	}
}

// GetNotificationEventType 根据 Emby 事件类型获取通知类型
func (w *EmbyPlaybackWebhook) GetNotificationEventType() string {
	switch w.Event {
	case "Playback.Start":
		return "playback_start"
	case "Playback.Pause":
		return "playback_pause"
	case "Playback.Stop":
		return "playback_stop"
	default:
		return ""
	}
}

// GetEventTypeEmoji 获取事件类型对应的表情符号
func (w *EmbyPlaybackWebhook) GetEventTypeEmoji() string {
	switch w.Event {
	case "Playback.Start":
		return "📺"
	case "Playback.Pause":
		return "⏸️"
	case "Playback.Stop":
		return "⏹️"
	default:
		return "📺"
	}
}

// GetEventTypeName 获取事件类型中文名称
func (w *EmbyPlaybackWebhook) GetEventTypeName() string {
	switch w.Event {
	case "Playback.Start":
		return "播放开始"
	case "Playback.Pause":
		return "播放暂停"
	case "Playback.Stop":
		return "播放停止"
	default:
		return "播放事件"
	}
}

// GetMediaTypeName 获取媒体类型中文名称
func (w *EmbyPlaybackWebhook) GetMediaTypeName() string {
	switch w.Item.Type {
	case "Movie":
		return "电影"
	case "Episode":
		return "剧集"
	default:
		return "媒体"
	}
}
