package models

import (
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/helpers"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Migrator struct {
	BaseModel
	VersionCode int `json:"version_code"` // 版本号
}

func (*Migrator) TableName() string {
	return "migrator"
}

// 数据库迁移
// 如果没有数据则创建
// 如果已有数据库则从数据库中获取版本，根据版本执行变更
func Migrate() {
	// sqliteDb := db.InitSqlite3(dbFile)
	maxVersion := 30
	// 先初始化所有表和基础数据
	if !InitDB(maxVersion) {
		// 初始化数据库版本表
		helpers.AppLogger.Info("已完成数据库初始化")
		return
	}
	var migrator Migrator = Migrator{}
	err := db.Db.Model(&migrator).First(&migrator).Error
	if err != nil {
		helpers.AppLogger.Errorf("获取数据库迁移表失败：%v", err)
	}
	db.Db.Statement.PrepareStmt = true
	if migrator.VersionCode == 1 {
		// 数据库版本低于最大版本，需要升级
		db.Db.AutoMigrate(DbDownloadTask{}, DbUploadTask{}, SyncPath{}, Sync{})
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 2 {
		// 数据库版本低于最大版本，需要升级
		db.Db.AutoMigrate(SyncFile{})
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 3 {
		// 数据库版本低于最大版本，需要升级
		db.Db.AutoMigrate(Account{})
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 4 {
		db.Db.AutoMigrate(ScrapeMediaFile{}, Media{}, MediaSeason{}, MediaEpisode{})
		// 给所有ScrapeMediaFile补充新增字段的值
		scrapePathMap := make(map[uint]*ScrapePath)
		scrapePathes := GetScrapePathes()
		for _, scrapePath := range scrapePathes {
			scrapePathMap[scrapePath.ID] = scrapePath
		}
		limit := 100
		offset := 0
		for {
			var scrapeMediaFiles []*ScrapeMediaFile
			db.Db.Model(&ScrapeMediaFile{}).Limit(limit).Offset(offset).Find(&scrapeMediaFiles)
			if len(scrapeMediaFiles) == 0 {
				break
			}
			for _, sm := range scrapeMediaFiles {
				sm.QueryRelation()
				sourcePath, exists := scrapePathMap[sm.ScrapePathId]
				if !exists {
					continue
				}
				sm.MediaType = sourcePath.MediaType
				sm.SourceType = sourcePath.SourceType
				sm.ScrapeType = sourcePath.ScrapeType
				sm.RenameType = sourcePath.RenameType
				sm.EnableCategory = sourcePath.EnableCategory
				sm.SourcePath = sourcePath.SourcePath
				sm.SourcePathId = sourcePath.SourcePathId
				sm.DestPath = sourcePath.DestPath
				sm.DestPathId = sourcePath.DestPathId
				helpers.AppLogger.Infof("刮削记录的所有新增字段已更新 %d", sm.ID)
				if sm.MediaType == MediaTypeOther {
					continue
				}
				if sm.Media == nil {
					continue
				}
				if sm.MediaType == MediaTypeMovie {
					sm.Media.VideoFileName = sm.NewVideoBaseName + sm.VideoExt
					if sm.SourceType != SourceType115 {
						sm.Media.VideoFileId = filepath.Join(sm.NewPathId, sm.NewVideoBaseName+sm.VideoExt)
					}
				} else {
					if sm.MediaEpisode == nil {
						continue
					}
					sm.MediaEpisode.VideoFileName = sm.NewVideoBaseName + sm.VideoExt
					if sm.SourceType != SourceType115 {
						sm.MediaEpisode.VideoFileId = filepath.Join(sm.NewPathId, sm.NewVideoBaseName+sm.VideoExt)
					}
				}

				sm.Media.PathId = sm.NewPathId
				if sm.SourceType != SourceType115 {
					sm.Media.Path = sm.NewPathId
					if sm.MediaType == MediaTypeTvShow {
						if sm.MediaEpisode == nil || sm.MediaSeason == nil {
							continue
						}
						sm.MediaSeason.Path = sm.NewSeasonPathId
						sm.MediaSeason.PathId = sm.NewSeasonPathId
					}
				} else {
					sm.Media.Path = filepath.Join(sm.DestPath, sm.CategoryName, sm.NewPathName)
					if sm.MediaType == MediaTypeTvShow {
						if sm.MediaEpisode == nil || sm.MediaSeason == nil {
							continue
						}
						sm.MediaSeason.Path = filepath.Join(sm.Media.Path, sm.NewSeasonPathName)
						sm.MediaSeason.PathId = sm.NewSeasonPathId
					}
				}
				sm.Media.ScrapePathId = sm.ScrapePathId
				sm.Media.Save()
				if sm.MediaType == MediaTypeTvShow {
					if sm.MediaEpisode == nil || sm.MediaSeason == nil {
						continue
					}
					sm.MediaSeason.ScrapePathId = sm.ScrapePathId
					sm.MediaEpisode.ScrapePathId = sm.ScrapePathId
					sm.MediaSeason.Save()
					sm.MediaEpisode.Save()
				}
			}
			db.Db.Save(&scrapeMediaFiles)
			offset += limit
		}
		err := db.Db.Model(&Media{}).Where("status = ?", "unscraped").Update("status", "scanned").Error
		if err != nil {
			helpers.AppLogger.Errorf("所有刮削结果表的状态更新失败，错误：%v", err)
		} else {
			helpers.AppLogger.Infof("所有刮削结果表的未刮削状态已从unscraped更新为scanned")
		}
		err = db.Db.Model(&Media{}).Where("status = ?", "scraped").Update("status", "renamed").Error
		if err != nil {
			helpers.AppLogger.Errorf("所有刮削结果表的状态更新失败，错误：%v", err)
		} else {
			helpers.AppLogger.Infof("所有刮削结果表的已刮削状态已从scraped更新为renamed")
		}

		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 5 {
		// 给下载任务添加m_time字段
		db.Db.AutoMigrate(DbDownloadTask{})
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 6 {
		// 给同步目录增加更多设置
		db.Db.AutoMigrate(SyncPath{})
		// 修改默认值
		updates := map[string]interface{}{
			"delete_dir":     -1,
			"download_meta":  -1,
			"upload_meta":    -1,
			"min_video_size": -1,
		}
		db.Db.Model(&SyncPath{}).Where("id > ?", 0).Updates(updates)
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 7 {
		// 给同步目录增加添加路径设置
		db.Db.AutoMigrate(SyncPath{}, Settings{})
		// 修改默认值
		updates := map[string]interface{}{
			"add_path": -1,
		}
		db.Db.Model(&SyncPath{}).Where("id > ?", 0).Updates(updates)
		// 修改配置表默认值
		updates = map[string]interface{}{
			"add_path": 2,
		}
		db.Db.Model(&Settings{}).Where("id > ?", 0).Updates(updates)
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 8 {
		// 创建新的通知渠道表
		db.Db.AutoMigrate(
			&NotificationChannel{},
			&TelegramChannelConfig{},
			&MeoWChannelConfig{},
			&BarkChannelConfig{},
			&ServerChanChannelConfig{},
			&NotificationRule{},
		)
		// 迁移现有的Telegram设置到新表
		migrateExistingNotificationSettings(db.Db)
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 9 {
		// 增加自定义Webhook通知渠道表
		db.Db.AutoMigrate(&CustomWebhookChannelConfig{})
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 10 {
		// Webhook 渠道配置增加鉴权与 QueryParam 字段
		db.Db.AutoMigrate(&CustomWebhookChannelConfig{})
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 11 {
		// 将account表的AppId字段替换为AppIdName
		// 查询所有Account
		// accounts := []Account{}
		// db.Db.Find(&accounts)
		// for _, account := range accounts {
		// appIdName := "自定义"
		// 	switch account.AppId {
		// 	case helpers.GlobalConfig.Open115AppId:
		// 		appIdName = "Q115-STRM"
		// 	case helpers.GlobalConfig.Open115TestAppId:
		// 		appIdName = "MQ的媒体库"
		// 	}
		// 	db.Db.Model(&Account{}).Where("id = ?", account.ID).Update("app_id", appIdName)
		// 	helpers.AppLogger.Infof("Account %d 的 AppId 字段已更新为 AppIdName：%s", account.ID, appIdName)
		// }
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 12 {
		// 备份相关表 + Emby同步相关表
		db.Db.AutoMigrate(
			BackupConfig{}, BackupRecord{},
			EmbyConfig{}, EmbyMediaItem{}, EmbyMediaSyncFile{}, EmbyLibrary{}, EmbyLibrarySyncPath{},
		)
		migrateEmbyConfig(db.Db)
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 13 {
		// 备份相关表 + Emby同步相关表
		db.Db.AutoMigrate(ApiKey{})
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 14 {
		// 添加EnableAuth字段到EmbyConfig表
		db.Db.AutoMigrate(EmbyConfig{})
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 15 {
		// 优化EmbyMediaSyncFile表，添加SyncPathId字段
		db.Db.AutoMigrate(EmbyMediaSyncFile{})
		// 给EmbyMediaSyncFile表补充新增的SyncPathId字段
		fillSyncPathIdInEmbyMediaSyncFile(db.Db)
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 16 {
		// 清空SyncFile，EmbyMediaSyncFile，DbDownloadTask表数据
		db.Db.Exec("DELETE FROM sync_files")
		db.Db.Exec("DELETE FROM emby_media_sync_files")
		db.Db.Exec("DELETE FORM db_download_tasks")
		db.Db.AutoMigrate(SyncFile{})
		// 删除已存在的同步缓存表
		db.Db.Exec("DROP TABLE IF EXISTS sync_files_cache")
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 17 {
		migrator.UpdateVersionCode(db.Db) // 增加到18
	}
	if migrator.VersionCode == 18 {
		// 给User表添加IsAdmin字段
		db.Db.AutoMigrate(SyncFile{})
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 19 {
		// 添加115请求统计表
		db.Db.AutoMigrate(&RequestStat{})
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 20 {
		// 删除不再使用的表
		db.Db.Migrator().DropTable("sync115_path", "sync_files_cache", "backup_task", "restore_task")
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 21 {
		db.Db.AutoMigrate(Settings{}) // 增加openlist限速新字段
		// 给新字段添加默认值
		updateData := make(map[string]interface{})
		// 将下载QPS默认改为1，防止限流
		updateData["download_threads"] = 1
		updateData["openlist_qps"] = 2
		updateData["openlist_retry"] = 1
		updateData["openlist_retry_delay"] = 60
		err := db.Db.Model(Settings{}).Where("id >= ?", 1).Updates(updateData).Error
		if err != nil {
			helpers.AppLogger.Errorf("更新Openlist限速设置默认值失败: %v", err)
		}
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 22 {
		// 给Settings表添加CheckMetaMtime字段
		db.Db.AutoMigrate(Settings{}, SyncPath{})
		// 默认修改false
		updateData := make(map[string]int)
		updateData["check_meta_mtime"] = -1
		// 给所有SyncPath设置默认值false
		db.Db.Model(SyncPath{}).Where("id >= ?", 1).Updates(updateData)
		// 给所有Settings设置默认值0
		updateData["check_meta_mtime"] = 0
		db.Db.Model(Settings{}).Where("id >= ?", 1).Updates(updateData)
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 23 {
		// 给Settings表添加CheckMetaMtime字段
		db.Db.AutoMigrate(Settings{}, SyncPath{})
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 24 {
		db.Db.AutoMigrate(BackupConfig{}, BackupRecord{})
		// 插入默认配置
		db.Db.Create(&BackupConfig{
			BaseModel:       BaseModel{ID: 1},
			BackupEnabled:   0,
			BackupPath:      "backups",
			BackupRetention: 7,
			BackupMaxCount:  7,
			BackupCompress:  1,
			BackupCron:      "0 2 * * *",
		})
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 25 {
		db.Db.AutoMigrate(SyncPath{})
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 26 {
		db.Db.AutoMigrate(BackupConfig{}, BackupRecord{}, MediaEpisode{})
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 27 {
		db.Db.AutoMigrate(ScrapeStrmPath{})
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 28 {
		db.Db.AutoMigrate(Media{}, MediaEpisode{})
		migrator.UpdateVersionCode(db.Db)
	}
	if migrator.VersionCode == 29 {
		db.Db.AutoMigrate(EmbyLibrarySyncPath{})
		migrator.UpdateVersionCode(db.Db)
	}
	helpers.AppLogger.Infof("当前数据库版本 %d", migrator.VersionCode)
}

func BatchCreateTable() error {
	db.Db.Statement.PrepareStmt = true

	var err error
	var lastErr error
	// 数据库版本表
	err = db.Db.AutoMigrate(Migrator{})
	if err != nil {
		lastErr = err
	}
	// 配置、用户、同步目录表
	err = db.Db.AutoMigrate(Settings{}, Sync{}, User{}, SyncPath{}, Account{})
	if err != nil {
		lastErr = err
	}
	err = db.Db.AutoMigrate(SyncFile{})
	if err != nil {
		lastErr = err
	}
	// 刮削相关表
	err = db.Db.AutoMigrate(ScrapeSettings{}, ScrapePath{}, MovieCategory{}, TvShowCategory{}, ScrapePathCategory{}, ScrapeMediaFile{}, Media{}, MediaSeason{}, MediaEpisode{}, ScrapeStrmPath{})
	if err != nil {
		lastErr = err
	}
	// 115请求统计表
	err = db.Db.AutoMigrate(&RequestStat{})
	if err != nil {
		lastErr = err
	}
	// Emby 同步相关表
	err = db.Db.AutoMigrate(EmbyConfig{}, EmbyMediaItem{}, EmbyMediaSyncFile{}, EmbyLibrary{}, EmbyLibrarySyncPath{})
	if err != nil {
		lastErr = err
	}
	// 下载队列
	err = db.Db.AutoMigrate(DbDownloadTask{}, DbUploadTask{})
	if err != nil {
		lastErr = err
	}
	// 通知渠道表
	err = db.Db.AutoMigrate(NotificationChannel{}, TelegramChannelConfig{}, MeoWChannelConfig{}, BarkChannelConfig{}, ServerChanChannelConfig{}, CustomWebhookChannelConfig{}, NotificationRule{})
	if err != nil {
		lastErr = err
	}
	// API Key认证表
	err = db.Db.AutoMigrate(ApiKey{})
	if err != nil {
		lastErr = err
	}
	// 备份恢复相关表
	err = db.Db.AutoMigrate(BackupConfig{}, BackupRecord{})
	if err != nil {
		lastErr = err
	}
	return lastErr
}

func InitMigrationTable(version int) {
	var migrator Migrator = Migrator{}
	migrator = Migrator{BaseModel: BaseModel{ID: 1}, VersionCode: version} // 初始版本为version
	db.Db.Create(&migrator)
	helpers.AppLogger.Infof("初始化数据库版本表，当前版本为%d", version)
}

func InitDB(version int) bool {
	// 初始化
	if db.Db.Migrator().HasTable(Migrator{}) {
		helpers.AppLogger.Info("数据库版本表已存在，跳过初始化数据库过程")
		return true
	}
	BatchCreateTable()
	InitMigrationTable(version)
	// 初始化默认配置
	InitSettings()
	// 初始化用户
	InitUser()
	// 初始化刮削配置
	InitScrapeSetting()
	helpers.AppLogger.Info("已完成数据库初始化")
	return false
}

func (m *Migrator) UpdateVersionCode(txOrDb *gorm.DB) {
	m.VersionCode++
	txOrDb.Updates(&m)
	helpers.AppLogger.Infof("同步库结构更新完毕，当前数据库版本：%d", m.VersionCode)
}

func InitSettings() {
	defaultSettings := Settings{}
	serr := db.Db.Model(&Settings{}).First(&defaultSettings).Error
	if !errors.Is(serr, gorm.ErrRecordNotFound) {
		return
	}
	// 插入默认值
	metaExtStr, _ := json.Marshal(helpers.GlobalConfig.Strm.MetaExt)
	videoExtStr, _ := json.Marshal(helpers.GlobalConfig.Strm.VideoExt)
	ipv4, _ := helpers.GetLocalIP()
	defaultSettings = Settings{
		// 设置默认值
		TelegramBotToken: "",
		TelegramChatId:   "",
		HttpProxy:        "",
		SettingStrm: SettingStrm{
			Cron:         helpers.GlobalConfig.Strm.Cron,
			MetaExt:      string(metaExtStr),
			VideoExt:     string(videoExtStr),
			MinVideoSize: helpers.GlobalConfig.Strm.MinVideoSize,
			DeleteDir:    0,
			UploadMeta:   0,
			DownloadMeta: 0,
			StrmBaseUrl:  fmt.Sprintf("http://%s:12333", ipv4),
		},
		SettingThreads: SettingThreads{
			DownloadThreads:    1,
			FileDetailThreads:  3,
			OpenlistQPS:        3,
			OpenlistRetry:      1,
			OpenlistRetryDelay: 60,
		},
	}
	db.Db.Create(&defaultSettings)
	helpers.AppLogger.Info("已默认添加配置")
}

func InitUser() {
	password, _ := bcrypt.GenerateFromPassword([]byte(helpers.GlobalConfig.AdminPassword), bcrypt.MinCost)
	defaultUser := User{
		// 设置默认值
		Username: helpers.GlobalConfig.AdminUsername,
		Password: string(password),
	}
	uerr := db.Db.Model(&User{}).First(&defaultUser).Error
	if errors.Is(uerr, gorm.ErrRecordNotFound) {
		db.Db.Create(&defaultUser)
	}
	helpers.AppLogger.Infof("已默认添加管理员用户：%s 密码：%s", helpers.GlobalConfig.AdminUsername, helpers.GlobalConfig.AdminPassword)
}

func InitScrapeSetting() {
	// 添加默认值
	scrapeSettings := ScrapeSettings{
		TmdbApiKey:      "",
		TmdbAccessToken: "",
		TmdbUrl:         "",
		TmdbImageUrl:    "",
		TmdbLanguage:    helpers.DEFAULT_TMDB_LANGUAGE,
		TmdbEnableProxy: true,
		EnableAi:        AiActionAssist,
	}
	db.Db.Create(&scrapeSettings)
	helpers.AppLogger.Info("已默认添加刮削设置")
	// 外语电影分类（ID为1，不可删除）
	waiyuDianying := MovieCategory{
		Name:     "外语电影",
		GenreIds: "[]",
		Language: "[]",
	}
	if err := db.Db.Create(&waiyuDianying).Error; err != nil {
		helpers.AppLogger.Errorf("添加外语电影分类失败：%v", err)
	} else {
		helpers.AppLogger.Info("已默认添加外语电影分类")
	}
	// 华语电影
	huayuiDianying := MovieCategory{
		Name:     "华语电影",
		GenreIds: "[]",
		Language: "[\"zh\", \"cn\", \"bo\",\"za\"]",
	}
	if err := db.Db.Create(&huayuiDianying).Error; err != nil {
		helpers.AppLogger.Errorf("添加华语电影分类失败：%v", err)
	} else {
		helpers.AppLogger.Info("已默认添加华语电影分类")
	}
	// 动画电影
	donghuaDianying := MovieCategory{
		Name:     "动画电影",
		GenreIds: "[16]",
		Language: "",
	}
	if err := db.Db.Create(&donghuaDianying).Error; err != nil {
		helpers.AppLogger.Errorf("添加动画电影分类失败：%v", err)
	} else {
		helpers.AppLogger.Info("已默认添加动画电影分类")
	}
	// 其他剧（ID为1，不可删除）
	qitaJu := TvShowCategory{
		Name:      "其他剧",
		GenreIds:  "",
		Countries: "",
	}
	if err := db.Db.Create(&qitaJu).Error; err != nil {
		helpers.AppLogger.Errorf("添加其他剧分类失败：%v", err)
	} else {
		helpers.AppLogger.Info("已默认添加其他剧分类")
	}
	// 国产剧
	guochanJU := TvShowCategory{
		Name:      "国产剧",
		GenreIds:  "",
		Countries: "[\"CN\",\"TW\", \"HK\", \"MO\"]",
	}
	if err := db.Db.Create(&guochanJU).Error; err != nil {
		helpers.AppLogger.Errorf("添加国产剧分类失败：%v", err)
	} else {
		helpers.AppLogger.Info("已默认添加国产剧分类")
	}
	// 欧美剧
	oumeiJu := TvShowCategory{
		Name:      "欧美剧",
		GenreIds:  "",
		Countries: "[\"US\",\"GB\", \"DE\", \"FR\", \"ES\", \"IT\", \"PT\", \"RU\", \"UA\"]",
	}
	if err := db.Db.Create(&oumeiJu).Error; err != nil {
		helpers.AppLogger.Errorf("添加欧美剧分类失败：%v", err)
	} else {
		helpers.AppLogger.Info("已默认添加欧美剧分类")
	}
	// 日韩剧
	rihanJU := TvShowCategory{
		Name:      "日韩泰剧",
		GenreIds:  "",
		Countries: "[\"JP\",\"KR\", \"KP\", \"TH\", \"IN\", \"SG\"]",
	}
	if err := db.Db.Create(&rihanJU).Error; err != nil {
		helpers.AppLogger.Errorf("添加日韩泰剧分类失败：%v", err)
	} else {
		helpers.AppLogger.Info("已默认添加日韩泰剧分类")
	}
	// 国漫
	guoman := TvShowCategory{
		Name:      "国漫",
		GenreIds:  "[16]",
		Countries: "[\"CN\",\"TW\", \"HK\",\"MO\"]",
	}
	if err := db.Db.Create(&guoman).Error; err != nil {
		helpers.AppLogger.Errorf("添加国漫分类失败：%v", err)
	} else {
		helpers.AppLogger.Info("已默认添加国漫分类")
	}
	// 日番
	rifan := TvShowCategory{
		Name:      "日番",
		GenreIds:  "[16]",
		Countries: "[\"JP\"]",
	}
	if err := db.Db.Create(&rifan).Error; err != nil {
		helpers.AppLogger.Errorf("添加日番分类失败：%v", err)
	} else {
		helpers.AppLogger.Info("已默认添加日番分类")
	}
	// 综艺
	zongyi := TvShowCategory{
		Name:      "综艺",
		GenreIds:  "[10764, 10767]",
		Countries: "",
	}
	if err := db.Db.Create(&zongyi).Error; err != nil {
		helpers.AppLogger.Errorf("添加综艺分类失败：%v", err)
	} else {
		helpers.AppLogger.Info("已默认添加综艺分类")
	}
	// 纪录片
	jilu := TvShowCategory{
		Name:      "纪录片",
		GenreIds:  "[99]",
		Countries: "",
	}
	if err := db.Db.Create(&jilu).Error; err != nil {
		helpers.AppLogger.Errorf("添加纪录片分类失败：%v", err)
	} else {
		helpers.AppLogger.Info("已默认添加纪录片分类")
	}
}

func migrateEmbyConfig(dbConn *gorm.DB) {
	var count int64
	if err := dbConn.Model(&EmbyConfig{}).Count(&count).Error; err != nil {
		return
	}
	if count > 0 {
		return
	}
	var settings Settings
	if err := dbConn.First(&settings).Error; err != nil {
		return
	}
	config := &EmbyConfig{
		EmbyUrl:    settings.EmbyUrl,
		EmbyApiKey: settings.EmbyApiKey,
		SyncCron:   settings.Cron,
	}
	dbConn.Create(config)
}

// migrateExistingNotificationSettings 迁移现有的通知设置
func migrateExistingNotificationSettings(dbConn *gorm.DB) {
	var settings Settings
	if err := dbConn.First(&settings).Error; err != nil {
		return
	}

	// 如果存在Telegram配置，创建新的记录
	if settings.UseTelegram == 1 && settings.TelegramBotToken != "" {
		channel := NotificationChannel{
			ChannelType: "telegram",
			ChannelName: "Telegram Bot",
			IsEnabled:   true,
		}
		if err := dbConn.Create(&channel).Error; err == nil {
			config := TelegramChannelConfig{
				ChannelID: channel.ID,
				BotToken:  settings.TelegramBotToken,
				ChatID:    settings.TelegramChatId,
				ProxyURL:  settings.HttpProxy,
			}
			dbConn.Create(&config)

			// 创建默认规则（所有事件都发送到此渠道）
			eventTypes := []string{
				"sync_finish", "sync_error", "scrape_finish",
				"system_alert", "media_added", "media_removed",
			}
			for _, eventType := range eventTypes {
				rule := NotificationRule{
					ChannelID: channel.ID,
					EventType: eventType,
					IsEnabled: true,
				}
				dbConn.Create(&rule)
			}
			helpers.AppLogger.Infof("已迁移Telegram通知配置到新表")
		}
	}

	// 如果存在MeoW配置，创建新的记录
	if settings.MeoWName != "" {
		channel := NotificationChannel{
			ChannelType: "meow",
			ChannelName: "MeoW",
			IsEnabled:   true,
		}
		if err := dbConn.Create(&channel).Error; err == nil {
			config := MeoWChannelConfig{
				ChannelID: channel.ID,
				Nickname:  settings.MeoWName,
				Endpoint:  "http://api.chuckfang.com",
			}
			dbConn.Create(&config)

			// 创建默认规则
			eventTypes := []string{
				"sync_finish", "sync_error", "scrape_finish",
				"system_alert", "media_added", "media_removed",
			}
			for _, eventType := range eventTypes {
				rule := NotificationRule{
					ChannelID: channel.ID,
					EventType: eventType,
					IsEnabled: true,
				}
				dbConn.Create(&rule)
			}
			helpers.AppLogger.Infof("已迁移MeoW通知配置到新表")
		}
	}
}

func fillSyncPathIdInEmbyMediaSyncFile(dbConn *gorm.DB) {
	limit := 100
	offset := 0
	for {
		var embyMediaSyncFiles []EmbyMediaSyncFile
		dbConn.Model(&EmbyMediaSyncFile{}).Limit(limit).Offset(offset).Find(&embyMediaSyncFiles)
		if len(embyMediaSyncFiles) == 0 {
			break
		}
		for _, embyMediaSyncFile := range embyMediaSyncFiles {
			// 用ID查询SyncFiles
			syncFile := GetSyncFileById(embyMediaSyncFile.SyncFileId)
			if syncFile == nil {
				continue
			}
			embyMediaSyncFile.SyncPathId = syncFile.SyncPathId
			dbConn.Save(&embyMediaSyncFile)
			helpers.AppLogger.Infof("为 EmbyMediaSyncFile %d 填充 SyncPathId %d 成功", embyMediaSyncFile.ID, syncFile.SyncPathId)
		}
		offset += limit
	}
}
