package helpers

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

var excludeKeyword = []string{
	"1080p GER", "2160p GRE", "720p", "1080p", "2160p", "4196p", "1920x1080", "1080i", "2160i", "4096i", "4kHDR", "片源", "原盘", "[8k]", "[4k]", "[2k]",
	"4k精品影视", "2k精品影视", "4k典藏版", "杜比视界", "杜比", "超清", "原盘", "未删减", "剧场版", "4k+",
	"CHT", "Bilibili", "CHS-JAP", "HK", "EN", "MULTi", "VF2", "60fps", "120fps", "AAC 2.0", "AAC2.0",
	"HEVC", "mkv", "mp4", "AVC", "flac", "avi", "flv", "QuickIO", "Uindex", "WEBLE", "YTS.MX", "hamivideo", "AC3.", "MP2", "MPEG-2", "MPEG-4", "MPEG",
	"DDP5.1", "DDP 5.1", "DDP 2.0", "ddp2.0", "ddp7.1", "ddp 7.1", "DDP",
	"TrueHD7.1", "truehd5.1", "truehd 5.1", "truehd5.1", "truehd5", "truehd 5", "truehd",
	"dd5.1", "dd7.1", "dd 7.1", "DD2.0", "dd 2.0", "dd5", "dd7", "Atmos", "DTS-HD", "MA 5.1", "MA5.1", "MA.5.1", "MA 2.0", "MA2.0", "MA.2.0", "MA7.1", "MA 7.1", "MA.7.1",
	"5.1 BONE", "5.1BONE", "CR",
	"VC-1", "DOVI", "NF", "ZmWeb", "DV", "HQ", "friDay", "amzn", "CC", "DTS", "dd+", "dvd5", "Extended", "Cut", "MVSTP",
	"2Audios", "2Audio", "2Audios5.1", "2Audio5.1", "2Audio", "3Audios", "3AUDIO", "8Audios", "8Audio", "5Audios", "5Audio",
	"sharphd", "NF", "HQ", "AAC", "GER", "MA", "FRA", "BFI", "EUR", "CN", "JP", "RU", "USA", "dd", "truehd", "HDR", "HDR10",
	"CTRLHD", "FraMeSToR", "10bit", "IMAX", "web-dl", "uhd", "HDMA5", "BlueRay", "bluray", "blu-ray", "blue-ray", "DBD-Raws", "DBD", "WEBRIP", "DreamHD", "Remux", "ExKinoRay", "PandaQT", "CMCTV", "HDSKY",
	"tptv", "HDTV", "mnhd", "hhweb", "frds", "iTunes", "hybrid", "blu",
	"c0kE", "mteam", "carpt", "1ptba", "SupaHacka", "lpcm2.0", "lpcm 1.0", "lpcm 2.0", "lpcm1.0", "lpcm", "hdspace", "diy", "F13@",
	// "h264", "x264", "x265", "h265", "h.264", "h.265", "x.264", "x.265", "5.1", "7.1", "2.0",
	"类型：剧情", "类型：动作", "类型：爱情", "类型：科幻", "类型：悬疑", "类型：恐怖", "类型：动画", "类型：家庭", "类型：儿童",
	"HD国语中字", "国语配音", "中文字幕", "简日双语", "繁日双语", "简繁日多语", "特效字幕", "国英多音轨", "满屏版", "国语中字", "无水印",
	"简繁外挂", "高码版", "高码率", "中字", "畅享版", "黄渤新片", "国粤配", "日版", "简英字幕", "简英", "简繁字幕", "简繁", "国语配音+中文字幕",
	"60帧率版本", "120帧率版本", "amazon", "超清修复版", "彩色修复版", "老片修复版", "超清无台标版", "超清有台标版", "默声修复版", "CCTV6-HD",
	"4KHDR片源", "黄渤新片", "中字畅享版",
	"(CR", "WEB", `NUKEHD`, "KR", "CUSTOM", "remastered", "FRE",
}

var preExclucdePatterns = []string{
	`【公众号：手机软件资源局】`,
	`CCTV\d\-HD`,
	`(LPCM)(\s|\.)?\d\.?\d?`,
	`(AAC|DDP|DD|TRUEHD|PCM)(\s|\.)?\d\.?\d?`,
	`MA\.?\d\.\d`,
	`H\.?26[4|5]`,
	`\w{4, 10}HD`,
	`\w{4, 10}PT`,
	`PT(\w|\W|\d){4, 10}`,
	`\-\s?[\w\d\p{han}]{2,10}$`,
	`\s(日本|德国|英国)$`,
	`\d{2}fps`,
}

type MediaInfo struct {
	Name    string `json:"name"`
	Year    int    `json:"year"`
	Season  int    `json:"season"`
	Episode int    `json:"episode"`
	TmdbId  int64  `json:"tmdbid"`
}

func ExtractTmdbId(name string) int64 {
	var tmdbPatterns = []string{
		`[\{|\[|【]{1}tmdbid-(\d+)[\}|\]】]{1}`,
		`[\{|\[|【]{1}tmdb-(\d+)[\}|\]】]{1}`,
		// 新增更多格式支持
		`[\{|\[|【]{0,1}tmdbid[=\-](\d+)[\}|\]|】]{0,1}`,
		`[\{|\[|【]{0,1}tmdb[=\-](\d+)[\}|\]|】]{0,1}`,
	}
	for _, pattern := range tmdbPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(name)
		// fmt.Printf("匹配到的tmdb id: %+v\n", matches)
		if len(matches) >= 2 {
			tmdbId, _ := strconv.ParseInt(matches[1], 10, 64)
			// 删除匹配的字符串
			return tmdbId
		}
	}
	return 0
}

// ExtractDoubanId 从文件名提取豆瓣 ID
func ExtractDoubanId(name string) int64 {
	var doubanPatterns = []string{
		`[\{|\[|【]{0,1}doubanid[=\-](\d+)[\}|\]|】]{0,1}`,
		`[\{|\[|【]{0,1}douban[=\-](\d+)[\}|\]|】]{0,1}`,
	}
	for _, pattern := range doubanPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(name)
		if len(matches) >= 2 {
			doubanId, _ := strconv.ParseInt(matches[1], 10, 64)
			return doubanId
		}
	}
	return 0
}

// ExtractID 从文件名提取 ID 信息（支持 TMDB 和豆瓣 ID）
// 返回: tmdbID, doubanID, mediaType, season, episode
func ExtractID(filename string) (int64, int64, string, int, int) {
	// 完整格式：{tmdbid=123;type=movie;s=1;e=1}
	fullPattern := `\{tmdbid=(\d+);type=(\w+);s=(\d+);e=(\d+)\}`
	re := regexp.MustCompile(fullPattern)
	matches := re.FindStringSubmatch(filename)
	if len(matches) >= 5 {
		tmdbID, _ := strconv.ParseInt(matches[1], 10, 64)
		mediaType := matches[2]
		season, _ := strconv.Atoi(matches[3])
		episode, _ := strconv.Atoi(matches[4])
		return tmdbID, 0, mediaType, season, episode
	}

	// 提取 TMDB ID
	tmdbID := ExtractTmdbId(filename)

	// 提取豆瓣 ID
	doubanID := ExtractDoubanId(filename)

	// 如果没有提取到任何 ID，返回 0
	if tmdbID == 0 && doubanID == 0 {
		return 0, 0, "", 0, 0
	}

	// 提取季集信息
	_, season, episode := ExtractSeasonEpisode(filename)

	// 提取媒体类型
	mediaType := ""
	if season > 0 || episode > 0 {
		mediaType = "tv"
	}

	return tmdbID, doubanID, mediaType, season, episode
}

func ExtractNameAndYear(name string) (string, int) {
	titleRegex := regexp.MustCompile(`(?i)([\p{han}|\d|\w|\s|：|\:]+)\s\((\d{4})\)`)
	matches := titleRegex.FindStringSubmatch(name)
	if len(matches) == 3 {
		year, _ := strconv.Atoi(matches[2])
		return matches[1], year
	}
	return "", 0
}

// 使用正则表达式从文件名中提取媒体信息
func ExtractMediaInfoRe(name string, isMovie bool, seEp bool, videoExt []string, excludePatterns ...string) *MediaInfo {
	info := &MediaInfo{
		Season:  -1,
		Episode: -1,
		Year:    0,
		TmdbId:  0,
	}
	// 先直接提取标题 (year)这种格式
	info.Name, info.Year = ExtractNameAndYear(name)
	info.TmdbId = ExtractTmdbId(name)
	//去除视频扩展名
	var ok bool = false
	for _, ext := range videoExt {
		name, ok = strings.CutSuffix(name, ext)
		if ok {
			break
		}
	}
	if info.Name == "" && info.Year == 0 {
		// 如果只有一个《》，则直接取其中的内容
		if strings.Count(name, "《") == 1 && strings.Count(name, "》") == 1 {
			matches := regexp.MustCompile(`《(.+?)》`).FindStringSubmatch(name)
			if len(matches) > 1 {
				info.Name = matches[1]
				name = strings.Replace(name, matches[0], " ", 1)
			}
		}
	}
	// fmt.Printf("移除最后的中括号内容后: %s\n", name)
	if !isMovie {
		name, info.Season, info.Episode = ExtractSeasonEpisode(name)
		if info.Name != "" && info.Year != 0 && info.Season != -1 && info.Episode != -1 {
			return info
		} else {
			if seEp && info.Season != -1 && info.Episode != -1 {
				return info
			}
		}
	} else {
		info.Season = -1
		info.Episode = -1
		if info.Name != "" && info.Year != 0 {
			return info
		}
		// 移除开头的序号
		name = regexp.MustCompile(`^\d+(\.|、)(《)?`).ReplaceAllString(name, "")
		name = regexp.MustCompile(`^\d+(《)`).ReplaceAllString(name, "")
	}
	// 移除结尾的[/+?]
	name = regexp.MustCompile(`\s(\[.+?\])$`).ReplaceAllString(name, "")
	if !isMovie {
		if info.Season != -1 && info.Episode != -1 {
			// 如果提取到了季和集，则删除第 x 集这种
			pattern := fmt.Sprintf(`第 %d 集`, info.Episode)
			name = strings.ReplaceAll(name, pattern, "")
			// fmt.Printf("删除第 %d 集后: %s\n", info.Episode, name)
		}
		pattern := `第\s?\d+\s?集`
		name = regexp.MustCompile(pattern).ReplaceAllString(name, "")
		// fmt.Printf("删除第 x 集后: %s\n", name)
	}
	// 从name中删除excludePatterns
	for _, p := range excludePatterns {
		name = strings.ReplaceAll(name, p, "")
	}
	// 删除所有emoji表情符号
	emojiRegex := regexp.MustCompile(`[\x{1F600}-\x{1F64F}]|[\x{1F300}-\x{1F5FF}]|[\x{1F680}-\x{1F6FF}]|[\x{1F1E0}-\x{1F1FF}]|[\x{2600}-\x{26FF}]|[\x{2700}-\x{27BF}]|[\x{1F900}-\x{1F9FF}]`)
	name = emojiRegex.ReplaceAllString(name, "")
	// 移除开头的字幕组等关键词
	name = RemovePreExlucde(name)
	names, maxDelimiter := SplitTitle(name)
	// fmt.Printf("分隔符: %s\n", maxDelimiter)
	// 对每一部分进行处理
	newNames := make([]string, 0)
	hasYear := false
	for _, n := range names {
		// f := n
		if n == "" {
			continue
		}
		ye := NewYearExtractor()
		if !hasYear {
			n, info.Year = ye.ExtractYear(n)
			if info.Year != 0 {
				hasYear = true
			}
		}
		if n == "" || n == "[]" || n == "{}" || n == "()" {
			continue
		}
		n = PreProcess(n, excludeKeyword...)
		if n == "" || n == "[]" || n == "{}" || n == "()" {
			continue
		}
		// fmt.Printf("处理: %s=>%s\n", f, n)
		// 如果n是
		newNames = append(newNames, n)
	}
	if info.Name != "" && info.Year != 0 && strings.Count(info.Name, " ") == 0 {
		return info
	}
	// 用分隔符连接newNames
	name = strings.Join(newNames, maxDelimiter)
	// fmt.Printf("连接后文件名: %s\n", name)
	// 移除结尾的发布组
	name = removeTitleGroup(name)
	name = cleanFilename(name)
	// fmt.Printf("预清理后的文件名: %s\n", name)
	name = finalProcess(name)
	info.Name = cleanTitle(name)
	return info
}

func SplitTitle(name string) ([]string, string) {
	//先把全角符号转为半角符号
	quanjiaoMap := map[string]string{
		"【": "[",
		"】": "]",
		"（": "(",
		"）": ")",
		"《": "<",
		"》": ">",
		"｜": "|",
		"－": "-",
	}
	for quanjiao, halfQuanjiao := range quanjiaoMap {
		name = strings.ReplaceAll(name, quanjiao, halfQuanjiao)
	}
	aC := strings.Count(name, "][")
	bC := strings.Count(name, ")(")
	cC := strings.Count(name, "}{")
	var maxDelimiter string = ""
	if aC >= 3 {
		maxDelimiter = "]["
	} else if bC >= 3 {
		maxDelimiter = ")("
	} else if cC >= 3 {
		maxDelimiter = "}{"
	}
	if maxDelimiter == "" {
		// 将文件名用出现次数最多的分隔符分隔
		delimiters := []string{" ", ".", "_", "-", "/", "|"}
		maxCount := 0

		for _, delimiter := range delimiters {
			count := strings.Count(name, delimiter)
			if count < 2 {
				continue
			}
			if count > maxCount {
				maxCount = count
				maxDelimiter = delimiter
			}
		}
	}
	var names []string
	if maxDelimiter != "" {
		// 使用分隔符分割name
		names = strings.Split(name, maxDelimiter)
	} else {
		names = []string{name}
	}
	return names, maxDelimiter
}

func PreProcess(name string, excludePatterns ...string) string {
	name = regexp.MustCompile(`(\[|\{).+?(\]|\})`).ReplaceAllString(name, "")
	if name == "" {
		return ""
	}
	// 去对开头的特殊字符
	if slices.Contains([]string{"]", ")", "}", "[", "(", "{"}, name[:1]) {
		name = name[1:]
	}
	// 移除要删除的关键词
	for _, pattern := range excludePatterns {
		p := strings.ToLower(pattern)
		n := strings.ToLower(name)
		if p == n {
			return ""
		}
	}
	// 移除分辨率模式
	name = regexp.MustCompile(`(?i)\[?(480|720|1080|1440|2160|2610|4096|8192)(p|i)\]?`).ReplaceAllString(name, "")
	// fmt.Printf("移除分辨率模式后文件名: %s\n", name)
	name = removeVideoCodec(name)
	// fmt.Printf("移除视频编码格式后文件名: %s\n", name)
	name = removeAudioCodes(name)
	// fmt.Printf("移除音频编码格式后文件名: %s\n", name)
	name = removeSubtitlePlatformCountry(name)
	// fmt.Printf("移除字幕、播放平台和国家代码后文件名: %s\n", name)
	// 移除要删除的关键词
	for _, pattern := range excludePatterns {
		if strings.EqualFold(name, pattern) {
			return ""
		}
	}
	// 替换重复的特殊字符
	spec := []string{"[]", "{}", "()"}
	for _, s := range spec {
		name = strings.ReplaceAll(name, s, "")
	}
	// 去掉头部的特殊字符
	spec = []string{"[", "{", "(", "|", "<"}
	for _, s := range spec {
		name = strings.TrimPrefix(name, s)
	}
	// 去掉尾部的特殊字符
	spec = []string{"]", "}", ")", "|", ">"}
	for _, s := range spec {
		name = strings.TrimSuffix(name, s)
	}
	if name == " " {
		return ""
	}
	return strings.TrimSpace(name)
}

// ExtractSeasonEpisode 提取季和集信息
func ExtractSeasonEpisode(name string) (string, int, int) {
	// fmt.Printf("提取季集前文件名: %s\n", name)
	patterns := []string{
		`(?i)S(\d{1,2})E[P]?(\d{1,3})`,         // S01E01 或者S01EP01
		`(?i)E[P]?(\d{1,3})`,                   // E01 或者 EP01
		`(?i)Season\s*(\d+)\s*Episode\s*(\d+)`, // Season 1 Episode 1
		`(?i)(\d{1,2})x(\d{1,3})`,              // 1x01
		`第\s*(\d+)\s*季.*第\s*(\d+)\s*集`,         // 中文格式：第 1 季第 1 集或第1季第1集
		`第\s*(\d+)\s*集`,                        // 中文格式：第 1 集或第1集
		`(?i)Vol[\.|\s]+(\d+)`,                 // 卷号
		`\s?(\d{1,3})$`,                        // 只有集，凡人修仙传 10.mp4或10.mp4
		`\-\s(\d{1,3})\s`,                      // 只有集，- 10 xxxx.mp4
		`(\d{2,4})[\s|\.|\_|\-]4[K|k]`,         // 01 4K或01_4K或01-4K或01.4K
		`\[(\d{1,3})\]`,                        // [01]这种格式
		`(\d{2,4})`,                            // 10.mkv这种格式
	}
	seasonNumber := -1
	episodeNumber := -1
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(name)
		// fmt.Printf("识别结果数量%d, 结果:%+v\n", len(matches), matches)
		if len(matches) > 2 {
			fmt.Sscanf(matches[1], "%d", &seasonNumber)
			fmt.Sscanf(matches[2], "%d", &episodeNumber)
			name = strings.Replace(name, matches[0], " ", 1)
			break
		}
		if len(matches) == 2 {
			fmt.Sscanf(matches[1], "%d", &episodeNumber)
			// fmt.Printf("识别到集数:%d\n", episodeNumber)
			name = strings.Replace(name, matches[0], " ", 1)
			break
		}
	}
	// fmt.Printf("提取到季集: S%dE%d，清除后文件名: %s\n", seasonNumber, episodeNumber, name)
	return name, seasonNumber, episodeNumber
}

// cleanFilename 清理文件名中的常见标记
func cleanFilename(name string) string {
	// fmt.Printf("合并特殊字符前的文件名: %s\n", name)
	// 合并多个空格
	space := regexp.MustCompile(`\s+`)
	name = space.ReplaceAllString(name, " ")
	// 合并多个.
	dot := regexp.MustCompile(`\.+`)
	name = dot.ReplaceAllString(name, ".")
	// 合并多个_
	underscore := regexp.MustCompile(`_+`)
	name = underscore.ReplaceAllString(name, "_")
	// 合并多个-
	dash := regexp.MustCompile(`-+`)
	name = dash.ReplaceAllString(name, "-")
	// 合并多个[]
	bracket := regexp.MustCompile(`\[\]+`)
	name = bracket.ReplaceAllString(name, "[]")
	// 合并多个=
	denghao := regexp.MustCompile(`\=+`)
	name = denghao.ReplaceAllString(name, "")
	// fmt.Printf("合并特殊字符后的文件名: %s\n", name)
	singleSpec := []string{".", "-", "|"}
	// hasSign := false
	for _, char := range singleSpec {
		c := strings.Count(name, char)
		if c == 1 {
			name = strings.ReplaceAll(name, char, " ")
			// hasSign = true
			break
		}
	}
	// 如果以特殊字符开头，则删除该字符
	specChar := []string{"(", "[", "{"}
	for _, char := range specChar {
		if after, ok := strings.CutPrefix(name, char); ok {
			name = after
			break
		}
	}
	specChar = []string{")", "]", "}"}
	for _, char := range specChar {
		if after, ok := strings.CutSuffix(name, char); ok {
			name = after
			break
		}
	}
	// 将特殊字符转为空格
	spec := []string{".", "-", "|"}
	for _, s := range spec {
		name = strings.ReplaceAll(name, s, " ")
	}
	// 如果name中有下划线，则用下划线继续分割
	if strings.Count(name, "_") >= 1 {
		name = strings.Split(name, "_")[0]
	}
	// if strings.Count(name, " / ") >= 1 {
	// 	name = strings.Split(name, " / ")[0]
	// }
	// fmt.Printf("去掉下划线后的文件名: %s\n", name)
	return strings.TrimSpace(name)
}

func removeTitleGroup(filename string) string {
	// 常见的发布组模式
	releaseGroupPatterns := []string{
		`MTeam$`, `TPTV$`, `SupaHacka$`, `c0kE$`, `HONE$`, `HDZ$`, `CHD$`,
		`CSWEB$`, `BLoz$`, `ADE$`, `TMT$`, `HDS$`, `HDH$`, `INCUBO$`,
		`GMA$`, `NoGrp$`, `MainFrame$`, `MNHD-FRDS$`, `SharpHD$`, `UBits$`,
		`ZmWeb$`, `MKu$`, `CMCTV$`, `ADWeb$`, `UBWEB$`, `Panda$`, `iFPD$`,
		`LWRTD$`, `ZTR$`, `CHDWEB$`, `PtBM$`, `WiKi$`, `BONE$`, `HHWEB$`,
		`MWeb$`, "SPWEB$", "DREAMHD$",
		`PTer$`, `MWeb$`, `NoGroup$`, `CMCT$`, `HDSky$`, `Hero$`, `HDSky$`,
		`CHDBits$`, `HDHome$`, `HDSpace$`, `QuickIO$`, `NUKEHD$`, `WEBLE`,
		`112114119$`,
	}

	// 移除发布组
	result := filename
	for _, pattern := range releaseGroupPatterns {
		re := regexp.MustCompile(`(?i)(\-[\p{han}\a-z|0-9]+)?(\-|@)?` + pattern)
		result = re.ReplaceAllString(result, "")
	}
	return result
}

func removeVideoCodec(filename string) string {
	// 常见的视频编码格式
	videoCodecPatterns := []string{
		`\d+bit`, `Atmos\-?`, `HEVC-\d{1,2}bit`, `SRTx2`,
		`\[[a-z]{2, 3}\]`,
		`杜比视界版本(版本)?`, `高码版`, `WEBRip`, `YTS.MX`,
		`(x|h)\.?(264|265)`, `(MPEG|mpg)\-?[1-4]`,
		`(\.\-\|)?Remux(\.|\-\|)?`,
		`Web\-DL(\s+\d+[p])?`,
		`HDTV\s+\d+[pi]`,
		`TrueHD\s?\d+\.\d+\-?`,
		`BluRay\s+\d+[p]`, `Blu-ray\s+\d+[p]`, `UHD\s+BluRay\s+\d+[p]`,
		`Remux\s+\d+[p]`,
		`DTS-HD MA\d+\.\d+`, `Atmos TrueHD\d+\.\d+`, `TrueHD\s+\d+\.\d+`,
		`DoVi`, `HDR10\+?`, `DV HDR`, `HDR10`, `HDR100`,
		`(.+?)片源`,
		`(4k|2k)\+?`,
		`\d{1,3}(帧率|fps)版本`,
		`\.?torrent$`, // 明显的视频和种子扩展名
	}

	// 移除视频编码格式
	result := filename
	for _, pattern := range videoCodecPatterns {
		re := regexp.MustCompile("(?i)" + pattern + `[\.|\s|\-|$]?`)
		result = re.ReplaceAllString(result, "")
	}
	return result
}

func removeAudioCodes(filename string) string {
	// 常见的音频编码格式
	audioCodecPatterns := []string{
		`(MA|DD|DV)\d\.\d`, `MKV\)`,
		`LPCM\d+\.\d+`, `AAC\s+\d+\.\d+`, `AAC\-?`, `AC\d+`,
		`TrueHD\s+\d+\.\d+`, `Atmos TrueHD\d+\.\d+`, `DTS-HD(\s)?MA\d+\.\d+`,
		`(5\.0|5\.1|7\.1)?\s?(2|3|5|7|8)Audio(s)?\s?(5\.0|5\.1|7\.1)?`,
		`DDP(\s)?(5\.1|7\.1)[\.|\s\-|$]?`, `DDP(5\.1|7\.1|5|7|8)[\.|\s\-|$]?`, `AAC(\s)?(2\.0|2\.1|5\.0|5\.1|7\.1|8|2|5|7)`,
		`\d\.\d`, `Audio`,
	}

	// 移除音频编码格式
	result := filename
	for _, pattern := range audioCodecPatterns {
		re := regexp.MustCompile("(?i)" + pattern)
		result = re.ReplaceAllString(result, "")
	}
	return result
}

func removeSubtitlePlatformCountry(filename string) string {
	// 常见的字幕、播放平台和国家代码格式
	platformCountryPatterns := []string{
		`无水印`, `IMAX满屏版`,
		`(HD)?(简|繁|日|英|中|粤|国)+字?畅享版`,
		`(HD)?(简|繁|日|英|中|粤|国)+版(\.\+\-\}。)?`,
		`(HD)?(简|繁|日|英|中|粤|国)+(多|双)?语(\+?配音|版)?(\.\+\-\}。)?`,
		`(HD)?(简|繁|日|英|中|粤|国)+(多|双)?音轨(\.\+\-\}。)?`,
		`(HD)?(简|繁|日|英|中|粤|国|特效)+(文|语)?字幕(\.\+\-\}。)?`,
		`(HD)?(简|繁|日|英|中|粤|国)+配(\.\+\-\}。)?`,
		`(HD)?(简|繁|日|英|中|粤|国)+语(简|繁|日|英|中|粤|国)+字(\.\+\-\}。)?`,
		`(HD)?(简|繁|日|英|中|粤|国)+字(\.\+\-\}。)?`,
		`类型：[\p{han}]{2,4}`,
		`精品影视(.+?)`,
		`\+`,
	}
	// 移除音频编码格式
	result := filename
	for _, pattern := range platformCountryPatterns {
		re := regexp.MustCompile("(?i)" + pattern)
		result = re.ReplaceAllString(result, "")
	}
	return result
}

// YearExtractor 年份提取器
type YearExtractor struct {
	// 匹配各种年份格式的正则表达式
	yearPatterns []*regexp.Regexp
	currentYear  int
}

// NewYearExtractor 创建年份提取器
func NewYearExtractor() *YearExtractor {
	currentYear := time.Now().Year()

	return &YearExtractor{
		currentYear: currentYear,
		yearPatterns: []*regexp.Regexp{
			// 1. 括号格式: (1999) [1999] {1999}
			regexp.MustCompile(`[(\[{](\b(?:19|20)\d{2}\b)[)\]]`),

			// 2. 点分隔格式: 1999. 或者 .1999.
			regexp.MustCompile(`\.(\b(?:19|20)\d{2}\b)\.`),

			// 3. 空格分隔格式: 1999 或者 1999年
			regexp.MustCompile(`\s(\b(?:19|20)\d{2}\b)(?:\s|年|$)`),

			// 4. 开头格式: 1999_ 或者 1999-
			regexp.MustCompile(`^(\b(?:19|20)\d{2}\b)[_\-\.]`),

			// 5. 结尾格式: _1999 或者 -1999
			regexp.MustCompile(`[_\-\.](\b(?:19|20)\d{2}\b)$`),

			// 6. 独立数字格式: 前后都有边界
			regexp.MustCompile(`\b(\b(?:19|20)\d{2}\b)\b`),
		},
	}
}

// ExtractYear 从文件名中提取年份并移除匹配部分
func (ye *YearExtractor) ExtractYear(filename string) (cleaned string, year int) {
	// 移除文件扩展名
	name := filename
	original := filename

	// 尝试每个模式，找到第一个有效的年份
	for _, pattern := range ye.yearPatterns {
		matches := pattern.FindAllStringSubmatch(name, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				if y, valid := ye.validateYear(match[1]); valid {
					// 移除匹配到的整个部分
					name = strings.Replace(name, match[0], "", 1)
					return ye.cleanResult(name), y
				}
			}
		}
	}

	return ye.cleanResult(original), 0
}

// validateYear 验证年份是否有效
func (ye *YearExtractor) validateYear(yearStr string) (int, bool) {
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		return 0, false
	}

	// 检查年份范围 (1900-当前年份+1，允许未来一年的电影)
	if year >= 1900 && year <= ye.currentYear+1 {
		return year, true
	}

	return 0, false
}

// cleanResult 清理结果字符串
func (ye *YearExtractor) cleanResult(text string) string {
	// 替换多个连续点号为单个点号
	text = regexp.MustCompile(`\.{2,}`).ReplaceAllString(text, ".")

	// 替换多个连续下划线为单个
	text = regexp.MustCompile(`_{2,}`).ReplaceAllString(text, "_")

	// 替换多个连续连字符为单个
	text = regexp.MustCompile(`-{2,}`).ReplaceAllString(text, "-")

	// 清理开头和结尾的特殊字符
	text = strings.Trim(text, " ._-")

	// 合并多个空格
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

// RemovePreExlucde 移除以start开头（可选），包含r中任意关键词，以end结尾（可选）的标签
func RemovePreExlucde(filename string) string {
	// 常见的字幕组格式
	audioCodecPatterns := []string{
		`[\p{han}|a-zA-Z|0-9]+(字幕组|字幕社)`,
		`^UIndex\s?\-?\s?`,
		`^\[[a-z]{2,3}\]`,
		`\[黒ネズミたち\]`,
	}
	// 移除音频编码格式
	result := filename
	for _, pattern := range audioCodecPatterns {
		re := regexp.MustCompile("(?i)" + pattern)
		result = re.ReplaceAllString(result, "")
	}
	for _, pattern := range preExclucdePatterns {
		re := regexp.MustCompile("(?i)" + pattern)
		result = re.ReplaceAllString(result, "")
	}
	return result
}

// 提取季编号
func ExtractSeasonsFromSeasonPath(text string) int {
	if len(text) == 0 {
		return -1
	}
	f := string(text[0])
	if f != "s" && f != "S" {
		AppLogger.Errorf("提取季编号失败，路径 %s 不是以s或S开头", text)
		return -1
	}
	// 如果text的首字母是s，则转成大写的S
	if f == "s" {
		text = "S" + text[1:]
	}
	pattern := `(?i)season\s+0*(\d+)`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(text)
	if matches != nil {
		// 将字符串数字转换为整数
		num, err := strconv.Atoi(matches[1])
		if err == nil {
			return num
		} else {
			AppLogger.Errorf("提取季编号失败，路径 %s 匹配的季编号 %s 不是整数", text, matches[1])
		}
	} else {
		AppLogger.Errorf("提取季编号失败，路径 %s 没有匹配的季编号", text)
	}
	pattern = `(?i)s(\d+)$`
	re = regexp.MustCompile(pattern)
	matches = re.FindStringSubmatch(text)
	if matches != nil {
		// 将字符串数字转换为整数
		num, err := strconv.Atoi(matches[1])
		if err == nil {
			return num
		} else {
			AppLogger.Errorf("提取季编号失败，路径 %s 匹配的季编号 %s 不是整数", text, matches[1])
		}
	} else {
		AppLogger.Errorf("提取季编号失败，路径 %s 没有匹配的季编号", text)
	}
	return -1
}

// 提取季编号
func ExtractSeasonFromTvshowPath(text string) int {
	if len(text) == 0 {
		return -1
	}
	f := string(text[0])
	if f != "s" && f != "S" {
		return -1
	}
	// 如果text的首字母是s，则转成大写的S
	if f == "s" {
		text = "S" + text[1:]
	}
	pattern := `(?i)S(\d{1,3})`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(text)
	if matches != nil {
		// 将字符串数字转换为整数
		num, err := strconv.Atoi(matches[1])
		if err == nil {
			return num
		}
	}
	return -1
}

// 清理标题
func cleanTitle(title string) string {
	title = strings.TrimSpace(title)

	// 移除开头和结尾的特殊字符
	title = regexp.MustCompile(`^[-\s.:\<\{\(\[]+`).ReplaceAllString(title, "")
	title = regexp.MustCompile(`[-\s.:\]\)\}\>]+$`).ReplaceAllString(title, "")

	// 规范化空格
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")

	return strings.ToLower(strings.TrimSpace(title))
}

// cleanTitle 清理标题
func finalProcess(title string) string {
	title = strings.TrimSpace(title)
	// 清除结尾的[]
	title = strings.TrimSuffix(title, "[]")
	title = strings.TrimPrefix(title, "[]")
	// fmt.Printf("清除[]后的标题: %s\n", title)
	// 只有一个.分隔
	if strings.Count(title, ".") == 1 {
		// 检查标题是否包含中文
		if !regexp.MustCompile(`[\p{Han}]+`).MatchString(title) {
			// 不包含中文，将.替换为空格
			title = strings.ReplaceAll(title, ".", " ")
		} else if strings.Count(title, ".") == 1 {
			subTitles := strings.Split(title, ".")
			title = subTitles[0]
		}
	}
	// 只有一个-分隔
	if strings.Count(title, "-") == 1 {
		subTitles := strings.Split(title, "-")
		title = subTitles[0]
	}
	// 只有一个_分隔
	if strings.Count(title, "_") == 1 {
		subTitles := strings.Split(title, "_")
		title = subTitles[0]
	}
	// 只有一个][分隔
	if strings.Count(title, "][") >= 1 {
		subTitles := strings.Split(title, "][")
		title = subTitles[0]
		// 去掉title开头的[和结尾的]
		title = strings.TrimPrefix(title, "[")
		title = strings.TrimSuffix(title, "]")
	}
	if strings.Count(title, "[]") == 1 {
		subTitles := strings.Split(title, "[]")
		title = subTitles[0]
	}
	if strings.Count(title, "[]") >= 1 {
		title = strings.ReplaceAll(title, "[]", " ")
		// 将多个空格替换为一个空格
		space := regexp.MustCompile(`\s+`)
		title = space.ReplaceAllString(title, " ")
		title = strings.TrimSpace(title)
		// fmt.Printf("清除特殊时将特殊字符替换为空格后的标题: %s\n", title)
	}
	if strings.Count(title, " ") == 1 {
		// 检查标题是否包含中文和英文
		if regexp.MustCompile(`[\p{Han}]+ [a-zA-Z]+`).MatchString(title) {
			subTitles := strings.Split(title, " ")
			title = subTitles[0]
		}
		// 检查是否只包含中文
		if regexp.MustCompile(`[\p{Han}]+ [\p{Han}]+`).MatchString(title) {
			subTitles := strings.Split(title, " ")
			title = subTitles[0]
		}
	}
	if strings.Count(title, "/") == 1 {
		subTitles := strings.Split(title, "/")
		title = subTitles[0]
	}
	if strings.Count(title, " ") >= 2 {
		// 检查标题是否包含中文和英文
		if regexp.MustCompile(`[\p{Han}]+ [a-zA-Z\s]+`).MatchString(title) {
			subTitles := strings.Split(title, " ")
			title = subTitles[0]
		}
	}
	// 如果title以[*?]开头，则剔除
	space := regexp.MustCompile(`^\[.*?\]`)
	title = space.ReplaceAllString(title, "")
	// 移除多余的空格
	space = regexp.MustCompile(`\s+`)
	title = space.ReplaceAllString(title, " ")
	title = strings.TrimSpace(title)
	// 将title中的特殊字符全部剔除
	title = regexp.MustCompile(`[\[\]\{\}]`).ReplaceAllString(title, "")
	// // 首字母大写
	// title = FirstLetterUpper(title)
	// fmt.Printf("最后得到的标题: %s\n", title)
	return strings.ToLower(title)
}

// ExtractMetadataFromPath 从文件路径提取元数据（多层级）
// 参考 MoviePilot 的实现，从文件名、父目录、上上级目录三个层级提取元数据并智能合并
func ExtractMetadataFromPath(filePath string, isMovie bool, videoExt []string, excludePatterns ...string) *MediaInfo {
	meta := &MediaInfo{
		Season:  -1,
		Episode: -1,
		Year:    0,
		TmdbId:  0,
	}

	// 1. 提取文件名元数据
	fileMeta := ExtractMediaInfoRe(filepath.Base(filePath), isMovie, !isMovie, videoExt, excludePatterns...)
	if fileMeta != nil {
		meta.Merge(fileMeta)
	}

	// 2. 提取父目录元数据
	parentDir := filepath.Base(filepath.Dir(filePath))
	parentMeta := ExtractMediaInfoRe(parentDir, isMovie, !isMovie, videoExt, excludePatterns...)
	if parentMeta != nil {
		meta.Merge(parentMeta)
	}

	// 3. 提取上上级目录元数据
	grandparentDir := filepath.Base(filepath.Dir(filepath.Dir(filePath)))
	grandparentMeta := ExtractMediaInfoRe(grandparentDir, isMovie, !isMovie, videoExt, excludePatterns...)
	if grandparentMeta != nil {
		meta.Merge(grandparentMeta)
	}

	return meta
}

// Merge 智能合并元数据
// 优先级：文件名 > 父目录 > 上上级目录
func (m *MediaInfo) Merge(other *MediaInfo) {
	if other == nil {
		return
	}

	// 如果当前缺失年份，从父目录借用
	if m.Year == 0 && other.Year != 0 {
		m.Year = other.Year
	}

	// 如果当前缺失季/集信息，从目录补全
	if m.Season == -1 && other.Season != -1 {
		m.Season = other.Season
	}
	if m.Episode == -1 && other.Episode != -1 {
		m.Episode = other.Episode
	}

	// TMDB ID 优先级：文件名 > 父目录 > 上上级目录
	if m.TmdbId == 0 && other.TmdbId != 0 {
		m.TmdbId = other.TmdbId
	}

	// 名称优先级：文件名 > 父目录 > 上上级目录
	// 如果当前没有名称，从父目录借用
	if m.Name == "" && other.Name != "" {
		m.Name = other.Name
	}
}
