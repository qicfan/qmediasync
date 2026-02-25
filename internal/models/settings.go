package models

import (
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/notificationmanager"
	"encoding/json"
	"strings"
)

var V115Login bool

type SettingThreads struct {
	DownloadThreads    int `form:"download_threads" json:"download_threads" binding:"required" gorm:"default:1"`          // 下载QPS
	FileDetailThreads  int `form:"file_detail_threads" json:"file_detail_threads" binding:"required" gorm:"default:1"`    // 115接口QPS
	OpenlistQPS        int `form:"openlist_qps" json:"openlist_qps" binding:"required" gorm:"default:3"`                  // OpenList QPS
	OpenlistRetry      int `form:"openlist_retry" json:"openlist_retry" binding:"required" gorm:"default:1"`              // OpenList 重试次数
	OpenlistRetryDelay int `form:"openlist_retry_delay" json:"openlist_retry_delay" binding:"required" gorm:"default:60"` // OpenList 重试间隔，单位秒
}

type SettingStrm struct {
	LocalProxy     int      `form:"local_proxy" json:"local_proxy" gorm:"default:0"`           // 是否使用本地代理，-1表示使用STRM设置，0表示不使用，1表示使用
	StrmBaseUrl    string   `form:"strm_base_url" json:"strm_base_url"`                        // STRM的基础URL，用于生成网盘的流媒体播放地址
	Cron           string   `form:"cron" json:"cron"`                                          // 定时任务表达式
	MinVideoSize   int64    `form:"min_video_size" json:"min_video_size"`                      // 最小视频大小，单位字节,-1表示使用STRM设置
	VideoExt       string   `json:"-"`                                                         // 视频文件扩展名，JSON格式
	VideoExtArr    []string `json:"video_ext_arr" gorm:"-"`                                    // 视频文件扩展名数组，不参与数据库操作，仅供前端使用
	MetaExt        string   `json:"-"`                                                         // 元数据文件扩展名，JSON格式
	MetaExtArr     []string `form:"meta_ext_arr" json:"meta_ext_arr" gorm:"-"`                 // 元数据文件扩展名数组，不参与数据库操作，仅供前端使用
	ExcludeName    string   `json:"-"`                                                         // 排除的文件名，JSON格式
	ExcludeNameArr []string `form:"exclude_name_arr" json:"exclude_name_arr" gorm:"-"`         // 排除的文件名数组，不参与数据库操作，仅供前端使用
	UploadMeta     int      `form:"upload_meta" json:"upload_meta" gorm:"default:0"`           // 是否上传元数据，-1表示使用STRM设置，0表示保留，1表示上传，2-表示删除
	DownloadMeta   int      `form:"download_meta" json:"download_meta" gorm:"default:0"`       // 是否下载元数据，-1表示使用STRM设置，0表示不下载，1表示下载
	DeleteDir      int      `form:"delete_dir" json:"delete_dir" gorm:"default: 1"`            // 是否删除目录，-1表示使用STRM设置，0表示不删除，1表示删除
	AddPath        int      `form:"add_path" json:"add_path" gorm:"default: 2"`                // 是否添加路径，默认-1(使用settings的值), 1- 表示添加路径， 2-表示不添加路径
	CheckMetaMtime int      `form:"check_meta_mtime" json:"check_meta_mtime" gorm:"default:0"` // 是否检查元数据文件修改时间，默认-1(使用settings的值), 0表示不检查，1表示检查
}

type Settings struct {
	BaseModel
	SettingThreads
	SettingStrm
	UseTelegram      int8   `json:"use_telegram"`                 // @deprecated 已迁移到TelegramChannelConfig 是否使用Telegram Bot通知
	TelegramBotToken string `json:"telegram_bot_token"`           // @deprecated 已迁移到TelegramChannelConfig Telegram Bot Token
	TelegramChatId   string `json:"telegram_chat_id"`             // @deprecated 已迁移到TelegramChannelConfig Telegram Chat ID
	MeoWName         string `json:"meow_name"`                    // @deprecated 已迁移到MeoWChannelConfig MeoW昵称，用于发送MeoW消息
	EmbyUrl          string `json:"emby_url"`                     // @deprecated 已迁移到EmbyConfig Emby的主机地址
	EmbyApiKey       string `json:"emby_api_key"`                 // @deprecated 已迁移到EmbyConfig Emby的API Key
	HttpProxy        string `json:"http_proxy"`                   // HTTP代理地址
	LocalProxy       int    `json:"local_proxy" gorm:"default:0"` // 是否启用本地代理，0表示不启用，1表示启用
}

func (t SettingThreads) ToMap() map[string]any {
	return map[string]any{
		"download_threads":     t.DownloadThreads,
		"file_detail_threads":  t.FileDetailThreads,
		"openlist_qps":         t.OpenlistQPS,
		"openlist_retry":       t.OpenlistRetry,
		"openlist_retry_delay": t.OpenlistRetryDelay,
	}
}

func (s SettingStrm) ToMap(isDb bool, isSetting bool) map[string]any {
	dataMap := map[string]any{
		"cron":             s.Cron,
		"min_video_size":   s.MinVideoSize,
		"delete_dir":       s.DeleteDir,
		"upload_meta":      s.UploadMeta,
		"download_meta":    s.DownloadMeta,
		"strm_base_url":    s.StrmBaseUrl,
		"add_path":         s.AddPath,
		"check_meta_mtime": s.CheckMetaMtime,
		"local_proxy":      s.LocalProxy,
	}
	if s.Cron == "" && isSetting {
		dataMap["cron"] = helpers.GlobalConfig.Strm.Cron // 使用默认配置
	} else {
		dataMap["cron"] = s.Cron
	}
	if before, ok := strings.CutSuffix(dataMap["strm_base_url"].(string), "/"); ok {
		dataMap["strm_base_url"] = before
	}
	if !isDb {
		// 不是数据库则返回Arr
		if s.MetaExt != "" {
			dataMap["meta_ext_arr"] = s.MetaExtArr
		} else if isSetting {
			// 从config.yml中读取默认的metaExt
			dataMap["meta_ext_arr"] = helpers.GlobalConfig.Strm.MetaExt
		}
		if s.VideoExt != "" {
			dataMap["video_ext_arr"] = s.VideoExtArr
		} else if isSetting {
			// 从config.yml中读取默认的视频扩展名
			dataMap["video_ext_arr"] = helpers.GlobalConfig.Strm.VideoExt
		}
		if s.ExcludeName != "" {
			dataMap["exclude_name_arr"] = s.ExcludeNameArr
		} else {
			// 从config.yml中读取默认的排除的文件名
			dataMap["exclude_name_arr"] = []string{}
		}
	} else {
		// 数据库
		dataMap["exclude_name"] = s.ExcludeName
		dataMap["meta_ext"] = s.MetaExt
		dataMap["video_ext"] = s.VideoExt
	}
	return dataMap
}

func (s SettingStrm) EncodeArr() *SettingStrm {
	// 全部转小写
	for i, v := range s.MetaExtArr {
		s.MetaExtArr[i] = strings.ToLower(v)
	}
	// 全部转小写
	for i, v := range s.VideoExtArr {
		s.VideoExtArr[i] = strings.ToLower(v)
	}
	// 全部转小写
	for i, v := range s.ExcludeNameArr {
		s.ExcludeNameArr[i] = strings.ToLower(v)
	}
	metaExtStr, err := json.Marshal(s.MetaExtArr)
	if err != nil {
		helpers.AppLogger.Errorf("将元数据扩展名转换为JSON字符串失败: %v", err)
		return nil
	}
	videoExtStr, err := json.Marshal(s.VideoExtArr)
	if err != nil {
		helpers.AppLogger.Errorf("将视频扩展名转换为JSON字符串失败: %v", err)
		return nil
	}
	// 排除的名字
	excludeNameStr, err := json.Marshal(s.ExcludeNameArr)
	if err != nil {
		helpers.AppLogger.Errorf("将排除的名字转换为JSON字符串失败: %v", err)
		return nil
	}
	s.ExcludeName = string(excludeNameStr)
	s.VideoExt = string(videoExtStr)
	s.MetaExt = string(metaExtStr)
	return &s
}

func (s SettingStrm) DecodeArr(isSetting bool) *SettingStrm {
	if s.MetaExt != "" {
		if err := json.Unmarshal([]byte(s.MetaExt), &s.MetaExtArr); err != nil {
			helpers.AppLogger.Errorf("将元数据扩展名转换为数组失败: %v", err)
			return nil
		}
	}
	if len(s.MetaExtArr) == 0 && isSetting {
		s.MetaExtArr = helpers.GlobalConfig.Strm.MetaExt
	}
	if s.VideoExt != "" {
		if err := json.Unmarshal([]byte(s.VideoExt), &s.VideoExtArr); err != nil {
			helpers.AppLogger.Errorf("将视频扩展名转换为数组失败: %v", err)
			return nil
		}
	}
	if len(s.VideoExtArr) == 0 && isSetting {
		s.VideoExtArr = helpers.GlobalConfig.Strm.VideoExt
	}
	if s.ExcludeName != "" {
		if err := json.Unmarshal([]byte(s.ExcludeName), &s.ExcludeNameArr); err != nil {
			helpers.AppLogger.Errorf("将排除的名字转换为数组失败: %v", err)
			return nil
		}
	}
	if len(s.ExcludeNameArr) == 0 {
		s.ExcludeNameArr = []string{}
	}
	if s.Cron == "" && isSetting {
		s.Cron = helpers.GlobalConfig.Strm.Cron
	}
	return &s
}

var SettingsGlobal = &Settings{}

func (settings *Settings) UpdateThreads(req SettingThreads) bool {
	settings.SettingThreads = req

	updateData := req.ToMap()

	err := db.Db.Model(settings).Where("id = ?", settings.ID).Updates(updateData).Error
	if err != nil {
		helpers.AppLogger.Errorf("更新线程数失败: %v", err)
		return false
	}
	// 重新初始化下载队列
	InitDQ()
	return true
}

// func (settings *Settings) UpdateTelegramBot(enabled bool, token string, chatId string) bool {
// 	if enabled {
// 		settings.UseTelegram = 1
// 	} else {
// 		settings.UseTelegram = 0
// 	}
// 	settings.TelegramBotToken = token
// 	settings.TelegramChatId = chatId
// 	updateData := make(map[string]any)
// 	updateData["use_telegram"] = settings.UseTelegram
// 	updateData["telegram_bot_token"] = token
// 	updateData["telegram_chat_id"] = chatId
// 	err := db.Db.Model(settings).Where("id = ?", settings.ID).Updates(updateData).Error
// 	if err != nil {
// 		helpers.AppLogger.Errorf("更新Telegram通知设置失败: %v", err)
// 		return false
// 	}
// 	InitNotificationManager()
// 	return true
// }

func (settings *Settings) UpdateHttpProxy(httpProxy string) bool {
	settings.HttpProxy = httpProxy
	updateData := make(map[string]interface{})
	updateData["http_proxy"] = httpProxy
	err := db.Db.Model(settings).Where("id = ?", settings.ID).Updates(updateData).Error
	if err != nil {
		helpers.AppLogger.Errorf("更新HTTP代理失败: %v", err)
		return false
	}
	InitNotificationManager()
	return true
}

func (settings *Settings) UpdateStrm(req SettingStrm) bool {
	strm := req.EncodeArr()
	if strm == nil {
		helpers.AppLogger.Errorf("编码STRM设置失败")
		return false
	}
	settings.SettingStrm = *strm

	// ctx := context.Background()
	updateData := strm.ToMap(true, true)
	// helpers.AppLogger.Infof("更新STRM设置: %+v", updateData)
	err := db.Db.Model(settings).Where("id = ?", settings.ID).Updates(updateData).Error
	// _, err = gorm.G[Settings](db.Db).Where("id = ?", settings.ID).Updates(ctx, updateData)
	if err != nil {
		helpers.AppLogger.Errorf("更新STRM设置失败: %v", err)
		return false
	}
	return true
}

func LoadSettings() {
	if err := db.Db.Take(SettingsGlobal).Error; err != nil {
		helpers.AppLogger.Errorf("load settings failed: %v", err)
		return
	}
	SettingsGlobal.SettingStrm = *SettingsGlobal.SettingStrm.DecodeArr(true)
	if SettingsGlobal.MinVideoSize == 104857600 {
		SettingsGlobal.MinVideoSize = 100
		db.Db.Save(SettingsGlobal)
	}
}

func InitNotificationManager() {
	// 初始化增强通知管理器
	// 传入代理获取回调函数，避免循环依赖
	enhancedManager := notificationmanager.NewEnhancedNotificationManager(db.Db, func() string {
		helpers.AppLogger.Infof("获取HTTP代理: %+v", SettingsGlobal.HttpProxy)
		if SettingsGlobal != nil {
			return SettingsGlobal.HttpProxy
		}
		return ""
	})
	if err := enhancedManager.LoadChannels(); err != nil {
		helpers.AppLogger.Warnf("加载通知渠道失败: %v", err)
	}
	notificationmanager.GlobalEnhancedNotificationManager = enhancedManager
}
