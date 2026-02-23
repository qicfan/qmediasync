package models

import (
	"Q115-STRM/internal/baidupan"
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/openai"
	"Q115-STRM/internal/openlist"
	"Q115-STRM/internal/v115open"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

type MediaType string

const (
	MediaTypeMovie  MediaType = "movie"  // 电影
	MediaTypeTvShow MediaType = "tvshow" // 剧集
	MediaTypeOther  MediaType = "other"  // 其他，无法刮削
)

type RenameType string

const (
	RenameTypeHardSymlink RenameType = "hard_symlink" // 硬链接
	RenameTypeSoftSymlink RenameType = "soft_symlink" // 软链接
	RenameTypeMove        RenameType = "move"         // 移动
	RenameTypeCopy        RenameType = "copy"         // 复制
)

type ScrapeFile struct {
	FileName string `json:"file_name"` // 文件名
	FileId   string `json:"file_id"`   // 文件ID
	Pid      string `json:"pid"`       // 文件夹ID
	Path     string `json:"path"`      // 文件路径
	PickCode string `json:"pick_code"` // 视频文件识别码
	IsDir    bool   `json:"is_dir"`    // 是否是目录
}

type ScrapeType string

const (
	ScrapeTypeOnly            ScrapeType = "only_scrape"       // 仅刮削
	ScrapeTypeScrapeAndRename ScrapeType = "scrape_and_rename" // 刮削和整理
	ScrapeTypeOnlyRename      ScrapeType = "only_rename"       // 仅整理
)

var SubtitleExtArr = []string{".ass", ".srt", ".ssa", ".vtt", ".sup", ".idx", ".sub"}
var ImageExtArr = []string{".jpg", ".png", ".jpeg", ".gif"}
var AllowdExtArr = append(SubtitleExtArr, append(ImageExtArr, []string{".nfo", ".mp3", ".flac", ".aas"}...)...)

type ScrapePathCategoryCollection struct {
	MovieCategory  []*MovieCategory      `json:"-" gorm:"-"`
	TvShowCategory []*TvShowCategory     `json:"-" gorm:"-"`
	PathCategory   []*ScrapePathCategory `json:"-" gorm:"-"`
}

// 刮削目录
// 开启分类最终路径为：DestPath/Category/文件夹名称模板/文件名称模板
// 关闭分类最终路径为：DestPath/文件夹名称模板/文件名称模板
type ScrapePath struct {
	BaseModel
	AccountId             uint                         `json:"account_id" form:"account_id"`                             // 账号ID
	SourceType            SourceType                   `json:"source_type" form:"source_type"`                           // 同步路径类型
	MediaType             MediaType                    `json:"media_type" form:"media_type"`                             // 媒体类型
	SourcePath            string                       `json:"source_path" form:"source_path"`                           // 源路径，绝对路径
	SourcePathId          string                       `json:"source_path_id" form:"source_path_id"`                     // 源路径ID，如果是115则是FileId，如果是Local则为空字符串，如果是openlist则是远程路径ID
	DestPath              string                       `json:"dest_path" form:"dest_path"`                               // 目标路径，绝对路径
	DestPathId            string                       `json:"dest_path_id" form:"dest_path_id"`                         // 目标路径ID，如果是115则是FileId，如果是Local则为空字符串，如果是openlist则是远程路径ID
	ScrapeType            ScrapeType                   `json:"scrape_type" form:"scrape_type"`                           // 刮削类型
	RenameType            RenameType                   `json:"rename_type" form:"rename_type"`                           // 重命名类型，非本地仅支持移动重命名
	FolderNameTemplate    string                       `json:"folder_name_template" form:"folder_name_template"`         // 文件夹名称模板，支持{{title}}、{{year}}、{{season}}、{{episode}}
	FileNameTemplate      string                       `json:"file_name_template" form:"file_name_template"`             // 文件名称模板，支持{{title}}、{{year}}、{{season}}、{{episode}}
	DeletedKeyword        string                       `json:"-" form:"-"`                                               // 要删除的关键词，json字符串数组，识别时会将数组中包含的关键字全部替换为空字符串
	DeleteKeyword         []string                     `json:"delete_keyword" form:"delete_keyword" gorm:"-"`            // 要删除的关键词，字符串数组，识别时会将数组中包含的关键字全部替换为空字符串
	EnableCategory        bool                         `json:"enable_category" form:"enable_category"`                   // 是否启用分类，开启时会根据分类名称创建文件夹
	VideoExt              string                       `json:"-" form:"-"`                                               // 视频文件扩展名，json字符串数组，例如："[\"mp4\",\"mkv\",\"avi\"]"
	VideoExtList          []string                     `json:"video_ext_list" form:"video_ext_list" gorm:"-"`            // 视频文件扩展名列表，字符串数组，例如：["mp4","mkv","avi"]
	MinVideoFileSize      int64                        `json:"min_video_file_size" form:"min_video_file_size"`           // 最小视频文件大小，单位为字节，默认值为0，即不限制最小文件大小
	ExcludeNoImageActor   bool                         `json:"exclude_no_image_actor" form:"exclude_no_image_actor"`     // 是否排除没有图片的演员，开启时会将没有图片的演员从演员列表中排除
	EnableAi              AiAction                     `json:"enable_ai" form:"enable_ai"`                               // 是否启用AI识别，开启时会使用AI识别视频文件的元数据
	AiPrompt              string                       `json:"ai_prompt" form:"ai_prompt"`                               // AI识别提示词，用于自定义AI识别的元数据
	ForceDeleteSourcePath bool                         `json:"force_delete_source_path" form:"force_delete_source_path"` // 是否强制删除源路径，开启时会强制删除源路径下的所有文件，包括子目录
	EnableCron            bool                         `json:"enable_cron" form:"enable_cron"`                           // 是否启用定时任务，开启时会根据定时任务规则定时刮削
	EnableFanartTv        bool                         `json:"enable_fanart_tv" form:"enable_fanart_tv"`                 // 是否启用 fanart.tv，开启时会从 fanart.tv 下载高清图
	IsScraping            bool                         `json:"is_scraping" form:"is_scraping"`                           // 是否正在刮削
	MaxThreads            int                          `json:"max_threads" form:"max_threads"`                           // 刮削最大线程数，默认值为5
	V115Client            *v115open.OpenClient         `json:"-" gorm:"-"`                                               // 115客户端
	BaiduPanClient        *baidupan.Client             `json:"-" gorm:"-"`                                               // 百度网盘客户端
	OpenListClient        *openlist.Client             `json:"-" gorm:"-"`                                               // openlist客户端
	ExistsFiles           map[string]bool              `json:"-" gorm:"-"`                                               // 已存在的文件，key为文件路径，value为是否存在
	ScrapeRootPath        string                       `json:"-" gorm:"-"`                                               // 刮削根路径
	Category              ScrapePathCategoryCollection `json:"-" gorm:"-"`
	CategoryMap           map[uint]string              `json:"-" gorm:"-"`
	// 完成的电视剧缓存，每次启动整理时清除，防止多次操作电视剧完成
	TvshowRenamedCache   map[uint]bool         `json:"-" gorm:"-"`
	EpisodeFinishChannel chan *ScrapeMediaFile `json:"-" gorm:"-"`
	Running              bool                  `json:"-" gorm:"-"`                   // 是否运行中
	mutex                sync.RWMutex          `json:"-" gorm:"-"`                   // 读写锁
	IsTaskRunning        int                   `json:"is_running" form:"-" gorm:"-"` // 是否正在运行
}

type ScrapeStrmPath struct {
	BaseModel
	ScrapePathID uint `json:"scrape_path_id" form:"scrape_path_id" gorm:"uniqueIndex:scrape_path_id_strm_path_id"` // 刮削目录ID
	StrmPathID   uint `json:"strm_path_id" form:"strm_path_id" gorm:"uniqueIndex:scrape_path_id_strm_path_id"`     // 同步目录ID
}

func (sp *ScrapePath) IsRunning() bool {
	sp.mutex.RLock()
	defer sp.mutex.RUnlock()
	return sp.Running
}

func (sp *ScrapePath) SetRunning() {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()
	sp.Running = true
	sp.IsScraping = true
	if err := db.Db.Model(sp).Update("is_scraping", sp.IsScraping).Error; err != nil {
		helpers.AppLogger.Errorf("更新刮削目录 %d 状态失败: %v", sp.ID, err)
	}
}

func (sp *ScrapePath) SetNotRunning() {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()
	sp.Running = false
	sp.IsScraping = false
	if err := db.Db.Model(sp).Update("is_scraping", sp.IsScraping).Error; err != nil {
		helpers.AppLogger.Errorf("更新刮削目录 %d 状态失败: %v", sp.ID, err)
	}
}

// 添加或者编辑同步目录
// 不能编辑同步源类型、网盘账号、媒体类型
func (m *ScrapePath) Save() error {
	// 转换媒体文件扩展名列表为json字符串
	if len(m.VideoExtList) > 0 {
		mediaExt, err := json.Marshal(m.VideoExtList)
		if err != nil {
			helpers.AppLogger.Errorf("转换视频文件扩展名列表失败: %v", err)
			return err
		}
		m.VideoExt = string(mediaExt)
	} else {
		m.VideoExt = ""
	}
	// 转换要删除的关键词列表为json字符串
	if len(m.DeleteKeyword) > 0 {
		keyword, err := json.Marshal(m.DeleteKeyword)
		if err != nil {
			helpers.AppLogger.Errorf("转换要删除的关键词列表失败: %v", err)
			return err
		}
		m.DeletedKeyword = string(keyword)
	} else {
		m.DeletedKeyword = ""
	}
	if m.ID == 0 {
		if m.MaxThreads > DEFAULT_LOCAL_MAX_THREADS {
			if m.SourceType != SourceTypeLocal || GlobalScrapeSettings.TmdbApiKey == "" {
				// 非本地和没有tmdb api key 时，最大线程数只能为默认值
				m.MaxThreads = DEFAULT_LOCAL_MAX_THREADS
			}
		}
		err := db.Db.Create(m).Error
		if err != nil {
			helpers.AppLogger.Errorf("添加刮削目录失败: %v", err)
			return err
		}
	} else {
		// 先读取旧数据
		isUpdateCategory := false
		oldScrapePath := GetScrapePathByID(m.ID)
		if oldScrapePath != nil {
			if oldScrapePath.DestPathId != m.DestPathId {
				isUpdateCategory = true
			}
		}
		// 特定情况下，只能改小，不能改大
		helpers.AppLogger.Infof("旧最大线程数：%d", oldScrapePath.MaxThreads)
		helpers.AppLogger.Infof("新最大线程数：%d", m.MaxThreads)
		helpers.AppLogger.Infof("是否本地目录：%s", m.SourceType)
		if m.MaxThreads > DEFAULT_LOCAL_MAX_THREADS {
			if oldScrapePath.SourceType != SourceTypeLocal || GlobalScrapeSettings.TmdbApiKey == "" {
				// 非本地和没有tmdb api key 时，最大线程数只能为默认值
				m.MaxThreads = DEFAULT_LOCAL_MAX_THREADS
			}
		}
		helpers.AppLogger.Infof("最大线程数：%d", m.MaxThreads)
		// 只能更新部分字段
		updates := map[string]interface{}{
			"scrape_type":              m.ScrapeType,
			"rename_type":              m.RenameType,
			"source_path":              m.SourcePath,
			"source_path_id":           m.SourcePathId,
			"dest_path":                m.DestPath,
			"dest_path_id":             m.DestPathId,
			"file_name_template":       m.FileNameTemplate,
			"folder_name_template":     m.FolderNameTemplate,
			"deleted_keyword":          m.DeletedKeyword,
			"enable_category":          m.EnableCategory,
			"video_ext":                m.VideoExt,
			"min_video_file_size":      m.MinVideoFileSize,
			"ai_prompt":                m.AiPrompt,
			"enable_ai":                m.EnableAi,
			"exclude_no_image_actor":   m.ExcludeNoImageActor,
			"force_delete_source_path": m.ForceDeleteSourcePath,
			"enable_fanart_tv":         m.EnableFanartTv,
			"max_threads":              m.MaxThreads,
		}
		if oldScrapePath.ScrapeType != ScrapeTypeOnly && m.ScrapeType == ScrapeTypeOnly {
			updates["dest_path"] = m.SourcePath
		}
		err := db.Db.Model(&ScrapePath{}).Where("id = ?", m.ID).Updates(updates).Error
		if err != nil {
			helpers.AppLogger.Errorf("更新刮削目录失败: %v", err)
			return err
		}
		// 如果改变了目标目录，则需要重建Category
		if isUpdateCategory {
			// 将ScrapePathCategory表相关的FileID字段清空
			db.Db.Model(&ScrapePathCategory{}).Where("scrape_path_id = ?", m.ID).Update("file_id", "")
		}
	}
	return nil
}

func (sp *ScrapePath) GetAccount() (*Account, error) {
	return GetAccountById(sp.AccountId)
}

type ScrapeMediaResult struct {
	TaskID int
	Result []*ScrapeFile
	Error  error
}

func (sp *ScrapePath) Init() bool {
	if sp.SourceType != SourceTypeLocal {
		// 初始化网盘客户端
		account, err := sp.GetAccount()
		if err != nil {
			helpers.AppLogger.Errorf("获取115账号失败: %v", err)
			return false
		}
		switch sp.SourceType {
		case SourceType115:
			sp.V115Client = account.Get115Client()
			if sp.V115Client == nil {
				helpers.AppLogger.Errorf("获取115客户端失败")
				return false
			}
			// helpers.AppLogger.Infof("获取115客户端成功")
		case SourceTypeOpenList:
			sp.OpenListClient = account.GetOpenListClient()
			if sp.OpenListClient == nil {
				helpers.AppLogger.Errorf("获取OpenList客户端失败")
				return false
			}
			// helpers.AppLogger.Infof("获取OpenList客户端成功")
		}
	}
	// 创建临时目录
	sp.ScrapeRootPath = filepath.Join(helpers.ConfigDir, "tmp", "刮削临时文件", fmt.Sprintf("%d", sp.ID), "电影或其他")
	if sp.MediaType == MediaTypeTvShow {
		sp.ScrapeRootPath = filepath.Join(helpers.ConfigDir, "tmp", "刮削临时文件", fmt.Sprintf("%d", sp.ID), "电视剧")
	}
	if err := os.MkdirAll(sp.ScrapeRootPath, 0777); err != nil {
		helpers.AppLogger.Errorf("创建临时目录失败: %v", err)
		return false
	}
	return true
}

func (sp *ScrapePath) GetMaxThreads() int {
	if sp.MaxThreads <= 0 {
		return DEFAULT_LOCAL_MAX_THREADS
	}
	return sp.MaxThreads
}

func (sp *ScrapePath) IsVideoFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	for _, videoExt := range sp.VideoExtList {
		if ext == strings.ToLower(videoExt) {
			return true
		}
	}
	return false
}

func (sp *ScrapePath) MakeScrapeMediaFile(path, pathId, fileName, fileId, pickCode string) *ScrapeMediaFile {
	mediaFile := &ScrapeMediaFile{
		ScrapePathId:   sp.ID,
		MediaType:      sp.MediaType,
		SourceType:     sp.SourceType,
		ScrapeType:     sp.ScrapeType,
		RenameType:     sp.RenameType,
		SourcePath:     sp.SourcePath,
		SourcePathId:   sp.SourcePathId,
		DestPath:       sp.DestPath,
		DestPathId:     sp.DestPathId,
		Path:           path,
		PathId:         pathId,
		VideoFilename:  fileName,
		VideoFileId:    fileId,
		VideoPickCode:  pickCode,
		Status:         ScrapeMediaStatusScanned,
		EnableCategory: sp.EnableCategory,
		SeasonNumber:   -1,
		EpisodeNumber:  -1,
		AudioCodec:     make([]*AudioCodec, 0),
		SubtitleCodec:  make([]*Subtitle, 0),
		TvshowFiles:    make([]*MediaMetaFiles, 0),
		SeasonFiles:    make([]*MediaMetaFiles, 0),
		MediaFiles: MediaFiles{
			ImageFiles:    make([]*MediaMetaFiles, 0),
			SubtitleFiles: make([]*MediaMetaFiles, 0),
		},
	}
	// 电视剧默认没有季目录，后续通过识别来判断是否有季目录
	if sp.MediaType == MediaTypeTvShow {
		mediaFile.TvshowPath = path
		mediaFile.TvshowPathId = pathId
		mediaFile.Path = ""
		mediaFile.PathId = ""
	}
	return mediaFile
}

// 将本地临时文件移动到本地目标路径
func (sp *ScrapePath) MoveLocalTempFileToDest(files []map[string]string) (bool, error) {
	if sp.SourceType != SourceTypeLocal {
		return true, fmt.Errorf("非本地文件刮削，无法移动到目标位置")
	}
	for _, file := range files {
		tempPath := file["SourcePath"]
		if !helpers.PathExists(tempPath) {
			continue
		}
		destPath := filepath.Join(file["DestPath"], file["FileName"])
		err := helpers.MoveFile(tempPath, destPath, true)
		if err != nil {
			helpers.AppLogger.Errorf("移动刮削临时文件 %s 到整理目标位置 %s 失败: %v", tempPath, destPath, err)
			return false, err
		}
		helpers.AppLogger.Infof("移动刮削临时文件 %s 到整理目标位置 %s 成功", tempPath, destPath)
	}
	return true, nil
}

func (sp *ScrapePath) GetDownloadUrl(videoPathOrUrl string) string {
	if sp.SourceType == SourceTypeLocal {
		return videoPathOrUrl
	}
	switch sp.SourceType {
	case SourceType115:
		videoPathOrUrl = sp.V115Client.GetDownloadUrl(context.Background(), videoPathOrUrl, v115open.DEFAULTUA, false)
	case SourceTypeOpenList:
		videoPathOrUrl = sp.OpenListClient.GetRawUrl(videoPathOrUrl)
	case SourceType123:
	}
	return videoPathOrUrl
}

// 给刮削目录生成二级目录文件夹
// 先检查是否有对应的数据库纪录,然后比对目录分类和分类列表的差异,没有的创建,删除的删除,改名的改名
func (sp *ScrapePath) GenerateCategory() {
	if !sp.EnableCategory {
		helpers.AppLogger.Infof("刮削目录 %s 未启用二级分类", sp.SourcePath)
		return
	}
	// 查询数据库ScrapePathCategory
	scrapePathCategory := GetAllScrapePathCategory(sp.ID)
	// 查询所有分类
	type categoryTmp struct {
		ID         uint   `json:"id"`
		Name       string `json:"name"`
		CategoryId uint   `json:"category_id"`
	}
	sp.Category = ScrapePathCategoryCollection{}
	added := make([]categoryTmp, 0)
	deleted := make([]*ScrapePathCategory, 0)
	spCList := make([]*ScrapePathCategory, 0)
	if sp.MediaType == MediaTypeMovie {
		categories := GetMovieCategory()
		sp.Category.MovieCategory = categories
		for _, category := range categories {
			// 检查数据库是否有对应的纪录
			exists := false
			var existsScrapeCategory *ScrapePathCategory
		movieloop:
			for _, dbCategory := range scrapePathCategory {
				if dbCategory.CategoryId == category.ID {
					if dbCategory.FileId == "" {
						helpers.AppLogger.Infof("二级分类 %s 已存在数据库记录，但是没有创建目标路径，ID=%d", category.Name, category.ID)
						existsScrapeCategory = dbCategory
						exists = false
						break movieloop
					}
					exists = true
				}
			}
			if !exists {
				helpers.AppLogger.Infof("二级分类 %s 准备创建目标路径", category.Name)
				if existsScrapeCategory == nil {
					added = append(added, categoryTmp{ID: 0, CategoryId: category.ID, Name: category.Name})
				} else {
					added = append(added, categoryTmp{ID: existsScrapeCategory.ID, CategoryId: category.ID, Name: category.Name})
				}
			}
		}
		// scrapePathCategory有categories没有的加入删除
		for _, scrapteCategory := range scrapePathCategory {
			exists := false
			for _, category := range categories {
				if scrapteCategory.CategoryId == category.ID {
					exists = true
					break
				}
			}
			if !exists {
				deleted = append(deleted, scrapteCategory)
			} else {
				spCList = append(spCList, scrapteCategory)
			}
		}
	} else if sp.MediaType == MediaTypeTvShow {
		categories := GetTvshowCategory()
		sp.Category.TvShowCategory = categories
		for _, category := range categories {
			// 检查数据库是否有对应的纪录
			exists := false
			var existsScrapeCategory *ScrapePathCategory
		dbloop:
			for _, dbCategory := range scrapePathCategory {
				if dbCategory.CategoryId == category.ID {
					if dbCategory.FileId == "" {
						existsScrapeCategory = dbCategory
						helpers.AppLogger.Infof("二级分类 %s 已存在数据库记录，但是没有创建目标路径，ID=%d", category.Name, category.ID)
						exists = false
						break dbloop
					}
					exists = true
				}
			}
			if !exists {
				helpers.AppLogger.Infof("二级分类 %s 准备创建目标路径", category.Name)
				if existsScrapeCategory == nil {
					added = append(added, categoryTmp{ID: 0, CategoryId: category.ID, Name: category.Name})
				} else {
					added = append(added, categoryTmp{ID: existsScrapeCategory.ID, CategoryId: category.ID, Name: category.Name})
				}
			}
		}
		for _, scrapteCategory := range scrapePathCategory {
			exists := false
			for _, category := range categories {
				if scrapteCategory.CategoryId == category.ID {
					exists = true
					break
				}
			}
			if !exists {
				deleted = append(deleted, scrapteCategory)
			} else {
				spCList = append(spCList, scrapteCategory)
			}
		}
	}
	// 处理添加
	for _, category := range added {
		fileId := ""
		var err error
		// 创建目录
		// 根据SourceType不同,调用各自接口创建目录
		switch sp.SourceType {
		case SourceType115:
			// 先查询是否存在
			categoryPath := filepath.Join(sp.DestPath, category.Name)
			detail, detailErr := sp.V115Client.GetFsDetailByPath(context.Background(), categoryPath)
			if detail != nil && detailErr == nil && detail.FileId != "" {
				helpers.AppLogger.Infof("目录 %s 已存在, 目录ID=%s, 返回值:%+v", categoryPath, detail.FileId, detail)
				fileId = detail.FileId
			} else {
				fileId, err = sp.V115Client.MkDir(context.Background(), sp.DestPathId, category.Name)
				if err != nil {
					helpers.AppLogger.Errorf("创建115目录失败: %v", err)
					continue
				}
			}
		case SourceTypeOpenList:
			fileId = sp.DestPathId + "/" + category.Name
			err = sp.OpenListClient.Mkdir(fileId)
			if err != nil {
				helpers.AppLogger.Errorf("创建OpenList目录失败: %v", err)
				continue
			}
		case SourceTypeLocal:
			fileId = filepath.Join(sp.DestPathId, category.Name)
			os.MkdirAll(fileId, 0777)
		case SourceType123:
		case SourceTypeBaiduPan:
			fileId = sp.DestPathId + "/" + category.Name
			// 先查询是否存在
			exists, _ := sp.BaiduPanClient.PathExists(context.Background(), fileId)
			if !exists {
				err = sp.BaiduPanClient.Mkdir(context.Background(), fileId)
				if err != nil {
					helpers.AppLogger.Errorf("创建百度网盘目录失败: %v", err)
					continue
				}
			} else {
				helpers.AppLogger.Infof("百度网盘目录 %s 已存在", fileId)
			}
		}
		if fileId == "" {
			helpers.AppLogger.Errorf("创建二级分类=%s 目录失败", category.Name)
			continue
		}
		// 保存目录ID
		spC, err := SaveScrapePathCategory(category.ID, sp.ID, category.CategoryId, fileId)
		if err != nil {
			helpers.AppLogger.Errorf("保存目录ID失败: %v", err)
			continue
		}
		spCList = append(spCList, spC)
		helpers.AppLogger.Infof("创建二级分类=%s 目录ID=%s 成功", category.Name, fileId)
	}
	// 处理删除
	for _, category := range deleted {
		// 不删除旧目录,防止误删文件
		// 删除数据库记录
		err := category.Delete()
		if err != nil {
			helpers.AppLogger.Errorf("删除数据库目录记录失败: %v", err)
			continue
		}
		helpers.AppLogger.Infof("删除二级分类目录 %d 成功", category.ID)
	}
	sp.Category.PathCategory = spCList
	sp.CategoryMap = sp.GetAllScrapePathCategory()
}

// 下载图片到指定文件
func (sp *ScrapePath) DownloadImages(parentPath, ua string, fileList map[string]string) {
	for fileName, url := range fileList {
		filePath := filepath.Join(parentPath, fileName)
		if url == "" {
			// helpers.AppLogger.Errorf("图片URL为空, 文件名 %s", fileName)
			continue
		}
		// helpers.AppLogger.Debugf("准备下载图片 %s => %s", url, filePath)
		if !helpers.PathExists(filePath) {
			err := helpers.DownloadFile(url, filePath, ua)
			if err != nil {
				helpers.AppLogger.Errorf("下载图片 %s 失败: %v", filePath, err)
			}
		} else {
			// helpers.AppLogger.Debugf("图片 %s 已存在，跳过下载", filePath)
		}
	}
}

func (sp *ScrapePath) CheckFileIsAllowed(fileName string, fileSize int64) bool {
	fileExt := filepath.Ext(fileName)
	if !sp.IsVideoFile(fileName) && !slices.Contains(AllowdExtArr, fileExt) {
		helpers.AppLogger.Infof("非视频或元数据文件不需要处理: %s", fileName)
		return false // 如果不需要处理，则跳过
	}
	if sp.IsVideoFile(fileName) && fileSize < sp.MinVideoFileSize*1024*1024 {
		helpers.AppLogger.Infof("视频文件%s大小%d小于%d最小要求，不需要处理", fileName, fileSize, sp.MinVideoFileSize*1024*1024)
		return false // 如果不需要处理，则跳过
	}
	return true
}

// 获取AI识别提示词
func (sp *ScrapePath) GetAiPrompt() string {
	var prompt string = sp.AiPrompt
	if prompt == "" {
		prompt = openai.DEFAULT_MOVIE_PROMPT
	}
	return prompt
}

// 打开或关闭定时任务
func (sp *ScrapePath) ToggleCron() error {
	if sp.EnableCron {
		sp.EnableCron = false
	} else {
		sp.EnableCron = true
	}
	return db.Db.Model(&ScrapePath{}).Where("id = ?", sp.ID).Updates(map[string]interface{}{
		"enable_cron": sp.EnableCron,
	}).Error
}

func (sp *ScrapePath) GetAllScrapePathCategory() map[uint]string {
	if !sp.EnableCategory {
		return nil
	}
	var spcs []*ScrapePathCategory
	db.Db.Model(&ScrapePathCategory{}).Where("scrape_path_id = ?", sp.ID).Find(&spcs)
	if len(spcs) == 0 {
		return nil
	}
	categoryMap := make(map[uint]string)
	for _, spc := range spcs {
		categoryMap[spc.ID] = spc.FileId
	}
	return categoryMap
}

func (sp *ScrapePath) GetMovieRealName(sm *ScrapeMediaFile, name string, filetype string) string {
	if filetype == "nfo" {
		return fmt.Sprintf("%s.nfo", sm.NewVideoBaseName)
	}
	if sm.ScrapeType == ScrapeTypeOnly {
		return fmt.Sprintf("%s-%s", sm.NewVideoBaseName, name)
	} else {
		return name
	}
}

func (sp *ScrapePath) GetRelatStrmPath() []*ScrapeStrmPath {
	return GetRelatStrmPathByScrapePathID(sp.ID)
}

func (sp *ScrapePath) SaveStrmPath(ids []uint) error {
	tx := db.Db.Begin()
	// 删除旧关联
	if err := tx.Where("scrape_path_id = ?", sp.ID).Delete(&ScrapeStrmPath{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	// 保存关联的同步目录
	var scrapeStrmPaths []*ScrapeStrmPath
	for _, id := range ids {
		scrapeStrmPaths = append(scrapeStrmPaths, &ScrapeStrmPath{
			ScrapePathID: sp.ID,
			StrmPathID:   id,
		})
	}
	if err := tx.Save(&scrapeStrmPaths).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (sp *ScrapePath) GetSyncPathes() []*SyncPath {
	var scrapeStrmPaths []*ScrapeStrmPath
	if err := db.Db.Where("scrape_path_id = ?", sp.ID).Find(&scrapeStrmPaths).Error; err != nil {
		return nil
	}
	var syncPathes []*SyncPath
	for _, sp := range scrapeStrmPaths {
		syncPath := GetSyncPathById(sp.StrmPathID)
		syncPathes = append(syncPathes, syncPath)
	}
	return syncPathes
}

// 根据给定的路径，查找命中的syncPath，判断路径是否以syncPath的RemotePath开头
func (sp *ScrapePath) GetSyncPathByPath(path string) *SyncPath {
	syncPathes := sp.GetSyncPathes()
	if len(syncPathes) == 0 {
		helpers.AppLogger.Errorf("刮削目录 %d 没有关联的同步目录", sp.ID)
		return nil
	}
	path = filepath.ToSlash(path)
	// 如果path不以/开头，则添加/
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	for _, syncPath := range syncPathes {
		// 如果syncPath的RemotePath不以/开头，则添加/
		remotePath := syncPath.RemotePath
		if !strings.HasPrefix(remotePath, "/") {
			remotePath = "/" + remotePath
		}
		if strings.HasPrefix(path, remotePath) {
			return syncPath
		}
		helpers.AppLogger.Debugf("路径 %s 不匹配同步目录 %s", path, remotePath)
	}
	return nil
}

func GetScrapePathCategoryById(id uint) *ScrapePathCategory {
	var spc ScrapePathCategory
	if err := db.Db.Where("id = ?", id).First(&spc).Error; err != nil {
		return nil
	}
	return &spc
}

// 查询刮削目录列表，不需要分页
func GetScrapePathes() []*ScrapePath {
	var scrapePathes []*ScrapePath
	db.Db.Model(&ScrapePath{}).Order("id DESC").Find(&scrapePathes)
	if len(scrapePathes) > 0 {
		// 将video_ext转为字符串数组
		for _, scrapePath := range scrapePathes {
			if scrapePath.VideoExt != "" {
				err := json.Unmarshal([]byte(scrapePath.VideoExt), &scrapePath.VideoExtList)
				if err != nil {
					helpers.AppLogger.Errorf("转换视频文件扩展名列表失败: %v", err)
				}
			} else {
				scrapePath.VideoExtList = []string{}
			}
			// 将deleted_keyword转为字符串数组
			if scrapePath.DeletedKeyword != "" {
				err := json.Unmarshal([]byte(scrapePath.DeletedKeyword), &scrapePath.DeleteKeyword)
				if err != nil {
					helpers.AppLogger.Errorf("转换要删除的关键词列表失败: %v", err)
				}
			} else {
				scrapePath.DeleteKeyword = []string{}
			}
		}
	}
	return scrapePathes
}

// 查询刮削目录详情
func GetScrapePathByID(id uint) *ScrapePath {
	var scrapePath ScrapePath
	err := db.Db.Model(&ScrapePath{}).Where("id = ?", id).First(&scrapePath).Error
	if err == nil {
		// 转换视频文件扩展名列表为json字符串
		if scrapePath.VideoExt != "" {
			err := json.Unmarshal([]byte(scrapePath.VideoExt), &scrapePath.VideoExtList)
			if err != nil {
				helpers.AppLogger.Errorf("转换视频文件扩展名列表失败: %v", err)
			}
		} else {
			scrapePath.VideoExtList = []string{}
		}
		// 将deleted_keyword转为字符串数组
		if scrapePath.DeletedKeyword != "" {
			err := json.Unmarshal([]byte(scrapePath.DeletedKeyword), &scrapePath.DeleteKeyword)
			if err != nil {
				helpers.AppLogger.Errorf("转换要删除的关键词列表失败: %v", err)
			}
		} else {
			scrapePath.DeleteKeyword = []string{}
		}

	}
	return &scrapePath
}

// 删除刮削目录
func DeleteScrapePath(id uint) error {
	err := db.Db.Delete(&ScrapePath{}, id).Error
	if err != nil {
		helpers.AppLogger.Errorf("删除刮削目录失败: %v", err)
		return err
	}
	// 删除ScrapePathCategory表中相关的记录
	err = db.Db.Delete(&ScrapePathCategory{}, "scrape_path_id = ?", id).Error
	if err != nil {
		helpers.AppLogger.Errorf("删除刮削目录分类失败: %v", err)
		return err
	}
	// 删除ScrapeMediaFile中所有未完成的记录
	err = db.Db.Delete(&ScrapeMediaFile{}, "scrape_path_id = ? AND status IN ?", id, []ScrapeMediaStatus{ScrapeMediaStatusScanned, ScrapeMediaStatusScrapeFailed, ScrapeMediaStatusScraped, ScrapeMediaStatusScraping}).Error
	if err != nil {
		helpers.AppLogger.Errorf("删除刮削目录文件失败: %v", err)
		return err
	}
	// 删除所有media / mediaSeason / mediaEpisode
	if err := db.Db.Where("scrape_path_id = ?", id).Delete(&Media{}).Error; err != nil {
		helpers.AppLogger.Errorf("删除Media失败: %v", err)
		return err
	}
	if err := db.Db.Where("scrape_path_id = ?", id).Delete(&MediaSeason{}).Error; err != nil {
		helpers.AppLogger.Errorf("删除MediaSeason失败: %v", err)
		return err
	}
	if err := db.Db.Where("scrape_path_id = ?", id).Delete(&MediaEpisode{}).Error; err != nil {
		helpers.AppLogger.Errorf("删除MediaEpisode失败: %v", err)
		return err
	}
	return nil
}

// 将整理中和刮削中改为未执行
func ResetScrapePathStatus() {
	updateData := make(map[string]interface{})
	updateData["is_scraping"] = false
	err := db.Db.Model(&ScrapePath{}).Where("is_scraping = ?", true).Updates(updateData).Error
	if err != nil {
		helpers.AppLogger.Errorf("重置所有刮削目录状态失败: %v", err)
	} else {
		helpers.AppLogger.Infof("重置所有刮削目录状态成功")
	}
}

// 查询刮削目录关联的STRM同步目录
func GetRelatStrmPathByScrapePathID(id uint) []*ScrapeStrmPath {
	var scrapeStrmPaths []*ScrapeStrmPath
	err := db.Db.Model(&ScrapeStrmPath{}).Where("scrape_path_id = ?", id).Find(&scrapeStrmPaths).Error
	if err != nil {
		helpers.AppLogger.Errorf("查询关联同步目录失败: %v", err)
		return nil
	}
	return scrapeStrmPaths
}

func GetTmdbImageUrl(path string) string {
	return fmt.Sprintf("%s/t/p/original%s", GlobalScrapeSettings.GetTmdbImageUrl(), path)
}
