package emby

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"math/rand"
	"os"
	"strings"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	embyclient "Q115-STRM/internal/embyclient-rest-go"
	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/models"
)

// 画布全局参数
const (
	PosterCanvasW  = 1000 // 画布宽度
	PosterCanvasH  = 1500 // 画布高度
	PosterCols     = 3    // 网格列数
	PosterRows     = 3    // 网格行数
	PosterMaxItems = 9    // 最多使用9张海报

	// 布局参数
	PosterMarginX = 80  // 左右边距
	PosterMarginY = 200 // 上下边距（底部为文字预留空间）
	PosterGutterX = 20  // 横向间距
	PosterGutterY = 20  // 纵向间距

	// 文字参数
	PosterBottomMargin = 100 // 文字底部到画布下方的距离
	PosterLineSpacing  = 20  // 中英文行间距
	PosterFontSizeEN   = 80  // 英文字号
	PosterFontSizeCH   = 120 // 中文字号

	// 海报效果参数
	PosterBorderRadius  = 8  // 圆角半径
	PosterShadowOffsetX = 3  // 阴影X偏移
	PosterShadowOffsetY = 3  // 阴影Y偏移
	PosterShadowBlur    = 5  // 阴影模糊半径
	PosterShadowAlpha   = 51 // 阴影透明度 20% (255*0.2≈51)
)

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
		time.Sleep(500 * time.Millisecond)
	}

	helpers.AppLogger.Info("所有媒体库封面生成完成")
	return nil
}

// generateSingleLibraryPoster 为单个媒体库生成封面
func generateSingleLibraryPoster(client *embyclient.Client, libraryId, libraryName string) error {
	// 1. 获取媒体项（请求更多以筛选有图片的）
	items, err := client.GetMediaItemsForPoster(libraryId, PosterMaxItems*2)
	if err != nil {
		return fmt.Errorf("获取媒体项失败: %w", err)
	}
	if len(items) == 0 {
		helpers.AppLogger.Warnf("媒体库 '%s' 没有可用的带图片媒体项，跳过", libraryName)
		return nil
	}

	// 2. 下载图片
	images := make([]image.Image, 0, len(items))
	for _, item := range items {
		if len(images) >= PosterMaxItems {
			break
		}
		data, contentType, err := client.DownloadItemImage(item.Id, 400)
		if err != nil {
			continue
		}
		var img image.Image
		if strings.Contains(contentType, "jpeg") || strings.Contains(contentType, "jpg") {
			img, err = jpeg.Decode(bytes.NewReader(data))
		} else if strings.Contains(contentType, "png") {
			img, err = png.Decode(bytes.NewReader(data))
		} else {
			img, _, err = image.Decode(bytes.NewReader(data))
		}
		if err != nil {
			continue
		}
		images = append(images, img)
	}

	if len(images) == 0 {
		return fmt.Errorf("媒体库 '%s' 没有成功下载到任何图片", libraryName)
	}

	// 不足9张时循环填充
	for len(images) < PosterMaxItems {
		images = append(images, images[len(images)%len(images)])
	}

	// 3. 生成封面：背景 → 布局拼接 → 文字渲染
	poster := generatePoster(images, libraryName)

	// 4. 编码为JPEG
	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, poster, &jpeg.Options{Quality: 90}); err != nil {
		return fmt.Errorf("编码封面图片失败: %w", err)
	}

	// 5. 上传到Emby
	if err := client.UploadItemImage(libraryId, buf.Bytes(), "image/jpeg"); err != nil {
		return fmt.Errorf("上传封面到Emby失败: %w", err)
	}

	return nil
}

// generatePoster 生成完整封面：背景 → 布局 → 文字
func generatePoster(images []image.Image, libraryName string) *image.RGBA {
	// 模块一：背景生成
	bg := generateGradientBackground(images[0])

	// 模块二：布局拼接
	composed := composePosterLayout(bg, images)

	// 模块三：文字渲染
	renderLibraryName(composed, libraryName)

	return composed
}

// ============================================================
// 模块一：背景生成
// ============================================================

type hslColor struct {
	H, S, L float64
}

// rgbToHSL RGB转HSL
func rgbToHSL(r, g, b uint8) hslColor {
	rf, gf, bf := float64(r)/255, float64(g)/255, float64(b)/255
	maxC := math.Max(rf, math.Max(gf, bf))
	minC := math.Min(rf, math.Min(gf, bf))
	delta := maxC - minC
	l := (maxC + minC) / 2
	var s float64
	if delta == 0 {
		s = 0
	} else {
		s = delta / (1 - math.Abs(2*l-1))
	}
	var h float64
	if delta != 0 {
		switch {
		case maxC == rf:
			h = 60 * (math.Mod((gf-bf)/delta, 6))
		case maxC == gf:
			h = 60 * ((bf-rf)/delta + 2)
		case maxC == bf:
			h = 60 * ((rf-gf)/delta + 4)
		}
	}
	if h < 0 {
		h += 360
	}
	return hslColor{H: h, S: s, L: l}
}

// hslToRGB HSL转RGB
func hslToRGB(h, s, l float64) (uint8, uint8, uint8) {
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return uint8((r + m) * 255), uint8((g + m) * 255), uint8((b + m) * 255)
}

// extractDominantColor 从图片提取主色调（采样法）
func extractDominantColor(img image.Image) (r, g, b uint8, ok bool) {
	// 缩放到100x100进行采样
	scaled := resizeImage(img, 100, 100)
	bounds := scaled.Bounds()

	// 统计颜色频率
	colorCount := make(map[uint32]int)
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 2 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 2 {
			cr, cg, cb, _ := scaled.At(x, y).RGBA()
			// 量化到32级减少颜色空间
			qr := (cr >> 11) << 5
			qg := (cg >> 11) << 5
			qb := (cb >> 11) << 5
			key := uint32(qr)<<16 | uint32(qg)<<8 | uint32(qb)
			colorCount[key]++
		}
	}

	// 按频率排序，取前10个
	type colorFreq struct {
		color uint32
		count int
	}
	var topColors []colorFreq
	for color, count := range colorCount {
		topColors = append(topColors, colorFreq{color, count})
	}
	// 简单排序取前10
	for i := 0; i < len(topColors) && i < 10; i++ {
		for j := i + 1; j < len(topColors); j++ {
			if topColors[j].count > topColors[i].count {
				topColors[i], topColors[j] = topColors[j], topColors[i]
			}
		}
	}

	// HSL合规校验
	for _, cf := range topColors {
		cr := uint8((cf.color >> 16) & 0xFF)
		cg := uint8((cf.color >> 8) & 0xFF)
		cb := uint8(cf.color & 0xFF)
		hsl := rgbToHSL(cr, cg, cb)
		// 明度合规：0.2 ≤ L ≤ 0.8
		if hsl.L < 0.2 || hsl.L > 0.8 {
			continue
		}
		// 饱和度合规：S ≥ 0.15
		if hsl.S < 0.15 {
			continue
		}
		return cr, cg, cb, true
	}

	return 0, 0, 0, false
}

// generateRandomHSL 生成合规随机色
func generateRandomHSL() (uint8, uint8, uint8) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	h := float64(rng.Intn(360))
	s := 0.3 + rng.Float64()*0.4
	l := 0.3 + rng.Float64()*0.4
	return hslToRGB(h, s, l)
}

// generateGradientBackground 生成左深右浅渐变背景
func generateGradientBackground(firstPoster image.Image) *image.RGBA {
	// 提取主色调
	bgR, bgG, bgB, ok := extractDominantColor(firstPoster)
	var mainHSL hslColor
	if ok {
		mainHSL = rgbToHSL(bgR, bgG, bgB)
	} else {
		// 兜底：合规随机色
		fallbackR, fallbackG, fallbackB := generateRandomHSL()
		mainHSL = rgbToHSL(fallbackR, fallbackG, fallbackB)
	}

	// 生成渐变两端色值
	// 左端：L * 0.7
	leftL := mainHSL.L * 0.7
	leftR, leftG, leftB := hslToRGB(mainHSL.H, mainHSL.S, leftL)
	// 右端：L * 1.3，最高0.95
	rightL := math.Min(mainHSL.L*1.3, 0.95)
	rightR, rightG, rightB := hslToRGB(mainHSL.H, mainHSL.S, rightL)

	// 创建画布
	canvas := image.NewRGBA(image.Rect(0, 0, PosterCanvasW, PosterCanvasH))

	// 逐列生成渐变
	for x := 0; x < PosterCanvasW; x++ {
		ratio := float64(x) / float64(PosterCanvasW-1)
		// 插值
		ir := uint8(float64(leftR)*(1-ratio) + float64(rightR)*ratio)
		ig := uint8(float64(leftG)*(1-ratio) + float64(rightG)*ratio)
		ib := uint8(float64(leftB)*(1-ratio) + float64(rightB)*ratio)
		// 设置整列像素
		for y := 0; y < PosterCanvasH; y++ {
			canvas.Set(x, y, color.RGBA{R: ir, G: ig, B: ib, A: 255})
		}
	}

	// 叠加12%黑色遮罩提升对比度
	for y := 0; y < PosterCanvasH; y++ {
		for x := 0; x < PosterCanvasW; x++ {
			cr, cg, cb, _ := canvas.At(x, y).RGBA()
			// 混合：原色 * 0.88 + 黑色 * 0.12
			nr := uint8(float64(cr>>8) * 0.88)
			ng := uint8(float64(cg>>8) * 0.88)
			nb := uint8(float64(cb>>8) * 0.88)
			canvas.Set(x, y, color.RGBA{R: nr, G: ng, B: nb, A: 255})
		}
	}

	return canvas
}

// ============================================================
// 模块二：布局拼接
// ============================================================

// composePosterLayout 将海报拼接到背景上（3x3网格，圆角+阴影）
func composePosterLayout(bg *image.RGBA, images []image.Image) *image.RGBA {
	// 计算海报尺寸
	availableW := PosterCanvasW - 2*PosterMarginX
	availableH := PosterCanvasH - 2*PosterMarginY
	posterW := (availableW - (PosterCols-1)*PosterGutterX) / PosterCols
	posterH := (availableH - (PosterRows-1)*PosterGutterY) / PosterRows

	// 先渲染阴影层，再渲染海报主体
	// 阴影层
	for i := 0; i < PosterMaxItems; i++ {
		col := i % PosterCols
		row := i / PosterCols
		x := PosterMarginX + col*(posterW+PosterGutterX)
		y := PosterMarginY + row*(posterH+PosterGutterY)
		renderShadow(bg, x+PosterShadowOffsetX, y+PosterShadowOffsetY, posterW, posterH)
	}

	// 海报主体（带圆角）
	for i := 0; i < PosterMaxItems; i++ {
		col := i % PosterCols
		row := i / PosterCols
		x := PosterMarginX + col*(posterW+PosterGutterX)
		y := PosterMarginY + row*(posterH+PosterGutterY)
		scaled := resizeAndCropImage(images[i], posterW, posterH)
		drawRoundedRect(bg, x, y, posterW, posterH, PosterBorderRadius, scaled)
	}

	// 整体提升5%对比度
	enhanceContrast(bg, 1.05)

	return bg
}

// renderShadow 渲染阴影
func renderShadow(dst *image.RGBA, x, y, w, h int) {
	// 简化的阴影：在指定区域叠加半透明黑色
	for sy := 0; sy < h; sy++ {
		for sx := 0; sx < w; sx++ {
			dx := x + sx
			dy := y + sy
			if dx < 0 || dx >= PosterCanvasW || dy < 0 || dy >= PosterCanvasH {
				continue
			}
			// 边缘渐变（模拟模糊效果）
			alpha := PosterShadowAlpha
			// 左边缘
			if sx < PosterShadowBlur {
				alpha = alpha * sx / PosterShadowBlur
			}
			// 右边缘
			if sx > w-PosterShadowBlur {
				alpha = alpha * (w - sx) / PosterShadowBlur
			}
			// 上边缘
			if sy < PosterShadowBlur {
				a := alpha * sy / PosterShadowBlur
				if a < alpha {
					alpha = a
				}
			}
			// 下边缘
			if sy > h-PosterShadowBlur {
				a := alpha * (h - sy) / PosterShadowBlur
				if a < alpha {
					alpha = a
				}
			}

			if alpha > 0 {
				cr, cg, cb, _ := dst.At(dx, dy).RGBA()
				nr := uint8(float64(cr>>8) * (1 - float64(alpha)/255))
				ng := uint8(float64(cg>>8) * (1 - float64(alpha)/255))
				nb := uint8(float64(cb>>8) * (1 - float64(alpha)/255))
				dst.Set(dx, dy, color.RGBA{R: nr, G: ng, B: nb, A: 255})
			}
		}
	}
}

// drawRoundedRect 绘制圆角矩形区域（从src图像取像素）
func drawRoundedRect(dst *image.RGBA, x, y, w, h, radius int, src image.Image) {
	for sy := 0; sy < h; sy++ {
		for sx := 0; sx < w; sx++ {
			// 圆角检测
			if !isInsideRoundedRect(sx, sy, w, h, radius) {
				continue
			}
			dx := x + sx
			dy := y + sy
			if dx < 0 || dx >= PosterCanvasW || dy < 0 || dy >= PosterCanvasH {
				continue
			}
			srcBounds := src.Bounds()
			srcX := srcBounds.Min.X + sx*srcBounds.Dx()/w
			srcY := srcBounds.Min.Y + sy*srcBounds.Dy()/h
			srcX = clampInt(srcX, srcBounds.Min.X, srcBounds.Max.X-1)
			srcY = clampInt(srcY, srcBounds.Min.Y, srcBounds.Max.Y-1)
			r, g, b, _ := src.At(srcX, srcY).RGBA()
			dst.Set(dx, dy, color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 255})
		}
	}
}

// isInsideRoundedRect 检查点是否在圆角矩形内
func isInsideRoundedRect(x, y, w, h, r int) bool {
	if x < 0 || y < 0 || x >= w || y >= h {
		return false
	}
	// 四个角的圆心
	corners := [][2]int{
		{r, r},
		{w - 1 - r, r},
		{r, h - 1 - r},
		{w - 1 - r, h - 1 - r},
	}
	for _, c := range corners {
		cx, cy := c[0], c[1]
		if (x < cx && x >= cx-r || x > cx+r && x <= cx+r) ||
			(y < cy && y >= cy-r || y > cy+r && y <= cy+r) {
			dx := x - cx
			dy := y - cy
			if dx*dx+dy*dy > r*r {
				return false
			}
		}
	}
	return true
}

// enhanceContrast 提升画布对比度
func enhanceContrast(dst *image.RGBA, factor float64) {
	for y := 0; y < PosterCanvasH; y++ {
		for x := 0; x < PosterCanvasW; x++ {
			r, g, b, _ := dst.At(x, y).RGBA()
			nr := clampInt(int(float64(r>>8)*factor), 0, 255)
			ng := clampInt(int(float64(g>>8)*factor), 0, 255)
			nb := clampInt(int(float64(b>>8)*factor), 0, 255)
			dst.Set(x, y, color.RGBA{R: uint8(nr), G: uint8(ng), B: uint8(nb), A: 255})
		}
	}
}

// ============================================================
// 模块三：文字渲染
// ============================================================

// renderLibraryName 在画布底部渲染媒体库名称
func renderLibraryName(canvas *image.RGBA, name string) {
	if name == "" {
		return
	}

	// 判断是否包含中文
	hasChinese := false
	for _, ch := range name {
		if ch > 0x4E00 && ch < 0x9FFF {
			hasChinese = true
			break
		}
	}

	// 计算文字位置
	chBottomY := PosterCanvasH - PosterBottomMargin

	// 尝试加载中文字体
	chFont := loadFont(PosterFontSizeCH)
	enFont := loadFont(PosterFontSizeEN)

	if hasChinese && chFont != nil {
		// 中文名称在下方（主标题）
		chTopY := chBottomY - PosterFontSizeCH
		chWidth := font.MeasureString(chFont, name).Ceil()
		chX := (PosterCanvasW - chWidth) / 2
		if chX < 0 {
			chX = 0
		}
		drawTextWithShadow(canvas, chFont, name, chX, chTopY, color.Black, 0.7)
		drawText(canvas, chFont, name, chX, chTopY, color.White)
	} else if enFont != nil {
		// 英文名称
		enTopY := chBottomY - PosterFontSizeEN
		enWidth := font.MeasureString(enFont, name).Ceil()
		enX := (PosterCanvasW - enWidth) / 2
		if enX < 0 {
			enX = 0
		}
		drawTextWithShadow(canvas, enFont, name, enX, enTopY, color.Black, 0.7)
		drawText(canvas, enFont, name, enX, enTopY, color.White)
	}
}

// loadFont 加载字体文件，兜底使用 basicfont
func loadFont(size int) font.Face {
	// 字体文件搜索路径
	fontPaths := []string{
		"/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc",
		"/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/TTF/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
	}

	for _, path := range fontPaths {
		fontData, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		opentypeFont, err := opentype.Parse(fontData)
		if err != nil {
			continue
		}
		face, err := opentype.NewFace(opentypeFont, &opentype.FaceOptions{
			Size:    float64(size),
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if err != nil {
			continue
		}
		return face
	}

	// 兜底：使用 basicfont（仅支持ASCII，不支持中文）
	return basicfont.Face7x13
}

// drawText 在画布上绘制文字
func drawText(dst *image.RGBA, face font.Face, text string, x, y int, c color.Color) {
	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(c),
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y + face.Metrics().Ascent.Ceil())},
	}
	d.DrawString(text)
}

// drawTextWithShadow 绘制带阴影的文字
func drawTextWithShadow(dst *image.RGBA, face font.Face, text string, x, y int, shadowColor color.Color, shadowAlpha float64) {
	// 直接在目标画布上绘制阴影（偏移2px，半透明黑色）
	shadowSrc := image.NewUniform(color.RGBA{R: 0, G: 0, B: 0, A: uint8(shadowAlpha * 255)})
	shadowDrawer := &font.Drawer{
		Dst:  dst,
		Src:  shadowSrc,
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(x + 2), Y: fixed.I(y + 2 + face.Metrics().Ascent.Ceil())},
	}
	shadowDrawer.DrawString(text)
}

// ============================================================
// 工具函数
// ============================================================

// resizeImage 简单缩放图片
func resizeImage(img image.Image, w, h int) image.Image {
	srcBounds := img.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			srcX := srcBounds.Min.X + x*srcBounds.Dx()/w
			srcY := srcBounds.Min.Y + y*srcBounds.Dy()/h
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}
	return dst
}

// resizeAndCropImage 缩放并居中裁剪图片到指定尺寸（等比例缩放+居中裁剪）
func resizeAndCropImage(img image.Image, targetW, targetH int) image.Image {
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

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

	// 居中偏移
	offsetX := (scaleW - targetW) / 2
	offsetY := (scaleH - targetH) / 2

	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	for y := 0; y < targetH; y++ {
		for x := 0; x < targetW; x++ {
			srcX := bounds.Min.X + (x+offsetX)*srcW/scaleW
			srcY := bounds.Min.Y + (y+offsetY)*srcH/scaleH
			srcX = clampInt(srcX, bounds.Min.X, bounds.Max.X-1)
			srcY = clampInt(srcY, bounds.Min.Y, bounds.Max.Y-1)
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}
	return dst
}

func clampInt(v, min, max int) int {
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
