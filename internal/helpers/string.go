package helpers

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	mathrand "math/rand"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/mozillazg/go-pinyin"
)

// 全局随机生成器，在包初始化时设置种子
var globalRand = mathrand.New(mathrand.NewSource(time.Now().UnixNano()))

func StringToInt(s string) int {
	if s == "" {
		return 0
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
}

func StringToInt64(s string) int64 {
	if s == "" {
		return 0
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return i
}

func UUID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	// 设置UUID版本(4)和变体(2)
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant is 10

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

func RandStr(length int) string {
	// 生成size长度的随机字符串
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[globalRand.Intn(len(charset))]
	}
	return string(b)
}

// MD5Hash returns the MD5 hash of the input string.
func MD5Hash(s string) string {
	hash := md5.Sum([]byte(s))
	return hex.EncodeToString(hash[:])
}

// SHA1Hash returns the SHA1 hash of the input string.
func SHA1Hash(s []byte) string {
	hash := sha1.Sum(s)
	return strings.ToUpper(hex.EncodeToString(hash[:]))
}

// FileSHA1Partial 读取指定文件的 offset 和 length，然后对读取的内容做 SHA1 哈希
func FileSHA1Partial(filePath string, offset int64, length int64) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	_, err = f.Seek(offset, io.SeekStart)
	if err != nil {
		return "", err
	}

	buf := make([]byte, length-offset+1)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", err
	}
	return SHA1Hash(buf[:n]), nil
}

// FileSHA1 计算文件的 SHA1 哈希
func FileSHA1(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hash := sha1.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(hash.Sum(nil))), nil
}

// 根据指定字符分割字符串，并且去掉分割完的每个子字符串的首尾空格
func SplitAndTrim(str, splitChar string) []string {
	// 将字符串按指定分隔符分割，并去除每个子串的前后空格
	if str == "" {
		return []string{}
	}
	parts := strings.Split(str, splitChar)
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}
	return parts
}

func UrlEncode(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "%2F")
}

// FirstLetterUpper 基础版本：首字母大写
func FirstLetterUpper(s string) string {
	if s == "" {
		return s
	}

	// 将字符串转换为 rune 切片以正确处理 Unicode
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// 将00000.00000格式的时长转为秒
func DurationStringToSecond(duration string) int64 {
	if duration == "" {
		return 0
	}
	parts := strings.Split(duration, ".")
	if len(parts) != 2 {
		return 0
	}
	seconds := StringToInt64(parts[0])
	return seconds
}

// 根据宽和高计算宽高比,返回一个小数点后三位的float64
func CalculateAspectRatio(width, height int64) float64 {
	if width == 0 || height == 0 {
		return 0
	}
	return float64(width) / float64(height)
}

func JsonString(v any) string {
	// json编码v
	jsonStr, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(jsonStr)
}

func StringJson[T any](s string) (T, error) {
	var v T
	err := json.Unmarshal([]byte(s), &v)
	if err != nil {
		return v, err
	}
	return v, nil
}

func FormatSeasonEpisode(seasonNumber, episodeNumber int) string {
	return fmt.Sprintf("S%02dE%02d", seasonNumber, episodeNumber)
}
func FormatSeason(seasonNumber int) string {
	return fmt.Sprintf("Season %d", seasonNumber)
}

func ParseYearFromDate(date string) int {
	if date == "" {
		return 0
	}
	year, err := strconv.Atoi(date[:4])
	if err != nil {
		return 0
	}
	return year
}

// TitleCase 将字符串中每个单词的首字母大写
func TitleCase(input string) string {
	if input == "" {
		return input
	}

	words := strings.Fields(input)
	for i, word := range words {
		// 处理每个单词
		runes := []rune(word)
		if len(runes) > 0 {
			// 首字母大写
			runes[0] = unicode.ToTitle(runes[0])
			words[i] = string(runes)
		}
	}

	return strings.Join(words, " ")
}

// truncateString 截断字符串
func TruncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength] + "..."
}

// ChineseToPinyin 将字符串中的中文转换为拼音首字母
// 返回 (是否包含中文, 替换后的字符串)
func ChineseToPinyin(s string) (bool, string) {
	if s == "" {
		return false, s
	}

	hasChinese := false
	var result strings.Builder

	for _, ch := range s {
		// 检查是否是中文字符 (CJK Unified Ideographs)
		if unicode.Is(unicode.Han, ch) {
			hasChinese = true
			// 使用 go-pinyin 库获取拼音首字母
			args := pinyin.Args{
				Style: pinyin.FirstLetter, // 首字母
			}
			pinyinList := pinyin.Pinyin(string(ch), args)
			if len(pinyinList) > 0 && len(pinyinList[0]) > 0 {
				result.WriteString(pinyinList[0][0])
			} else {
				result.WriteRune(ch)
			}
		} else {
			result.WriteRune(ch)
		}
	}

	return hasChinese, result.String()
}
