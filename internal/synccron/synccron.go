package synccron

import (
	"Q115-STRM/internal/baidupan"
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/emby"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/notificationmanager"
	"Q115-STRM/internal/scrape"
	"Q115-STRM/internal/v115open"
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

var GlobalCron *cron.Cron
var SyncCron *cron.Cron
var TokenCron *cron.Cron
var ScrapeCron *cron.Cron

func StartSyncCron() {
	// 查询所有同步目录
	syncPaths, _ := models.GetSyncPathList(1, 10000000, true, "")
	if len(syncPaths) == 0 {
		// helpers.AppLogger.Info("没有找到同步目录")
		return
	}
	for _, syncPath := range syncPaths {
		// 没开启定时任务或者自定义CRON表达式的同步目录跳过
		if syncPath.SettingStrm.Cron != "" {
			helpers.AppLogger.Infof("同步目录 %d 已启用自定义的定时任务，cron表达式: %s", syncPath.ID, syncPath.SettingStrm.Cron)
			continue
		}
		// 将同步目录ID添加到处理队列，而不是直接执行
		taskObj := &NewSyncTask{
			ID:           syncPath.ID,
			SourcePath:   "",
			SourcePathId: "",
			TargetPath:   "",
			AccountId:    syncPath.AccountId,
			IsFile:       false,
			TaskType:     SyncTaskTypeStrm,
			SourceType:   syncPath.SourceType,
		}
		if err := AddNewSyncTask(taskObj); err != nil {
			helpers.AppLogger.Errorf("将同步任务添加到队列失败: %s", err.Error())
			continue
		}
	}
}

// 开始刮削整理任务
func startScrapeCron() {
	// 查询所有刮削目录
	scrapePaths := models.GetScrapePathes("")
	if len(scrapePaths) == 0 {
		// helpers.AppLogger.Info("没有找到刮削目录")
		return
	}
	for _, scrapePath := range scrapePaths {
		if !scrapePath.EnableCron {
			continue
		}
		// 如果自定义了cron字段，则不走通用定时任务
		if scrapePath.CronExpression != "" {
			helpers.AppLogger.Infof("刮削目录 %d 已启用自定义定时任务，cron表达式: %s，跳过通用定时任务", scrapePath.ID, scrapePath.CronExpression)
			continue
		}
		// 将刮削目录ID添加到处理队列，而不是直接执行
		taskObj := &NewSyncTask{
			ID:           scrapePath.ID,
			SourcePath:   "",
			SourcePathId: "",
			TargetPath:   "",
			AccountId:    scrapePath.AccountId,
			IsFile:       false,
			TaskType:     SyncTaskTypeScrape,
			SourceType:   scrapePath.SourceType,
		}
		if err := AddNewSyncTask(taskObj); err != nil {
			helpers.AppLogger.Errorf("将刮削任务添加到队列失败: %s", err.Error())
			continue
		} else {
			helpers.AppLogger.Infof("创建刮削任务成功并已添加到执行队列，刮削目录ID: %d，刮削目录:%s，目标目录：%s", scrapePath.ID, scrapePath.SourcePath, scrapePath.DestPath)
		}
	}
}

func RefreshOAuthAccessToken() {
	// 刷新115的访问凭证
	// 取所有115类型的账号
	accounts, _ := models.GetAllAccount()
	now := time.Now().Unix()
	for _, account := range accounts {
		if account.RefreshToken == "" {
			helpers.AppLogger.Infof("账号 %d 没有刷新token，跳过", account.ID)
			continue
		}
		if account.SourceType == models.SourceType115 {
			// helpers.AppLogger.Infof("当前时间: %d, 过期时间：%d", now, account.TokenExpiriesTime-3600)
			if account.TokenExpiriesTime-1800 > now {
				helpers.AppLogger.Infof("115账号token未过期，账号ID: %d, 115用户名：%s， 过期时间：%s", account.ID, account.Username, time.Unix(account.TokenExpiriesTime-1800, 0).Format("2006-01-02 15:04:05"))
				continue
			}
			helpers.AppLogger.Infof("开始刷新115账号token，账号ID: %d, 115用户名：%s", account.ID, account.Username)
			// 刷新115的访问凭证
			client := account.Get115Client()
			tokenData, err := client.RefreshToken(account.RefreshToken)
			if err != nil {
				helpers.AppLogger.Errorf("刷新115访问凭证失败: %s", err.Error())
				// 清空token
				account.ClearToken(err.Error())
				ctx := context.Background()
				notif := &models.Notification{
					Type:      models.SystemAlert,
					Title:     "🔐 115开放平台访问凭证已失效",
					Content:   fmt.Sprintf("账号ID：%d\n用户名：%s\n请重新授权\n⏰ 时间: %s", int(account.ID), account.Username, time.Now().Format("2006-01-02 15:04:05")),
					Timestamp: time.Now(),
					Priority:  models.HighPriority,
				}
				if notificationmanager.GlobalEnhancedNotificationManager != nil {
					if err := notificationmanager.GlobalEnhancedNotificationManager.SendNotification(ctx, notif); err != nil {
						helpers.AppLogger.Errorf("发送访问凭证失效通知失败: %v", err)
					}
				}
				continue
			}
			// 更新账号的token
			if suc := account.UpdateToken(tokenData.AccessToken, tokenData.RefreshToken, tokenData.ExpiresIn); !suc {
				helpers.AppLogger.Errorf("更新115账号token失败")
				continue
			}
			// 更新其他客户端的token
			v115open.UpdateToken(account.ID, tokenData.AccessToken, tokenData.RefreshToken)
			// 刷新成功，更新账号的token
			helpers.AppLogger.Infof("刷新115账号token成功，账号ID: %d, 新到期时间: %d => %s", account.ID, tokenData.ExpiresIn, time.Unix(account.TokenExpiriesTime, 0).Format("2006-01-02 15:04:05"))
			continue
		}
		if account.SourceType == models.SourceTypeBaiduPan {
			// 刷新百度网盘的访问凭证
			if account.TokenExpiriesTime-86400 > now {
				// helpers.AppLogger.Infof("百度网盘账号token未过期，账号ID: %d, 百度网盘用户名：%s， 过期时间：%s", account.ID, account.Username, time.Unix(account.TokenExpiriesTime-86400, 0).Format("2006-01-02 15:04:05"))
				continue
			}
			// 向授权服务器发送刷新请求，拿到新token
			resp, err := baidupan.RefreshToken(account.ID, account.RefreshToken)
			if err != nil {
				helpers.AppLogger.Errorf("刷新百度网盘token失败: %s", err.Error())
				// 清空token
				account.ClearToken(err.Error())
				ctx := context.Background()
				notif := &models.Notification{
					Type:      models.SystemAlert,
					Title:     "🔐 百度网盘开放平台访问凭证已失效",
					Content:   fmt.Sprintf("账号ID：%d\n用户名：%s\n请重新授权\n⏰ 时间: %s", int(account.ID), account.Username, time.Now().Format("2006-01-02 15:04:05")),
					Timestamp: time.Now(),
					Priority:  models.HighPriority,
				}
				if notificationmanager.GlobalEnhancedNotificationManager != nil {
					if err := notificationmanager.GlobalEnhancedNotificationManager.SendNotification(ctx, notif); err != nil {
						helpers.AppLogger.Errorf("发送访问凭证失效通知失败: %v", err)
					}
				}
				continue
			}
			// 更新账号的token
			if suc := account.UpdateToken(resp.AccessToken, resp.RefreshToken, resp.ExpiresIn); !suc {
				helpers.AppLogger.Errorf("更新百度网盘账号token失败")
				continue
			}
			// 更新其他客户端的token
			baidupan.UpdateToken(account.ID, resp.AccessToken)
			// 刷新成功，更新账号的token
			helpers.AppLogger.Infof("刷新百度网盘账号token成功，账号ID: %d, 新到期时间: %d => %s", account.ID, resp.ExpiresIn, time.Unix(resp.ExpiresIn, 0).Format("2006-01-02 15:04:05"))
			continue
		}
	}
}

func startClearDownloadUploadTasks() {
	helpers.AppLogger.Info("开始清除3天前的上传任务")
	models.ClearExpireUploadTasks()
	helpers.AppLogger.Info("开始清除3天前的下载任务")
	models.ClearExpireDownloadTasks()
}

var RollBackCronStart bool = false

func StartScrapeRollbackCron() {
	if RollBackCronStart {
		helpers.AppLogger.Info("刮削回滚任务已在运行")
		return
	}
	RollBackCronStart = true
	defer func() {
		RollBackCronStart = false
	}()
	go func() {
		limit := 10
		offset := 0
		for {
			// 从数据库中获取所有状态为回滚中的记录
			var mediaFiles []*models.ScrapeMediaFile
			err := db.Db.Where("status = ?", models.ScrapeMediaStatusRollbacking).Limit(limit).Offset(offset).Find(&mediaFiles).Error
			if err != nil {
				helpers.AppLogger.Errorf("获取刮削失败的媒体文件失败: %v", err)
				return
			}
			if len(mediaFiles) == 0 {
				// helpers.AppLogger.Info("没有刮削失败的媒体文件")
				return
			}
			helpers.AppLogger.Infof("获取到 %d 个刮削失败的媒体文件", len(mediaFiles))
			// 遍历所有媒体文件，进行回滚操作
			for _, mediaFile := range mediaFiles {
				scrapePath := models.GetScrapePathByID(mediaFile.ScrapePathId)
				scrape := scrape.NewScrape(scrapePath)
				err := scrape.Rollback(mediaFile)
				if err != nil {
					helpers.AppLogger.Errorf("回滚媒体文件 %s 失败: %v", mediaFile.Name, err)
				} else {
					helpers.AppLogger.Infof("成功回滚媒体文件 %s", mediaFile.Name)
				}
			}
			// 每次处理完休息10秒
			time.Sleep(10 * time.Second)
		}
	}()

}

func InitTokenCron() {
	if TokenCron != nil {
		TokenCron.Stop()
	}
	TokenCron = cron.New()
	TokenCron.AddFunc("*/2 * * * *", func() {
		// helpers.AppLogger.Info("定时刷新115的访问凭证")
		RefreshOAuthAccessToken()
	})
	TokenCron.Start()
}

// 初始化定时任务
func InitCron() {
	if GlobalCron != nil {
		GlobalCron.Stop()
	}
	GlobalCron = cron.New()
	GlobalCron.AddFunc("0 1 * * *", func() {
		startClearDownloadUploadTasks()
	})
	GlobalCron.AddFunc(models.SettingsGlobal.Cron, func() {
		// helpers.AppLogger.Info("启动115网盘同步任务")
		StartSyncCron()
	})
	GlobalCron.AddFunc("0 0 * * *", func() {
		// 每天0点清理过期的同步记录
		// helpers.AppLogger.Info("清理过期的同步记录")
		models.ClearExpiredSyncRecords(1) // 保留3天内的记录
	})

	GlobalCron.AddFunc("*/13 * * * *", func() {
		// helpers.AppLogger.Info("启动刮削任务")
		startScrapeCron()
	})
	if config, err := models.GetEmbyConfig(); err == nil {
		if config.EmbyApiKey != "" && config.EmbyUrl != "" && config.SyncEnabled == 1 {
			GlobalCron.AddFunc(config.SyncCron, func() {
				if _, err := emby.PerformEmbySync(); err != nil {
					helpers.AppLogger.Errorf("Emby同步失败: %v", err)
				}
			})
		}
	}
	GlobalCron.AddFunc("*/2 * * * *", func() {
		// helpers.AppLogger.Info("启动刮削回滚任务")
		StartScrapeRollbackCron()
	})
	GlobalCron.AddFunc("0 * * * *", func() {
		// 每小时清理一次请求统计数据，只保留最近24小时
		if err := models.CleanOldRequestStatsByHours(24); err != nil {
			helpers.AppLogger.Errorf("清理请求统计数据失败: %v", err)
		} else {
			helpers.AppLogger.Infof("已清理24小时前的请求统计数据")
		}
	})
	GlobalCron.AddFunc("0 4 * * *", func() {
		// 每天4点修复数据库表的主键序列
		// 修复数据库，重建所有表
		err := models.BatchCreateTable()
		if err != nil {
			helpers.AppLogger.Errorf("修复数据库失败: %v", err)
			return
		} else {
			helpers.AppLogger.Infof("已重建全部数据表（不影响已存在的表和数据）")
		}
		if err := models.BatchRepairTableSeq(); err != nil {
			helpers.AppLogger.Errorf("修复数据库表的主键序列失败: %v", err)
		} else {
			helpers.AppLogger.Infof("已修复所有表的主键序列")
		}
	})

	addBackupCron()

	GlobalCron.Start()
}

// 初始化STRM同步目录的定时任务
func InitSyncCron() {
	if SyncCron != nil {
		helpers.AppLogger.Info("已存在同步目录的定时任务，先停止")
		SyncCron.Stop()
	}
	SyncCron = cron.New()
	// 查询所有同步目录
	syncPaths, _ := models.GetSyncPathList(1, 10000000, true, "")
	if len(syncPaths) == 0 {
		helpers.AppLogger.Info("没有启用定时任务的同步目录")
		return
	}
	for _, syncPath := range syncPaths {
		if syncPath.Cron == "" {
			helpers.AppLogger.Infof("同步目录 %d 未启用自定义的定时任务", syncPath.ID)
			continue
		}
		helpers.AppLogger.Infof("已添加同步目录 %d 的定时任务，cron表达式: %s", syncPath.ID, syncPath.Cron)
		SyncCron.AddFunc(syncPath.Cron, func() {
			// 将同步目录ID添加到处理队列，而不是直接执行
			taskObj := &NewSyncTask{
				ID:           syncPath.ID,
				SourcePath:   "",
				SourcePathId: "",
				TargetPath:   "",
				AccountId:    syncPath.AccountId,
				IsFile:       false,
				TaskType:     SyncTaskTypeStrm,
				SourceType:   syncPath.SourceType,
			}
			if err := AddNewSyncTask(taskObj); err != nil {
				helpers.AppLogger.Errorf("将同步任务添加到队列失败: %s", err.Error())
				return
			}
		})
	}
	SyncCron.Start()
}

// 初始化刮削目录的自定义定时任务
func InitScrapeCron() {
	if ScrapeCron != nil {
		helpers.AppLogger.Info("已存在刮削目录的定时任务，先停止")
		ScrapeCron.Stop()
	}
	ScrapeCron = cron.New()

	// 查询所有刮削目录
	scrapePaths := models.GetScrapePathes("")
	if len(scrapePaths) == 0 {
		helpers.AppLogger.Info("没有启用自定义定时任务的刮削目录")
		return
	}

	for _, scrapePath := range scrapePaths {
		// 未启用定时任务或没有自定义Cron表达式的跳过
		if !scrapePath.EnableCron || scrapePath.CronExpression == "" {
			helpers.AppLogger.Infof("刮削目录 %d 未启用自定义定时任务", scrapePath.ID)
			continue
		}

		helpers.AppLogger.Infof("已添加刮削目录 %d 的定时任务，cron表达式: %s", scrapePath.ID, scrapePath.CronExpression)

		// 注意：需要在闭包中捕获scrapePath的值，避免循环变量问题
		scrapePathID := scrapePath.ID
		accountId := scrapePath.AccountId
		sourceType := scrapePath.SourceType

		ScrapeCron.AddFunc(scrapePath.CronExpression, func() {
			// 将刮削目录ID添加到处理队列，而不是直接执行
			taskObj := &NewSyncTask{
				ID:           scrapePathID,
				SourcePath:   "",
				SourcePathId: "",
				TargetPath:   "",
				AccountId:    accountId,
				IsFile:       false,
				TaskType:     SyncTaskTypeScrape,
				SourceType:   sourceType,
			}
			if err := AddNewSyncTask(taskObj); err != nil {
				helpers.AppLogger.Errorf("将刮削任务添加到队列失败: %s", err.Error())
				return
			}
			helpers.AppLogger.Infof("创建刮削任务成功并已添加到执行队列，刮削目录ID: %d", scrapePathID)
		})
	}
	ScrapeCron.Start()
}

func addBackupCron() {
	backupConfig := models.GetOrCreateBackupConfig()
	if backupConfig.BackupEnabled == 0 || backupConfig.BackupCron == "" {
		return
	}
	_, err := GlobalCron.AddFunc(backupConfig.BackupCron, func() {
		helpers.AppLogger.Info("开始执行定时自动备份")
		helpers.Publish(helpers.BackupCronEevent, nil)
	})

	if err != nil {
		helpers.AppLogger.Errorf("添加备份定时任务失败: %v", err)
	} else {
		helpers.AppLogger.Infof("已添加自动备份定时任务，cron表达式: %s", backupConfig.BackupCron)
	}
}
