package emby

import (
	embyclientrestgo "Q115-STRM/internal/embyclient-rest-go"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
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

	helpers.AppLogger.Infof("[Emby同步] 开始同步，EmbyUrl=%s", config.EmbyUrl)

	client := embyclientrestgo.NewClient(config.EmbyUrl, config.EmbyApiKey)
	users, err := client.GetUsersWithAllLibrariesAccess()
	if err != nil {
		return 0, err
	}
	if len(users) == 0 {
		return 0, errors.New("没有找到可访问全部媒体库的Emby用户")
	}
	helpers.AppLogger.Infof("[Emby同步] 选中用户: %s (ID: %s)", users[0].Name, users[0].ID)

	libs, err := client.GetAllMediaLibraries()
	if err != nil {
		return 0, err
	}
	if len(libs) == 0 {
		return 0, errors.New("未获取到任何Emby媒体库")
	}
	helpers.AppLogger.Infof("[Emby同步] Emby返回媒体库列表(%d个): %v", len(libs), func() []string {
		names := make([]string, 0, len(libs))
		for _, l := range libs {
			names = append(names, fmt.Sprintf("%s(ID:%s)", l.Name, l.ID))
		}
		return names
	}())
	if err := models.UpsertEmbyLibraries(libs); err != nil {
		helpers.AppLogger.Warnf("保存媒体库信息失败: %v", err)
	}

	// 根据配置过滤媒体库
	helpers.AppLogger.Infof("[Emby同步] SyncAllLibraries=%d, SelectedLibraries=%s", config.SyncAllLibraries, config.SelectedLibraries)
	if config.SyncAllLibraries == 0 {
		var selectedLibIds []string
		if err := json.Unmarshal([]byte(config.SelectedLibraries), &selectedLibIds); err == nil {
			// 创建 ID 到库的映射
			libMap := make(map[string]embyclientrestgo.EmbyLibrary)
			for _, lib := range libs {
				libMap[lib.ID] = lib
			}

			// 只保留选中的媒体库
			filteredLibs := make([]embyclientrestgo.EmbyLibrary, 0, len(selectedLibIds))
			for _, id := range selectedLibIds {
				if lib, ok := libMap[id]; ok {
					filteredLibs = append(filteredLibs, lib)
				}
			}
			libs = filteredLibs

			// // 清理未选中的媒体库数据
			// if err := models.CleanupUnselectedEmbyLibraryData(selectedLibIds); err != nil {
			// 	helpers.AppLogger.Warnf("清理未选中媒体库数据失败: %v", err)
			// }
		} else {
			helpers.AppLogger.Warnf("解析选中的媒体库列表失败: %v", err)
		}
	}

	if len(libs) == 0 {
		helpers.AppLogger.Errorf("[Emby同步] 过滤后没有可同步的媒体库，请检查媒体库同步选择配置")
		return 0, nil
	}
	helpers.AppLogger.Infof("[Emby同步] 过滤后待同步媒体库(%d个): %v", len(libs), func() []string {
		names := make([]string, 0, len(libs))
		for _, l := range libs {
			names = append(names, fmt.Sprintf("%s(ID:%s)", l.Name, l.ID))
		}
		return names
	}())

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
		var skippedNoPickCode int64
		var savedCount int64
		var saveFailCount int64
		for task := range jobs {
			pickCode, mediaPath, err := extractPickCode(task.Item.MediaSources)
			if err != nil {
				skippedNoPickCode++
				if skippedNoPickCode <= 10 {
					helpers.AppLogger.Errorf("[Emby同步] Item被跳过(无pickcode): name=%s type=%s id=%s mediaSources=%d path=%s", task.Item.Name, task.Item.Type, task.Item.Id, len(task.Item.MediaSources), mediaPath)
				}
				continue
			}
			pathStr := mediaPath
			if pathStr == "" {
				pathStr = task.Item.Path
			}
			mediaItem := &models.EmbyMediaItem{
				ItemId:            task.Item.Id,
				ItemIdInt:         helpers.StringToInt64(task.Item.Id),
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
			// 将DateCreated转成时间戳赋值给DateCreatedTime
			if mediaItem.DateCreated != "" {
				if t, err := time.Parse(time.RFC3339, mediaItem.DateCreated); err == nil {
					mediaItem.DateCreatedTime = t.Unix()
				}
			}
			// 将DateModified转成时间戳赋值给DateModifiedTime
			if mediaItem.DateModified != "" {
				if t, err := time.Parse(time.RFC3339, mediaItem.DateModified); err == nil {
					mediaItem.DateModifiedTime = t.Unix()
				}
			}
			if err := models.CreateOrUpdateEmbyMediaItem(mediaItem); err != nil {
				saveFailCount++
				if saveFailCount <= 10 {
					helpers.AppLogger.Errorf("[Emby同步] 保存媒体项失败: id=%s name=%s err=%v", task.Item.Id, task.Item.Name, err)
				}
				continue
			}
			savedCount++
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
			time.Sleep(100 * time.Millisecond) // 休息100毫秒，避免对Emby API的过度请求，也让其他协程有机会写入数据库
		}
	}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go worker()
	}

	for _, lib := range libs {
		helpers.AppLogger.Infof("[Emby同步] 开始同步媒体库: %s (ID: %s)", lib.Name, lib.ID)
		items, gerr := client.GetMediaItemsByLibraryID(lib.ID, 0)
		if gerr != nil {
			helpers.AppLogger.Errorf("[Emby同步] 获取媒体库Items失败: library=%s (ID:%s), error=%v", lib.Name, lib.ID, gerr)
			continue
		}
		helpers.AppLogger.Infof("[Emby同步] 媒体库 %s 返回 %d 个Items", lib.Name, len(items))
		if len(items) == 0 {
			helpers.AppLogger.Errorf("[Emby同步] 媒体库 %s (ID:%s) 返回0个Items，请检查Emby中该媒体库是否有内容，以及IncludeItemTypes过滤条件", lib.Name, lib.ID)
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
	helpers.AppLogger.Infof("[Emby同步] 同步完成，总处理 %d 个项目", processed)
	if processed == 0 {
		helpers.AppLogger.Errorf("[Emby同步] ⚠️ 同步结果为0！可能原因：1)媒体库过滤后为空 2)Items的MediaSource.Path不含pickcode 3)Emby API返回空结果。请查看上方日志排查")
	}
	return int(processed), nil
}

// 增量同步item id 所属的 媒体库
func IncrementalSyncEmbyMediaItems(itemId string) error {
	// 检查是否已有任务在运行，避免并发执行
	if IsEmbySyncRunning() {
		helpers.AppLogger.Warnf("Emby同步任务已在运行，跳过本次定时执行")
		return nil
	}
	config, cerr := models.GetEmbyConfig()
	if config.EmbyUrl == "" || config.EmbyApiKey == "" {
		return errors.New("Emby Url或ApiKey为空")
	}
	if cerr != nil || config.SyncEnabled != 1 {
		return errors.New("Emby同步未启用")
	}
	if !atomic.CompareAndSwapInt32(&embySyncRunning, 0, 1) {
		return errors.New("Emby同步任务已在运行")
	}
	defer atomic.StoreInt32(&embySyncRunning, 0)

	client := embyclientrestgo.NewClient(config.EmbyUrl, config.EmbyApiKey)
	users, err := client.GetUsersWithAllLibrariesAccess()
	if err != nil {
		return err
	}
	if len(users) == 0 {
		return errors.New("没有找到可访问全部媒体库的Emby用户")
	}
	// 查询item id 所属的媒体库
	librarys, err := client.GetItemLibraryId(itemId)
	if err != nil {
		return err
	}
	if len(librarys) == 0 {
		return errors.New("没有找到可访问的媒体库")
	}
	for _, lib := range librarys {
		lastDateCreatedTime := models.GetLastItemDateCreatedTimeByLibraryID(lib.ID)
		if lastDateCreatedTime == 0 {
			helpers.AppLogger.Warnf("获取媒体库%s最后一此同步时间失败，可能是因为没有同步过任何媒体项", lib.ID)
			continue
		}
		items, gerr := client.GetMediaItemsByLibraryID(lib.ID, lastDateCreatedTime)
		if gerr != nil {
			helpers.AppLogger.Warnf("获取媒体库%s失败: %v", lib.ID, gerr)
			continue
		}
		for _, item := range items {
			pickCode, mediaPath, err := extractPickCode(item.MediaSources)
			// pickCode, mediaPath := "", ""
			if err != nil {
				continue
			}
			pathStr := mediaPath
			if pathStr == "" {
				pathStr = item.Path
			}
			mediaItem := &models.EmbyMediaItem{
				ItemId:            item.Id,
				ItemIdInt:         helpers.StringToInt64(item.Id),
				ServerId:          "",
				Name:              item.Name,
				Type:              item.Type,
				ParentId:          item.ParentId,
				SeriesId:          item.SeriesId,
				SeasonId:          item.SeasonId,
				SeasonName:        item.SeasonName,
				SeriesName:        item.SeriesName,
				LibraryId:         lib.ID,
				Path:              pathStr,
				PickCode:          pickCode,
				MediaSourcePath:   mediaPath,
				IndexNumber:       item.IndexNumber,
				ParentIndexNumber: item.ParentIndexNumber,
				ProductionYear:    item.ProductionYear,
				PremiereDate:      item.PremiereDate,
				DateCreated:       item.DateCreated,  // 2026-01-21T16:00:00.0000000Z
				DateModified:      item.DateModified, // 2026-01-21T16:00:00.0000000Z
				IsFolder:          item.IsFolder,
			}
			// 将DateCreated转成时间戳赋值给DateCreatedTime
			if item.DateCreated != "" {
				if t, err := time.Parse(time.RFC3339, item.DateCreated); err == nil {
					mediaItem.DateCreatedTime = t.Unix()
				}
			}
			// 将DateModified转成时间戳赋值给DateModifiedTime
			if item.DateModified != "" {
				if t, err := time.Parse(time.RFC3339, item.DateModified); err == nil {
					mediaItem.DateModifiedTime = t.Unix()
				}
			}
			if err := models.CreateOrUpdateEmbyMediaItem(mediaItem); err != nil {
				helpers.AppLogger.Errorf("保存Emby媒体项失败 id=%s name=%s err=%v", item.Id, item.Name, err)
				continue
			}
			if pickCode != "" {
				if sf := models.GetFileByPickCode(pickCode); sf != nil {
					if err := models.CreateEmbyMediaSyncFile(item.Id, sf.ID, pickCode, sf.SyncPathId); err != nil {
						helpers.AppLogger.Warnf("关联SyncFile失败 item=%s pickcode=%s err=%v", item.Id, pickCode, err)
					}
					models.CreateOrUpdateEmbyLibrarySyncPath(lib.ID, sf.SyncPathId, lib.Name)
				}
			}
			time.Sleep(100 * time.Millisecond) // 休息100毫秒，避免对Emby API的过度请求，也让其他协程有机会写入数据库
		}
	}

	return nil
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
