package embyclientrestgo

import (
	"Q115-STRM/internal/helpers"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
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

type PersonDto struct {
	ID   string `json:"Id,omitempty"`
	Name string `json:"Name,omitempty"`
	Role string `json:"Role,omitempty"`
	Type string `json:"Type,omitempty"` // e.g., Actor, Director
}

type BaseItemDtoV2 struct {
	Name string `json:"Name,omitempty"`
	// The id.
	Id                string            `json:"Id,omitempty"`
	MediaStreams      []MediaStreamV2   `json:"MediaStreams,omitempty"`
	Type              string            `json:"Type,omitempty"`
	ParentId          string            `json:"ParentId,omitempty"`
	SeriesId          string            `json:"SeriesId,omitempty"`
	SeriesName        string            `json:"SeriesName,omitempty"`
	SeasonId          string            `json:"SeasonId,omitempty"`
	SeasonName        string            `json:"SeasonName,omitempty"`
	Path              string            `json:"Path,omitempty"`
	IndexNumber       int               `json:"IndexNumber,omitempty"`
	ParentIndexNumber int               `json:"ParentIndexNumber,omitempty"`
	ProductionYear    int               `json:"ProductionYear,omitempty"`
	PremiereDate      string            `json:"PremiereDate,omitempty"`
	DateCreated       string            `json:"DateCreated,omitempty"`
	DateModified      string            `json:"DateModified,omitempty"`
	IsFolder          bool              `json:"IsFolder,omitempty"`
	MediaSources      []MediaSource     `json:"MediaSources,omitempty"`
	CommunityRating   float64           `json:"CommunityRating,omitempty"`
	Genres            []string          `json:"Genres,omitempty"`
	People            []PersonDto       `json:"People,omitempty"`
	Overview          string            `json:"Overview,omitempty"`
	ImageTags         map[string]string `json:"ImageTags,omitempty"`
}

type MediaSource struct {
	Path string `json:"Path,omitempty"`
	Name string `json:"Name,omitempty"`
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

type AncestorDto struct {
	ID       string `json:"Id,omitempty"`
	Name     string `json:"Name,omitempty"`
	Path     string `json:"Path,omitempty"`
	FileName string `json:"FileName,omitempty"`
	IsFolder bool   `json:"IsFolder,omitempty"`
	ParentId string `json:"ParentId,omitempty"`
	Type     string `json:"Type,omitempty"`
}

type VirtualFolderDto struct {
	ID                 string   `json:"Id,omitempty"`
	ItemId             string   `json:"ItemId,omitempty"`
	Guid               string   `json:"Guid,omitempty"`
	PrimaryImageItemId string   `json:"PrimaryImageItemId,omitempty"`
	PrimaryImageTag    string   `json:"PrimaryImageTag,omitempty"`
	Name               string   `json:"Name,omitempty"`
	CollectionType     string   `json:"CollectionType,omitempty"`
	Locations          []string `json:"Locations,omitempty"`
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
func (c *Client) GetMediaItemsByLibraryID(libraryID string, lastDateCreatedTime int64) ([]BaseItemDtoV2, error) {
	const (
		limit  = 100 // 每次请求获取的项目数
		fields = "DateCreated,DateModified,ParentId,PremiereDate,MediaStreams"
	)

	var allItems []BaseItemDtoV2
	startIndex := 0
	firstRequest := true

	// 构建基础 URL
	baseURL, err := url.Parse(fmt.Sprintf("%s/emby/Items", c.embyURL))
	if err != nil {
		return nil, fmt.Errorf("解析基础 URL 时出错: %w", err)
	}

mainloop:
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
		params.Add("SortBy", "DateCreated")   // 入库时间
		params.Add("SortOrder", "Descending") // 倒叙排列
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
		for _, item := range response.Items {
			// helpers.AppLogger.Debugf("处理项目 %+v", item)
			var dateCreatedTime int64 = 0
			if item.DateCreated != "" {
				if t, err := time.Parse(time.RFC3339, item.DateCreated); err == nil {
					dateCreatedTime = t.Unix()
				}
			}
			if dateCreatedTime == lastDateCreatedTime {
				helpers.AppLogger.Infof("找到最后一个项目 %s =>%d", item.Id, lastDateCreatedTime)
				break mainloop
			} else {
				allItems = append(allItems, item)
			}
		}

		// allItems = append(allItems, response.Items...)

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

	// 当没有找到符合条件的用户时，打印ERROR级别详细日志
	if len(usersWithAllAccess) == 0 {
		helpers.AppLogger.Errorf("没有找到能访问全部媒体库的用户，共检查 %d 个用户：", len(users))
		for _, user := range users {
			helpers.AppLogger.Errorf("  ❌ 用户[%s](ID: %s) 不满足全库权限条件，原因：未开启全部文件夹访问(EnableAllFolders=false)", user.Name, user.ID)
		}
	} else {
		// 调试模式下打印详细信息
		for _, user := range users {
			if user.Policy.EnableAllFolders {
				helpers.AppLogger.Debugf("  ✅ 用户[%s](ID: %s) 满足全库权限条件，原因：开启了所有文件夹访问(EnableAllFolders=true)", user.Name, user.ID)
			} else {
				helpers.AppLogger.Debugf("  ❌ 用户[%s](ID: %s) 不满足全库权限条件，原因：未开启全部文件夹访问(EnableAllFolders=false)", user.Name, user.ID)
			}
		}
	}

	return usersWithAllAccess, nil
}

// 刷新媒体库
func (c *Client) RefreshLibrary(libraryId string, libraryName string) error {
	// Construct the request URL
	url := fmt.Sprintf("%s/emby/Items/%s/Refresh?api_key=%s&Fields=MediaStreams", c.embyURL, libraryId, c.apiKey)
	err := helpers.PostUrl(url)
	if err != nil {
		return err
	}
	helpers.AppLogger.Infof("已触发Emby媒体库 %s => %s 刷新", libraryId, libraryName)
	return nil
}

func (c *Client) GetItemDetailByUser(itemId string, userID string) (*BaseItemDtoV2, error) {
	// Construct the request URL
	url := fmt.Sprintf("%s/emby/Users/%s/Items/%s?api_key=%s", c.embyURL, userID, itemId, c.apiKey)
	helpers.AppLogger.Debugf("获取Emby媒体详情 URL: %s", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	// helpers.AppLogger.Debugf("GET %s", baseURL.String())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("错误: 收到非 200 状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response BaseItemDtoV2
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("解析 json 时出错: %w", err)
	}
	return &response, nil
}

// 通过item id 查询所属的 媒体库id，返回数组
// 先查询item的Ancestors获取item所属的文件夹id，取倒数第二个文件夹路径做为顶层文件夹路径
// 再查询/Library/VirtualFolders获取顶层文件夹路径对应的媒体库id
func (c *Client) GetItemLibraryId(itemId string) ([]VirtualFolderDto, error) {
	ancestors, err := c.GetItemAncestors(itemId)
	if err != nil {
		return nil, err
	}
	// 提取倒数第二个文件夹路径做顶层文件夹路径
	libraryFolderDto := ancestors[len(ancestors)-2]
	libraryPath := libraryFolderDto.Path
	// 查询顶层文件夹路径对应的媒体库id
	virtualFolders, err := c.GetLibraryVirtualFolders()
	if err != nil {
		return nil, err
	}
	// 提取所有媒体库id
	var librarys []VirtualFolderDto
	for _, virtualFolder := range virtualFolders {
		if slices.Contains(virtualFolder.Locations, libraryPath) {
			librarys = append(librarys, virtualFolder)
			continue
		}
	}
	return librarys, nil
}

func (c *Client) GetItemAncestors(itemId string) ([]AncestorDto, error) {
	// Construct the request URL
	url := fmt.Sprintf("%s/emby/Items/%s/Ancestors?api_key=%s", c.embyURL, itemId, c.apiKey)

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
	var ancestors []AncestorDto
	if err := json.Unmarshal(body, &ancestors); err != nil {
		return nil, fmt.Errorf("解析 json 时出错: %w", err)
	}
	// 提取所有文件夹id
	return ancestors, nil
}

// 获取所有媒体库的详情包括文件夹
func (c *Client) GetLibraryVirtualFolders() ([]VirtualFolderDto, error) {
	// Construct the request URL
	url := fmt.Sprintf("%s/emby/Library/VirtualFolders?api_key=%s", c.embyURL, c.apiKey)

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
	var virtualFolders []VirtualFolderDto
	if err := json.Unmarshal(body, &virtualFolders); err != nil {
		return nil, fmt.Errorf("解析 json 时出错: %w", err)
	}

	return virtualFolders, nil
}

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

		items, err := client.GetMediaItemsByLibraryID(lib.ID, 0)
		if err != nil {
			helpers.AppLogger.Errorf("获取媒体库 '%s' 中的项目失败: %v", lib.Name, err)
			continue // 继续处理下一个媒体库
		}

		helpers.AppLogger.Infof("在 '%s' 中找到 %d 个影视剧", lib.Name, len(items))
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
				// 检查每个媒体项目的播放信息
				url := fmt.Sprintf("%s/emby/Items/%s/PlaybackInfo?api_key=%s", embyURL, item.Id, apiKey)
				task := make(map[string]string)
				task["url"] = url
				task["item_id"] = item.Id
				task["item_name"] = item.Name
				tasks = append(tasks, task)
			}
		}
		// wg.Wait()
	}
	return tasks
}
