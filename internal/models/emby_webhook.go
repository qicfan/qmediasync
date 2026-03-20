package models

import (
	"fmt"
	"time"
)

// EmbyPlaybackWebhook Emby 播放事件 Webhook 消息结构
type EmbyPlaybackWebhook struct {
	Event   string              `json:"Event"` // playback.start/playback.pause/playback.stop
	User    EmbyPlaybackUser    `json:"User"`
	Item    EmbyPlaybackItem    `json:"Item"`
	Session EmbyPlaybackSession `json:"Session"`
}

// EmbyPlaybackUser 播放用户信息
type EmbyPlaybackUser struct {
	Name string `json:"Name"`
	ID   string `json:"Id"`
}

// EmbyPlaybackSession 播放会话信息
type EmbyPlaybackSession struct {
	DeviceName   string           `json:"DeviceName"`
	Client       string           `json:"Client"`
	PlaybackInfo EmbyPlaybackInfo `json:"PlaybackInfo"`
}

// EmbyPlaybackInfo 播放信息
type EmbyPlaybackInfo struct {
	PositionTicks int64           `json:"PositionTicks"` // 当前播放位置（单位：1/10000微秒）
	PlaySessionId string          `json:"PlaySessionId"` // 播放会话ID
	MediaSource   EmbyMediaSource `json:"MediaSource"`
}

// EmbyMediaSource 媒体源信息
type EmbyMediaSource struct {
	RunTimeTicks int64 `json:"RunTimeTicks"` // 总时长（单位：1/10000微秒）
}

// EmbyPlaybackItem Emby 播放媒体项信息
type EmbyPlaybackItem struct {
	Name           string            `json:"Name"`
	Type           string            `json:"Type"` // Movie/Episode
	OriginalTitle  string            `json:"OriginalTitle,omitempty"`
	ProductionYear int               `json:"ProductionYear,omitempty"`
	PremiereDate   string            `json:"PremiereDate,omitempty"`
	SeriesName     string            `json:"SeriesName,omitempty"`        // 剧集名称
	SeasonNumber   int               `json:"ParentIndexNumber,omitempty"` // 季号（剧集）
	EpisodeNumber  int               `json:"IndexNumber,omitempty"`       // 集号（剧集）
	ImageTags      map[string]string `json:"ImageTags,omitempty"`         // 图片标签
	ID             string            `json:"Id"`                          // 媒体ID
}

// GetUserID 获取用户ID
func (w *EmbyPlaybackWebhook) GetUserID() string {
	return w.User.ID
}

// GetUserName 获取用户名
func (w *EmbyPlaybackWebhook) GetUserName() string {
	return w.User.Name
}

// GetDeviceName 获取设备名称
func (w *EmbyPlaybackWebhook) GetDeviceName() string {
	return w.Session.DeviceName
}

// GetClientName 获取客户端名称
func (w *EmbyPlaybackWebhook) GetClientName() string {
	return w.Session.Client
}

// GetPlaybackDuration 获取播放时长（毫秒，仅Stop事件）
func (w *EmbyPlaybackWebhook) GetPlaybackDuration() int64 {
	if w.Event != "playback.stop" {
		return 0
	}
	// 将PositionTicks转换为毫秒（1 Tick = 100纳秒）
	return w.Session.PlaybackInfo.PositionTicks / 10000
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
	case "playback.start":
		return "playback_start"
	case "playback.pause":
		return "playback_pause"
	case "playback.stop":
		return "playback_stop"
	default:
		return ""
	}
}

// GetEventTypeEmoji 获取事件类型对应的表情符号
func (w *EmbyPlaybackWebhook) GetEventTypeEmoji() string {
	switch w.Event {
	case "playback.start":
		return "📺"
	case "playback.pause":
		return "⏸️"
	case "playback.stop":
		return "⏹️"
	default:
		return "📺"
	}
}

// GetEventTypeName 获取事件类型中文名称
func (w *EmbyPlaybackWebhook) GetEventTypeName() string {
	switch w.Event {
	case "playback.start":
		return "播放开始"
	case "playback.pause":
		return "播放暂停"
	case "playback.stop":
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
