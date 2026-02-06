package models

import (
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/notificationmanager"
	"encoding/json"
	"strings"
)

var V115Login bool

type Settings struct {
	BaseModel
	UseTelegram      int8   `json:"use_telegram"`       // 是否使用Telegram Bot通知
	TelegramBotToken string `json:"telegram_bot_token"` // Telegram Bot Token
	TelegramChatId   string `json:"telegram_chat_id"`   // Telegram Chat ID
	MeoWName         string `json:"meow_name"`          // MeoW昵称，用于发送MeoW消息

	HttpProxy string `json:"http_proxy"` // HTTP代理地址

	StrmBaseUrl    string   `json:"strm_base_url"`                     // STRM的基础URL，用于生成115的流媒体播放地址
	Cron           string   `json:"cron"`                              // 定时任务表达式
	MinVideoSize   int64    `json:"min_video_size"`                    // 最小视频大小，单位字节
	ExcludeName    string   `json:"-"`                                 // 排除的名字，目录或者文件等于这个名字都会排除不处理，JSON格式，一个字符串数组
	ExcludeNameArr []string `json:"exclude_name_arr" gorm:"-"`         // 排除的名字数组，JSON格式
	VideoExt       string   `json:"-"`                                 // 视频文件扩展名数组，JSON格式
	VideoExtArr    []string `json:"video_ext_arr" gorm:"-"`            // 视频文件扩展名数组，JSON格式
	MetaExt        string   `json:"-"`                                 // 元数据的扩展名数组，JSON格式
	MetaExtArr     []string `json:"meta_ext_arr" gorm:"-"`             // 元数据的扩展名数组，JSON格式
	UploadMeta     int      `json:"upload_meta" gorm:"default:0"`      // 是否上传元数据，0表示保留，1表示上传，2-表示删除
	DownloadMeta   int      `json:"download_meta" gorm:"default:1"`    // 是否下载元数据，0表示不下载，1表示下载
	DeleteDir      int      `json:"delete_dir" gorm:"default:1"`       // 是否删除目录，0表示不删除，1表示删除
	AddPath        int      `json:"add_path" gorm:"default:2"`         // 是否添加路径，1- 表示添加路径， 2-表示不添加路径
	CheckMetaMtime int      `json:"check_meta_mtime" gorm:"default:0"` // 是否检查元数据文件修改时间，默认0， 如果1，网盘新则下载，网盘旧就上传（UploadMeta=1时）

	LocalProxy int `json:"local_proxy" gorm:"default:0"` // 是否启用本地代理，0表示不启用，1表示启用

	EmbyUrl    string `json:"emby_url"`     // Emby的主机地址
	EmbyApiKey string `json:"emby_api_key"` // Emby的API Key

	DownloadThreads    int `json:"download_threads" gorm:"default:1"`      // 下载qps
	FileDetailThreads  int `json:"file_detail_threads" gorm:"default:1"`   // 115接口qps
	OpenlistQPS        int `json:"openlist_qps" gorm:"default:3"`          // Openlist接口qps，默认3
	OpenlistRetry      int `json:"openlist_retry" gorm:"default:1"`        // Openlist重试次数，默认1
	OpenlistRetryDelay int `json:"openlist_retry_delay" gorm:"default:60"` // Openlist重试间隔，默认60秒，躲避QPM限流

	// 百度网盘四维度限速
	BaiDuPanQPS int `json:"baidupan_qps" gorm:"default:10"`     // 百度网盘每秒请求数限制，默认10
	BaiDuPanQPM int `json:"baidupan_qpm" gorm:"default:600"`    // 百度网盘每分钟请求数限制，默认600
	BaiDuPanQPH int `json:"baidupan_qph" gorm:"default:36000"`  // 百度网盘每小时请求数限制，默认36000
	BaiDuPanQPT int `json:"baidupan_qpt" gorm:"default:864000"` // 百度网盘每天请求数限制，默认864000

}

var SettingsGlobal = &Settings{}

func (settings *Settings) UpdateThreads(downloadThreads int, fileDetailThreads, openlistQPS, openlistRetry, openlistRetryDelay int, baidupanQPS, baidupanQPM, baidupanQPH, baidupanQPT int) bool {
	settings.DownloadThreads = downloadThreads
	settings.FileDetailThreads = fileDetailThreads
	settings.OpenlistQPS = openlistQPS
	settings.OpenlistRetry = openlistRetry
	settings.OpenlistRetryDelay = openlistRetryDelay
	// 更新百度网盘限速
	settings.BaiDuPanQPS = baidupanQPS
	settings.BaiDuPanQPM = baidupanQPM
	settings.BaiDuPanQPH = baidupanQPH
	settings.BaiDuPanQPT = baidupanQPT
	
	updateData := make(map[string]interface{})
	updateData["download_threads"] = downloadThreads
	updateData["file_detail_threads"] = fileDetailThreads
	updateData["openlist_qps"] = openlistQPS
	updateData["openlist_retry"] = openlistRetry
	updateData["openlist_retry_delay"] = openlistRetryDelay
	// 百度网盘限速
	updateData["baidupan_qps"] = baidupanQPS
	updateData["baidupan_qpm"] = baidupanQPM
	updateData["baidupan_qph"] = baidupanQPH
	updateData["baidupan_qpt"] = baidupanQPT
	
	err := db.Db.Model(settings).Where("id = ?", settings.ID).Updates(updateData).Error
	if err != nil {
		helpers.AppLogger.Errorf("更新线程数失败: %v", err)
		return false
	}
	// 重新初始化下载队列
	InitDQ()
	return true
}

func (settings *Settings) GetThreads() map[string]int {
	return map[string]int{
		"download_threads":     settings.DownloadThreads,
		"file_detail_threads":  settings.FileDetailThreads,
		"openlist_qps":         settings.OpenlistQPS,
		"openlist_retry":       settings.OpenlistRetry,
		"openlist_retry_delay": settings.OpenlistRetryDelay,
		// 百度网盘限速
		"baidupan_qps":         settings.BaiDuPanQPS,
		"baidupan_qpm":         settings.BaiDuPanQPM,
		"baidupan_qph":         settings.BaiDuPanQPH,
		"baidupan_qpt":         settings.BaiDuPanQPT,
	}
}

func (settings *Settings) UpdateTelegramBot(enabled bool, token string, chatId string) bool {
	if enabled {
		settings.UseTelegram = 1
	} else {
		settings.UseTelegram = 0
	}
	settings.TelegramBotToken = token
	settings.TelegramChatId = chatId
	updateData := make(map[string]interface{})
	updateData["use_telegram"] = settings.UseTelegram
	updateData["telegram_bot_token"] = token
	updateData["telegram_chat_id"] = chatId
	err := db.Db.Model(settings).Where("id = ?", settings.ID).Updates(updateData).Error
	if err != nil {
		helpers.AppLogger.Errorf("更新Telegram通知设置失败: %v", err)
		return false
	}
	InitNotificationManager()
	return true
}

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

// func (settings *Settings) UpdateEmbyUrl(embyUrl string, embyApiKey string) bool {
// 	settings.EmbyUrl = embyUrl
// 	settings.EmbyApiKey = embyApiKey
// 	updateData := make(map[string]interface{})
// 	updateData["emby_url"] = embyUrl
// 	updateData["emby_api_key"] = embyApiKey
// 	err := db.Db.Model(settings).Where("id = ?", settings.ID).Updates(updateData).Error
// 	if err != nil {
// 		helpers.AppLogger.Errorf("更新Emby地址失败: %v", err)
// 		return false
// 	}
// 	if config.C != nil {
// 		config.C.Emby.Host = embyUrl // 更新emby302反代的配置
// 	}
// 	return true
// }

func (settings *Settings) UpdateStrm(strmBaseUrl string, cron string, metaExt []string, videoExt []string, minVideoSize int64, uploadMeta int, deleteDir int, localProxy int, excludeName []string, downloadMeta int, addPath int, checkMetaMtime int) bool {
	settings.StrmBaseUrl = strmBaseUrl
	settings.Cron = cron
	// 全部转小写
	for i, v := range metaExt {
		metaExt[i] = strings.ToLower(v)
	}
	// 全部转小写
	for i, v := range videoExt {
		videoExt[i] = strings.ToLower(v)
	}
	// 全部转小写
	for i, v := range excludeName {
		excludeName[i] = strings.ToLower(v)
	}
	metaExtStr, err := json.Marshal(metaExt)
	if err != nil {
		helpers.AppLogger.Errorf("将元数据扩展名转换为JSON字符串失败: %v", err)
		return false
	}
	videoExtStr, err := json.Marshal(videoExt)
	if err != nil {
		helpers.AppLogger.Errorf("将视频扩展名转换为JSON字符串失败: %v", err)
		return false
	}
	// 排除的名字
	excludeNameStr, err := json.Marshal(excludeName)
	if err != nil {
		helpers.AppLogger.Errorf("将排除的名字转换为JSON字符串失败: %v", err)
		return false
	}
	settings.ExcludeName = string(excludeNameStr)
	settings.VideoExt = string(videoExtStr)
	settings.MetaExt = string(metaExtStr)
	settings.UploadMeta = uploadMeta
	settings.DownloadMeta = downloadMeta
	settings.DeleteDir = deleteDir
	settings.LocalProxy = localProxy
	settings.AddPath = addPath
	settings.CheckMetaMtime = checkMetaMtime

	// 最小视频大小
	settings.MinVideoSize = minVideoSize

	helpers.AppLogger.Infof("排除的名字: %v", excludeName)
	// ctx := context.Background()
	updateData := make(map[string]interface{})
	updateData["strm_base_url"] = strmBaseUrl
	updateData["cron"] = cron
	updateData["meta_ext"] = settings.MetaExt
	updateData["video_ext"] = settings.VideoExt
	updateData["min_video_size"] = minVideoSize
	updateData["upload_meta"] = uploadMeta
	updateData["delete_dir"] = deleteDir
	updateData["local_proxy"] = localProxy
	updateData["exclude_name"] = settings.ExcludeName
	updateData["download_meta"] = downloadMeta
	updateData["add_path"] = addPath
	updateData["check_meta_mtime"] = checkMetaMtime
	err = db.Db.Model(settings).Where("id = ?", settings.ID).Updates(updateData).Error
	// _, err = gorm.G[Settings](db.Db).Where("id = ?", settings.ID).Updates(ctx, updateData)
	if err != nil {
		helpers.AppLogger.Errorf("更新STRM设置失败: %v", err)
		return false
	}
	settings.MetaExtArr = metaExt
	settings.VideoExtArr = videoExt
	settings.ExcludeNameArr = excludeName
	return true
}

func LoadSettings() {
	if err := db.Db.Take(SettingsGlobal).Error; err != nil {
		helpers.AppLogger.Errorf("load settings failed: %v", err)
		return
	}
	json.Unmarshal([]byte(SettingsGlobal.MetaExt), &SettingsGlobal.MetaExtArr)
	json.Unmarshal([]byte(SettingsGlobal.VideoExt), &SettingsGlobal.VideoExtArr)
	json.Unmarshal([]byte(SettingsGlobal.ExcludeName), &SettingsGlobal.ExcludeNameArr)
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
