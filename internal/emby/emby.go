package emby

import (
	embyclientrestgo "Q115-STRM/internal/embyclient-rest-go"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"errors"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
)

var embySyncRunning int32

// IsEmbySyncRunning 检查是否有Emby同步任务正在运行
func IsEmbySyncRunning() bool {
	return atomic.LoadInt32(&embySyncRunning) == 1
}

func SetEmbySyncRunning(running bool) {
	if running {
		atomic.StoreInt32(&embySyncRunning, 1)
	} else {
		atomic.StoreInt32(&embySyncRunning, 0)
	}
}

type embySyncTask struct {
	LibraryId   string
	LibraryName string
	Item        embyclientrestgo.BaseItemDtoV2
}

// 同步Emby媒体库到本地数据库
func PerformEmbySync() (int, error) {
	// 检查是否已有任务在运行，避免并发执行
	if IsEmbySyncRunning() {
		helpers.AppLogger.Warnf("Emby同步任务已在运行，跳过本次定时执行")
		return 0, nil
	}
	config, cerr := models.GetEmbyConfig()
	if config.EmbyUrl == "" || config.EmbyApiKey == "" {
		return 0, errors.New("Emby Url或ApiKey为空")
	}
	if cerr != nil || config.SyncEnabled != 1 {
		return 0, errors.New("Emby同步未启用")
	}
	if !atomic.CompareAndSwapInt32(&embySyncRunning, 0, 1) {
		return 0, errors.New("Emby同步任务已在运行")
	}
	defer atomic.StoreInt32(&embySyncRunning, 0)

	client := embyclientrestgo.NewClient(config.EmbyUrl, config.EmbyApiKey)
	users, err := client.GetUsersWithAllLibrariesAccess()
	if err != nil {
		return 0, err
	}
	if len(users) == 0 {
		return 0, errors.New("没有找到可访问全部媒体库的Emby用户")
	}

	libs, err := client.GetAllMediaLibraries()
	if err != nil {
		return 0, err
	}
	if len(libs) == 0 {
		return 0, errors.New("未获取到任何Emby媒体库")
	}
	if err := models.UpsertEmbyLibraries(libs); err != nil {
		helpers.AppLogger.Warnf("保存媒体库信息失败: %v", err)
	}

	// 准备并发池
	workerCount := 2
	jobs := make(chan embySyncTask, workerCount*2)
	var wg sync.WaitGroup
	var mu sync.Mutex
	validItemIds := make([]string, 0, 256)
	var processed int64
	// clientHttp := &http.Client{Timeout: 30 * time.Second}

	worker := func() {
		defer wg.Done()
		for task := range jobs {
			pickCode, mediaPath, err := extractPickCode(task.Item.MediaSources)
			// pickCode, mediaPath := "", ""
			if err != nil {
				// helpers.AppLogger.Warnf("从MediaSource中查询PickCode失败 item=%s name=%s path=%s err=%v", task.Item.Id, task.Item.Name, mediaPath, err)
				// 没有pickcode不入库
				continue
			}
			pathStr := mediaPath
			if pathStr == "" {
				pathStr = task.Item.Path
			}
			mediaItem := &models.EmbyMediaItem{
				ItemId:            task.Item.Id,
				ServerId:          "",
				Name:              task.Item.Name,
				Type:              task.Item.Type,
				ParentId:          task.Item.ParentId,
				SeriesId:          task.Item.SeriesId,
				SeasonId:          task.Item.SeasonId,
				SeasonName:        task.Item.SeasonName,
				SeriesName:        task.Item.SeriesName,
				LibraryId:         task.LibraryId,
				Path:              pathStr,
				PickCode:          pickCode,
				MediaSourcePath:   mediaPath,
				IndexNumber:       task.Item.IndexNumber,
				ParentIndexNumber: task.Item.ParentIndexNumber,
				ProductionYear:    task.Item.ProductionYear,
				PremiereDate:      task.Item.PremiereDate,
				DateCreated:       task.Item.DateCreated,
				DateModified:      task.Item.DateModified,
				IsFolder:          task.Item.IsFolder,
			}
			if err := models.CreateOrUpdateEmbyMediaItem(mediaItem); err != nil {
				helpers.AppLogger.Errorf("保存Emby媒体项失败 id=%s name=%s err=%v", task.Item.Id, task.Item.Name, err)
				continue
			}
			mu.Lock()
			validItemIds = append(validItemIds, task.Item.Id)
			mu.Unlock()
			atomic.AddInt64(&processed, 1)
			if pickCode != "" {
				if sf := models.GetFileByPickCode(pickCode); sf != nil {
					if err := models.CreateEmbyMediaSyncFile(task.Item.Id, sf.ID, pickCode, sf.SyncPathId); err != nil {
						helpers.AppLogger.Warnf("关联SyncFile失败 item=%s pickcode=%s err=%v", task.Item.Id, pickCode, err)
					}
					models.CreateOrUpdateEmbyLibrarySyncPath(task.LibraryId, sf.SyncPathId, task.LibraryName)
				}
			}
		}
	}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go worker()
	}

	for _, lib := range libs {
		items, gerr := client.GetMediaItemsByLibraryID(lib.ID)
		if gerr != nil {
			helpers.AppLogger.Warnf("获取媒体库%s失败: %v", lib.Name, gerr)
			continue
		}
		for _, item := range items {
			jobs <- embySyncTask{LibraryId: lib.ID, LibraryName: lib.Name, Item: item}
		}
	}
	close(jobs)
	wg.Wait()

	if processed > 0 {
		if err := models.CleanupOrphanedEmbyMediaItems(validItemIds); err != nil {
			helpers.AppLogger.Warnf("清理过期Emby媒体项失败: %v", err)
		}
	}
	if err := models.UpdateLastSyncTime(); err != nil {
		helpers.AppLogger.Warnf("更新Emby最后同步时间失败: %v", err)
	}
	helpers.AppLogger.Infof("Emby同步完成，处理 %d 个项目", processed)
	return int(processed), nil
}

func extractPickCode(ms []embyclientrestgo.MediaSource) (string, string, error) {
	code := ""
	pathStr := ""
	for _, src := range ms {
		code = extractPickCodeFromPath(src.Path)
		pathStr = src.Path
		if code != "" {
			return code, pathStr, nil
		}
	}
	return code, pathStr, errors.New("未从Item.MediaSource.Path中解析到pickcode")
}

func extractPickCodeFromPath(path string) string {
	if path == "" {
		return ""
	}
	// 如果不以http开头，则跳过
	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		return ""
	}
	u, err := url.Parse(path)
	if err != nil {
		return ""
	}
	if code := u.Query().Get("pickcode"); code != "" {
		return code
	}
	if code := u.Query().Get("pick_code"); code != "" {
		return code
	}
	// 检查路径是否为openlist格式，openlist中path等于pickcode, 格式为/d/{path}(?sign=xxx)
	// 判断路径是否以/d开头
	// if strings.HasPrefix(u.Path, "/d/") {
	// 	// 选取/d之后的部分作为pickcode
	// 	return path
	// }
	return path
}

var EmbyMediaInfoStart bool = false

func StartParseEmbyMediaInfo() {
	if EmbyMediaInfoStart {
		helpers.AppLogger.Info("Emby库同步任务已在运行")
		return
	}
	if models.GlobalEmbyConfig.EmbyUrl == "" || models.GlobalEmbyConfig.EmbyApiKey == "" {
		helpers.AppLogger.Info("Emby Url或ApiKey为空，无法同步emby库来提取视频信息")
		return
	}
	EmbyMediaInfoStart = true
	defer func() {
		EmbyMediaInfoStart = false
	}()
	// 放入协程运行
	go func() {
		tasks := embyclientrestgo.ProcessLibraries(models.GlobalEmbyConfig.EmbyUrl, models.GlobalEmbyConfig.EmbyApiKey, []string{})
		helpers.AppLogger.Infof("Emby库收集媒体信息已完成，共发现 %d 个影视剧需要提取媒体信息", len(tasks))
		for _, itemTask := range tasks {
			task := models.AddDownloadTaskFromEmbyMedia(itemTask["url"], itemTask["item_id"], itemTask["item_name"])
			if task == nil {
				helpers.AppLogger.Errorf("添加Emby媒体信息提取任务失败: Emby ItemID: %s, 名称: %s", itemTask["item_id"], itemTask["item_name"])
				continue
			}
			helpers.AppLogger.Infof("Emby媒体信息提取已加入操作队列: Emby ItemID: %s, 名称: %s", itemTask["item_id"], itemTask["item_name"])
		}
	}()
}

var embyUserId string = ""

// 查询Emby媒体详情
func GetEmbyItemDetail(itemId string) *embyclientrestgo.BaseItemDtoV2 {
	if models.GlobalEmbyConfig.EmbyUrl == "" || models.GlobalEmbyConfig.EmbyApiKey == "" {
		helpers.AppLogger.Info("Emby Url或ApiKey为空，无法查询Emby媒体详情")
		return nil
	}
	client := embyclientrestgo.NewClient(models.GlobalEmbyConfig.EmbyUrl, models.GlobalEmbyConfig.EmbyApiKey)
	if embyUserId == "" {
		// 获取有权限的用户
		users, err := client.GetUsersWithAllLibrariesAccess()
		if err != nil {
			helpers.AppLogger.Errorf("获取用户失败: %v", err)
			return nil
		}
		if len(users) == 0 {
			helpers.AppLogger.Errorf("没有找到可以访问所有媒体库的用户")
			return nil
		}
		// 使用第一个有权限的用户
		embyUserId = users[0].ID
	}
	item, err := client.GetItemDetailByUser(itemId, embyUserId)
	if err != nil {
		helpers.AppLogger.Errorf("获取Emby媒体 %s 用户ID %s 详情失败： %s", itemId, embyUserId, err.Error())
		return nil
	}
	return item
}
