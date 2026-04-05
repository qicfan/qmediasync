package font

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var (
	fontsFS embed.FS
)

const (
	ZhFontFileName  = "chaohei.ttf"
	EnFontFileName  = "EmblemaOne.woff2"
)

type FontManager struct {
	fontsDir string
	zhFontPath string
	enFontPath string
}

var globalFontManager *FontManager

func InitFontManager(dataDir string) error {
	fontsDir := filepath.Join(dataDir, "fonts")
	if err := os.MkdirAll(fontsDir, 0755); err != nil {
		return fmt.Errorf("创建字体目录失败: %w", err)
	}
	
	globalFontManager = &FontManager{
		fontsDir: fontsDir,
		zhFontPath: filepath.Join(fontsDir, ZhFontFileName),
		enFontPath: filepath.Join(fontsDir, EnFontFileName),
	}
	
	return nil
}

func GetFontManager() *FontManager {
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

func (fm *FontManager) ExtractEmbeddedFonts() error {
	if fontsFS == nil {
		return errors.New("没有嵌入的字体文件")
	}
	
	if !fm.FontExists("zh") {
		if err := fm.extractFont(ZhFontFileName, fm.zhFontPath); err != nil {
			return fmt.Errorf("提取中文字体失败: %w", err)
		}
	}
	
	if !fm.FontExists("en") {
		if err := fm.extractFont(EnFontFileName, fm.enFontPath); err != nil {
			return fmt.Errorf("提取英文字体失败: %w", err)
		}
	}
	
	return nil
}

func (fm *FontManager) extractFont(srcName, dstPath string) error {
	srcFile, err := fontsFS.Open(srcName)
	if err != nil {
		return fmt.Errorf("打开嵌入字体文件失败: %w", err)
	}
	defer srcFile.Close()
	
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("创建字体文件失败: %w", err)
	}
	defer dstFile.Close()
	
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("复制字体文件失败: %w", err)
	}
	
	return nil
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
		return errors.New("无效的字体文件格式")
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
		return true, "embedded", fontPath, nil
	}
	
	return false, "", "", nil
}
