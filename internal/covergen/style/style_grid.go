package style

import (
	"Q115-STRM/internal/covergen/color"
	"Q115-STRM/internal/covergen/font"
	"Q115-STRM/internal/helpers"
	"bytes"
	"fmt"
	"image"
	stdcolor "image/color"
	"image/draw"
	"image/jpeg"
	"os"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

type StyleGridConfig struct {
	ZhFontSize   float64
	EnFontSize   float64
	TitleSpacing float64
	MultiBlur    bool
	ColorRatio   float64
	Resolution   string
	UsePrimary   bool
	TitleConfig  string
}

func GenerateGridCover(imagePaths []string, libraryName string, config StyleGridConfig) ([]byte, error) {
	if len(imagePaths) == 0 {
		return nil, fmt.Errorf("没有提供图片")
	}
	
	images, err := loadImages(imagePaths)
	if err != nil {
		return nil, fmt.Errorf("加载图片失败: %w", err)
	}
	
	width, height := GetImageResolution(config.Resolution)
	
	gridImg := createGridImage(images, width, height, config.MultiBlur)
	
	dominantColor := color.ExtractDominantColor(images[0], 0.3)
	
	finalImg := AddGradientOverlay(gridImg, dominantColor, 0.6)
	
	fontManager := font.GetFontManager()
	if fontManager == nil {
		return nil, fmt.Errorf("字体管理器未初始化")
	}
	
	zhFontPath := fontManager.GetZhFontPath()
	enFontPath := fontManager.GetEnFontPath()
	
	titleCfg := ParseTitleConfig(libraryName, config.TitleConfig)
	
	finalImg, err = DrawTitles(finalImg, titleCfg.MainTitle, titleCfg.SubTitle, 
		zhFontPath, enFontPath, config.ZhFontSize, config.EnFontSize, config.TitleSpacing)
	if err != nil {
		return nil, fmt.Errorf("绘制标题失败: %w", err)
	}
	
	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, finalImg, &jpeg.Options{Quality: 90}); err != nil {
		return nil, fmt.Errorf("编码图片失败: %w", err)
	}
	
	return buf.Bytes(), nil
}

func loadImages(imagePaths []string) ([]image.Image, error) {
	images := make([]image.Image, 0, len(imagePaths))
	
	for _, path := range imagePaths {
		imgFile, err := os.Open(path)
		if err != nil {
			helpers.AppLogger.Warnf("打开图片失败: %s, 错误: %v", path, err)
			continue
		}
		
		img, _, err := image.Decode(imgFile)
		imgFile.Close()
		if err != nil {
			helpers.AppLogger.Warnf("解码图片失败: %s, 错误: %v", path, err)
			continue
		}
		
		images = append(images, img)
	}
	
	if len(images) == 0 {
		return nil, fmt.Errorf("没有成功加载任何图片")
	}
	
	for len(images) < 9 {
		images = append(images, images[0])
	}
	
	return images, nil
}

func createGridImage(images []image.Image, targetWidth, targetHeight int, multiBlur bool) image.Image {
	const gridRows = 3
	const gridCols = 3
	const gridGap = 4
	
	cellWidth := (targetWidth - gridGap*(gridCols-1)) / gridCols
	cellHeight := (targetHeight - gridGap*(gridRows-1)) / gridRows
	
	gridImg := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	
	drawBackground(gridImg, images[0], targetWidth, targetHeight)
	
	for row := 0; row < gridRows; row++ {
		for col := 0; col < gridCols; col++ {
			index := row*gridCols + col
			if index >= len(images) {
				break
			}
			
			cellImg := cropToSquare(images[index])
			cellImg = resizeImage(cellImg, cellWidth, cellHeight)
			
			x := col * (cellWidth + gridGap)
			y := row * (cellHeight + gridGap)
			
			draw.Draw(gridImg, 
				image.Rect(x, y, x+cellWidth, y+cellHeight), 
				cellImg, 
				cellImg.Bounds().Min, 
				draw.Src)
		}
	}
	
	if multiBlur {
		blurredImg := GaussianBlur(images[0], 50)
		
		for row := 0; row < gridRows; row++ {
			for col := 0; col < gridCols; col++ {
				x := col * (cellWidth + gridGap)
				y := row * (cellHeight + gridGap)
				
				for i := 0; i < gridGap && col < gridCols-1; i++ {
					for py := 0; py < cellHeight; py++ {
						c := blurredImg.At(x+cellWidth+i, y+py)
						gridImg.Set(x+cellWidth+i, y+py, c)
					}
				}
				
				for i := 0; i < gridGap && row < gridRows-1; i++ {
					for px := 0; px < cellWidth; px++ {
						c := blurredImg.At(x+px, y+cellHeight+i)
						gridImg.Set(x+px, y+cellHeight+i, c)
					}
				}
			}
		}
	}
	
	return gridImg
}

func cropToSquare(img image.Image) image.Image {
	if img == nil {
		return nil
	}
	
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	
	size := width
	if height < width {
		size = height
	}
	
	offsetX := (width - size) / 2
	offsetY := (height - size) / 2
	
	square := image.NewRGBA(image.Rect(0, 0, size, size))
	
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			srcX := offsetX + x
			srcY := offsetY + y
			
			if srcX >= bounds.Min.X && srcX < bounds.Max.X && 
			   srcY >= bounds.Min.Y && srcY < bounds.Max.Y {
				c := img.At(srcX, srcY)
				square.Set(x, y, c)
			}
		}
	}
	
	return square
}

func resizeImage(img image.Image, targetWidth, targetHeight int) image.Image {
	if img == nil {
		return nil
	}
	
	srcWidth := img.Bounds().Dx()
	srcHeight := img.Bounds().Dy()
	
	resized := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	
	for y := 0; y < targetHeight; y++ {
		for x := 0; x < targetWidth; x++ {
			srcX := x * srcWidth / targetWidth
			srcY := y * srcHeight / targetHeight
			
			if srcX < srcWidth && srcY < srcHeight {
				c := img.At(srcX, srcY)
				resized.Set(x, y, c)
			}
		}
	}
	
	return resized
}

func drawBackground(dst draw.Image, src image.Image, width, height int) {
	if src == nil {
		return
	}
	
	blurred := GaussianBlur(src, 50)
	
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			srcX := x * src.Bounds().Dx() / width
			srcY := y * src.Bounds().Dy() / height
			
			if srcX < src.Bounds().Dx() && srcY < src.Bounds().Dy() {
				c := blurred.At(srcX, srcY)
				dst.Set(x, y, c)
			}
		}
	}
}
