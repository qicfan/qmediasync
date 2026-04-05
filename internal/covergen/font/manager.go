package font

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"Q115-STRM/internal/helpers"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
)

const (
	ZhFontFileName    = "chaohei.ttf"
	EnFontFileName    = "Melete.otf"
	ZhFontDownloadURL = "https://github.com/justzerock/MoviePilot-Plugins/raw/main/fonts/chaohei.ttf"
	EnFontDownloadURL = "https://github.com/justzerock/MoviePilot-Plugins/raw/main/fonts/Melete.otf"
)

type FontManager struct {
	fontsDir   string
	zhFontPath string
	enFontPath string
}

var globalFontManager *FontManager
var fontManagerOnce sync.Once

func InitFontManager(dataDir string) error {
	fontsDir := filepath.Join(dataDir, "fonts")
	if err := os.MkdirAll(fontsDir, 0755); err != nil {
		return fmt.Errorf("创建字体目录失败: %w", err)
	}
	globalFontManager = &FontManager{
		fontsDir:   fontsDir,
		zhFontPath: filepath.Join(fontsDir, ZhFontFileName),
		enFontPath: filepath.Join(fontsDir, EnFontFileName),
	}
	return nil
}

func GetFontManager() *FontManager {
	if globalFontManager == nil {
		fontManagerOnce.Do(func() {
			fontsDir := filepath.Join(helpers.DataDir, "fonts")
			os.MkdirAll(fontsDir, 0755)
			globalFontManager = &FontManager{
				fontsDir:   fontsDir,
				zhFontPath: filepath.Join(fontsDir, ZhFontFileName),
				enFontPath: filepath.Join(fontsDir, EnFontFileName),
			}
		})
	}
	return globalFontManager
}

func (fm *FontManager) GetFontsDir() string {
	return fm.fontsDir
}

func (fm *FontManager) GetZhFontPath() string {
	return fm.zhFontPath
}

func (fm *FontManager) GetEnFontPath() string {
	return fm.enFontPath
}

func (fm *FontManager) FontExists(fontType string) bool {
	var fontPath string
	switch fontType {
	case "zh":
		fontPath = fm.zhFontPath
	case "en":
		fontPath = fm.enFontPath
	default:
		return false
	}
	_, err := os.Stat(fontPath)
	return !os.IsNotExist(err)
}

func (fm *FontManager) ValidateFont(fontPath string) error {
	file, err := os.Open(fontPath)
	if err != nil {
		return fmt.Errorf("打开字体文件失败: %w", err)
	}
	defer file.Close()
	header := make([]byte, 4)
	if _, err := io.ReadFull(file, header); err != nil {
		return fmt.Errorf("读取字体文件头失败: %w", err)
	}
	isTTF := header[0] == 0x00 && header[1] == 0x01 && header[2] == 0x00 && header[3] == 0x00
	isWOFF := string(header[0:4]) == "wOFF"
	if !isTTF && !isWOFF {
		return fmt.Errorf("无效的字体文件格式")
	}
	return nil
}

func (fm *FontManager) GetFontInfo(fontType string) (available bool, source string, path string, err error) {
	var fontPath string
	switch fontType {
	case "zh":
		fontPath = fm.zhFontPath
	case "en":
		fontPath = fm.enFontPath
	default:
		return false, "", "", fmt.Errorf("不支持的字体类型: %s", fontType)
	}
	if fm.FontExists(fontType) {
		if err := fm.ValidateFont(fontPath); err != nil {
			return false, "", "", err
		}
		return true, "local", fontPath, nil
	}
	return false, "", fontPath, nil
}

func (fm *FontManager) EnsureFonts() error {
	if !fm.FontExists("zh") {
		if err := downloadFont(ZhFontDownloadURL, fm.zhFontPath); err != nil {
			helpers.AppLogger.Warnf("下载中文字体失败: %v", err)
		}
	}
	if !fm.FontExists("en") {
		if err := downloadFont(EnFontDownloadURL, fm.enFontPath); err != nil {
			helpers.AppLogger.Warnf("下载英文字体失败: %v", err)
		}
	}
	return nil
}

func downloadFont(url, destPath string) error {
	helpers.AppLogger.Infof("正在下载字体: %s", url)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("下载字体失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载字体失败，状态码: %d", resp.StatusCode)
	}
	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("创建字体文件失败: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(destPath)
		return fmt.Errorf("写入字体文件失败: %w", err)
	}
	return nil
}

func ScaleFontSize(baseSize float64, canvasHeight int) float64 {
	return baseSize * (float64(canvasHeight) / 1080.0)
}

func MeasureTextWidth(fnt font.Face, text string) int {
	width := 0
	for _, r := range text {
		adv, ok := fnt.GlyphAdvance(r)
		if !ok {
			continue
		}
		width += adv.Ceil()
	}
	return width
}

func LoadFontFace(fontPath string, fontSize float64) (font.Face, error) {
	if fontPath == "" {
		return nil, fmt.Errorf("字体路径为空")
	}
	fontData, err := os.ReadFile(fontPath)
	if err != nil {
		return nil, fmt.Errorf("读取字体文件失败: %w", err)
	}
	parsedFont, err := truetype.Parse(fontData)
	if err != nil {
		return nil, fmt.Errorf("解析字体文件失败: %w", err)
	}
	face := truetype.NewFace(parsedFont, &truetype.Options{
		Size: fontSize,
		DPI:  72,
	})
	return face, nil
}
