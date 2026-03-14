package models

import (
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/openai"
	"Q115-STRM/internal/tmdb"
	"encoding/json"
	"fmt"
)

type AiAction string

const (
	AiActionOff     AiAction = "off"     // 关闭AI识别
	AiActionAssist  AiAction = "assist"  // 辅助识别，如果预设规则无法识别，则使用AI识别结果
	AiActionEnforce AiAction = "enforce" // 强制识别，仅使用AI识别结果
)

// 刮削基本设置
type ScrapeSettings struct {
	BaseModel
	TmdbUrl           string   `json:"tmdb_url" form:"tmdb_url"`                       // TMDB API URL，如果不设置则使用默认值
	TmdbImageUrl      string   `json:"tmdb_image_url" form:"tmdb_image_url"`           // TMDB 图片URL，如果不设置则使用默认值
	TmdbApiKey        string   `json:"api_key" form:"api_key"`                         // TMDB API KEY，如果不设置则使用默认值
	TmdbAccessToken   string   `json:"tmdb_access_token" form:"tmdb_access_token"`     // TMDB Access Token，如果不设置则使用默认值
	TmdbLanguage      string   `json:"tmdb_language" form:"tmdb_language"`             // TMDB 语言，默认值为"zh-CN"
	TmdbImageLanguage string   `json:"tmdb_image_language" form:"tmdb_image_language"` // TMDB 图片语言，默认值为"en-US"
	TmdbEnableProxy   bool     `json:"tmdb_enable_proxy" form:"tmdb_enable_proxy"`     // 是否启用TMDB代理
	EnableAi          AiAction `json:"enable_ai" form:"enable_ai"`                     // 是否启用AI识别
	AiBaseUrl         string   `json:"ai_base_url" form:"ai_base_url"`                 // AI识别基础URL
	AiApiKey          string   `json:"ai_api_key" form:"ai_api_key"`                   // AI识别API KEY
	AiModelName       string   `json:"ai_model_name" form:"ai_model_name"`             // AI识别模型名称
	AiPrompt          string   `json:"ai_prompt" form:"ai_prompt"`                     // AI识别提示词，如果留空则使用默认值
	AiTimeout         int      `json:"ai_timeout" form:"ai_timeout"`                   // AI识别超时时间，单位秒，默认值为:120
}

const (
	DEFAULT_LOCAL_MAX_THREADS = 5
)

var GlobalScrapeSettings = &ScrapeSettings{}

func LoadScrapeSettings() *ScrapeSettings {
	if err := db.Db.Take(GlobalScrapeSettings).Error; err != nil {
		helpers.AppLogger.Errorf("加载刮削设置失败: %v", err)
		return nil
	}

	helpers.AppLogger.Infof("从数据库读取刮削设置成功")
	return GlobalScrapeSettings
}

func (s *ScrapeSettings) GetTmdbApiKey() string {
	if s.TmdbApiKey == "" {
		return helpers.DEFAULT_TMDB_API_KEY
	}
	return s.TmdbApiKey
}

func (s *ScrapeSettings) GetTmdbAccessToken() string {
	if s.TmdbAccessToken == "" {
		return helpers.DEFAULT_TMDB_ACCESS_TOKEN
	}
	return s.TmdbAccessToken
}

func (s *ScrapeSettings) GetTmdbLanguage() string {
	if s.TmdbLanguage == "" {
		return helpers.DEFAULT_TMDB_LANGUAGE
	}
	return s.TmdbLanguage
}

func (s *ScrapeSettings) GetTmdbImageLanguage() string {
	if s.TmdbImageLanguage == "" {
		return helpers.DEFAULT_TMDB_IMAGE_LANGUAGE
	}
	return s.TmdbImageLanguage
}

func (s *ScrapeSettings) GetTmdbApiUrl() string {
	if s.TmdbUrl == "" {
		return helpers.DEFAULT_TMDB_API_URL
	}
	return s.TmdbUrl
}

func (s *ScrapeSettings) GetTmdbImageUrl() string {
	if s.TmdbImageUrl == "" {
		return helpers.DEFAULT_TMDB_IMAGE_URL
	}
	return s.TmdbImageUrl
}

func (s *ScrapeSettings) GetTmdbProxyUrl() string {
	if s.TmdbEnableProxy {
		return SettingsGlobal.HttpProxy
	}
	return ""
}

func (s *ScrapeSettings) GetTmdbClient() *tmdb.Client {
	return tmdb.NewClient(s.GetTmdbApiKey(), s.GetTmdbAccessToken(), s.GetTmdbApiUrl(), s.GetTmdbLanguage(), s.GetTmdbProxyUrl())
}

// 保存tmdb设置
func (s *ScrapeSettings) SaveTmdb(apiKey, accessToken string, apiUrl string, imageUrl string, language string, imageLanguage string, enableProxy bool) error {
	s.TmdbApiKey = apiKey
	s.TmdbAccessToken = accessToken
	s.TmdbUrl = apiUrl
	s.TmdbImageUrl = imageUrl
	s.TmdbLanguage = language
	s.TmdbImageLanguage = imageLanguage
	s.TmdbEnableProxy = enableProxy
	updateData := make(map[string]interface{})
	updateData["tmdb_api_key"] = apiKey
	updateData["tmdb_access_token"] = accessToken
	updateData["tmdb_url"] = apiUrl
	updateData["tmdb_image_url"] = imageUrl
	updateData["tmdb_language"] = language
	updateData["tmdb_image_language"] = imageLanguage
	updateData["tmdb_enable_proxy"] = enableProxy
	err := db.Db.Model(ScrapeSettings{}).Where("id = ?", s.ID).Updates(updateData).Error
	if err != nil {
		helpers.AppLogger.Errorf("更新TMDB设置失败: %v", err)
		return err
	}
	return nil
}

// 获取AI识别提示词
func (s *ScrapeSettings) GetAiPrompt() string {
	var prompt string = s.AiPrompt
	if prompt == "" {
		prompt = openai.DEFAULT_MOVIE_PROMPT
	}
	return prompt
}

func (s *ScrapeSettings) GetAiApiKey() string {
	if s.EnableAi == AiActionAssist && s.AiApiKey == "" {
		return helpers.DEFAULT_SC_API_KEY
	}
	return s.AiApiKey
}

func (s *ScrapeSettings) GetAiBaseUrl() string {
	if s.EnableAi == AiActionAssist && s.AiBaseUrl == "" {
		return openai.DEFAULT_API_BASE_URL
	}
	return s.AiBaseUrl
}

func (s *ScrapeSettings) GetAiTimeout() int {
	if s.AiTimeout == 0 {
		return openai.DEFAULT_TIMEOUT
	}
	return s.AiTimeout
}

func (s *ScrapeSettings) GetAiModelName() string {
	if s.EnableAi == AiActionAssist && s.AiModelName == "" {
		return openai.DEFAULT_MODEL_NAME
	}
	return s.AiModelName
}

func (s *ScrapeSettings) GetAiClient() *openai.Client {
	return openai.NewClient(s.GetAiApiKey(), s.GetAiBaseUrl(), s.GetAiModelName(), s.GetAiTimeout())
}

// 保存AI识别设置
func (s *ScrapeSettings) SaveAi(apiKey string, baseUrl string, modelName string, timeout int) error {
	s.AiApiKey = apiKey
	s.AiBaseUrl = baseUrl
	s.AiModelName = modelName
	s.AiTimeout = timeout
	updateData := make(map[string]interface{})
	updateData["ai_api_key"] = apiKey
	updateData["ai_base_url"] = baseUrl
	updateData["ai_model_name"] = modelName
	updateData["ai_timeout"] = s.AiTimeout
	// helpers.AppLogger.Infof("更新AI识别设置: %+v", updateData)
	err := db.Db.Model(s).Where("id = ?", s.ID).Updates(updateData).Error
	if err != nil {
		helpers.AppLogger.Errorf("更新AI识别设置失败: %v", err)
		return err
	}
	return nil
}

// 测试TMDB是否配置正确
func (s *ScrapeSettings) TestTmdb() bool {
	client := s.GetTmdbClient()
	return client.TestToken()
}

// 测试AI是否配置正确
func (s *ScrapeSettings) TestAi() error {
	if s.EnableAi == AiActionOff {
		return nil
	}
	prompt := s.GetAiPrompt()
	filename := "[银色子弹字幕组][名侦探柯南][第74集 死神阵内杀人事件][WEBRIP][简日双语MP4/繁日双语MP4/简繁日多语MKV][1080P]"
	client := s.GetAiClient()
	mediaInfo, err := client.TakeMoiveName(filename, prompt)
	if err != nil {
		helpers.AppLogger.Errorf("测试AI识别失败: %v", err)
		return err
	}
	helpers.AppLogger.Infof("测试AI识别成功: 文件名：%s, AI识别结果：%+v", filename, mediaInfo)
	if mediaInfo.Name == "名侦探柯南" {
		return nil
	}
	return fmt.Errorf("测试AI识别失败，识别出的电影名称不是名侦探柯南")
}

func (s *ScrapeSettings) ExtractByAi(filename string) (*openai.MediaInfoAI, error) {
	client := s.GetAiClient()
	return client.TakeMoiveName(filename, s.GetAiPrompt())
}

// 电影分类
// 默认ID=1, 外语剧分类，语言为空，无法修改
// 语言为空则不匹配TMDB的original_language
// 分类ID为空则不匹配TMDB的genre_ids
// tmdb的genre_id是一个数组，只要数组中有一个分类ID匹配，就认为是该分类
// 新增和修改分类时给所有电影类型的刮削目录创建分类文件夹(如果不存在的话)
type MovieCategory struct {
	BaseModel
	Name          string   `json:"name"`                    // 分类名称
	GenreIds      string   `json:"-"`                       // 分类ID，json数字数组
	Language      string   `json:"-"`                       // 语言，json字符串数组，如果为空则排除所有已设置的语言
	GenreIdArray  []int    `json:"genre_id_array" gorm:"-"` // 分类ID数组
	LanguageArray []string `json:"language_array" gorm:"-"` // 语言数组
}

// 剧集分类
// 默认ID=1, 外语剧分类，语言为空，无法修改
// 语言为空则不匹配TMDB的original_language
// 分类ID为空则不匹配TMDB的genre_ids
// tmdb的genre_id是一个数组，只要数组中有一个分类ID匹配，就认为是该分类
// 新增和修改分类时给所有剧集类型的刮削目录创建分类文件夹(如果不存在的话)
type TvShowCategory struct {
	BaseModel
	Name         string   `json:"name"`                    // 分类名称
	GenreIds     string   `json:"-"`                       // 分类ID，json数字数组
	Countries    string   `json:"-"`                       // 国家，json字符串数组，如果为空则排除所有已设置的国家
	GenreIdArray []int    `json:"genre_id_array" gorm:"-"` // 分类ID数组
	CountryArray []string `json:"country_array" gorm:"-"`  // 国家数组
}

// 刮削目录分类
// 一个刮削目录可以对应多个分类，每个分类有一个文件ID，用于指定分类下的文件存储路径
type ScrapePathCategory struct {
	BaseModel
	ScrapePathId uint   `json:"scrape_path_id"` // 刮削目录ID
	CategoryId   uint   `json:"category_id"`    // 分类ID
	FileId       string `json:"file_id"`        // 文件ID，115文件ID,123文件夹ID,local和openlist则为相对路径
}

// 查询电影分类列表，不分页
func GetMovieCategory() []*MovieCategory {
	var movieGenre []*MovieCategory
	db.Db.Model(&MovieCategory{}).Find(&movieGenre)
	// 将GenreIds和Language转换为数组
	for _, genre := range movieGenre {
		genre.GenreIdArray = []int{}
		genre.LanguageArray = []string{}

		if genre.GenreIds != "" {
			err := json.Unmarshal([]byte(genre.GenreIds), &genre.GenreIdArray)
			if err != nil {
				helpers.AppLogger.Errorf("转换电影分类ID失败: %v", err)
			}
		}
		if genre.Language != "" {
			err := json.Unmarshal([]byte(genre.Language), &genre.LanguageArray)
			if err != nil {
				helpers.AppLogger.Errorf("转换语言失败: %v", err)
			}
		}
	}
	return movieGenre
}

func GetTvshowCategory() []*TvShowCategory {
	var tvshowGenre []*TvShowCategory
	db.Db.Model(&TvShowCategory{}).Find(&tvshowGenre)
	// 将GenreIds和Language转换为数组
	for _, genre := range tvshowGenre {
		genre.GenreIdArray = []int{}
		genre.CountryArray = []string{}

		if genre.GenreIds != "" {
			err := json.Unmarshal([]byte(genre.GenreIds), &genre.GenreIdArray)
			if err != nil {
				helpers.AppLogger.Errorf("转换剧集分类ID失败: %v", err)
			}
		}
		if genre.Countries != "" {
			err := json.Unmarshal([]byte(genre.Countries), &genre.CountryArray)
			if err != nil {
				helpers.AppLogger.Errorf("转换国家失败: %v", err)
			}
		}
	}
	return tvshowGenre
}

// 保存或者更新电影分类
func (m *MovieCategory) Save(name string, genreIdsArray []int, languageArray []string) error {
	m.Name = name
	m.GenreIdArray = genreIdsArray
	m.LanguageArray = languageArray
	if len(genreIdsArray) == 0 {
		m.GenreIds = ""
	} else {
		genreIds, err := json.Marshal(genreIdsArray)
		if err != nil {
			helpers.AppLogger.Errorf("转换电影分类ID失败: %v", err)
			return err
		}
		m.GenreIds = string(genreIds)
	}
	if len(languageArray) == 0 {
		m.Language = ""
	} else {
		language, err := json.Marshal(languageArray)
		if err != nil {
			helpers.AppLogger.Errorf("转换语言失败: %v", err)
			return err
		}
		m.Language = string(language)
	}
	if m.ID == 0 {
		db.Db.Save(m)
	} else {
		db.Db.Save(m)
	}
	return nil
}

// 保存或者更新剧集分类
func (m *TvShowCategory) Save(name string, genreIdsArray []int, countryArray []string) error {
	m.Name = name
	m.GenreIdArray = genreIdsArray
	m.CountryArray = countryArray
	if len(genreIdsArray) == 0 {
		m.GenreIds = ""
	} else {
		genreIds, err := json.Marshal(genreIdsArray)
		if err != nil {
			helpers.AppLogger.Errorf("转换剧集分类ID失败: %v", err)
			return err
		}
		m.GenreIds = string(genreIds)
	}
	if len(countryArray) == 0 {
		m.Countries = ""
	} else {
		countries, err := json.Marshal(countryArray)
		if err != nil {
			helpers.AppLogger.Errorf("转换国家失败: %v", err)
			return err
		}
		m.Countries = string(countries)
	}
	if m.ID == 0 {
		db.Db.Save(m)
	} else {
		db.Db.Save(m)
	}
	return nil
}

// 删除电影分类
func DeleteMovieCategory(id uint) error {
	delErr := db.Db.Delete(&MovieCategory{}, id).Error
	if delErr != nil {
		helpers.AppLogger.Errorf("删除电影分类失败: %v", delErr)
		return delErr
	}
	// 尝试删除关联的ScrapePathCategory
	if delErr = db.Db.Delete(&ScrapePathCategory{}, "category_id = ?", id).Error; delErr != nil {
		helpers.AppLogger.Errorf("删除关联的ScrapePathCategory失败: %v", delErr)
		return delErr
	}
	return nil
}

// 删除电视剧分类
func DeleteTvshowCategory(id uint) error {
	delErr := db.Db.Delete(&TvShowCategory{}, id).Error
	if delErr != nil {
		helpers.AppLogger.Errorf("删除剧集分类失败: %v", delErr)
		return delErr
	}
	// 尝试删除关联的ScrapePathCategory
	if delErr = db.Db.Delete(&ScrapePathCategory{}, "category_id = ?", id).Error; delErr != nil {
		helpers.AppLogger.Errorf("删除关联的ScrapePathCategory失败: %v", delErr)
		return delErr
	}
	return nil
}

// 查询所有刮削目录分类
func GetAllScrapePathCategory(scrapePathId uint) []*ScrapePathCategory {
	var scrapePathCategory []*ScrapePathCategory
	db.Db.Model(&ScrapePathCategory{}).Where("scrape_path_id = ?", scrapePathId).Find(&scrapePathCategory)
	return scrapePathCategory
}

func SaveScrapePathCategory(spcId, scrapePathId uint, categoryId uint, fileId string) (*ScrapePathCategory, error) {
	var scrapePathCategory *ScrapePathCategory
	var err error
	if spcId > 0 {
		scrapePathCategory = GetScrapePathCategoryById(spcId)
		scrapePathCategory.FileId = fileId
		err = db.Db.Save(scrapePathCategory).Error
	} else {
		scrapePathCategory = &ScrapePathCategory{
			ScrapePathId: scrapePathId,
			CategoryId:   categoryId,
			FileId:       fileId,
		}
		// 保存新记录
		err = db.Db.Save(scrapePathCategory).Error
	}
	if err != nil {
		helpers.AppLogger.Errorf("保存刮削目录分类失败: %v", err)
		return nil, err
	}
	return scrapePathCategory, nil
}

func (s *ScrapePathCategory) Delete() error {
	return db.Db.Delete(s).Error
}
