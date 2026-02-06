package helpers

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

var Version = "0.0.1"
var ReleaseDate = "2025-11-07"

type configLog struct {
	File       string `yaml:"file"`
	V115       string `yaml:"v115"`
	OpenList   string `yaml:"openList"`
	TMDB       string `yaml:"tmdb"`
	Web        string `yaml:"web"`
	SyncLogDir string `yaml:"syncLogDir"` // 同步任务的日志目录，每个同步任务会生成一个日志文件，文件名为任务ID
}
type configDb struct {
	File      string `yaml:"file"`
	CacheSize int    `yaml:"cacheSize"`
}
type configStrm struct {
	VideoExt     []string `yaml:"videoExt"`
	MinVideoSize int64    `yaml:"minVideoSize"` // 最小视频大小，单位字节
	MetaExt      []string `yaml:"metaExt"`
	Cron         string   `yaml:"cron"` // 定时任务表达式
}
type Config struct {
	Log              configLog  `yaml:"log"`
	Db               configDb   `yaml:"db"`
	JwtSecret        string     `yaml:"jwtSecret"`
	WebHost          string     `yaml:"webHost"`
	Strm             configStrm `yaml:"strm"`
	Open115AppId     string     `yaml:"open115AppId"`
	Open115TestAppId string     `yaml:"open115TestAppId"`
	BaiDuPanAppId    string     `yaml:"baiDuPanAppId"`
}

var GlobalConfig Config
var RootDir string
var ConfigDir string
var DataDir string
var IsRelease bool
var Guid string
var FANART_API_KEY = ""
var DEFAULT_TMDB_ACCESS_TOKEN = ""
var DEFAULT_TMDB_API_KEY = ""
var DEFAULT_SC_API_KEY = ""
var ENCRYPTION_KEY = ""

func InitConfig() error {
	GlobalConfig = Config{
		Log: configLog{
			File:     "logs/app.log",
			V115:     "logs/115.log",
			OpenList: "logs/openList.log",
			TMDB:     "logs/tmdb.log",
		},
		Db: configDb{
			File:      "db.db",
			CacheSize: 20971520, // 默认20MB
		},
		JwtSecret: "Q115-STRM-JWT-TOKEN-250706",
		WebHost:   ":12333",
		Strm: configStrm{
			VideoExt:     []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".webm", ".flv", ".avi", ".ts", ".m4v"},
			MinVideoSize: 100, // 默认100MB
			MetaExt:      []string{".jpg", ".jpeg", ".png", ".webp", ".gif", ".nfo", ".srt", ".ass", ".svg", ".sup", ".lrc"},
			Cron:         "0 * * * *", // 默认每小时执行一次
		},
		Open115AppId:     "",
		Open115TestAppId: "",
		BaiDuPanAppId:    "QMediaSync",
	}
	return nil
}

// LoadEnvFromFile loads environment variables from a simple KEY=VALUE file.
// Lines starting with # and blank lines are ignored. Keys are trimmed; values are kept as-is.
func LoadEnvFromFile(envPath string) error {
	f, err := os.Open(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("环境变量配置文件不存在: %s\n", envPath)
			return nil
		}
		return err
	}
	defer f.Close()
	fmt.Printf("已加载环境变量配置文件：%s\n", envPath)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		if key == "" {
			continue
		}
		value := line[idx+1:]
		os.Setenv(key, value)
		// fmt.Printf("Loaded env: %s=%s\n", key, value)
	}

	return scanner.Err()
}
