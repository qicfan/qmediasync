package controllers

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/notificationmanager"
	"Q115-STRM/internal/synccron"
	"context"
	"strconv"
	"time"
)

// TaskType 任务类型枚举
type TaskType string

const (
	TaskTypeStrm   TaskType = "strm"
	TaskTypeScrape TaskType = "scrape"
)

// checkAndExtractSingleParam 检查并提取单个任务参数
// args: 参数列表
// 返回错误信息（如果参数格式错误）和提取的任务ID（如果成功）
// 如果没有参数或参数为空，返回空错误和0
func checkAndExtractSingleParam(args []string) (string, uint) {
	if len(args) > 0 && args[0] != "" {
		param := args[0]
		// 检查参数是否以#开头且长度大于1
		if !(len(param) > 1 && param[0] == '#') {
			return "❌ 参数格式错误，请使用 #数字 格式", 0
		}
		// 尝试将参数转换为uint
		numStr := param[1:]
		id, parseErr := strconv.ParseUint(numStr, 10, 32)
		if parseErr != nil {
			return "❌ 参数格式错误，请使用 #数字 格式", 0
		}
		return "", uint(id)
	}
	return "", 0
}

// checkAndExtractMoreParam 检查并提取多个任务参数
// args: 参数列表
// 返回错误信息（如果参数格式错误）和提取的任务ID列表（如果成功）
func checkAndExtractMoreParam(args []string) (string, []uint) {
	var taskIDs []uint
	for _, arg := range args {
		if arg != "" {
			// 检查参数是否以#开头且长度大于1
			if !(len(arg) > 1 && arg[0] == '#') {
				return "❌ 参数格式错误，请使用 #数字 #数字 格式", nil
			}
			// 尝试将参数转换为uint
			numStr := arg[1:]
			id, parseErr := strconv.ParseUint(numStr, 10, 32)
			if parseErr != nil {
				return "❌ 参数格式错误，请使用 #数字 #数字 格式", nil
			}
			taskIDs = append(taskIDs, uint(id))
		}
	}
	return "", taskIDs
}

// runStrmTask 执行STRM同步任务并在完成后发送通知
// args: 可选参数，传入同步目录ID时只同步指定目录
// isFullSync: 是否执行全量同步
func runStrmTask(args []string, isFullSync bool) string {
	// 检查参数格式
	if errMsg, _ := checkAndExtractSingleParam(args); errMsg != "" {
		return errMsg
	}

	// 先返回开始执行的消息
	go func() {
		var taskIDs []uint
		var title, content string

		// 设置通知信息
		if isFullSync {
			title = "✅ 全量STRM同步完成"
			content = "所有全量STRM同步任务已执行完毕"
		} else {
			title = "✅ 增量STRM同步完成"
			content = "所有增量STRM同步任务已执行完毕"
		}

		// 检查是否传入了目录ID
		if len(args) > 0 && args[0] != "" {
			// 检查并提取参数
			if _, taskID := checkAndExtractSingleParam(args); taskID > 0 {

				// 获取指定同步目录
				syncPath := models.GetSyncPathById(taskID)
				if syncPath != nil {
					// 如果是全量同步，设置标志
					if isFullSync {
						syncPath.SetIsFullSync(true)
					}
					// 同步指定目录
					synccron.AddNewSyncTask(taskID, synccron.SyncTaskTypeStrm)
					taskIDs = []uint{taskID}
					// 设置通知内容
					if isFullSync {
						content = "目录：" + syncPath.RemotePath + "，全量STRM同步任务已执行完毕"
					} else {
						content = "目录：" + syncPath.RemotePath + "，增量STRM同步任务已执行完毕"
					}
				}
			}
		}

		// 如果没有指定目录，执行所有目录
		if len(taskIDs) == 0 {
			// 获取所有同步目录
			allSyncPaths, _ := models.GetSyncPathList(1, 10000000, false)
			for _, syncPath := range allSyncPaths {
				// 全量同步时设置标志
				if isFullSync {
					syncPath.SetIsFullSync(true)
				}
				// 同步目录
				synccron.AddNewSyncTask(syncPath.ID, synccron.SyncTaskTypeStrm)
				taskIDs = append(taskIDs, syncPath.ID)
			}
			// 设置通知内容
			if isFullSync {
				content = "目录：全部，全量STRM同步任务已执行完毕"
			} else {
				content = "目录：全部，增量STRM同步任务已执行完毕"
			}
		}

		// 检查是否有任务
		if len(taskIDs) == 0 {
			return
		}

		// 等待所有任务执行完成
		time.Sleep(2 * time.Second) // 等待任务队列初始化

		// 监控任务的状态
		waitForTasksCompletion(taskIDs, synccron.SyncTaskTypeStrm)

		// 所有任务执行完成，发送通知
		ctx := context.Background()
		notif := &models.Notification{
			Type:      models.SystemAlert,
			Title:     title,
			Content:   content,
			Timestamp: time.Now(),
			Priority:  models.NormalPriority,
		}
		if notificationmanager.GlobalEnhancedNotificationManager != nil {
			notificationmanager.GlobalEnhancedNotificationManager.SendNotification(ctx, notif)
		}
	}()

	// 返回开始执行的消息
	if isFullSync {
		return "🔄 开始执行全量STRM同步"
	}
	return "🔄 开始执行增量STRM同步"
}

// runScrapeTask 执行刮削任务并在完成后发送通知
// args: 可选参数，传入刮削目录ID时只执行指定目录
func runScrapeTask(args []string) string {
	// 检查参数格式
	if errMsg, _ := checkAndExtractSingleParam(args); errMsg != "" {
		return errMsg
	}

	// 先返回开始执行的消息
	go func() {
		var taskIDs []uint
		var title, content string

		// 设置通知信息
		title = "✅ 刮削任务完成"
		content = "所有刮削任务已执行完毕"

		// 检查是否传入了目录ID
		if len(args) > 0 && args[0] != "" {
			// 检查并提取参数
			if _, taskID := checkAndExtractSingleParam(args); taskID > 0 {

				// 获取指定刮削目录
				scrapePath := models.GetScrapePathByID(taskID)
				if scrapePath != nil {
					// 执行刮削任务
					synccron.AddNewSyncTask(taskID, synccron.SyncTaskTypeScrape)
					taskIDs = []uint{taskID}
					// 设置通知内容
					content = "目录：" + scrapePath.SourcePath + "，刮削任务已执行完毕"
				}
			}
		}

		// 如果没有指定目录，执行所有目录
		if len(taskIDs) == 0 {
			// 获取所有刮削目录
			allScrapePaths := models.GetScrapePathes()
			for _, scrapePath := range allScrapePaths {
				// 执行刮削任务
				synccron.AddNewSyncTask(scrapePath.ID, synccron.SyncTaskTypeScrape)
				taskIDs = append(taskIDs, scrapePath.ID)
			}
			// 设置通知内容
			content = "目录：全部，刮削任务已执行完毕"
		}

		// 检查是否有任务
		if len(taskIDs) == 0 {
			return
		}

		// 等待所有任务执行完成
		time.Sleep(2 * time.Second) // 等待任务队列初始化

		// 监控任务的状态
		waitForTasksCompletion(taskIDs, synccron.SyncTaskTypeScrape)

		// 所有任务执行完成，发送通知
		ctx := context.Background()
		notif := &models.Notification{
			Type:      models.SystemAlert,
			Title:     title,
			Content:   content,
			Timestamp: time.Now(),
			Priority:  models.NormalPriority,
		}
		if notificationmanager.GlobalEnhancedNotificationManager != nil {
			notificationmanager.GlobalEnhancedNotificationManager.SendNotification(ctx, notif)
		}
	}()

	// 返回开始执行的消息
	return "🔄 开始执行刮削任务"
}

// SyncStrmInc 执行增量STRM同步并在完成后发送通知
// args: 可选参数，传入同步目录ID时只同步指定目录
func SyncStrmInc(args []string) string {
	return runStrmTask(args, false)
}

// SyncStrnFull 执行全量STRM同步并在完成后发送通知
// args: 可选参数，传入同步目录ID时只同步指定目录
func SyncStrnFull(args []string) string {
	return runStrmTask(args, true)
}

// Scrape 执行刮削任务并在完成后发送通知
// args: 可选参数，传入刮削目录ID时只执行指定目录的刮削
func Scrape(args []string) string {
	return runScrapeTask(args)
}

// waitForTasksCompletion 等待指定任务完成
func waitForTasksCompletion(taskIDs []uint, taskType synccron.SyncTaskType) {
	if len(taskIDs) == 0 {
		return
	}
	allCompleted := false
	for !allCompleted {
		time.Sleep(5 * time.Second)
		allCompleted = true
		for _, taskID := range taskIDs {
			status := synccron.CheckNewTaskStatus(taskID, taskType)
			if status == synccron.TaskStatusWaiting || status == synccron.TaskStatusRunning {
				allCompleted = false
				break
			}
		}
	}
}

// runScrapeThenSync 先执行刮削任务，完成后再执行同步任务
// args: 参数格式为 #数字 #数字，分别代表刮削目录ID和同步目录ID
// 如果参数为0，则执行所有目录的操作
func runScrapeThenSync(args []string, title string) string {
	// 检查参数格式
	if errMsg, _ := checkAndExtractMoreParam(args); errMsg != "" {
		return errMsg
	}

	// 先返回开始执行的消息
	go func() {
		// 解析参数
		_, extractedIDs := checkAndExtractMoreParam(args)
		taskIDs := make([]uint, 2)
		handleAllPaths := make([]bool, 2)
		for i := range handleAllPaths {
			handleAllPaths[i] = true
		}

		for i := 0; i < 2 && i < len(extractedIDs); i++ {
			id := extractedIDs[i]
			taskIDs[i] = id
			handleAllPaths[i] = (id == 0)
		}

		// 记录任务执行情况
		var taskResults []string

		// 执行刮削任务
		{
			var currentTaskIDs []uint

			if handleAllPaths[0] {
				// 执行所有刮削目录的任务
				allScrapePaths := models.GetScrapePathes()
				for _, scrapePath := range allScrapePaths {
					// 执行刮削任务
					synccron.AddNewSyncTask(scrapePath.ID, synccron.SyncTaskTypeScrape)
					currentTaskIDs = append(currentTaskIDs, scrapePath.ID)
				}
				taskResults = append(taskResults, "目录：全部，刮削已完成")
			} else {
				// 执行指定刮削目录的任务
				scrapePath := models.GetScrapePathByID(taskIDs[0])
				if scrapePath != nil {
					// 执行刮削任务
					synccron.AddNewSyncTask(taskIDs[0], synccron.SyncTaskTypeScrape)
					currentTaskIDs = []uint{taskIDs[0]}
					taskResults = append(taskResults, "目录："+scrapePath.SourcePath+"，刮削已完成")
				}
			}

			// 等待任务开始执行
			time.Sleep(5 * time.Second)

			// 监控任务完成
			waitForTasksCompletion(currentTaskIDs, synccron.SyncTaskTypeScrape)

			// 等待上传下载任务完成
			time.Sleep(15 * time.Second)
		}

		// 执行同步任务
		{
			var currentTaskIDs []uint

			if handleAllPaths[1] {
				// 执行所有同步目录的任务
				allSyncPaths, _ := models.GetSyncPathList(1, 10000000, false)
				for _, syncPath := range allSyncPaths {
					// 同步目录
					synccron.AddNewSyncTask(syncPath.ID, synccron.SyncTaskTypeStrm)
					currentTaskIDs = append(currentTaskIDs, syncPath.ID)
				}
				taskResults = append(taskResults, "目录：全部，增量STRM同步已完成")
			} else {
				// 执行指定同步目录的任务
				syncPath := models.GetSyncPathById(taskIDs[1])
				if syncPath != nil {
					synccron.AddNewSyncTask(taskIDs[1], synccron.SyncTaskTypeStrm)
					currentTaskIDs = []uint{taskIDs[1]}
					taskResults = append(taskResults, "目录："+syncPath.RemotePath+"，增量STRM同步已完成")
				}
			}

			// 等待任务开始执行
			time.Sleep(5 * time.Second)

			// 监控任务完成
			waitForTasksCompletion(currentTaskIDs, synccron.SyncTaskTypeStrm)
		}

		// 构建通知内容
		content := ""
		for _, result := range taskResults {
			content += result + "\n"
		}
		if content == "" {
			content = "所有任务已全部执行完毕"
		}

		// 发送完成通知
		ctx := context.Background()
		notif := &models.Notification{
			Type:      models.SystemAlert,
			Title:     title,
			Content:   content,
			Timestamp: time.Now(),
			Priority:  models.NormalPriority,
		}
		if notificationmanager.GlobalEnhancedNotificationManager != nil {
			notificationmanager.GlobalEnhancedNotificationManager.SendNotification(ctx, notif)
		}
	}()

	return "🔄 开始执行任务序列"
}

// runSyncThenScrape 先执行同步任务，完成后再执行刮削任务
// args: 参数格式为 #数字 #数字，分别代表同步目录ID和刮削目录ID
// 如果参数为0，则执行所有目录的操作
func runSyncThenScrape(args []string, title string) string {
	// 检查参数格式
	if errMsg, _ := checkAndExtractMoreParam(args); errMsg != "" {
		return errMsg
	}

	// 先返回开始执行的消息
	go func() {
		// 解析参数
		_, extractedIDs := checkAndExtractMoreParam(args)
		taskIDs := make([]uint, 2)
		handleAllPaths := make([]bool, 2)
		for i := range handleAllPaths {
			handleAllPaths[i] = true
		}

		for i := 0; i < 2 && i < len(extractedIDs); i++ {
			id := extractedIDs[i]
			taskIDs[i] = id
			handleAllPaths[i] = (id == 0)
		}

		// 记录任务执行情况
		var taskResults []string

		// 执行同步任务
		{
			var currentTaskIDs []uint

			if handleAllPaths[0] {
				// 执行所有同步目录的任务
				allSyncPaths, _ := models.GetSyncPathList(1, 10000000, false)
				for _, syncPath := range allSyncPaths {
					// 同步目录
					synccron.AddNewSyncTask(syncPath.ID, synccron.SyncTaskTypeStrm)
					currentTaskIDs = append(currentTaskIDs, syncPath.ID)
				}
				taskResults = append(taskResults, "目录：全部，增量STRM同步已完成")
			} else {
				// 执行指定同步目录的任务
				syncPath := models.GetSyncPathById(taskIDs[0])
				if syncPath != nil {
					synccron.AddNewSyncTask(taskIDs[0], synccron.SyncTaskTypeStrm)
					currentTaskIDs = []uint{taskIDs[0]}
					taskResults = append(taskResults, "目录："+syncPath.RemotePath+"，增量STRM同步已完成")
				}
			}

			// 等待任务开始执行
			time.Sleep(5 * time.Second)

			// 监控任务完成
			waitForTasksCompletion(currentTaskIDs, synccron.SyncTaskTypeStrm)

			// 等待上传下载任务完成
			time.Sleep(15 * time.Second)
		}

		// 执行刮削任务
		{
			var currentTaskIDs []uint
			var hasNewScrapeFiles bool

			if handleAllPaths[1] {
				// 执行所有刮削目录的任务
				allScrapePaths := models.GetScrapePathes()
				for _, scrapePath := range allScrapePaths {
					// 刮削开始前检查是否有新文件
					newScrapeFilesCount := models.GetScannedScrapeMediaFilesTotal(scrapePath.ID, scrapePath.MediaType)
					if newScrapeFilesCount > 0 {
						hasNewScrapeFiles = true
					}
					// 执行刮削任务
					synccron.AddNewSyncTask(scrapePath.ID, synccron.SyncTaskTypeScrape)
					currentTaskIDs = append(currentTaskIDs, scrapePath.ID)
				}
				taskResults = append(taskResults, "目录：全部，刮削已完成")
			} else {
				// 执行指定刮削目录的任务
				scrapePath := models.GetScrapePathByID(taskIDs[1])
				if scrapePath != nil {
					// 刮削开始前检查是否有新文件
					newScrapeFilesCount := models.GetScannedScrapeMediaFilesTotal(scrapePath.ID, scrapePath.MediaType)
					if newScrapeFilesCount > 0 {
						hasNewScrapeFiles = true
					}
					// 执行刮削任务
					synccron.AddNewSyncTask(taskIDs[1], synccron.SyncTaskTypeScrape)
					currentTaskIDs = []uint{taskIDs[1]}
					taskResults = append(taskResults, "目录："+scrapePath.SourcePath+"，刮削已完成")
				}
			}

			// 等待任务开始执行
			time.Sleep(5 * time.Second)

			// 监控任务完成
			waitForTasksCompletion(currentTaskIDs, synccron.SyncTaskTypeScrape)

			// 刮削任务完成后，如果有新文件，触发Emby媒体库刷新
			if hasNewScrapeFiles {
				var refreshIDs []uint
				// 使用同步任务的ID（第一个任务）
				if !handleAllPaths[0] && taskIDs[0] > 0 {
					// 使用同步任务的ID
					syncPath := models.GetSyncPathById(taskIDs[0])
					if syncPath != nil {
						refreshIDs = append(refreshIDs, taskIDs[0])
						helpers.AppLogger.Infof("添加同步目录到Emby刷新列表: %s (ID: %d)", syncPath.RemotePath, taskIDs[0])
					}
				} else if handleAllPaths[0] {
					// 如果是全部同步，使用所有同步目录的ID
					allSyncPaths, _ := models.GetSyncPathList(1, 10000000, true)
					for _, syncPath := range allSyncPaths {
						refreshIDs = append(refreshIDs, syncPath.ID)
						helpers.AppLogger.Infof("添加同步目录到Emby刷新列表: %s (ID: %d)", syncPath.RemotePath, syncPath.ID)
					}
				}

				// 如果有需要刷新的目录，等待30秒后执行刷新
				if len(refreshIDs) > 0 {
					// 等待30秒，确保文件操作完成
					go func(ids []uint) {
						time.Sleep(30 * time.Second)
						// 对需要刷新的目录触发Emby媒体库刷新
						for _, taskID := range ids {
							models.RefreshEmbyLibraryBySyncPathId(taskID)
						}
					}(refreshIDs)
				}
			}
		}

		// 构建通知内容
		content := ""
		for _, result := range taskResults {
			content += result + "\n"
		}
		if content == "" {
			content = "所有任务已全部执行完毕"
		}

		// 发送完成通知
		ctx := context.Background()
		notif := &models.Notification{
			Type:      models.SystemAlert,
			Title:     title,
			Content:   content,
			Timestamp: time.Now(),
			Priority:  models.NormalPriority,
		}
		if notificationmanager.GlobalEnhancedNotificationManager != nil {
			notificationmanager.GlobalEnhancedNotificationManager.SendNotification(ctx, notif)
		}
	}()

	return "🔄 开始执行任务序列"
}

// ScrapeThenSync 先执行刮削任务，完成后再执行同步任务
// args: 参数格式为 #数字 #数字，分别代表刮削目录ID和同步目录ID
// 如果参数为0，则执行所有目录的操作
func ScrapeThenSync(args []string) string {
	return runScrapeThenSync(args, "✅ 刮削后同步完成")
}

// SyncThenScrape 先执行同步任务，完成后再执行刮削任务
// args: 参数格式为 #数字 #数字，分别代表同步目录ID和刮削目录ID
// 如果参数为0，则执行所有目录的操作
func SyncThenScrape(args []string) string {
	return runSyncThenScrape(args, "✅ 同步后刮削完成")
}

func StartListenTelegramBot() {
	mgr := notificationmanager.GlobalEnhancedNotificationManager

	myCommands := map[string]func([]string) string{
		"strm_inc":    SyncStrmInc,
		"strm_sync":   SyncStrnFull,
		"scrape":      Scrape,
		"scrape_sync": ScrapeThenSync,
		"sync_scrape": SyncThenScrape,
	}

	mgr.RegisterTelegramCommands(myCommands)
	mgr.StartAll()
}
