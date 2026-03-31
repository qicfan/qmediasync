package emby

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"strings"
	"time"

	embyclient "Q115-STRM/internal/embyclient-rest-go"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
)

// LibraryPosterGridCols 封面网格列数
const LibraryPosterGridCols = 4

// LibraryPosterGridRows 封面网格行数
const LibraryPosterGridRows = 5

// LibraryPosterItemWidth 单个缩略图宽度
const LibraryPosterItemWidth = 200

// LibraryPosterItemHeight 单个缩略图高度
const LibraryPosterItemHeight = 280

// LibraryPosterMaxItems 最多使用多少个媒体项
const LibraryPosterMaxItems = LibraryPosterGridCols * LibraryPosterGridRows

// GenerateAllLibraryPosters 为所有媒体库生成封面
func GenerateAllLibraryPosters() error {
	config, err := models.GetEmbyConfig()
	if err != nil {
		return fmt.Errorf("获取Emby配置失败: %w", err)
	}
	if config.EnableLibraryPoster != 1 {
		helpers.AppLogger.Info("媒体库封面生成功能未启用，跳过")
		return nil
	}
	if config.EmbyUrl == "" || config.EmbyApiKey == "" {
		return fmt.Errorf("Emby配置不完整，URL或API Key为空")
	}

	client := embyclient.NewClient(config.EmbyUrl, config.EmbyApiKey)

	libraries, err := client.GetAllMediaLibraries()
	if err != nil {
		return fmt.Errorf("获取媒体库列表失败: %w", err)
	}

	// 根据用户配置过滤媒体库
	var filteredLibraries []embyclient.EmbyLibrary
	if config.SyncAllLibraries == 1 {
		filteredLibraries = libraries
	} else {
		// 解析选中的媒体库列表
		var selectedIds []string
		if err := json.Unmarshal([]byte(config.SelectedLibraries), &selectedIds); err != nil {
			return fmt.Errorf("解析选中媒体库失败: %w", err)
		}
		for _, lib := range libraries {
			if contains(selectedIds, lib.ID) {
				filteredLibraries = append(filteredLibraries, lib)
			}
		}
	}

	helpers.AppLogger.Infof("开始为 %d 个媒体库生成封面", len(filteredLibraries))

	for _, lib := range filteredLibraries {
		helpers.AppLogger.Infof("正在处理媒体库: %s (%s)", lib.Name, lib.ID)
		if err := generateSingleLibraryPoster(client, lib.ID, lib.Name); err != nil {
			helpers.AppLogger.Errorf("为媒体库 '%s' 生成封面失败: %v", lib.Name, err)
			continue
		}
		helpers.AppLogger.Infof("媒体库 '%s' 封面生成成功", lib.Name)
		time.Sleep(500 * time.Millisecond) // 避免请求过于频繁
	}

	helpers.AppLogger.Info("所有媒体库封面生成完成")
	return nil
}

// generateSingleLibraryPoster 为单个媒体库生成封面
func generateSingleLibraryPoster(client *embyclient.Client, libraryId, libraryName string) error {
	// 1. 获取媒体项
	items, err := client.GetMediaItemsForPoster(libraryId, LibraryPosterMaxItems)
	if err != nil {
		return fmt.Errorf("获取媒体项失败: %w", err)
	}
	if len(items) == 0 {
		helpers.AppLogger.Warnf("媒体库 '%s' 没有可用的带图片媒体项，跳过", libraryName)
		return nil
	}
	helpers.AppLogger.Infof("媒体库 '%s' 找到 %d 个带图片的媒体项", libraryName, len(items))

	// 2. 下载图片
	images := make([]image.Image, 0, len(items))
	for i, item := range items {
		if i >= LibraryPosterMaxItems {
			break
		}
		data, contentType, err := client.DownloadItemImage(item.Id, LibraryPosterItemWidth*2)
		if err != nil {
			helpers.AppLogger.Warnf("下载媒体项 '%s' 图片失败: %v", item.Name, err)
			continue
		}

		// 解码图片
		var img image.Image
		if strings.Contains(contentType, "jpeg") || strings.Contains(contentType, "jpg") {
			img, err = jpeg.Decode(bytes.NewReader(data))
		} else if strings.Contains(contentType, "png") {
			img, err = png.Decode(bytes.NewReader(data))
		} else {
			img, _, err = image.Decode(bytes.NewReader(data))
		}
		if err != nil {
			helpers.AppLogger.Warnf("解码媒体项 '%s' 图片失败: %v", item.Name, err)
			continue
		}

		images = append(images, img)
	}

	if len(images) == 0 {
		helpers.AppLogger.Warnf("媒体库 '%s' 没有成功下载到任何图片，跳过", libraryName)
		return nil
	}
	helpers.AppLogger.Infof("成功下载 %d 张图片用于生成封面", len(images))

	// 3. 拼接网格封面
	composedImg := composePosterGrid(images)

	// 4. 编码为JPEG
	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, composedImg, &jpeg.Options{Quality: 85}); err != nil {
		return fmt.Errorf("编码封面图片失败: %w", err)
	}

	// 5. 上传到Emby
	if err := client.UploadItemImage(libraryId, buf.Bytes(), "image/jpeg"); err != nil {
		return fmt.Errorf("上传封面到Emby失败: %w", err)
	}

	return nil
}

// composePosterGrid 将图片拼接成网格封面
func composePosterGrid(images []image.Image) image.Image {
	totalWidth := LibraryPosterGridCols * LibraryPosterItemWidth
	totalHeight := LibraryPosterGridRows * LibraryPosterItemHeight

	// 创建画布，白色背景
	dst := image.NewRGBA(image.Rect(0, 0, totalWidth, totalHeight))
	white := image.White
	draw.Draw(dst, dst.Bounds(), white, image.Point{}, draw.Src)

	for i, img := range images {
		if i >= LibraryPosterGridCols*LibraryPosterGridRows {
			break
		}

		col := i % LibraryPosterGridCols
		row := i / LibraryPosterGridCols

		x := col * LibraryPosterItemWidth
		y := row * LibraryPosterItemHeight

		// 缩放图片到目标尺寸并居中裁剪
		scaledImg := resizeAndCropImage(img, LibraryPosterItemWidth, LibraryPosterItemHeight)

		// 绘制到画布
		destRect := image.Rect(x, y, x+LibraryPosterItemWidth, y+LibraryPosterItemHeight)
		draw.Draw(dst, destRect, scaledImg, image.Point{}, draw.Over)
	}

	return dst
}

// resizeAndCropImage 缩放并居中裁剪图片到指定尺寸
func resizeAndCropImage(img image.Image, targetW, targetH int) image.Image {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	// 计算缩放比例，保持宽高比并填充目标尺寸
	srcRatio := float64(srcW) / float64(srcH)
	targetRatio := float64(targetW) / float64(targetH)

	var scaleW, scaleH int
	if srcRatio > targetRatio {
		// 原图更宽，按高度缩放
		scaleH = targetH
		scaleW = int(float64(srcW) * float64(targetH) / float64(srcH))
	} else {
		// 原图更高，按宽度缩放
		scaleW = targetW
		scaleH = int(float64(srcH) * float64(targetW) / float64(srcW))
	}

	// 创建目标图片
	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))

	// 简单的缩放实现（最近邻插值，足够用于封面）
	for y := 0; y < targetH; y++ {
		for x := 0; x < targetW; x++ {
			// 计算在缩放图中的坐标
			srcX := x * scaleW / targetW
			srcY := y * scaleH / targetH
			// 计算在原图中的坐标（居中偏移）
			srcX = srcX + (scaleW-targetW)/2
			srcY = srcY + (scaleH-targetH)/2
			// 边界检查
			srcX = clamp(srcX, 0, scaleW-1)
			srcY = clamp(srcY, 0, scaleH-1)
			// 采样原图像素
			origX := srcX * srcW / scaleW
			origY := srcY * srcH / scaleH
			origX = clamp(origX, 0, srcW-1)
			origY = clamp(origY, 0, srcH-1)
			dst.Set(x, y, img.At(bounds.Min.X+origX, bounds.Min.Y+origY))
		}
	}

	return dst
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
