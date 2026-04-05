package covergen

import (
	"Q115-STRM/internal/covergen/style"
	"Q115-STRM/internal/helpers"
	embyclient "Q115-STRM/internal/embyclient-rest-go"
	"fmt"
	"sync"
	"time"
)

var (
	engineInstance *CoverGenEngine
	engineOnce     sync.Once
)

type CoverGenEngine struct {
	isRunning      bool
	mutex          sync.Mutex
	config         *CoverGenConfig
	lastRunResults []CoverGenResult
	lastRunTime    string
}

func GetCoverGenEngine() *CoverGenEngine {
	engineOnce.Do(func() {
		engineInstance = &CoverGenEngine{
			config: getDefaultConfig(),
		}
	})
	return engineInstance
}

func getDefaultConfig() *CoverGenConfig {
	return &CoverGenConfig{
		ZhFontSize:   170,
		EnFontSize:   75,
		TitleSpacing: 40,
		BlurSize:     50,
		ColorRatio:   0.8,
		Resolution:   "480p",
		UsePrimary:   false,
		MultiBlur:    true,
		TitleConfig:  "# 配置封面标题（按媒体库名称对应）\n# 媒体库名称:\n# - 主标题\n# - 副标题\n",
		SortBy:       CoverSortRandom,
	}
}

func (e *CoverGenEngine) GetConfig() *CoverGenConfig {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	
	return e.config
}

func (e *CoverGenEngine) UpdateConfig(newConfig *CoverGenConfig) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	
	e.config = newConfig
	return nil
}

func (e *CoverGenEngine) GetStatus() *CoverGenStatus {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	
	return &CoverGenStatus{
		IsRunning:      e.isRunning,
		LastRunTime:    e.lastRunTime,
		LastRunResults: e.lastRunResults,
	}
}

func (e *CoverGenEngine) GenerateCovers(req *CoverGenRequest) (*CoverGenSummary, error) {
	e.mutex.Lock()
	if e.isRunning {
		e.mutex.Unlock()
		return nil, fmt.Errorf("封面生成任务正在运行中，请稍后重试")
	}
	e.isRunning = true
	e.mutex.Unlock()
	
	defer func() {
		e.mutex.Lock()
		e.isRunning = false
		e.mutex.Unlock()
	}()
	
	client, err := NewEmbyImageClient()
	if err != nil {
		return nil, fmt.Errorf("创建Emby客户端失败: %w", err)
	}
	
	libraries, err := client.client.GetAllMediaLibraries()
	if err != nil {
		return nil, fmt.Errorf("获取媒体库列表失败: %w", err)
	}
	
	var targetLibraries []embyclient.EmbyLibrary
	if len(req.LibraryIDs) > 0 {
		for _, lib := range libraries {
			for _, id := range req.LibraryIDs {
				if lib.ID == id {
					targetLibraries = append(targetLibraries, lib)
					break
				}
			}
		}
	} else {
		targetLibraries = libraries
	}
	
	if len(targetLibraries) == 0 {
		return nil, fmt.Errorf("没有找到目标媒体库")
	}
	
	results := make([]CoverGenResult, 0, len(targetLibraries))
	
	for _, lib := range targetLibraries {
		startTime := time.Now()
		
		result, err := e.generateCoverForLibrary(client, lib, req.Style)
		durationMs := time.Since(startTime).Milliseconds()
		
		if err != nil {
			helpers.AppLogger.Errorf("生成媒体库 %s 封面失败: %v", lib.Name, err)
			results = append(results, CoverGenResult{
				LibraryID:   lib.ID,
				LibraryName: lib.Name,
				Success:     false,
				Message:     err.Error(),
				DurationMs:  durationMs,
			})
		} else {
			results = append(results, CoverGenResult{
				LibraryID:   lib.ID,
				LibraryName: lib.Name,
				Success:     true,
				Message:     "封面已更新",
				DurationMs:  durationMs,
			})
		}
	}
	
	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}
	
	e.mutex.Lock()
	e.lastRunResults = results
	e.lastRunTime = time.Now().Format("2006-01-02 15:04:05")
	e.mutex.Unlock()
	
	return &CoverGenSummary{
		Total:   len(results),
		Success: successCount,
		Failed:  len(results) - successCount,
		Results: results,
	}, nil
}

func (e *CoverGenEngine) generateCoverForLibrary(client *EmbyImageClient, library embyclient.EmbyLibrary, styleType CoverStyle) error {
	switch styleType {
	case CoverStyleSingle:
		return e.generateSingleCover(client, library)
	case CoverStyleGrid:
		return e.generateGridCover(client, library)
	default:
		return fmt.Errorf("不支持的封面样式: %s", styleType)
	}
}

func (e *CoverGenEngine) generateSingleCover(client *EmbyImageClient, library embyclient.EmbyLibrary) error {
	images, err := client.DownloadImagesForLibrary(library.ID, 1, e.config.UsePrimary, e.config.SortBy)
	if err != nil {
		return fmt.Errorf("下载图片失败: %w", err)
	}
	defer CleanupTempImages(images)
	
	if len(images) == 0 {
		return fmt.Errorf("没有可用的图片")
	}
	
	config := style.StyleSingleConfig{
		ZhFontSize:   e.config.ZhFontSize,
		EnFontSize:   e.config.EnFontSize,
		TitleSpacing: e.config.TitleSpacing,
		BlurSize:     e.config.BlurSize,
		ColorRatio:   e.config.ColorRatio,
		Resolution:   e.config.Resolution,
		UsePrimary:   e.config.UsePrimary,
		TitleConfig:  e.config.TitleConfig,
	}
	
	coverData, err := style.GenerateSingleCover(images[0].LocalPath, library.Name, config)
	if err != nil {
		return fmt.Errorf("生成封面失败: %w", err)
	}
	
	if err := client.UploadCoverToEmby(library.ID, coverData); err != nil {
		return fmt.Errorf("上传封面到Emby失败: %w", err)
	}
	
	helpers.AppLogger.Infof("媒体库 %s 单图封面生成并上传成功", library.Name)
	return nil
}

func (e *CoverGenEngine) generateGridCover(client *EmbyImageClient, library embyclient.EmbyLibrary) error {
	images, err := client.DownloadImagesForLibrary(library.ID, 9, e.config.UsePrimary, e.config.SortBy)
	if err != nil {
		return fmt.Errorf("下载图片失败: %w", err)
	}
	defer CleanupTempImages(images)
	
	if len(images) == 0 {
		return fmt.Errorf("没有可用的图片")
	}
	
	imagePaths := make([]string, 0, len(images))
	for _, img := range images {
		imagePaths = append(imagePaths, img.LocalPath)
	}
	
	config := style.StyleGridConfig{
		ZhFontSize:   e.config.ZhFontSize,
		EnFontSize:   e.config.EnFontSize,
		TitleSpacing: e.config.TitleSpacing,
		MultiBlur:    e.config.MultiBlur,
		ColorRatio:   e.config.ColorRatio,
		Resolution:   e.config.Resolution,
		UsePrimary:   e.config.UsePrimary,
		TitleConfig:  e.config.TitleConfig,
	}
	
	coverData, err := style.GenerateGridCover(imagePaths, library.Name, config)
	if err != nil {
		return fmt.Errorf("生成封面失败: %w", err)
	}
	
	if err := client.UploadCoverToEmby(library.ID, coverData); err != nil {
		return fmt.Errorf("上传封面到Emby失败: %w", err)
	}
	
	helpers.AppLogger.Infof("媒体库 %s 九宫格封面生成并上传成功", library.Name)
	return nil
}

func (e *CoverGenEngine) PreviewCover(libraryID string, styleType CoverStyle, customTitle []string) ([]byte, error) {
	client, err := NewEmbyImageClient()
	if err != nil {
		return nil, fmt.Errorf("创建Emby客户端失败: %w", err)
	}
	
	libraries, err := client.client.GetAllMediaLibraries()
	if err != nil {
		return nil, fmt.Errorf("获取媒体库列表失败: %w", err)
	}
	
	var targetLibrary *embyclient.EmbyLibrary
	for _, lib := range libraries {
		if lib.ID == libraryID {
			targetLibrary = &lib
			break
		}
	}
	
	if targetLibrary == nil {
		return nil, fmt.Errorf("未找到媒体库: %s", libraryID)
	}
	
	images, err := client.DownloadImagesForLibrary(libraryID, 1, e.config.UsePrimary, e.config.SortBy)
	if err != nil {
		return nil, fmt.Errorf("下载图片失败: %w", err)
	}
	defer CleanupTempImages(images)
	
	if len(images) == 0 {
		return nil, fmt.Errorf("没有可用的图片")
	}
	
	switch styleType {
	case CoverStyleSingle:
		config := style.StyleSingleConfig{
			ZhFontSize:   e.config.ZhFontSize,
			EnFontSize:   e.config.EnFontSize,
			TitleSpacing: e.config.TitleSpacing,
			BlurSize:     e.config.BlurSize,
			ColorRatio:   e.config.ColorRatio,
			Resolution:   e.config.Resolution,
			UsePrimary:   e.config.UsePrimary,
			TitleConfig:  e.config.TitleConfig,
		}
		
		if len(customTitle) >= 2 {
			config.TitleConfig = fmt.Sprintf("%s:\n- %s\n- %s", targetLibrary.Name, customTitle[0], customTitle[1])
		}
		
		return style.GenerateSingleCover(images[0].LocalPath, targetLibrary.Name, config)
		
	case CoverStyleGrid:
		images9, err := client.DownloadImagesForLibrary(libraryID, 9, e.config.UsePrimary, e.config.SortBy)
		if err != nil {
			return nil, fmt.Errorf("下载图片失败: %w", err)
		}
		defer CleanupTempImages(images9)
		
		imagePaths := make([]string, 0, len(images9))
		for _, img := range images9 {
			imagePaths = append(imagePaths, img.LocalPath)
		}
		
		config := style.StyleGridConfig{
			ZhFontSize:   e.config.ZhFontSize,
			EnFontSize:   e.config.EnFontSize,
			TitleSpacing: e.config.TitleSpacing,
			MultiBlur:    e.config.MultiBlur,
			ColorRatio:   e.config.ColorRatio,
			Resolution:   e.config.Resolution,
			UsePrimary:   e.config.UsePrimary,
			TitleConfig:  e.config.TitleConfig,
		}
		
		if len(customTitle) >= 2 {
			config.TitleConfig = fmt.Sprintf("%s:\n- %s\n- %s", targetLibrary.Name, customTitle[0], customTitle[1])
		}
		
		return style.GenerateGridCover(imagePaths, targetLibrary.Name, config)
		
	default:
		return nil, fmt.Errorf("不支持的封面样式: %s", styleType)
	}
}
