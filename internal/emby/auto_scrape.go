package emby

import (
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/scrape"
	"time"
)

// TriggerAutoScrape 触发自动刮削任务
// itemId: Emby媒体项ID
// itemName: Emby媒体项名称（用于日志）
func TriggerAutoScrape(itemId, itemName string) {
	helpers.AppLogger.Infof("[AutoScrape] 触发自动刮削 - Emby Item ID: %s, 名称: %s", itemId, itemName)

	// 延迟30秒后执行
	time.AfterFunc(30*time.Second, func() {
		executeAutoScrape(itemId, itemName)
	})
}

// executeAutoScrape 执行自动刮削任务
func executeAutoScrape(itemId, itemName string) {
	helpers.AppLogger.Infof("[AutoScrape] 开始执行自动刮削 - Emby Item ID: %s, 名称: %s", itemId, itemName)

	// 1. 查询 Emby 媒体同步文件关联
	embyItemIdInt := helpers.StringToInt(itemId)
	var embyMediaSyncFile models.EmbyMediaSyncFile
	if err := db.Db.Where("emby_item_id = ?", uint(embyItemIdInt)).First(&embyMediaSyncFile).Error; err != nil {
		helpers.AppLogger.Warnf("[AutoScrape] 未找到 Emby Item ID %s 关联的同步文件，跳过自动刮削: %v", itemId, err)
		return
	}

	// 2. 查询同步文件信息
	syncFile := models.GetSyncFileById(embyMediaSyncFile.SyncFileId)
	if syncFile == nil {
		helpers.AppLogger.Warnf("[AutoScrape] 未找到同步文件 ID %d，跳过自动刮削", embyMediaSyncFile.SyncFileId)
		return
	}

	// 3. 查询关联的刮削路径
	scrapePaths := models.GetScrapePathsBySyncPathID(embyMediaSyncFile.SyncPathId)
	if len(scrapePaths) == 0 {
		helpers.AppLogger.Warnf("[AutoScrape] 同步路径 ID %d 未关联任何刮削路径，跳过自动刮削", embyMediaSyncFile.SyncPathId)
		return
	}

	helpers.AppLogger.Infof("[AutoScrape] 查询到 %d 个关联的刮削路径", len(scrapePaths))

	// 4. 遍历刮削路径，创建刮削任务
	for _, scrapePath := range scrapePaths {
		go createAndExecuteScrapeTask(itemId, itemName, syncFile, scrapePath)
	}
}

// createAndExecuteScrapeTask 创建并执行刮削任务
func createAndExecuteScrapeTask(itemId, itemName string, syncFile *models.SyncFile, scrapePath *models.ScrapePath) {
	helpers.AppLogger.Infof("[AutoScrape] 开始处理刮削路径 - Emby Item ID: %s, 刮削路径: %s, 刮削方式: %s",
		itemId, scrapePath.SourcePath, scrapePath.ScrapeType)

	// 1. 检查刮削路径是否正在运行
	if scrapePath.IsRunning() {
		helpers.AppLogger.Warnf("[AutoScrape] 刮削路径 %s 正在运行，跳过自动刮削", scrapePath.SourcePath)
		return
	}

	// 2. 检查 STRM 文件是否存在
	if syncFile == nil {
		helpers.AppLogger.Warnf("[AutoScrape] STRM 文件不存在，跳过自动刮削 - Emby Item ID: %s", itemId)
		return
	}

	// 3. 检查是否已经刮削过
	var existingScrapeFile models.ScrapeMediaFile
	err := db.Db.Where("source_path = ? AND video_filename = ?",
		syncFile.Path, syncFile.FileName).First(&existingScrapeFile).Error
	if err == nil && existingScrapeFile.Status != models.ScrapeMediaStatusUnscanned {
		helpers.AppLogger.Infof("[AutoScrape] 文件已刮削过，跳过 - Emby Item ID: %s, 文件: %s, 状态: %s",
			itemId, syncFile.FileName, existingScrapeFile.Status)
		return
	}

	// 4. 触发刮削流程（扫描整个目录，刮削系统会自动跳过已刮削的文件）
	helpers.AppLogger.Infof("[AutoScrape] 触发刮削流程 - Emby Item ID: %s, 刮削路径: %s",
		itemId, scrapePath.SourcePath)

	s := scrape.NewScrape(scrapePath)
	success := s.Start()

	if success {
		helpers.AppLogger.Infof("[AutoScrape] 刮削成功 - Emby Item ID: %s, 文件: %s", itemId, syncFile.FileName)
	} else {
		helpers.AppLogger.Errorf("[AutoScrape] 刮削失败 - Emby Item ID: %s, 文件: %s", itemId, syncFile.FileName)
	}
}
