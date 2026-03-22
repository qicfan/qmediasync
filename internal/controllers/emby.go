package controllers

import (
	"Q115-STRM/internal/emby"
	embyclientrestgo "Q115-STRM/internal/embyclient-rest-go"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/notification"
	"Q115-STRM/internal/notificationmanager"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type EmbyEvent struct {
	Title    string `json:"Title"`
	Date     string `json:"Date"`
	Event    string `json:"Event"`
	Severity string `json:"Severity"`
	Server   struct {
		Name    string `json:"Name"`
		ID      string `json:"Id"`
		Version string `json:"Version"`
	} `json:"Server"`
	Item struct {
		Name              string            `json:"Name"`
		ID                string            `json:"Id"`
		Type              string            `json:"Type"`
		IsFolder          bool              `json:"IsFolder"`
		FileName          string            `json:"FileName"`
		Path              string            `json:"Path"`
		Overview          string            `json:"Overview"`
		SeriesName        string            `json:"SeriesName"`
		SeasonName        string            `json:"SeasonName"`
		SeriesId          string            `json:"SeriesId"`
		SeasonId          string            `json:"SeasonId"`
		IndexNumber       int               `json:"IndexNumber"`
		ParentIndexNumber int               `json:"ParentIndexNumber"`
		ProductionYear    int               `json:"ProductionYear"`
		Genres            []string          `json:"Genres"`
		ImageTags         map[string]string `json:"ImageTags"`
	} `json:"Item"`
}

var refreshLibraryLock bool = false
var refreshLibraryLockMu = sync.Mutex{}

type newSeries struct {
	ID          string        // 剧的ID
	Name        string        // 剧的名称
	Seasons     map[int][]int // 季的集ID列表
	LastUpdated time.Time     // 最后更新时间
}

var newSeriesBuffer = make(map[string]newSeries)
var newSeriesBufferMu = sync.Mutex{}

// 删除事件缓冲区
var deletedSeriesBuffer = make(map[string]newSeries)
var deletedSeriesBufferMu = sync.Mutex{}

// 播放事件去重缓存
var playbackEventCache = make(map[string]time.Time)
var playbackEventCacheMu = sync.Mutex{}

// 定义一个轮询剧集的协程，如果没有启动则第一次收到通知时启动它
var newSeriesBufferTickerStarted bool = false
var newSeriesBufferTickerStartedMu = sync.Mutex{}

// Webhook Emby事件回调（公开接口）
// @Summary Emby Webhook
// @Description 接收Emby的事件回调（library.new）并触发通知/元数据提取
// @Tags Emby管理
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /emby/webhook [post]
func Webhook(ctx *gin.Context) {
	// 将请求的body内容完整打印到日志
	var body []byte
	if ctx.Request.Body != nil {
		body, _ = io.ReadAll(ctx.Request.Body)
		helpers.AppLogger.Infof("emby webhook body: %s", string(body))
	}
	if body == nil || (models.GlobalEmbyConfig != nil && (models.GlobalEmbyConfig.EmbyUrl == "" || models.GlobalEmbyConfig.EmbyApiKey == "")) {
		ctx.JSON(http.StatusOK, gin.H{
			"message": "webhook",
		})
		return
	}

	// 检查是否启用鉴权
	if models.GlobalEmbyConfig.EnableAuth == 1 {
		// 从query参数获取api_key
		apiKey := ctx.Query("api_key")
		if apiKey == "" {
			ctx.JSON(http.StatusUnauthorized, gin.H{
				"message": "Unauthorized: api_key is required",
			})
			return
		}

		// 验证API Key
		_, err := models.ValidateAPIKey(apiKey)
		if err != nil {
			helpers.AppLogger.Errorf("emby webhook api_key验证失败: %v", err)
			ctx.JSON(http.StatusUnauthorized, gin.H{
				"message": "Unauthorized: invalid api_key",
			})
			return
		}
	}

	// 处理 body内容，解析成json
	var event EmbyEvent
	// 如果解析失败，记录错误日志并返回
	err := json.Unmarshal(body, &event)
	if err != nil {
		helpers.AppLogger.Errorf("emby webhook bind json error: %v", err)
		ctx.JSON(http.StatusOK, gin.H{
			"message": "webhook",
		})
		return
	}
	if event.Event == "library.new" {
		// 新入库通知
		// 如果是Episode就先存起来，等待10s，如果后续有通series的library.new事件就合并通知
		// 触发通知
		go func() {
			if event.Item.Type == "Episode" {
				addItemToEpisodeBuffer(event.Item.SeriesId, event.Item.ParentIndexNumber, event.Item.IndexNumber)
				return
			}
			if event.Item.Type == "Movie" {
				sendNewMovieNotification(event.Item.ID)
			}

		}()
		if event.Item.Type == "Movie" || event.Item.Type == "Episode" {
			// 触发媒体信息提取
			if models.GlobalEmbyConfig != nil && models.GlobalEmbyConfig.EnableExtractMediaInfo == 1 {
				go func() {
					// 获取Emby地址和Emby Api Key
					url := fmt.Sprintf("%s/emby/Items/%s/PlaybackInfo?api_key=%s", models.GlobalEmbyConfig.EmbyUrl, event.Item.ID, models.GlobalEmbyConfig.EmbyApiKey)
					models.AddDownloadTaskFromEmbyMedia(url, event.Item.ID, event.Item.Name)
					if err != nil {
						helpers.AppLogger.Errorf("触发Emby信息提取失败 错误: %v", err)
					}
				}()
				// 触发自动刮削（延迟30秒）
				go emby.TriggerAutoScrape(event.Item.ID, event.Item.Name)
			} else {
				helpers.AppLogger.Infof("Emby媒体信息提取功能未启用，跳过媒体信息提取")
			}
		}
		// 1分钟后同步一次Emby媒体库
		go func() {
			refreshLibraryLockMu.Lock()
			if refreshLibraryLock {
				refreshLibraryLockMu.Unlock()
				return
			}
			refreshLibraryLock = true
			refreshLibraryLockMu.Unlock()
			defer func() {
				refreshLibraryLockMu.Lock()
				refreshLibraryLock = false
				refreshLibraryLockMu.Unlock()
			}()
			time.Sleep(1 * time.Minute)
			emby.PerformEmbySync()
		}()
	}
	if event.Event == "library.deleted" {
		// 删除媒体通知
		if helpers.IsRelease {
			helpers.AppLogger.Infof("Emby媒体已删除 %+v", event.Item)
		}
		// 触发通知
		// 删除消息也应该按照新入库消息一样对剧集进行分组
		go func() {
			if event.Item.Type == "Episode" {
				addItemToDeletedEpisodeBuffer(event.Item.SeriesId, event.Item.ParentIndexNumber, event.Item.IndexNumber, event.Item.SeriesName)
				return
			}
			if event.Item.Type == "Movie" {
				sendDeletedMovieNotification(event.Item.ID, event.Item.Name)
			}
		}()
		if event.Item.Type == "Movie" || event.Item.Type == "Episode" || event.Item.Type == "Season" || event.Item.Type == "Series" {
			// 触发联动删除
			if models.GlobalEmbyConfig != nil && models.GlobalEmbyConfig.EnableDeleteNetdisk == 1 {
				// 检查是否允许删除媒体库
				// if !models.IsDeleteNetdiskLibraryEnabled(event.) {
				// 	helpers.AppLogger.Infof("Emby媒体库 %s 未配置允许删除，跳过删除", event.Item.LibraryId)
				// 	return
				// }
				switch event.Item.Type {
				case "Movie":
					// 电影：在网盘中将视频文件的父目录一起删除
					// 查找Item.Id对应的SyncFileId
					models.DeleteNetdiskMovieByEmbyItemId(event.Item.ID)
				case "Episode":
					// 集：删除视频文件+元数据（nfo、封面)
					// 查找Item.Id对应的SyncFileId
					models.DeleteNetdiskEpisodeByEmbyItemId(event.Item.ID)
				case "Season":
					// 季：先检查视频文件的父目录，如果父目录是季文件夹则删除该文件夹；如果父目录是有tvshow的目录则仅删除季下所有集对应的视频文件+元数据（nfo、封面)
					// 查找EmbyMediaItem.SeasonId = item.Id的记录，取其中一条的ItemId对应的SyncFileId的SyncFile.Path作为季目录来处理
					models.DeleteNetdiskSeasonByItemId(event.Item.ID)
				case "Series":
					// 剧：在网盘中将tvshow.nfo的父目录删除
					// 查找EmbyMediaItem.SeriesId = item.Id的记录，取其中一条的ItemId对应的SyncFileId的SyncFile.Path作为季目录来处理
					models.DeleteNetdiskTvshowByItemId(event.Item.ID)
				default:
				}
			}
		}
	}
	// 处理播放事件（playback.start、playback.pause、playback.stop）
	if event.Event == "playback.start" || event.Event == "playback.pause" || event.Event == "playback.stop" {
		go handlePlaybackEvent(body, event)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "webhook",
	})
}

func addItemToEpisodeBuffer(seriesId string, seasonNumber, episodeNumber int) {
	newSeriesBufferMu.Lock()
	defer newSeriesBufferMu.Unlock()
	if _, exists := newSeriesBuffer[seriesId]; !exists {
		newSeriesBuffer[seriesId] = newSeries{
			ID:          seriesId,
			Seasons:     make(map[int][]int),
			LastUpdated: time.Now(),
		}
	}
	series := newSeriesBuffer[seriesId]
	if _, exists := series.Seasons[seasonNumber]; !exists {
		series.Seasons[seasonNumber] = make([]int, 0)
	}
	series.Seasons[seasonNumber] = append(series.Seasons[seasonNumber], episodeNumber)
	series.LastUpdated = time.Now()
	newSeriesBuffer[seriesId] = series
	helpers.AppLogger.Infof("已将剧集添加到新剧集缓冲区 seriesID=%s season=%d episode=%d", seriesId, seasonNumber, episodeNumber)
	// 启动轮询协程
	newSeriesBufferTickerStartedMu.Lock()
	defer newSeriesBufferTickerStartedMu.Unlock()
	if !newSeriesBufferTickerStarted {
		newSeriesBufferTickerStarted = true
		go startNewSeriesBufferTicker()
	}
}

func addItemToDeletedEpisodeBuffer(seriesId string, seasonNumber, episodeNumber int, seriesName string) {
	deletedSeriesBufferMu.Lock()
	defer deletedSeriesBufferMu.Unlock()
	if _, exists := deletedSeriesBuffer[seriesId]; !exists {
		deletedSeriesBuffer[seriesId] = newSeries{
			ID:          seriesId,
			Name:        seriesName,
			Seasons:     make(map[int][]int),
			LastUpdated: time.Now(),
		}
	}
	series := deletedSeriesBuffer[seriesId]
	if _, exists := series.Seasons[seasonNumber]; !exists {
		series.Seasons[seasonNumber] = make([]int, 0)
	}
	series.Seasons[seasonNumber] = append(series.Seasons[seasonNumber], episodeNumber)
	series.LastUpdated = time.Now()
	deletedSeriesBuffer[seriesId] = series
	helpers.AppLogger.Infof("已将剧集添加到删除剧集缓冲区 seriesID=%s season=%d episode=%d", seriesId, seasonNumber, episodeNumber)
	// 启动轮询协程
	newSeriesBufferTickerStartedMu.Lock()
	defer newSeriesBufferTickerStartedMu.Unlock()
	if !newSeriesBufferTickerStarted {
		newSeriesBufferTickerStarted = true
		go startNewSeriesBufferTicker()
	}
}

// TestAddItemToEpisodeBuffer 测试addItemToEpisodeBuffer函数
func TestAddItemToEpisodeBuffer() {
	// 清空缓冲区
	newSeriesBufferMu.Lock()
	newSeriesBuffer = make(map[string]newSeries)
	newSeriesBufferMu.Unlock()

	// 测试添加第一个剧集
	seriesId := "64647"
	addItemToEpisodeBuffer(seriesId, 1, 9)
	addItemToEpisodeBuffer(seriesId, 1, 8)
	addItemToEpisodeBuffer(seriesId, 1, 5)
	addItemToEpisodeBuffer(seriesId, 1, 4)
	addItemToEpisodeBuffer(seriesId, 1, 3)
	addItemToEpisodeBuffer(seriesId, 1, 1)
	time.Sleep(3 * time.Second)
	addItemToEpisodeBuffer(seriesId, 2, 1)
	addItemToEpisodeBuffer(seriesId, 2, 2)
	addItemToEpisodeBuffer(seriesId, 2, 3)
}

func startNewSeriesBufferTicker() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		<-ticker.C
		helpers.AppLogger.Infof("检查剧集缓冲区，新增缓冲区大小=%d，删除缓冲区大小=%d", len(newSeriesBuffer), len(deletedSeriesBuffer))
		now := time.Now()

		// 处理新增缓冲区
		for _, series := range newSeriesBuffer {
			helpers.AppLogger.Infof("检查新增剧集 seriesID=%s 最后更新时间=%s", series.ID, series.LastUpdated.Format("2006-01-02 15:04:05"))
			if now.Sub(series.LastUpdated) >= 10*time.Second {
				helpers.AppLogger.Infof("新剧集缓冲区达到触发时间，发送入库通知 seriesID=%s 季数=%d", series.ID, len(series.Seasons))
				// 触发通知
				go sendNewSeriesNotification(series.ID, series.Seasons)
				// 从缓冲区删除，锁定
				delete(newSeriesBuffer, series.ID)
			} else {
				// 还没到时间，继续等待
				helpers.AppLogger.Infof("等待更多剧集入库通知 seriesID=%s 已缓存季数=%d", series.ID, len(series.Seasons))
			}
		}

		// 处理删除缓冲区
		for _, series := range deletedSeriesBuffer {
			helpers.AppLogger.Infof("检查删除剧集 seriesID=%s 最后更新时间=%s", series.ID, series.LastUpdated.Format("2006-01-02 15:04:05"))
			if now.Sub(series.LastUpdated) >= 10*time.Second {
				helpers.AppLogger.Infof("删除剧集缓冲区达到触发时间，发送删除通知 seriesID=%s 季数=%d", series.ID, len(series.Seasons))
				// 触发通知
				go sendDeletedSeriesNotification(series.ID, series.Name, series.Seasons)
				// 从缓冲区删除，锁定
				delete(deletedSeriesBuffer, series.ID)
			} else {
				// 还没到时间，继续等待
				helpers.AppLogger.Infof("等待更多剧集删除通知 seriesID=%s 已缓存季数=%d", series.ID, len(series.Seasons))
			}
		}

		// 检查是否还有数据需要处理，如果没有则退出协程
		if len(newSeriesBuffer) == 0 && len(deletedSeriesBuffer) == 0 {
			helpers.AppLogger.Infof("剧集缓冲区已清空，停止轮询协程")
			newSeriesBufferTickerStartedMu.Lock()
			newSeriesBufferTickerStarted = false
			newSeriesBufferTickerStartedMu.Unlock()
			return
		}
	}
}

var notificationTemplate = `
{{title}} ({{year}})

🆔 评分: {{rate}}
🎬 类型: {{genes}}
👤 主演: {{actors}}
⏰ 入库时间: {{addedTime}}

📝 简介
{{overview}}
`

// 发送新电影消息
func sendNewMovieNotification(itemId string) {
	detail := emby.GetEmbyItemDetail(itemId)
	if detail == nil {
		helpers.AppLogger.Errorf("获取Emby媒体 %s 详情失败，无法发送新电影通知", itemId)
		return
	}
	// 使用变量格式化通知内容
	content := strings.ReplaceAll(notificationTemplate, "{{title}}", detail.Name)
	content = strings.ReplaceAll(content, "{{year}}", fmt.Sprintf("%d", detail.ProductionYear))
	content = strings.ReplaceAll(content, "{{rate}}", fmt.Sprintf("%.1f", detail.CommunityRating))
	// 拼接流派
	if len(detail.Genres) == 0 {
		content = strings.ReplaceAll(content, "{{genes}}", "暂无数据")
	} else {
		genes := strings.Join(detail.Genres, ", ")
		content = strings.ReplaceAll(content, "{{genes}}", genes)
	}
	// 拼接主演
	actors := ""
	if len(detail.People) > 0 {
		actorNames := make([]string, 0)
		// 计数
		actorCount := 0
		for _, person := range detail.People {
			if person.Type == "Actor" {
				actorNames = append(actorNames, person.Name)
				actorCount++
			}
			if actorCount >= 5 {
				break
			}
		}
		actors = strings.Join(actorNames, ", ")
	} else {
		actors = "暂无数据"
	}
	content = strings.ReplaceAll(content, "{{actors}}", actors)
	// 通过格式化detail.DateCreated字段得到入库时间，格式：2025-12-10T16:00:00.0000000Z
	addedTime := time.Now().Format("2006-01-02 15:04:05")
	if detail.DateCreated != "" {
		if parsedTime, err := time.Parse(time.RFC3339, detail.DateCreated); err == nil {
			addedTime = parsedTime.Format("2006-01-02 15:04:05")
		}
	}
	content = strings.ReplaceAll(content, "{{addedTime}}", addedTime)
	// 简介
	overview := detail.Overview
	if overview == "" {
		overview = "暂无简介"
	}
	content = strings.ReplaceAll(content, "{{overview}}", overview)
	// seasonepisodes占位符替换为空
	content = strings.ReplaceAll(content, "{{seasonepisodes}}", "")
	helpers.AppLogger.Infof("已格式化完成通知内容 movieId=%s\n%s", itemId, content)
	sendNewItemNotification(content, detail, "电影")
}

func sendNewSeriesNotification(seriesId string, seasons map[int][]int) {
	detail := emby.GetEmbyItemDetail(seriesId)
	if detail == nil {
		helpers.AppLogger.Errorf("获取Emby媒体 %s 详情失败，无法发送新剧集通知", seriesId)
		return
	}
	// 使用变量格式化通知内容
	content := strings.ReplaceAll(notificationTemplate, "{{title}}", detail.Name)
	content = strings.ReplaceAll(content, "{{year}}", fmt.Sprintf("%d", detail.ProductionYear))
	if detail.CommunityRating > 0 {
		content = strings.ReplaceAll(content, "{{rate}}", fmt.Sprintf("%.1f", detail.CommunityRating))
	} else {
		content = strings.ReplaceAll(content, "{{rate}}", "暂无数据")
	}
	// 拼接流派
	if len(detail.Genres) == 0 {
		content = strings.ReplaceAll(content, "{{genes}}", "暂无数据")
	} else {
		genes := strings.Join(detail.Genres, ", ")
		content = strings.ReplaceAll(content, "{{genes}}", genes)
	}

	// 拼接主演
	actors := ""
	if len(detail.People) > 0 {
		actorNames := make([]string, 0)
		// 计数
		actorCount := 0
		for _, person := range detail.People {
			if person.Type == "Actor" {
				actorNames = append(actorNames, person.Name)
				actorCount++
			}
			if actorCount >= 5 {
				break
			}
		}
		actors = strings.Join(actorNames, ", ")
		content = strings.ReplaceAll(content, "{{actors}}", actors)
	} else {
		content = strings.ReplaceAll(content, "{{actors}}", "暂无数据")
	}

	// 入库时间
	addedTime := time.Now().Format("2006-01-02 15:04:05")
	content = strings.ReplaceAll(content, "{{addedTime}}", addedTime)
	// 简介
	overview := detail.Overview
	if overview == "" {
		overview = "暂无简介"
	}
	content = strings.ReplaceAll(content, "{{overview}}", overview)
	// 拼接季集信息,格式：S1E1-E3; S2E1,E5
	seasonEpisodes := formatSeasonEpisodes(seasons)
	if seasonEpisodes != "" {
		seasonEpisodes = fmt.Sprintf("📺 入库季集: %s\n", seasonEpisodes)
	}
	content = strings.ReplaceAll(content, "⏰ 入库时间:", fmt.Sprintf("%s\n⏰ 入库时间: ", seasonEpisodes))
	sendNewItemNotification(content, detail, "电视剧")
}

func sendNewItemNotification(content string, detail *embyclientrestgo.BaseItemDtoV2, mediaType string) {
	imagePath := ""
	if detail.ImageTags != nil {
		imageUrl := ""
		// 检查是否有backdrop或者banner
		if tag, ok := detail.ImageTags["backdrop"]; ok {
			imageUrl = fmt.Sprintf("%s/emby/Items/%s/Images/Backdrop?tag=%s&api_key=%s", models.GlobalEmbyConfig.EmbyUrl, detail.Id, tag, models.GlobalEmbyConfig.EmbyApiKey)
		} else if tag, ok := detail.ImageTags["Primary"]; ok {
			imageUrl = fmt.Sprintf("%s/emby/Items/%s/Images/Primary?tag=%s&api_key=%s", models.GlobalEmbyConfig.EmbyUrl, detail.Id, tag, models.GlobalEmbyConfig.EmbyApiKey)
		}
		if imageUrl != "" {
			// 将图片下载/tmp目录，作为通知图片
			posterPath := filepath.Join(os.TempDir(), fmt.Sprintf("%s.jpg", detail.Id))
			derr := helpers.DownloadFile(imageUrl, posterPath, "Q115-STRM")
			if derr != nil {
				helpers.AppLogger.Errorf("下载Emby海报失败: %v", derr)
			} else {
				imagePath = posterPath
			}
		}
	}
	notif := &models.Notification{
		Type:      models.MediaAdded,
		Title:     fmt.Sprintf("📚 Emby %s 入库通知", mediaType),
		Content:   content,
		Timestamp: time.Now(),
		Priority:  models.NormalPriority,
	}
	if imagePath != "" {
		notif.Image = imagePath
	}
	if notificationmanager.GlobalEnhancedNotificationManager != nil {
		if err := notificationmanager.GlobalEnhancedNotificationManager.SendNotification(context.Background(), notif); err != nil {
			helpers.AppLogger.Errorf("发送媒体入库通知失败: %v", err)
		}
	}
	// 删除临时图片文件
	if imagePath != "" {
		os.Remove(imagePath)
	}
}

// 发送删除电影通知
func sendDeletedMovieNotification(itemId, itemName string) {
	content := fmt.Sprintf("电影名称：%s\n⏰ 删除时间: %s", itemName, time.Now().Format("2006-01-02 15:04:05"))
	notif := &models.Notification{
		Type:      models.MediaRemoved,
		Title:     "🗑️ Emby媒体删除通知",
		Content:   content,
		Timestamp: time.Now(),
		Priority:  models.NormalPriority,
	}
	if notificationmanager.GlobalEnhancedNotificationManager != nil {
		if err := notificationmanager.GlobalEnhancedNotificationManager.SendNotification(context.Background(), notif); err != nil {
			helpers.AppLogger.Errorf("发送媒体删除 %s => %s通知失败: %v", itemId, itemName, err)
		}
	}
}

// 发送删除剧集分组通知
func sendDeletedSeriesNotification(seriesId string, seriesName string, seasons map[int][]int) {
	// 拼接季集信息,格式：S1E1-E3; S2E1,E5
	seasonEpisodes := formatSeasonEpisodes(seasons)

	content := fmt.Sprintf("电视剧名称：%s\n删除季集：%s\n⏰ 删除时间: %s", seriesName, seasonEpisodes, time.Now().Format("2006-01-02 15:04:05"))
	notif := &models.Notification{
		Type:      models.MediaRemoved,
		Title:     "🗑️ Emby媒体删除通知",
		Content:   content,
		Timestamp: time.Now(),
		Priority:  models.NormalPriority,
	}
	if notificationmanager.GlobalEnhancedNotificationManager != nil {
		if err := notificationmanager.GlobalEnhancedNotificationManager.SendNotification(context.Background(), notif); err != nil {
			helpers.AppLogger.Errorf("发送媒体删除通知失败: %s (%s) 错误:%v", seriesId, seriesName, err)
		}
	}
}

func formatSeasonEpisodes(seasons map[int][]int) string {
	if len(seasons) == 0 {
		return ""
	}

	seasonNumbers := make([]int, 0, len(seasons))
	for seasonNumber := range seasons {
		seasonNumbers = append(seasonNumbers, seasonNumber)
	}
	sort.Ints(seasonNumbers)

	seasonStrArr := make([]string, 0, len(seasons))
	for _, seasonNumber := range seasonNumbers {
		episodes := seasons[seasonNumber]
		if len(episodes) == 0 {
			continue
		}
		// 去重处理，避免同一集多次触发事件导致重复显示
		episodes = removeDuplicates(episodes)

		sort.Ints(episodes)
		seasonStr := fmt.Sprintf("S%d", seasonNumber)

		start := episodes[0]
		prev := episodes[0]
		for i := 1; i < len(episodes); i++ {
			if episodes[i] != prev+1 {
				if start == prev {
					seasonStr += fmt.Sprintf("E%d, ", start)
				} else {
					seasonStr += fmt.Sprintf("E%d-E%d, ", start, prev)
				}
				start = episodes[i]
			}
			prev = episodes[i]
		}
		if start == prev {
			seasonStr += fmt.Sprintf("E%d, ", start)
		} else {
			seasonStr += fmt.Sprintf("E%d-E%d, ", start, prev)
		}

		seasonStr = strings.TrimSuffix(seasonStr, ", ")
		seasonStrArr = append(seasonStrArr, seasonStr)
	}

	return strings.Join(seasonStrArr, "; ")
}

func removeDuplicates(episodes []int) []int {
	seen := make(map[int]struct{})
	result := make([]int, 0, len(episodes))
	for _, ep := range episodes {
		if _, exists := seen[ep]; !exists {
			seen[ep] = struct{}{}
			result = append(result, ep)
		}
	}
	return result
}

// handlePlaybackEvent 处理 Emby 播放事件
func handlePlaybackEvent(body []byte, event EmbyEvent) {
	// 解析完整的播放事件数据
	var playbackWebhook models.EmbyPlaybackWebhook
	if err := json.Unmarshal(body, &playbackWebhook); err != nil {
		helpers.AppLogger.Errorf("解析播放事件失败: %v", err)
		return
	}

	// 检查去重（1分钟内不重复通知）
	cacheKey := fmt.Sprintf("%s_%s_%s_%s_%s",
		playbackWebhook.GetUserID(),
		playbackWebhook.Item.Type,
		playbackWebhook.Item.Name,
		playbackWebhook.GetDeviceName(),
		playbackWebhook.Event,
	)

	playbackEventCacheMu.Lock()
	if lastTime, exists := playbackEventCache[cacheKey]; exists {
		if time.Since(lastTime) < 1*time.Minute {
			helpers.AppLogger.Infof("播放事件去重跳过: %s (%v前)", cacheKey, time.Since(lastTime))
			playbackEventCacheMu.Unlock()
			return
		}
	}
	playbackEventCache[cacheKey] = time.Now()

	// 清理过期的缓存项（清理超过5分钟的缓存）
	for key, timestamp := range playbackEventCache {
		if time.Since(timestamp) > 5*time.Minute {
			delete(playbackEventCache, key)
		}
	}
	playbackEventCacheMu.Unlock()

	// 构造并发送通知
	notif := createPlaybackNotification(&playbackWebhook)
	imagePath := notif.Image // 保存图片路径以便后续清理
	if notificationmanager.GlobalEnhancedNotificationManager != nil {
		if err := notificationmanager.GlobalEnhancedNotificationManager.SendNotification(context.Background(), notif); err != nil {
			helpers.AppLogger.Errorf("发送播放通知失败: %v", err)
		}
	}

	// 删除临时图片文件
	if imagePath != "" {
		os.Remove(imagePath)
	}
}

// createPlaybackNotification 构造播放通知
func createPlaybackNotification(webhook *models.EmbyPlaybackWebhook) *notification.Notification {
	// 构造通知内容
	title := fmt.Sprintf("%s %s %s ", webhook.GetEventTypeEmoji(), webhook.GetEventTypeName(), webhook.Item.Name)
	content := formatPlaybackNotificationContent(webhook)

	// 下载海报图片（如果有）
	imagePath := ""
	if webhook.Item.ImageTags != nil {
		if tag, ok := webhook.Item.ImageTags["Primary"]; ok {
			imageUrl := fmt.Sprintf("%s/emby/Items/%s/Images/Primary?tag=%s&api_key=%s",
				models.GlobalEmbyConfig.EmbyUrl,
				webhook.Item.ID,
				tag,
				models.GlobalEmbyConfig.EmbyApiKey)
			posterPath := filepath.Join(os.TempDir(), fmt.Sprintf("%s_playback.jpg", webhook.Item.ID))
			derr := helpers.DownloadFile(imageUrl, posterPath, "QMediaSync")
			if derr != nil {
				helpers.AppLogger.Errorf("下载Emby海报失败: %v", derr)
			} else {
				imagePath = posterPath
			}
		}
	}

	// 构造通知元数据
	metadata := map[string]interface{}{}
	playbackDuration := webhook.GetPlaybackDuration()
	if playbackDuration > 0 {
		metadata["观看时长"] = models.FormatPlaybackDuration(playbackDuration)
	}

	notif := &notification.Notification{
		Type:      notification.NotificationType(webhook.GetNotificationEventType()),
		Title:     title,
		Content:   content,
		Metadata:  metadata,
		Timestamp: time.Now(),
		Priority:  notification.NormalPriority,
	}

	// 如果有图片，添加到通知
	if imagePath != "" {
		notif.Image = imagePath
	}

	return notif
}

// formatPlaybackNotificationContent 格式化播放通知内容
func formatPlaybackNotificationContent(webhook *models.EmbyPlaybackWebhook) string {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "用户：%s\n", webhook.GetUserName())
	fmt.Fprintf(&buf, "设备：%s (%s)\n", webhook.GetDeviceName(), webhook.GetClientName())
	// buf.WriteString(webhook.Item.Name)
	if webhook.Item.Type == "Episode" {
		fmt.Fprintf(&buf, "电视剧：%s\n", webhook.Item.SeriesName)
		fmt.Fprintf(&buf, "季集：S%dE%d\n", webhook.Item.SeasonNumber, webhook.Item.EpisodeNumber)
	}

	return buf.String()
}
