package models

import "Q115-STRM/internal/db"

// EmbyConfig 独立的Emby配置表
type EmbyConfig struct {
	BaseModel
	EmbyUrl                 string `json:"emby_url" gorm:"type:varchar(500)"`
	EmbyApiKey              string `json:"emby_api_key" gorm:"type:varchar(200)"`
	EnableDeleteNetdisk     int    `json:"enable_delete_netdisk" gorm:"default:0"`
	EnableRefreshLibrary    int    `json:"enable_refresh_library" gorm:"default:0"`
	EnableMediaNotification int    `json:"enable_media_notification" gorm:"default:0"`
	EnableExtractMediaInfo  int    `json:"enable_extract_media_info" gorm:"default:0"`
	EnableAuth              int    `json:"enable_auth" gorm:"default:0"`
	SyncEnabled             int    `json:"sync_enabled" gorm:"default:1"`
	SyncCron                string `json:"sync_cron" gorm:"type:varchar(100);default:'*/5 * * * *'"`
	LastSyncTime            int64  `json:"last_sync_time" gorm:"default:0"`
	// DeleteNetdiskLibrary    string `json:"delete_netdisk_library" gorm:"type:varchar(200);default:''"` // 允许联动删除的媒体库ID，用,分隔, 空表示允许全部
	// 播放通知配置
	EnablePlaybackNotification int    `json:"enable_playback_notification" gorm:"default:0"`
	EnablePlayStart            int    `json:"enable_play_start" gorm:"default:0"`
	EnablePlayStop             int    `json:"enable_play_stop" gorm:"default:0"`
	EnablePlayPause            int    `json:"enable_play_pause" gorm:"default:0"`
	PlaybackUserFilter         string `json:"playback_user_filter" gorm:"type:varchar(500);default:''"`
	PlaybackDeviceFilter       string `json:"playback_device_filter" gorm:"type:varchar(500);default:''"`
}

func (*EmbyConfig) TableName() string {
	return "emby_config"
}

var GlobalEmbyConfig *EmbyConfig

// GetEmbyConfig 获取Emby配置
func GetEmbyConfig() (*EmbyConfig, error) {
	if GlobalEmbyConfig != nil {
		return GlobalEmbyConfig, nil
	}
	config := &EmbyConfig{}
	if err := db.Db.First(config).Error; err != nil {
		return nil, err
	}
	GlobalEmbyConfig = config
	return GlobalEmbyConfig, nil
}

// Update 更新配置
func (c *EmbyConfig) Update(updates map[string]interface{}) error {
	return db.Db.Model(c).Updates(updates).Error
}
