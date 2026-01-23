package models

import (
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/helpers"
	"encoding/json"
	"errors"
	"fmt"
	"os"
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
	dbFile := filepath.Join(helpers.RootDir, helpers.GlobalConfig.Db.File)
	sqliteDb := db.GetDb(dbFile)
	maxVersion := 11
	if sqliteDb != nil {
		// 从sqlite迁移数据到postgres
		moveSqliteToPostres(sqliteDb, maxVersion)
		// 关闭sqlite连接，然后将数据库文件备份
		sqldb, _ := sqliteDb.DB()
		if sqldb != nil {
			sqldb.Close()
		}
		os.Rename(dbFile, dbFile+".bak")
		helpers.AppLogger.Infof("sqlite数据库已备份为：%s", dbFile+".bak")
	} else {
		// 先初始化所有表和基础数据
		if !InitDB(maxVersion) {
			// 初始化数据库版本表
			helpers.AppLogger.Info("已完成数据库初始化")
			return
		}
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
		accounts := []Account{}
		db.Db.Find(&accounts)
		for _, account := range accounts {
			appIdName := "自定义"
			switch account.AppId {
			case helpers.GlobalConfig.Open115AppId:
				appIdName = "Q115-STRM"
			case helpers.GlobalConfig.Open115TestAppId:
				appIdName = "MQ的媒体库"
			}
			db.Db.Model(&Account{}).Where("id = ?", account.ID).Update("app_id", appIdName)
			helpers.AppLogger.Infof("Account %d 的 AppId 字段已更新为 AppIdName：%s", account.ID, appIdName)
		}
		migrator.UpdateVersionCode(db.Db)
	}
	helpers.AppLogger.Infof("当前数据库版本 %d", migrator.VersionCode)
}

func BatchCreateTable() {
	// 数据库版本表
	db.Db.AutoMigrate(Migrator{})
	// 配置、用户、同步目录表
	db.Db.AutoMigrate(Settings{}, Sync{}, User{}, SyncPath{}, Account{})
	db.Db.AutoMigrate(SyncFile{}, Sync115Path{})
	// 刮削相关表
	db.Db.AutoMigrate(ScrapeSettings{}, ScrapePath{}, MovieCategory{}, TvShowCategory{}, ScrapePathCategory{}, ScrapeMediaFile{}, Media{}, MediaSeason{}, MediaEpisode{})
	// 下载队列
	db.Db.AutoMigrate(DbDownloadTask{}, DbUploadTask{})
	// 通知渠道表
	db.Db.AutoMigrate(NotificationChannel{}, TelegramChannelConfig{}, MeoWChannelConfig{}, BarkChannelConfig{}, ServerChanChannelConfig{}, CustomWebhookChannelConfig{}, NotificationRule{})
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

func moveSqliteToPostres(sqliteDb *gorm.DB, version int) {
	BatchCreateTable()
	// 将用到的model数据从sqliteDb迁移到db.Db
	var accounts []*Account
	sqliteDb.Order("id").Find(&accounts)
	for _, account := range accounts {
		newAccount := Account{
			Name:              account.Name,
			UserId:            account.UserId,
			Username:          account.Username,
			Password:          account.Password,
			Token:             account.Token,
			RefreshToken:      account.RefreshToken,
			TokenExpiriesTime: account.TokenExpiriesTime,
			BaseUrl:           account.BaseUrl,
			SourceType:        account.SourceType,
			AppId:             account.AppId,
		}
		if err := db.Db.Create(&newAccount).Error; err != nil {
			helpers.AppLogger.Errorf("迁移Account数据失败：%v", err)
		} else {
			helpers.AppLogger.Infof("迁移Account数据成功：%d", newAccount.ID)
		}
	}

	var movieCategories []*MovieCategory
	sqliteDb.Order("id").Find(&movieCategories)
	for _, movieCategory := range movieCategories {
		newMovieCategory := MovieCategory{
			Name:     movieCategory.Name,
			GenreIds: movieCategory.GenreIds,
			Language: movieCategory.Language,
		}
		if err := db.Db.Create(&newMovieCategory).Error; err != nil {
			helpers.AppLogger.Errorf("迁移MovieCategory数据失败：%v", err)
		} else {
			helpers.AppLogger.Infof("迁移MovieCategory数据成功：%d", newMovieCategory.ID)
		}
	}

	var scrapePaths []*ScrapePath
	sqliteDb.Order("id").Find(&scrapePaths)
	for _, scrapePath := range scrapePaths {
		newScrapePath := ScrapePath{
			AccountId:             scrapePath.AccountId,
			SourceType:            scrapePath.SourceType,
			MediaType:             scrapePath.MediaType,
			ScrapeType:            scrapePath.ScrapeType,
			SourcePath:            scrapePath.SourcePath,
			SourcePathId:          scrapePath.SourcePathId,
			DestPath:              scrapePath.DestPath,
			DestPathId:            scrapePath.DestPathId,
			RenameType:            scrapePath.RenameType,
			FolderNameTemplate:    scrapePath.FolderNameTemplate,
			FileNameTemplate:      scrapePath.FileNameTemplate,
			DeletedKeyword:        scrapePath.DeletedKeyword,
			EnableCategory:        scrapePath.EnableCategory,
			VideoExt:              scrapePath.VideoExt,
			MinVideoFileSize:      scrapePath.MinVideoFileSize,
			ExcludeNoImageActor:   scrapePath.ExcludeNoImageActor,
			EnableAi:              scrapePath.EnableAi,
			AiPrompt:              scrapePath.AiPrompt,
			ForceDeleteSourcePath: scrapePath.ForceDeleteSourcePath,
			EnableCron:            scrapePath.EnableCron,
			EnableFanartTv:        scrapePath.EnableFanartTv,
			IsScraping:            scrapePath.IsScraping,
			MaxThreads:            scrapePath.MaxThreads,
		}
		if scrapePath.MaxThreads == 0 {
			newScrapePath.MaxThreads = DEFAULT_LOCAL_MAX_THREADS
		}
		if err := db.Db.Create(&newScrapePath).Error; err != nil {
			helpers.AppLogger.Errorf("迁移ScrapePath数据失败：%v", err)
		} else {
			helpers.AppLogger.Infof("迁移ScrapePath数据成功：%d", newScrapePath.ID)
		}
	}

	var scrapeSettings ScrapeSettings
	sqliteDb.Find(&scrapeSettings)
	newScrapeSetting := ScrapeSettings{
		TmdbUrl:           scrapeSettings.TmdbUrl,
		TmdbImageUrl:      scrapeSettings.TmdbImageUrl,
		TmdbApiKey:        scrapeSettings.TmdbApiKey,
		TmdbAccessToken:   scrapeSettings.TmdbAccessToken,
		TmdbLanguage:      scrapeSettings.TmdbLanguage,
		TmdbImageLanguage: scrapeSettings.TmdbImageLanguage,
		TmdbEnableProxy:   scrapeSettings.TmdbEnableProxy,
	}
	if err := db.Db.Create(&newScrapeSetting).Error; err != nil {
		helpers.AppLogger.Errorf("迁移ScrapeSettings数据失败：%v", err)
	} else {
		helpers.AppLogger.Infof("迁移ScrapeSettings数据成功：%d", newScrapeSetting.ID)
	}

	var settings Settings
	sqliteDb.Find(&settings)
	newSettings := Settings{
		UseTelegram:       settings.UseTelegram,
		TelegramBotToken:  settings.TelegramBotToken,
		TelegramChatId:    settings.TelegramChatId,
		MeoWName:          settings.MeoWName,
		HttpProxy:         settings.HttpProxy,
		StrmBaseUrl:       settings.StrmBaseUrl,
		Cron:              settings.Cron,
		MetaExt:           settings.MetaExt,
		VideoExt:          settings.VideoExt,
		MinVideoSize:      settings.MinVideoSize,
		UploadMeta:        settings.UploadMeta,
		DownloadMeta:      settings.DownloadMeta,
		DeleteDir:         settings.DeleteDir,
		LocalProxy:        settings.LocalProxy,
		ExcludeName:       settings.ExcludeName,
		EmbyUrl:           settings.EmbyUrl,
		EmbyApiKey:        settings.EmbyApiKey,
		DownloadThreads:   settings.DownloadThreads,
		FileDetailThreads: settings.FileDetailThreads,
	}
	if err := db.Db.Create(&newSettings).Error; err != nil {
		helpers.AppLogger.Errorf("迁移Settings数据失败：%v", err)
	} else {
		helpers.AppLogger.Infof("迁移Settings数据成功：%d", newSettings.ID)
	}

	var syncPathes []*SyncPath
	sqliteDb.Order("id").Find(&syncPathes)
	for _, syncPath := range syncPathes {
		newSyncPath := SyncPath{
			BaseCid:      syncPath.BaseCid,
			LocalPath:    syncPath.LocalPath,
			RemotePath:   syncPath.RemotePath,
			SourceType:   syncPath.SourceType,
			AccountId:    syncPath.AccountId,
			EnableCron:   syncPath.EnableCron,
			LastSyncAt:   syncPath.LastSyncAt,
			CustomConfig: syncPath.CustomConfig,
			SyncPathSetting: SyncPathSetting{
				VideoExt:     syncPath.VideoExt,
				MetaExt:      syncPath.MetaExt,
				ExcludeName:  syncPath.ExcludeName,
				MinVideoSize: -1,
				UploadMeta:   -1,
				DownloadMeta: -1,
				DeleteDir:    -1,
			},
		}
		if err := db.Db.Create(&newSyncPath).Error; err != nil {
			helpers.AppLogger.Errorf("迁移SyncPath数据失败：%v", err)
		} else {
			helpers.AppLogger.Infof("迁移SyncPath数据成功：%d", newSyncPath.ID)
		}
	}

	var tvShows []*TvShowCategory
	sqliteDb.Order("id").Find(&tvShows)
	for _, tvShow := range tvShows {
		newTvShow := TvShowCategory{
			Name:      tvShow.Name,
			GenreIds:  tvShow.GenreIds,
			Countries: tvShow.Countries,
		}
		if err := db.Db.Create(&newTvShow).Error; err != nil {
			helpers.AppLogger.Errorf("迁移TvShowCategory数据失败：%v", err)
		} else {
			helpers.AppLogger.Infof("迁移TvShowCategory数据成功：%d", newTvShow.ID)
		}
	}

	var users []*User
	sqliteDb.Order("id").Find(&users)
	for _, user := range users {
		newUser := User{
			Username: user.Username,
			Password: user.Password,
		}
		if err := db.Db.Create(&newUser).Error; err != nil {
			helpers.AppLogger.Errorf("迁移User数据失败：%v", err)
		} else {
			helpers.AppLogger.Infof("迁移User数据成功：%d", newUser.ID)
		}
	}

	helpers.AppLogger.Info("已完成数据库迁移，数据已从sqlite迁移至postgres")
	InitMigrationTable(version)
	// 删除全部config/libs
	libsPath := filepath.Join(helpers.RootDir, "config", "libs")
	os.RemoveAll(libsPath)
	helpers.AppLogger.Infof("已删除所有config/libs目录下的文件")
}

func ResetAutoIncrementToNext(db *gorm.DB, tableName string, maxID uint) error {
	if maxID == 0 {
		// 查询最大值
		var max uint
		db.Table(tableName).Order("id DESC").Limit(1).Select("id").Scan(&max)
		// db.Debug().Raw("SELECT MAX(id) FROM ?", tableName).Scan(&max)
		maxID = max
	}
	nextID := maxID + 1
	// ALTER SEQUENCE public.users_id_seq RESTART WITH 2;
	sql := fmt.Sprintf("ALTER SEQUENCE public.%s_id_seq RESTART WITH %d", tableName, nextID)
	return db.Exec(sql).Error
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
		TelegramBotToken:  "",
		TelegramChatId:    "",
		HttpProxy:         "",
		Cron:              helpers.GlobalConfig.Strm.Cron,
		MetaExt:           string(metaExtStr),
		VideoExt:          string(videoExtStr),
		MinVideoSize:      helpers.GlobalConfig.Strm.MinVideoSize,
		DeleteDir:         0,
		UploadMeta:        1,
		DownloadMeta:      1,
		StrmBaseUrl:       fmt.Sprintf("http://%s:12333", ipv4),
		DownloadThreads:   5,
		FileDetailThreads: 3,
	}
	db.Db.Create(&defaultSettings)
	helpers.AppLogger.Info("已默认添加配置")
}

func InitUser() {
	password, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.MinCost)
	defaultUser := User{
		// 设置默认值
		Username: "admin",
		Password: string(password),
	}
	uerr := db.Db.Model(&User{}).First(&defaultUser).Error
	if errors.Is(uerr, gorm.ErrRecordNotFound) {
		db.Db.Create(&defaultUser)
	}
	helpers.AppLogger.Info("已默认添加管理员用户")
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
