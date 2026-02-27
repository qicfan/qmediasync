package controllers

import (
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
	"Q115-STRM/internal/synccron"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// 基础配置
// TMDB API KEY或者Access Token设置
// 1. TMDB API KEY或者Access Token设置查询和保存
// 2. AI识别设置查询和保存
// 分类规则
// 1 分类列表（分电影和电视剧）
// 2 分类添加和编辑(分电影和电视剧)
// 分类数据
// 1. 电影分类数组
// 2. 电视剧分类数组
// 语言数组
// 1. 语言数组

type TmdbSettings struct {
	TmdbApiKey        string `json:"tmdb_api_key" form:"tmdb_api_key"`
	TmdbAccessToken   string `json:"tmdb_access_token" form:"tmdb_access_token"`
	TmdbUrl           string `json:"tmdb_url" form:"tmdb_url"`
	TmdbImageUrl      string `json:"tmdb_image_url" form:"tmdb_image_url"`
	TmdbLanguage      string `json:"tmdb_language" form:"tmdb_language"`
	TmdbImageLanguage string `json:"tmdb_image_language" form:"tmdb_image_language"`
	TmdbEnableProxy   bool   `json:"tmdb_enable_proxy" form:"tmdb_enable_proxy"`
}

type AiSettings struct {
	EnableAi    models.AiAction `json:"enable_ai" form:"enable_ai"`
	AiApiKey    string          `json:"ai_api_key" form:"ai_api_key"`
	AiBaseUrl   string          `json:"ai_base_url" form:"ai_base_url"`
	AiModelName string          `json:"ai_model_name" form:"ai_model_name"`
	AiPrompt    string          `json:"ai_prompt" form:"ai_prompt"`
	AiTimeout   int             `json:"ai_timeout" form:"ai_timeout"`
}

type MovieCategoryReq struct {
	ID            uint     `json:"id" form:"id"`
	Name          string   `json:"name" form:"name"`
	LanguageArray []string `json:"language_array" form:"language_array"`
	GenreIDArray  []int    `json:"genre_id_array" form:"genre_id_array"`
}

type TvshowCategoryReq struct {
	ID           uint     `json:"id" form:"id"`
	Name         string   `json:"name" form:"name"`
	CountryArray []string `json:"country_array" form:"country_array"`
	GenreIDArray []int    `json:"genre_id_array" form:"genre_id_array"`
}

// GetTmdbSettings 获取TMDB设置
// @Summary 获取TMDB设置
// @Description 获取当前的TMDB API配置
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/tmdb [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetTmdbSettings(c *gin.Context) {
	tmdbSettings := TmdbSettings{
		TmdbApiKey:        models.GlobalScrapeSettings.TmdbApiKey,
		TmdbAccessToken:   models.GlobalScrapeSettings.TmdbAccessToken,
		TmdbUrl:           models.GlobalScrapeSettings.TmdbUrl,
		TmdbImageUrl:      models.GlobalScrapeSettings.TmdbImageUrl,
		TmdbLanguage:      models.GlobalScrapeSettings.TmdbLanguage,
		TmdbImageLanguage: models.GlobalScrapeSettings.TmdbImageLanguage,
		TmdbEnableProxy:   models.GlobalScrapeSettings.TmdbEnableProxy,
	}
	c.JSON(http.StatusOK, APIResponse[TmdbSettings]{Code: Success, Message: "", Data: tmdbSettings})
}

// SaveTmdbSettings 保存TMDB设置
// @Summary 保存TMDB设置
// @Description 保存或更新TMDB API配置
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Param tmdb_api_key body string false "TMDB API Key"
// @Param tmdb_access_token body string false "TMDB Access Token"
// @Param tmdb_url body string false "TMDB服务器地址"
// @Param tmdb_image_url body string false "TMDB图片服务器地址"
// @Param tmdb_language body string false "TMDB默认语言"
// @Param tmdb_image_language body string false "TMDB图片语言"
// @Param tmdb_enable_proxy body boolean false "是否启用代理"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/tmdb [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func SaveTmdbSettings(c *gin.Context) {
	reqData := TmdbSettings{}
	if err := c.ShouldBindJSON(&reqData); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	if err := models.GlobalScrapeSettings.SaveTmdb(reqData.TmdbApiKey, reqData.TmdbAccessToken, reqData.TmdbUrl, reqData.TmdbImageUrl, reqData.TmdbLanguage, reqData.TmdbImageLanguage, reqData.TmdbEnableProxy); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "保存TMDB设置成功", Data: nil})
}

// TestTmdbSettings 测试TMDB设置
// @Summary 测试TMDB连接
// @Description 测试指定的TMDB配置是否有效
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Param tmdb_api_key body string false "TMDB API Key"
// @Param tmdb_access_token body string false "TMDB Access Token"
// @Param tmdb_url body string false "TMDB服务器地址"
// @Param tmdb_image_url body string false "TMDB图片服务器地址"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/tmdb-test [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func TestTmdbSettings(c *gin.Context) {
	reqData := TmdbSettings{}
	if err := c.ShouldBindJSON(&reqData); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	tmpScrapeSetting := &models.ScrapeSettings{
		TmdbApiKey:        reqData.TmdbApiKey,
		TmdbAccessToken:   reqData.TmdbAccessToken,
		TmdbUrl:           reqData.TmdbUrl,
		TmdbImageUrl:      reqData.TmdbImageUrl,
		TmdbLanguage:      reqData.TmdbLanguage,
		TmdbImageLanguage: reqData.TmdbImageLanguage,
		TmdbEnableProxy:   reqData.TmdbEnableProxy,
	}
	testResult := tmpScrapeSetting.TestTmdb()
	c.JSON(http.StatusOK, APIResponse[bool]{Code: Success, Message: "", Data: testResult})
}

// SaveAiSettings 保存AI识别设置
// @Summary 保存AI识别设置
// @Description 保存或更新AI识别模型的配置
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Param enable_ai body string false "是否启用AI识别"
// @Param ai_api_key body string false "AI API Key"
// @Param ai_base_url body string false "AI服务器地址"
// @Param ai_model_name body string false "AI模型名称"
// @Param ai_timeout body integer false "AI超时时间"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/ai-settings [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func SaveAiSettings(c *gin.Context) {
	reqData := AiSettings{}
	if err := c.ShouldBindJSON(&reqData); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	if err := models.GlobalScrapeSettings.SaveAi(reqData.AiApiKey, reqData.AiBaseUrl, reqData.AiModelName, reqData.AiTimeout); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "保存AI识别设置成功", Data: nil})
}

// TestAiSettings 测试AI识别设置
// @Summary 测试AI识别连接
// @Description 测试指定的AI模型配置是否有效
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Param ai_api_key body string true "AI API Key"
// @Param ai_base_url body string true "AI服务器地址"
// @Param ai_model_name body string true "AI模型名称"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/ai-test [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func TestAiSettings(c *gin.Context) {
	reqData := AiSettings{}
	if err := c.ShouldBindJSON(&reqData); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	if reqData.AiApiKey == "" || reqData.AiBaseUrl == "" || reqData.AiModelName == "" {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "必须配置API Key、接口地址模型名称", Data: nil})
		return
	}
	tmpScrapeSetting := &models.ScrapeSettings{
		AiApiKey:    reqData.AiApiKey,
		AiBaseUrl:   reqData.AiBaseUrl,
		AiModelName: reqData.AiModelName,
	}
	testResult := tmpScrapeSetting.TestAi()
	if testResult != nil {
		c.JSON(http.StatusOK, APIResponse[error]{Code: BadRequest, Message: testResult.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[error]{Code: Success, Message: "测试AI识别成功", Data: nil})
}

// GetAiSettings 获取AI识别设置
// @Summary 获取AI识别设置
// @Description 获取当前的AI识别模型配置
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/ai-settings [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetAiSettings(c *gin.Context) {
	aiSettings := AiSettings{
		AiApiKey:    models.GlobalScrapeSettings.AiApiKey,
		AiBaseUrl:   models.GlobalScrapeSettings.AiBaseUrl,
		AiModelName: models.GlobalScrapeSettings.AiModelName,
		AiTimeout:   models.GlobalScrapeSettings.AiTimeout,
	}
	c.JSON(http.StatusOK, APIResponse[AiSettings]{Code: Success, Message: "", Data: aiSettings})
}

// GetMovieGenre 获取电影分类
// @Summary 获取电影分类
// @Description 获取TMDB电影分类列表
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/movie-genre [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetMovieGenre(c *gin.Context) {
	movieGenre := helpers.MovieGenres
	c.JSON(http.StatusOK, APIResponse[[]helpers.Genre]{Code: Success, Message: "", Data: movieGenre})
}

// GetTvshowGenre 获取电视剧分类
// @Summary 获取电视剧分类
// @Description 获取TMDB电视剧分类列表
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/tvshow-genre [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetTvshowGenre(c *gin.Context) {
	tvshowGenre := helpers.TvshowGenres
	c.JSON(http.StatusOK, APIResponse[[]helpers.Genre]{Code: Success, Message: "", Data: tvshowGenre})
}

// GetLanguage 获取语言列表
// @Summary 获取语言列表
// @Description 获取所有支持的语言列表
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/language [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetLanguage(c *gin.Context) {
	language := helpers.Languages
	c.JSON(http.StatusOK, APIResponse[[]helpers.Language]{Code: Success, Message: "", Data: language})
}

// GetCountries 获取国家列表
// @Summary 获取国家列表
// @Description 获取所有支持的国家列表
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/countries [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetCountries(c *gin.Context) {
	countries := helpers.Countries
	c.JSON(http.StatusOK, APIResponse[[]helpers.Country]{Code: Success, Message: "", Data: countries})
}

// GetMovieCategories 获取电影分类列表
// @Summary 获取电影分类列表
// @Description 获取已配置的电影分类列表
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/movie-categories [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetMovieCategories(c *gin.Context) {
	movieGenre := models.GetMovieCategory()
	c.JSON(http.StatusOK, APIResponse[[]*models.MovieCategory]{Code: Success, Message: "", Data: movieGenre})
}

// GetTvshowCategories 获取电视剧分类列表
// @Summary 获取电视剧分类列表
// @Description 获取已配置的电视剧分类列表
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/tvshow-categories [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetTvshowCategories(c *gin.Context) {
	tvshowGenre := models.GetTvshowCategory()
	c.JSON(http.StatusOK, APIResponse[[]*models.TvShowCategory]{Code: Success, Message: "", Data: tvshowGenre})
}

// SaveMovieCategory 保存电影分类
// @Summary 保存电影分类
// @Description 创建或更新电影分类配置
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Param id body integer false "分类ID，不填为新增"
// @Param name body string true "分类名称"
// @Param language_array body []string true "语言代码数组"
// @Param genre_id_array body []integer true "分类ID数组"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/movie-categories [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func SaveMovieCategory(c *gin.Context) {
	reqData := MovieCategoryReq{}
	if err := c.ShouldBindJSON(&reqData); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	movieCategory := &models.MovieCategory{
		BaseModel: models.BaseModel{
			ID: reqData.ID,
		},
		Name: reqData.Name,
	}
	if err := movieCategory.Save(reqData.Name, reqData.GenreIDArray, reqData.LanguageArray); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "保存电影分类成功", Data: nil})
}

// SaveTvshowCategory 保存电视剧分类
// @Summary 保存电视剧分类
// @Description 创建或更新电视剧分类配置
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Param id body integer false "分类ID，不填为新增"
// @Param name body string true "分类名称"
// @Param country_array body []string true "国家代码数组"
// @Param genre_id_array body []integer true "分类ID数组"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/tvshow-categories [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func SaveTvshowCategory(c *gin.Context) {
	reqData := TvshowCategoryReq{}
	if err := c.ShouldBindJSON(&reqData); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	tvshowCategory := &models.TvShowCategory{
		BaseModel: models.BaseModel{
			ID: reqData.ID,
		},
		Name: reqData.Name,
	}
	if err := tvshowCategory.Save(reqData.Name, reqData.GenreIDArray, reqData.CountryArray); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "保存电视剧分类成功", Data: nil})
}

// DeleteMovieCategory 删除电影分类
// @Summary 删除电影分类
// @Description 删除指定的电影分类
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Param id path integer true "分类ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/movie-categories/:id [delete]
// @Security JwtAuth
// @Security ApiKeyAuth
func DeleteMovieCategory(c *gin.Context) {
	id := helpers.StringToInt(c.Param("id"))

	if err := models.DeleteMovieCategory(uint(id)); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "删除电影分类成功", Data: nil})
}

// DeleteTvshowCategory 删除电视剧分类
// @Summary 删除电视剧分类
// @Description 删除指定的电视剧分类
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Param id path integer true "分类ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/tvshow-categories/:id [delete]
// @Security JwtAuth
// @Security ApiKeyAuth
func DeleteTvshowCategory(c *gin.Context) {
	id := helpers.StringToInt(c.Param("id"))
	if err := models.DeleteTvshowCategory(uint(id)); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "删除电视剧分类成功", Data: nil})
}

// GetScrapePathes 获取刮削路径列表
// @Summary 获取刮削路径列表
// @Description 获取所有配置的刮削路径列表
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/pathes [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetScrapePathes(c *gin.Context) {
	sourceType := c.Query("source_type")
	scrapePathes := models.GetScrapePathes(sourceType)
	for _, scrapePath := range scrapePathes {
		// 检查是否正在运行
		scrapePath.IsTaskRunning = synccron.CheckNewTaskStatus(scrapePath.ID, synccron.SyncTaskTypeScrape)
	}
	c.JSON(http.StatusOK, APIResponse[[]*models.ScrapePath]{Code: Success, Message: "", Data: scrapePathes})
}

// GetScrapePath 获取刮削路径详情
// @Summary 获取刮削路径详情
// @Description 根据ID获取指定刮削路径的详细配置
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Param id path integer true "刮削路径ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/pathes/:id [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetScrapePath(c *gin.Context) {
	id := helpers.StringToInt(c.Param("id"))
	scrapePath := models.GetScrapePathByID(uint(id))
	if scrapePath == nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "刮削目录不存在", Data: nil})
		return
	}
	if scrapePath.EnableAi == "" {
		scrapePath.EnableAi = models.AiActionOff
	}
	c.JSON(http.StatusOK, APIResponse[*models.ScrapePath]{Code: Success, Message: "", Data: scrapePath})
}

// SaveScrapePath 保存刮削路径
// @Summary 保存刮削路径
// @Description 创建或更新刮削路径配置
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Param id body integer false "路径ID，不填为新增"
// @Param source_type body integer true "来源类型"
// @Param source_path body string true "来源路径"
// @Param dest_path body string true "目标路径"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/pathes [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func SaveScrapePath(c *gin.Context) {
	reqData := models.ScrapePath{}
	if err := c.ShouldBindJSON(&reqData); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	// 如果是115，用ID查询实际的目录
	if reqData.SourceType == models.SourceType115 {
		// 用ID查询实际的目录
		account, err := models.GetAccountById(reqData.AccountId)
		if err != nil {
			c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
			return
		}
		// 用ID查询实际的目录
		sourcePath := models.GetPathByPathFileId(account, reqData.SourcePathId)
		if sourcePath == "" {
			c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "查询来源目录失败", Data: nil})
			return
		}
		reqData.SourcePath = sourcePath
		if reqData.DestPathId != "" {
			destPath := models.GetPathByPathFileId(account, reqData.DestPathId)
			if destPath == "" {
				c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "查询目标目录失败", Data: nil})
				return
			}
			reqData.DestPath = destPath
		}
	}
	helpers.AppLogger.Infof("最大线程数：%d", reqData.MaxThreads)
	if err := reqData.Save(); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "保存刮削目录成功", Data: nil})
}

// DeleteScrapePath 删除刮削路径
// @Summary 删除刮削路径
// @Description 删除指定的刮削路径
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Param id path integer true "路径ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/pathes/:id [delete]
// @Security JwtAuth
// @Security ApiKeyAuth
func DeleteScrapePath(c *gin.Context) {
	id := helpers.StringToInt(c.Param("id"))
	if err := models.DeleteScrapePath(uint(id)); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "删除刮削目录成功", Data: nil})
}

// ScanScrapePath 启动刮削路径扫描
// @Summary 启动刮削任务
// @Description 启动指定刮削路径的扫描任务
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Param id body integer true "路径ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/pathes/start [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func ScanScrapePath(c *gin.Context) {
	type ScanScrapePathReq struct {
		ID uint `json:"id" form:"id"`
	}
	reqData := ScanScrapePathReq{}
	if err := c.ShouldBindJSON(&reqData); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	// 查询ScrapePath
	scrapePath := models.GetScrapePathByID(reqData.ID)
	if scrapePath == nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "刮削目录不存在", Data: nil})
		return
	}
	if err := synccron.AddNewSyncTask(scrapePath.ID, synccron.SyncTaskTypeScrape); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "添加刮削任务失败: " + err.Error(), Data: nil})
		return
	}

	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "刮削任务已添加到队列", Data: nil})
}

// GetScrapeRecords 获取刮削记录
// @Summary 获取刮削记录
// @Description 分页获取刮削任务记录，支持筛选
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Param page query integer false "页码"
// @Param pageSize query integer false "每页数量"
// @Param type query string false "媒体类型（movie/tvshow）"
// @Param status query string false "记录状态"
// @Param name query string false "流屛名称"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/records [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func GetScrapeRecords(c *gin.Context) {
	page := helpers.StringToInt(c.Query("page"))
	if page == 0 {
		page = 1
	}
	pageSize := helpers.StringToInt(c.Query("pageSize"))
	if pageSize == 0 {
		pageSize = 100
	}
	mediaType := c.Query("type")
	status := c.Query("status")
	name := c.Query("name")
	scrapePathesCache := make(map[uint]*models.ScrapePath)
	total, scrapeRecords := models.GetScrapeMediaFiles(page, pageSize, mediaType, status, name)
	type scrapeMediaResp struct {
		ID              uint   `json:"id"`
		Type            string `json:"type"`
		Path            string `json:"path"`
		FileName        string `json:"file_name"`
		MediaName       string `json:"media_name"`
		OriginalName    string `json:"original_name"`
		Year            int    `json:"year"`
		SeasonNumber    int    `json:"season_number"`
		EpisodeNumber   int    `json:"episode_number"`
		Genre           string `json:"genre"`
		Country         string `json:"country"`
		Language        string `json:"language"`
		Status          string `json:"status"`
		TmdbID          int64  `json:"tmdb_id"`
		NewPath         string `json:"new_path"` // 二级分类 + 文件夹
		NewFile         string `json:"new_file"` // 新文件名
		EpisodeName     string `json:"episode_name"`
		Resolution      string `json:"resolution"`       // 分辨率
		ResolutionLevel string `json:"resolution_level"` // 分辨率等级
		IsHDR           bool   `json:"is_hdr"`           // 是否HDR
		AudioCount      int    `json:"audio_count"`      // 音频轨道数量
		SubtitleCount   int    `json:"subtitle_count"`   // 字幕轨道数量
		CreatedAt       int64  `json:"created_at"`       // 创建时间
		UpdatedAt       int64  `json:"updated_at"`       // 更新时间
		ScrapedAt       int64  `json:"scraped_at"`       // 刮削时间
		ScannedAt       int64  `json:"scanned_at"`       // 扫描时间
		RenamedAt       int64  `json:"renamed_at"`       // 整理时间
		FailedReason    string `json:"failed_reason"`    // 失败原因
		CategoryName    string `json:"category_name"`    // 分类名称
		NewDestPath     string `json:"new_dest_path"`    // 新路径
		NewDestName     string `json:"new_dest_name"`    // 新文件名
		PathIsScraping  bool   `json:"path_is_scraping"` // 是否正在刮削
		PathIsRenaming  bool   `json:"path_is_renaming"` // 是否正在整理
		SourceFullPath  string `json:"source_full_path"` // 原始路径
		DestFullPath    string `json:"dest_full_path"`   // 目标路径
		SourceType      string `json:"source_type"`      // 原始类型
		RenameType      string `json:"rename_type"`      // 重命名类型
		ScrapeType      string `json:"scrape_type"`      // 刮削类型
	}
	type scrapeListResp struct {
		Total int64              `json:"total"`
		List  []*scrapeMediaResp `json:"list"`
	}
	resp := scrapeListResp{
		Total: total,
		List:  make([]*scrapeMediaResp, len(scrapeRecords)),
	}
	for i, scrapeMedia := range scrapeRecords {
		var scrapePath *models.ScrapePath
		var ok bool
		if scrapePath, ok = scrapePathesCache[scrapeMedia.ScrapePathId]; !ok {
			scrapePath = models.GetScrapePathByID(scrapeMedia.ScrapePathId)
			if scrapePath == nil {
				continue
			}
			scrapePathesCache[scrapeMedia.ScrapePathId] = scrapePath
		}
		if scrapePath == nil {
			continue
		}
		sourcePath := scrapeMedia.GetRemoteFullMoviePath()
		destPath := scrapeMedia.GetDestFullMoviePath()
		if scrapeMedia.MediaType == models.MediaTypeTvShow {
			sourcePath = scrapeMedia.GetRemoteFullSeasonPath()
			destPath = scrapeMedia.GetDestFullSeasonPath()
		}
		sourcePath = filepath.ToSlash(filepath.Join(sourcePath, scrapeMedia.VideoFilename))
		destPath = filepath.ToSlash(filepath.Join(destPath, scrapeMedia.NewVideoBaseName+scrapeMedia.VideoExt))

		resp.List[i] = &scrapeMediaResp{
			Type:            string(scrapeMedia.MediaType),
			ID:              scrapeMedia.ID,
			Path:            scrapeMedia.Path,
			FileName:        scrapeMedia.VideoFilename,
			MediaName:       scrapeMedia.Name,
			Year:            scrapeMedia.Year,
			SeasonNumber:    scrapeMedia.SeasonNumber,
			EpisodeNumber:   scrapeMedia.EpisodeNumber,
			Status:          string(scrapeMedia.Status),
			TmdbID:          scrapeMedia.TmdbId,
			NewPath:         filepath.ToSlash(filepath.Join(scrapeMedia.CategoryName, scrapeMedia.NewPathName)),
			NewFile:         scrapeMedia.NewVideoBaseName + scrapeMedia.VideoExt,
			Resolution:      scrapeMedia.Resolution,
			ResolutionLevel: scrapeMedia.ResolutionLevel,
			IsHDR:           scrapeMedia.IsHDR,
			AudioCount:      len(scrapeMedia.AudioCodec),
			SubtitleCount:   len(scrapeMedia.SubtitleCodec),
			CreatedAt:       scrapeMedia.CreatedAt,
			UpdatedAt:       scrapeMedia.UpdatedAt,
			ScrapedAt:       scrapeMedia.ScrapeTime,
			ScannedAt:       scrapeMedia.ScanTime,
			RenamedAt:       scrapeMedia.RenameTime,
			CategoryName:    scrapeMedia.CategoryName,
			NewDestPath:     scrapeMedia.NewPathName,
			NewDestName:     scrapeMedia.NewVideoBaseName + scrapeMedia.VideoExt,
			FailedReason:    scrapeMedia.FailedReason,
			SourceFullPath:  sourcePath,
			DestFullPath:    destPath,
			SourceType:      string(scrapePath.SourceType),
			RenameType:      string(scrapePath.RenameType),
			ScrapeType:      string(scrapePath.ScrapeType),
		}
		if scrapeMedia.MediaType == models.MediaTypeTvShow {
			if scrapeMedia.MediaEpisode != nil {
				resp.List[i].EpisodeName = scrapeMedia.MediaEpisode.EpisodeName
			}
			resp.List[i].NewPath = filepath.ToSlash(filepath.Join(resp.List[i].NewPath, scrapeMedia.NewSeasonPathName))
		}
		if scrapePath.IsScraping && slices.Contains([]models.ScrapeMediaStatus{models.ScrapeMediaStatusScanned, models.ScrapeMediaStatusScraping}, scrapeMedia.Status) {
			resp.List[i].PathIsScraping = scrapePath.IsScraping
		}
		// if scrapeMedia.Media != nil {
		// 	if len(scrapeMedia.Media.Genres) > 0 {
		// 		for _, genre := range scrapeMedia.Media.Genres {
		// 			resp.List[i].Genre += genre.Name + ", "
		// 		}
		// 	}
		// 	if len(scrapeMedia.Media.OriginCountry) > 0 {
		// 		countries := ""
		// 		for _, country := range scrapeMedia.Media.OriginCountry {
		// 			if countries == "" {
		// 				countries = helpers.GetCountryName(country)
		// 			} else {
		// 				countries += ", " + helpers.GetCountryName(country)
		// 			}

		// 		}
		// 		resp.List[i].Country = countries
		// 	}
		// 	if scrapeMedia.Media.OriginalLanguage != "" {
		// 		resp.List[i].Language = helpers.GetLanguageName(scrapeMedia.Media.OriginalLanguage)
		// 	}
		// 	if scrapeMedia.Media.OriginalName != "" {
		// 		resp.List[i].OriginalName = scrapeMedia.Media.OriginalName
		// 	}
		// }
	}
	c.JSON(http.StatusOK, APIResponse[scrapeListResp]{Code: Success, Message: "", Data: resp})
}

// ScrapeTmpImage 获取刮削临时图片
// @Summary 获取临时图片
// @Description 返回刮削临时文件中的图片内容
// @Tags 刮削管理
// @Accept json
// @Produce image/jpeg
// @Param path query string true "图片路径"
// @Param type query string true "媒体类型"
// @Success 200 {file} file "图片文件"
// @Failure 200 {object} object
// @Router /scrape/tmp-image [get]
func ScrapeTmpImage(c *gin.Context) {
	imagePath := c.Query("path")
	mediaType := models.MediaType(c.Query("type"))
	if imagePath == "" {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "路径不能为空", Data: nil})
		return
	}
	imageRootPath := filepath.Join(helpers.ConfigDir, "tmp", "刮削临时文件")
	if mediaType == models.MediaTypeTvShow {
		imageRootPath = filepath.Join(imageRootPath, "电视剧")
	} else {
		imageRootPath = filepath.Join(imageRootPath, "电影或其他")
	}
	imagePath = filepath.Join(imageRootPath, imagePath)
	// 读取文件
	file, err := os.Open(imagePath)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "读取文件失败: " + err.Error(), Data: nil})
		return
	}
	defer file.Close()
	// 读取文件内容
	fileContent, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "读取文件内容失败: " + err.Error(), Data: nil})
		return
	}
	// 返回文件内容
	c.Data(http.StatusOK, "image/jpeg", fileContent)
}

// ExportScrapeRecords 导出Scrape记录
// @Summary 导出刮削记录
// @Description 将选中的刮削记录导出json文件
// @Tags 刮削管理
// @Accept json
// @Produce application/json
// @Param ids query string true "记录ID，用逗号分隔"
// @Success 200 {file} file "json文件"
// @Failure 200 {object} object
// @Router /scrape/records/export [get]
// @Security JwtAuth
// @Security ApiKeyAuth
func ExportScrapeRecords(c *gin.Context) {
	ids := c.Query("ids")
	if ids == "" {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "请选择要导出的记录", Data: nil})
		return
	}
	// ids用,分隔
	idList := strings.Split(ids, ",")
	// 转成uint数组
	idUintList := make([]uint, 0)
	for _, id := range idList {
		idUint, _ := strconv.ParseUint(id, 10, 32)
		idUintList = append(idUintList, uint(idUint))
	}
	scrapeRecords := models.GetScrapeMediaFilesByIds(idUintList)
	if len(scrapeRecords) == 0 {
		c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "没有找到要导出的记录", Data: nil})
		return
	}
	// 生成txt文件内容
	type exportMediaResult struct {
		Path          string `json:"path"`
		FileName      string `json:"filename"`
		TvPath        string `json:"tv_path"`
		Name          string `json:"name"`
		Year          int    `json:"year"`
		SeasonNumber  int    `json:"season_number"`
		EpisodeNumber int    `json:"episode_number"`
	}
	exportMediaList := make([]exportMediaResult, 0)
	for _, scrapeMedia := range scrapeRecords {
		exportMediaList = append(exportMediaList, exportMediaResult{
			Path:          scrapeMedia.Path,
			FileName:      scrapeMedia.VideoFilename,
			TvPath:        scrapeMedia.TvshowPath,
			Name:          scrapeMedia.Name,
			Year:          scrapeMedia.Year,
			SeasonNumber:  scrapeMedia.SeasonNumber,
			EpisodeNumber: scrapeMedia.EpisodeNumber,
		})
	}
	// json 格式化exportMediaList
	exportMediaListJson, err := json.MarshalIndent(exportMediaList, "", "  ")
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "格式化导出记录失败: " + err.Error(), Data: nil})
		return
	}
	c.Header("Content-Disposition", "attachment; filename=刮削记录.json")
	// 返回json文件内容,触发浏览器下载
	c.Data(http.StatusOK, "application/json", exportMediaListJson)
}

// ReScrape 重新刮削记录
// @Summary 重新刮削
// @Description 重新刮削指定记录，使用新提供的名称和年份
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Param id body integer true "记录ID"
// @Param name body string true "新名称"
// @Param year body integer true "新年份"
// @Param tmdb_id body integer false "TMDBid"
// @Param season body integer false "季数"
// @Param episode body integer false "集数"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/re-scrape [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func ReScrape(c *gin.Context) {
	type reScrapeReq struct {
		ID      uint  `json:"id"`
		TmdbId  int64 `json:"tmdb_id"`
		Season  int   `json:"season"`
		Episode int   `json:"episode"`
	}
	var req reScrapeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "请求参数错误: " + err.Error(), Data: nil})
		return
	}
	// 使用ID查询ScrapeMediaFile
	scrapeMedia := models.GetScrapeMediaFileById(req.ID)
	if scrapeMedia == nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "没有找到要重新刮削的记录", Data: nil})
		return
	}
	scrapePath := models.GetScrapePathByID(scrapeMedia.ScrapePathId)
	if scrapePath == nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "没有找到要重新刮削的记录的刮削目录", Data: nil})
		return
	}
	oldStatus := scrapeMedia.Status
	err := scrapeMedia.ReScrape("", 0, req.TmdbId, req.Season, req.Episode)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "重新刮削失败: " + err.Error(), Data: nil})
		return
	}
	if oldStatus == models.ScrapeMediaStatusRenamed {
		synccron.StartScrapeRollbackCron() // 触发一次
		c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "操作成功，已将文件移动并重命名到源目录，下次扫描时会使用新的名称和年份进行刮削", Data: nil})
	} else {
		data := make(map[string]any)
		data["name"] = scrapeMedia.Name
		data["year"] = scrapeMedia.Year
		data["tmdb_id"] = scrapeMedia.TmdbId
		c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "操作成功，下次扫描时会使用新的名称和年份进行刮削", Data: data})
	}
}

// 清除所有刮削失败的记录
func ClearFailedScrapeRecords(c *gin.Context) {
	err := models.ClearFailedScrapeRecords([]uint{})
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "操作成功，所有失败的刮削记录已清除", Data: nil})
}

// FinishScrapeMediaFile 完成刮削记录
// @Summary 标记记录为已整理
// @Description 完成整理中的记录，不继续整理
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Param id body integer true "记录ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/finish [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func FinishScrapeMediaFile(c *gin.Context) {
	type finishScrapeReq struct {
		ID uint `json:"id"`
	}
	var req finishScrapeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "请求参数错误: " + err.Error(), Data: nil})
		return
	}
	// 使用ID查询ScrapeMediaFile
	scrapeMedia := models.GetScrapeMediaFileById(req.ID)
	if scrapeMedia == nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "没有找到要整理的记录", Data: nil})
		return
	}
	scrapeMedia.FinishFromRenaming()
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "操作成功，记录已标记为已整理", Data: nil})
}

// DeleteScrapeMediaFile 删除刮削记录
// @Summary 删除刮削记录
// @Description 删除选中的刮削记录
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Param ids query string true "记录ID，用逗号分隔"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/records [delete]
// @Security JwtAuth
// @Security ApiKeyAuth
func DeleteScrapeMediaFile(c *gin.Context) {
	ids := c.Query("ids")
	if ids == "" {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "请选择要导出的记录", Data: nil})
		return
	}
	// ids用,分隔
	idList := strings.Split(ids, ",")
	// 转成uint数组
	idUintList := make([]uint, 0)
	for _, id := range idList {
		idUint, _ := strconv.ParseUint(id, 10, 32)
		idUintList = append(idUintList, uint(idUint))
	}
	// 删除记录
	err := models.ClearFailedScrapeRecords(idUintList)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "删除记录失败: " + err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "操作成功，记录已删除", Data: nil})
}

// RenameFailedScrapeMediaFile 标记记录为待整理
// @Summary 标记为待整理
// @Description 将选中的刮削记录标记为待整理
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Param ids query string true "记录ID，用逗号分隔"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/rename-failed [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func RenameFailedScrapeMediaFile(c *gin.Context) {
	ids := c.Query("ids")
	if ids == "" {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "请选择要导出的记录", Data: nil})
		return
	}
	// ids用,分隔
	idList := strings.Split(ids, ",")
	// 转成uint数组
	idUintList := make([]uint, 0)
	for _, id := range idList {
		idUint, _ := strconv.ParseUint(id, 10, 32)
		idUintList = append(idUintList, uint(idUint))
	}
	// 将这些ID对应的记录标记为待整理
	err := models.RenameFailedScrapeRecords(idUintList)
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "标记记录失败: " + err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "操作成功，所选记录已标记为待整理", Data: nil})
}

// ToggleScrapePathCron 切换刮削路径的定时任务
// @Summary 切换定时刮削
// @Description 开启或关闭刮削路径的定时戮削任务
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Param id body integer true "路径ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/pathes/toggle-cron [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func ToggleScrapePathCron(c *gin.Context) {
	type toggleScrapePathCronReq struct {
		ID uint `json:"id"`
	}
	var req toggleScrapePathCronReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "请求参数错误: " + err.Error(), Data: nil})
		return
	}
	// 使用ID查询ScrapePath
	scrapePath := models.GetScrapePathByID(req.ID)
	if scrapePath == nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "没有找到要操作的记录", Data: nil})
		return
	}
	// 切换定时刮削
	err := scrapePath.ToggleCron()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "切换定时刮削失败: " + err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "操作成功，定时刮削已切换", Data: nil})
}

// StopScrape 停止刮削任务
// @Summary 停止刮削任务
// @Description 停止指定刮削路径的正运行刮削任务
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Param id body integer true "路径ID"
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/pathes/stop [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func StopScrape(c *gin.Context) {
	type ScanScrapePathReq struct {
		ID uint `json:"id" form:"id"`
	}
	var req ScanScrapePathReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "请求参数错误: " + err.Error(), Data: nil})
		return
	}
	synccron.CancelNewSyncTask(req.ID, synccron.SyncTaskTypeScrape)
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "操作成功，刮削任务已停止", Data: nil})
}

// TruncateAllScrapeRecords 一键清空所有刮削记录
// @Summary 清空所有记录
// @Description 清除数据库中所有的刮削记录
// @Tags 刮削管理
// @Accept json
// @Produce json
// @Success 200 {object} object
// @Failure 200 {object} object
// @Router /scrape/truncate-all [post]
// @Security JwtAuth
// @Security ApiKeyAuth
func TruncateAllScrapeRecords(c *gin.Context) {
	err := models.TruncateAllScrapeRecords()
	if err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "清空刮削记录失败: " + err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "操作成功，所有刮削记录已清空", Data: nil})
}

func SaveScrapeStrmPath(c *gin.Context) {
	var req struct {
		ScrapePathID uint   `json:"scrape_path_id" form:"scrape_path_id"` // 刮削目录ID
		SyncPathIDs  []uint `json:"sync_path_ids" form:"sync_path_ids"`   // 同步目录ID列表
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		helpers.AppLogger.Errorf("绑定JSON数据失败: %v", err)
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "绑定JSON数据失败: " + err.Error(), Data: nil})
		return
	}
	scrapePath := models.GetScrapePathByID(req.ScrapePathID)
	if scrapePath == nil {
		helpers.AppLogger.Errorf("刮削目录不存在: %v", req.ScrapePathID)
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "刮削目录不存在", Data: nil})
		return
	}
	if err := scrapePath.SaveStrmPath(req.SyncPathIDs); err != nil {
		helpers.AppLogger.Errorf("保存刮削目录关联的同步目录失败: %v", err)
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "保存刮削目录关联的同步目录失败: " + err.Error(), Data: nil})
		return
	}
	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "操作成功", Data: nil})
}

func GetScrapeStrmPaths(c *gin.Context) {
	type GetScrapePathStrmPathsReq struct {
		ID uint `json:"scrape_path_id" form:"scrape_path_id"`
	}
	var req GetScrapePathStrmPathsReq
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "请求参数错误: " + err.Error(), Data: nil})
		return
	}
	scrapePath := models.GetScrapePathByID(req.ID)
	if scrapePath == nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "没有找到要操作的记录", Data: nil})
		return
	}
	ssp := scrapePath.GetRelatStrmPath()
	syncPathIds := make([]uint, 0)
	for _, sp := range ssp {
		syncPathIds = append(syncPathIds, sp.StrmPathID)
	}

	c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "操作成功", Data: syncPathIds})
}

type TmdbSearchResp struct {
	TmdbID        int    `json:"tmdb_id"`
	Title         string `json:"title"`
	OriginalTitle string `json:"original_title"`
	Year          int    `json:"year"`
	PosterUrl     string `json:"poster_url"`
	Overview      string `json:"overview"`
}

func TmdbSearch(c *gin.Context) {
	type TmdbSearchReq struct {
		Name   string           `json:"name" form:"name"`
		Year   int              `json:"year" form:"year"`
		Type   models.MediaType `json:"type" form:"type" binding:"required"`
		TmdbId int              `json:"tmdb_id" form:"tmdb_id"`
	}
	var req TmdbSearchReq
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "请求参数错误: " + err.Error(), Data: nil})
		return
	}
	if req.Name == "" && req.TmdbId == 0 {
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "请输入名称或TMDB ID", Data: nil})
		return
	}
	tmdbClient := models.GlobalScrapeSettings.GetTmdbClient()
	switch req.Type {
	case models.MediaTypeMovie:
		if req.TmdbId == 0 {
			// 搜索电影
			resp, err := tmdbClient.SearchMovie(req.Name, req.Year, models.GlobalScrapeSettings.GetTmdbLanguage(), true, false)
			if err != nil {
				c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "搜索电影失败: " + err.Error(), Data: nil})
				return
			}
			if len(resp.Results) == 0 {
				c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "没有找到电影", Data: nil})
				return
			}
			// 转换为响应结构体
			tmdbResp := make([]TmdbSearchResp, 0)
			for _, r := range resp.Results {
				tmdbResp = append(tmdbResp, TmdbSearchResp{
					TmdbID:        int(r.ID),
					Title:         r.Title,
					OriginalTitle: r.OriginalTitle,
					Year:          helpers.ParseYearFromDate(r.ReleaseDate),
					PosterUrl:     models.GetTmdbImageUrl(r.PosterPath),
					Overview:      r.Overview,
				})
			}
			c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "操作成功", Data: tmdbResp})
			return
		} else {
			resp, err := tmdbClient.GetMovieDetail(int64(req.TmdbId), models.GlobalScrapeSettings.GetTmdbLanguage())
			if err != nil {
				c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "获取电影详情失败: " + err.Error(), Data: nil})
				return
			}
			tmdbResp := make([]TmdbSearchResp, 0)
			// 转换为响应结构体
			tmdbResp = append(tmdbResp, TmdbSearchResp{
				TmdbID:        int(resp.ID),
				Title:         resp.Title,
				OriginalTitle: resp.OriginalTitle,
				Year:          helpers.ParseYearFromDate(resp.ReleaseDate),
				PosterUrl:     models.GetTmdbImageUrl(resp.PosterPath),
				Overview:      resp.Overview,
			})
			c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "操作成功", Data: tmdbResp})
			return
		}
	case models.MediaTypeTvShow:
		if req.TmdbId == 0 {
			// 搜索电视剧
			resp, err := tmdbClient.SearchTv(req.Name, req.Year, models.GlobalScrapeSettings.GetTmdbLanguage(), true)
			if err != nil {
				c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "搜索电视剧失败: " + err.Error(), Data: nil})
				return
			}
			if len(resp.Results) == 0 {
				c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "没有找到电视剧", Data: nil})
				return
			}
			// 转换为响应结构体
			tmdbResp := make([]TmdbSearchResp, 0)
			for _, r := range resp.Results {
				tmdbResp = append(tmdbResp, TmdbSearchResp{
					TmdbID:        int(r.ID),
					Title:         r.Name,
					OriginalTitle: r.OriginalName,
					Year:          helpers.ParseYearFromDate(r.FirstAirDate),
					PosterUrl:     models.GetTmdbImageUrl(r.PosterPath),
					Overview:      r.Overview,
				})
			}
			c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "操作成功", Data: tmdbResp})
			return
		} else {
			resp, err := tmdbClient.GetTvDetail(int64(req.TmdbId), models.GlobalScrapeSettings.GetTmdbLanguage())
			if err != nil {
				c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "获取电视剧详情失败: " + err.Error(), Data: nil})
				return
			}
			tmdbResp := make([]TmdbSearchResp, 0)
			// 转换为响应结构体
			tmdbResp = append(tmdbResp, TmdbSearchResp{
				TmdbID:        int(resp.ID),
				Title:         resp.Name,
				OriginalTitle: resp.OriginalName,
				Year:          helpers.ParseYearFromDate(resp.FirstAirDate),
				PosterUrl:     models.GetTmdbImageUrl(resp.PosterPath),
				Overview:      resp.Overview,
			})
			c.JSON(http.StatusOK, APIResponse[any]{Code: Success, Message: "操作成功", Data: tmdbResp})
			return
		}
	default:
		c.JSON(http.StatusOK, APIResponse[any]{Code: BadRequest, Message: "请求参数错误: 类型必须是 movie 或 tv_show", Data: nil})
		return
	}

}
