package style

import (
	covercolor "Q115-STRM/internal/covergen/color"
	coverfont "Q115-STRM/internal/covergen/font"
	"Q115-STRM/internal/helpers"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"math"
	"os"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
)

type StyleSingleConfig struct {
	ZhFontSize   float64
	EnFontSize   float64
	TitleSpacing float64
	BlurSize     int
	ColorRatio   float64
	Resolution   string
	UsePrimary   bool
	TitleConfig  string
}

type TitleConfig struct {
	MainTitle string
	SubTitle  string
}

func ParseTitleConfig(libraryName, titleConfig string) TitleConfig {
	return TitleConfig{
		MainTitle: libraryName,
		SubTitle:  libraryName,
	}
}

func GenerateSingleCover(imagePath string, libraryName string, config StyleSingleConfig) ([]byte, error) {
	imgFile, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("打开图片失败: %w", err)
	}
	defer imgFile.Close()

	img, _, err := image.Decode(imgFile)
	if err != nil {
		return nil, fmt.Errorf("解码图片失败: %w", err)
	}

	width, height := GetImageResolution(config.Resolution)

	resizedImg := resizeAndCropImage(img, width, height)

	blurredImg := GaussianBlur(resizedImg, config.BlurSize)

	foregroundImg := resizeToFitImage(img, width, height)

	finalImg := blendImages(blurredImg, foregroundImg, config.ColorRatio)

	dominantColor := covercolor.ExtractDominantColor(img, 0.3)

	finalImg = AddGradientOverlay(finalImg, dominantColor, 0.6)

	fontManager := coverfont.GetFontManager()
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

func GetImageResolution(res string) (int, int) {
	switch res {
	case "1080p":
		return 1920, 1080
	case "720p":
		return 1280, 720
	case "480p":
		return 854, 480
	default:
		return 1920, 1080
	}
}

func resizeAndCropImage(img image.Image, targetWidth, targetHeight int) image.Image {
	if img == nil {
		return nil
	}

	srcWidth := img.Bounds().Dx()
	srcHeight := img.Bounds().Dy()

	targetRatio := float64(targetWidth) / float64(targetHeight)
	srcRatio := float64(srcWidth) / float64(srcHeight)

	var cropWidth, cropHeight int
	var offsetX, offsetY int

	if srcRatio > targetRatio {
		cropHeight = srcHeight
		cropWidth = int(float64(srcHeight) * targetRatio)
		offsetX = (srcWidth - cropWidth) / 2
		offsetY = 0
	} else {
		cropWidth = srcWidth
		cropHeight = int(float64(srcWidth) / targetRatio)
		offsetX = 0
		offsetY = (srcHeight - cropHeight) / 2
	}

	croppedImg := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))

	for y := 0; y < targetHeight; y++ {
		for x := 0; x < targetWidth; x++ {
			srcX := offsetX + int(float64(x)*float64(cropWidth)/float64(targetWidth))
			srcY := offsetY + int(float64(y)*float64(cropHeight)/float64(targetHeight))

			if srcX >= img.Bounds().Dx() || srcY >= img.Bounds().Dy() {
				continue
			}

			c := img.At(srcX, srcY)
			croppedImg.Set(x, y, c)
		}
	}

	return croppedImg
}

func GaussianBlur(img image.Image, radius int) image.Image {
	if img == nil || radius <= 0 {
		return img
	}

	bounds := img.Bounds()
	blurred := image.NewRGBA(bounds)

	kernelSize := radius*2 + 1
	kernel := make([]float64, kernelSize*kernelSize)
	sum := 0.0

	for i := -radius; i <= radius; i++ {
		for j := -radius; j <= radius; j++ {
			val := math.Exp(-(float64(i*i) + float64(j*j)) / (2.0 * float64(radius*radius)))
			kernel[(i+radius)*kernelSize+(j+radius)] = val
			sum += val
		}
	}

	for i := range kernel {
		kernel[i] /= sum
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			var r, g, b, a float64

			for i := -radius; i <= radius; i++ {
				for j := -radius; j <= radius; j++ {
					srcX := x + j
					srcY := y + i

					if srcX >= bounds.Min.X && srcX < bounds.Max.X &&
						srcY >= bounds.Min.Y && srcY < bounds.Max.Y {
						c := img.At(srcX, srcY)
						cr, cg, cb, ca := c.RGBA()
						k := kernel[(i+radius)*kernelSize+(j+radius)]
						r += float64(cr) * k
						g += float64(cg) * k
						b += float64(cb) * k
						a += float64(ca) * k
					}
				}
			}

			blurred.Set(x, y, color.RGBA{
				R: uint8(r / 256),
				G: uint8(g / 256),
				B: uint8(b / 256),
				A: uint8(a / 256),
			})
		}
	}

	return blurred
}

func resizeToFitImage(img image.Image, targetWidth, targetHeight int) image.Image {
	if img == nil {
		return nil
	}

	srcWidth := img.Bounds().Dx()
	srcHeight := img.Bounds().Dy()

	scale := math.Min(float64(targetWidth)/float64(srcWidth), float64(targetHeight)/float64(srcHeight))

	newWidth := int(float64(srcWidth) * scale)
	newHeight := int(float64(srcHeight) * scale)

	offsetX := (targetWidth - newWidth) / 2
	offsetY := (targetHeight - newHeight) / 2

	resized := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))

	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			srcX := int(float64(x) * float64(srcWidth) / float64(newWidth))
			srcY := int(float64(y) * float64(srcHeight) / float64(newHeight))

			if srcX < srcWidth && srcY < srcHeight {
				c := img.At(srcX, srcY)
				resized.Set(offsetX+x, offsetY+y, c)
			}
		}
	}

	return resized
}

func blendImages(background, foreground image.Image, ratio float64) image.Image {
	if background == nil {
		return foreground
	}
	if foreground == nil {
		return background
	}

	bounds := background.Bounds()
	result := image.NewRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			bg := background.At(x, y)
			fg := foreground.At(x, y)

			br, bgb, bba, _ := bg.RGBA()
			fr, fgb, fba, _ := fg.RGBA()

			r := uint8((float64(br)*(1-ratio) + float64(fr)*ratio) / 256)
			g := uint8((float64(bgb)*(1-ratio) + float64(fgb)*ratio) / 256)
			b := uint8((float64(bba)*(1-ratio) + float64(fba)*ratio) / 256)

			result.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	return result
}

func AddGradientOverlay(img image.Image, dominantColor covercolor.RGB, maxAlpha float64) image.Image {
	if img == nil {
		return nil
	}

	bounds := img.Bounds()
	result := image.NewRGBA(bounds)

	overlayHeight := int(float64(bounds.Dy()) * 0.4)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			baseColor := img.At(x, y)

			alpha := 0.0
			if y >= bounds.Max.Y-overlayHeight {
				alpha = maxAlpha * (1.0 - float64(bounds.Max.Y-y)/float64(overlayHeight))
			}

			br, bgb, bba, _ := baseColor.RGBA()

			r := float64(br)*(1-alpha) + float64(dominantColor.R)*alpha*256
			g := float64(bgb)*(1-alpha) + float64(dominantColor.G)*alpha*256
			b := float64(bba)*(1-alpha) + float64(dominantColor.B)*alpha*256

			result.Set(x, y, color.RGBA{
				R: uint8(r / 256),
				G: uint8(g / 256),
				B: uint8(b / 256),
				A: 255,
			})
		}
	}

	return result
}

func DrawTitles(img image.Image, mainTitle, subTitle string,
	zhFontPath, enFontPath string, zhSize, enSize, spacing float64) (image.Image, error) {
	if img == nil {
		return nil, fmt.Errorf("图片为空")
	}

	bounds := img.Bounds()
	result := image.NewRGBA(bounds)
	draw.Draw(result, bounds, img, bounds.Min, draw.Src)

	bottomPadding := int(float64(bounds.Dy()) * 0.15)

	zhFont, err := loadFont(zhFontPath)
	if err != nil {
		helpers.AppLogger.Warnf("加载中文字体失败: %v，使用默认字体", err)
	}

	enFont, err := loadFont(enFontPath)
	if err != nil {
		helpers.AppLogger.Warnf("加载英文字体失败: %v，使用默认字体", err)
	}

	if zhFont != nil {
		ctx := freetype.NewContext()
		ctx.SetDPI(72)
		ctx.SetFont(zhFont)
		ctx.SetFontSize(zhSize)
		ctx.SetClip(result.Bounds())
		ctx.SetDst(result)
		ctx.SetSrc(image.NewUniform(color.White))

		pt := freetype.Pt(bounds.Min.X+60, bounds.Max.Y-bottomPadding)

		if _, err := ctx.DrawString(mainTitle, pt); err != nil {
			helpers.AppLogger.Warnf("绘制主标题失败: %v", err)
		} else {
			bottomPadding -= int(spacing + zhSize)
		}
	}

	if enFont != nil {
		ctx := freetype.NewContext()
		ctx.SetDPI(72)
		ctx.SetFont(enFont)
		ctx.SetFontSize(enSize)
		ctx.SetClip(result.Bounds())
		ctx.SetDst(result)
		ctx.SetSrc(image.NewUniform(color.White))

		pt := freetype.Pt(bounds.Min.X+60, bounds.Max.Y-bottomPadding)

		if _, err := ctx.DrawString(subTitle, pt); err != nil {
			helpers.AppLogger.Warnf("绘制副标题失败: %v", err)
		}
	}

	return result, nil
}

func loadFont(fontPath string) (*truetype.Font, error) {
	if fontPath == "" {
		return nil, fmt.Errorf("字体路径为空")
	}

	fontData, err := os.ReadFile(fontPath)
	if err != nil {
		return nil, fmt.Errorf("读取字体文件失败: %w", err)
	}

	return truetype.Parse(fontData)
}
