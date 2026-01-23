package embyclientrestgo

import (
	"Q115-STRM/internal/helpers"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client 是与 Emby API 交互的客户端。
type Client struct {
	embyURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient 创建一个新的 Emby API 客户端。
func NewClient(embyURL, apiKey string) *Client {
	return &Client{
		embyURL: embyURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // 添加合理的超时
		},
	}
}

// EmbyLibrary 表示 Emby 中的单个媒体库。
type EmbyLibrary struct {
	Name string `json:"Name"`
	ID   string `json:"Id"`
}

// EmbyLibrariesResponse 是 /Library/MediaFolders 端点响应的结构。
type EmbyLibrariesResponse struct {
	Items []EmbyLibrary `json:"Items"`
}

// UserPolicy represents the policy settings for a user.
type UserPolicy struct {
	// Gets or sets a value indicating whether [enable all folders].
	EnableAllFolders bool `json:"EnableAllFolders"`
}

// UserDto represents a user in Emby.
type UserDto struct {
	Name   string     `json:"Name"`
	ID     string     `json:"Id"`
	Policy UserPolicy `json:"Policy"`
}

type BaseItemDtoV2 struct {
	Name string `json:"Name,omitempty"`
	// The id.
	Id           string          `json:"Id,omitempty"`
	MediaStreams []MediaStreamV2 `json:"MediaStreams,omitempty"`
}

type MediaStreamV2 struct {
	// The codec.    Probe Field: `codec_name`    Applies to: `MediaBrowser.Model.Entities.MediaStreamType.Video`, `MediaBrowser.Model.Entities.MediaStreamType.Audio`, `MediaBrowser.Model.Entities.MediaStreamType.Subtitle`    Related Enums: `T:Emby.Media.Model.Enums.VideoMediaTypes`, `Emby.Media.Model.Enums.AudioMediaTypes`, `Emby.Media.Model.Enums.SubtitleMediaTypes`.
	Codec string `json:"Codec,omitempty"`
	Type  string `json:"Type,omitempty"`
}

type QueryResultBaseItemDto struct {
	Items            []BaseItemDtoV2 `json:"Items,omitempty"`
	TotalRecordCount int32           `json:"TotalRecordCount,omitempty"`
}

// GetAllMediaLibraries 从 Emby 服务器检索所有媒体库。
func (c *Client) GetAllMediaLibraries() ([]EmbyLibrary, error) {
	// 构造请求 URL
	url := fmt.Sprintf("%s/emby/Library/MediaFolders?api_key=%s", c.embyURL, c.apiKey)

	// 创建一个新的 HTTP 请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求时出错: %w", err)
	}

	// 设置请求头
	req.Header.Set("Accept", "application/json")

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求时出错: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("错误: 收到非 200 状态码: %d", resp.StatusCode)
	}

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体时出错: %w", err)
	}

	// 解析 JSON 响应
	var librariesResponse EmbyLibrariesResponse
	if err := json.Unmarshal(body, &librariesResponse); err != nil {
		// 尝试解析为单个项目，以应对某些 Emby 版本可能返回不同结构的情况
		var singleLibrary EmbyLibrary
		if err2 := json.Unmarshal(body, &singleLibrary); err2 == nil && singleLibrary.Name != "" {
			return []EmbyLibrary{singleLibrary}, nil
		}
		return nil, fmt.Errorf("解析 json 时出错: %w", err)
	}

	return librariesResponse.Items, nil
}

// GetMediaItemsByLibraryID 从指定的媒体库中检索所有媒体项目。
// 它会自动处理分页并为每个项目请求详细字段。
func (c *Client) GetMediaItemsByLibraryID(libraryID string) ([]BaseItemDtoV2, error) {
	const (
		limit  = 100 // 每次请求获取的项目数
		fields = "MediaStreams"
	)

	var allItems []BaseItemDtoV2
	startIndex := 0
	firstRequest := true

	// 构建基础 URL
	baseURL, err := url.Parse(fmt.Sprintf("%s/emby/Items", c.embyURL))
	if err != nil {
		return nil, fmt.Errorf("解析基础 URL 时出错: %w", err)
	}

	for {
		// 设置查询参数
		params := url.Values{}
		params.Add("ParentId", libraryID)
		params.Add("api_key", c.apiKey)
		params.Add("StartIndex", fmt.Sprintf("%d", startIndex))
		params.Add("Limit", fmt.Sprintf("%d", limit))
		params.Add("Recursive", "true")
		params.Add("IncludeItemTypes", "Movie,Video,Episode")
		params.Add("Fields", fields)
		baseURL.RawQuery = params.Encode()

		req, err := http.NewRequest("GET", baseURL.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("创建请求时出错: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		// helpers.AppLogger.Debugf("GET %s", baseURL.String())
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("发送请求时出错: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("错误: 收到非 200 状态码: %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("读取响应体时出错: %w", err)
		}

		var response QueryResultBaseItemDto
		if err := json.Unmarshal(body, &response); err != nil {
			return nil, fmt.Errorf("解析 json 时出错: %w", err)
		}

		if firstRequest {
			if response.TotalRecordCount > 0 {
				allItems = make([]BaseItemDtoV2, 0, response.TotalRecordCount)
			}
			firstRequest = false
		}

		allItems = append(allItems, response.Items...)

		// 检查是否已获取所有项目
		if len(response.Items) == 0 || len(allItems) >= int(response.TotalRecordCount) {
			break
		}

		// 准备下一页
		startIndex += len(response.Items)
	}

	return allItems, nil
}

// CheckPlaybackInfo sends a request to get playback info for a media item and checks for success.
func (c *Client) CheckPlaybackInfo(item BaseItemDtoV2, userID string) error {
	// Construct the request URL
	url := fmt.Sprintf("%s/emby/Items/%s/PlaybackInfo?api_key=%s", c.embyURL, item.Id, c.apiKey)
	// Prepare the request body
	requestBody, err := json.Marshal(map[string]string{
		"UserId": userID,
	})
	if err != nil {
		return fmt.Errorf("序列化请求体失败: %w", err)
	}

	var lastErr error

	for i := 0; i < 1; i++ {
		// Create a new HTTP POST request
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
		if err != nil {
			return fmt.Errorf("创建 POST 请求失败: %w", err) // This error is not retryable
		}

		// Set request headers
		req.Header.Set("Content-Type", "application/json")

		// Send the request
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("发送 POST 请求失败: %w", err)
			helpers.AppLogger.Errorf("第 %d 次尝试失败: %v。3秒后重试...", i+1, err)
			time.Sleep(3 * time.Second)
			continue
		}

		// Check the response status code
		if resp.StatusCode == http.StatusOK {
			helpers.AppLogger.Infof("影视剧 %s 的媒体信息请求成功 (用户: %s)", item.Name, userID)
			resp.Body.Close()
			return nil
		}

		// If status code is not OK, record the error and retry.
		lastErr = fmt.Errorf("请求失败，收到非 200 状态码: %d", resp.StatusCode)
		helpers.AppLogger.Errorf("第 %d 次尝试失败，状态码: %d。3秒后重试...", i+1, resp.StatusCode)
		resp.Body.Close() // It's important to close the body to prevent resource leaks.
		time.Sleep(3 * time.Second)
	}

	helpers.AppLogger.Errorf("1次尝试后，获取影视剧 %s (用户: %s) 的媒体信息失败。最后错误: %v", item.Name, userID, lastErr)
	return fmt.Errorf("获取媒体信息失败，重试1次后: %w", lastErr)
}

// GetUsersWithAllLibrariesAccess retrieves all users from Emby and filters for those with access to all libraries.
func (c *Client) GetUsersWithAllLibrariesAccess() ([]UserDto, error) {
	// Construct the request URL
	url := fmt.Sprintf("%s/emby/Users?api_key=%s", c.embyURL, c.apiKey)

	// Create a new HTTP request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求时出错: %w", err)
	}

	// Set request headers
	req.Header.Set("Accept", "application/json")

	// Send the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求时出错: %w", err)
	}
	defer resp.Body.Close()

	// Check the response status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("错误: 收到非 200 状态码: %d", resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体时出错: %w", err)
	}

	// Parse the JSON response
	var users []UserDto
	if err := json.Unmarshal(body, &users); err != nil {
		return nil, fmt.Errorf("解析 json 时出错: %w", err)
	}

	// Filter users who have access to all media libraries
	var usersWithAllAccess []UserDto
	for _, user := range users {
		if user.Policy.EnableAllFolders {
			usersWithAllAccess = append(usersWithAllAccess, user)
		}
	}

	return usersWithAllAccess, nil
}

// // 获取媒体详情
// func (c *Client) GetMediaItemDetails(userId, itemID string) (*BaseItemDtoV2, error) {
// 	// Construct the request URL
// 	url := fmt.Sprintf("%s/emby/Users/%s/Items/%s?api_key=%s&Fields=MediaStreams", c.embyURL, userId, itemID, c.apiKey)

// }

// 刷新所有媒体库媒体流数据
func ProcessLibraries(embyURL, apiKey string, excludeIds []string) []map[string]string {
	// 创建一个新的 Emby 客户端
	client := NewClient(embyURL, apiKey)

	libs, err := client.GetAllMediaLibraries()
	if err != nil {
		helpers.AppLogger.Errorf("获取媒体库失败%v", err)
		return nil
	}
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
	// 为了高效查找，将 excludeIds 转换为 map，同时忽略空字符串。
	excludeMap := make(map[string]struct{})
	for _, id := range excludeIds {
		if id != "" {
			excludeMap[id] = struct{}{}
		}
	}

	// 使用第一个有权限的用户
	userID := users[0].ID
	helpers.AppLogger.Infof("使用用户 '%s' (ID: %s) 来检查播放信息", users[0].Name, userID)
	sum := 0
	tasks := make([]map[string]string, 0)
	for _, lib := range libs {
		if _, exists := excludeMap[lib.ID]; exists {
			helpers.AppLogger.Infof("媒体库%s在排除列表\n", lib.Name)
			continue
		}

		items, err := client.GetMediaItemsByLibraryID(lib.ID)
		if err != nil {
			helpers.AppLogger.Errorf("获取媒体库 '%s' 中的项目失败: %v", lib.Name, err)
			continue // 继续处理下一个媒体库
		}

		helpers.AppLogger.Infof("在 '%s' 中找到 %d 个影视剧", lib.Name, len(items))
		// var wg sync.WaitGroup
		// sem := make(chan struct{}, 5) // 创建一个容量为 3 的信号量
		//处理数据量

		for _, item := range items {
			helpers.AppLogger.Infof("处理项目 %s : %s，共 %d 个媒体流", item.Id, item.Name, len(item.MediaStreams))
			nonSubtitleStreamCount := 0
			for _, stream := range item.MediaStreams {
				if stream.Type != "Subtitle" {
					nonSubtitleStreamCount++
				}
			}
			if nonSubtitleStreamCount < 2 {
				sum++
				// wg.Add(1)
				// sem <- struct{}{} // 获取一个信号量，如果通道已满则阻塞
				// go func(item BaseItemDtoV2) {
				// 	defer wg.Done()
				// 	defer func() { <-sem }() // 释放信号量
				// 检查每个媒体项目的播放信息
				url := fmt.Sprintf("%s/emby/Items/%s/PlaybackInfo?api_key=%s", embyURL, item.Id, apiKey)
				task := make(map[string]string)
				task["url"] = url
				task["item_id"] = item.Id
				task["item_name"] = item.Name
				tasks = append(tasks, task)
				// helpers.AppLogger.Infof("添加任务 %s : %s => %s 到队列", item.Id, item.Name, url)
				// err := client.CheckPlaybackInfo(item, userID)
				// if err != nil {
				// 	log.Printf("检查项目 %s 的播放信息失败: %v", item.Name, err)
				// }
				// }(item)
			}
		}
		// wg.Wait()
	}
	return tasks
}

// // 根据媒体库id刷新媒体流数据
// func ProcessLibrariesById(embyURL, apiKey, libId string) {
// 	// 创建一个新的 Emby 客户端
// 	client := NewClient(embyURL, apiKey)
// 	// 获取有权限的用户
// 	users, err := client.GetUsersWithAllLibrariesAccess()
// 	if err != nil {
// 		helpers.AppLogger.Errorf("获取用户失败: %v", err)
// 		return
// 	}
// 	if len(users) == 0 {
// 		helpers.AppLogger.Errorf("没有找到可以访问所有媒体库的用户")
// 		return
// 	}
// 	// 使用第一个有权限的用户
// 	userID := users[0].ID
// 	helpers.AppLogger.Infof("使用用户 '%s' (ID: %s) 来检查影视剧的媒体信息", users[0].Name, userID)

// 	items, err := client.GetMediaItemsByLibraryID(libId)
// 	if err != nil {
// 		helpers.AppLogger.Errorf("获取媒体库 '%s' 中的影视剧失败: %v", libId, err)
// 	}

// 	fmt.Printf("在 '%s' 中找到 %d 个媒体项:\n", libId, len(items))
// 	var wg sync.WaitGroup
// 	sem := make(chan struct{}, 5) // 创建一个容量为 3 的信号量
// 	//处理数据量
// 	sum := 0
// 	for _, item := range items {
// 		nonSubtitleStreamCount := 0
// 		for _, stream := range item.MediaStreams {
// 			if stream.Type != "Subtitle" {
// 				nonSubtitleStreamCount++
// 			}
// 		}
// 		if nonSubtitleStreamCount < 2 {
// 			sum++
// 			wg.Add(1)
// 			sem <- struct{}{} // 获取一个信号量，如果通道已满则阻塞
// 			go func(item BaseItemDtoV2) {
// 				defer wg.Done()
// 				defer func() { <-sem }() // 释放信号量
// 				// 检查每个媒体项目的播放信息
// 				err := client.CheckPlaybackInfo(item, userID)
// 				if err != nil {
// 					log.Printf("检查项目 %s 的播放信息失败: %v", item.Name, err)
// 				}
// 			}(item)
// 		}
// 	}
// 	wg.Wait()
// 	log.Printf("=========一共处理数据量%d============", sum)
// }
