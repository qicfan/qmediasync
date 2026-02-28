package helpers

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

var Version = "0.0.1"
var ReleaseDate = "2025-11-07"

type DbEngine string

const (
	DbEngineSqlite   DbEngine = "sqlite"
	DbEnginePostgres DbEngine = "postgres"
	DbEngineUnset    DbEngine = ""
)

type PostgresType string

const (
	PostgresTypeEmbedded PostgresType = "embedded"
	PostgresTypeExternal PostgresType = "external"
)

type ConfigLog struct {
	File       string `yaml:"file"`
	V115       string `yaml:"v115"`
	OpenList   string `yaml:"openList"`
	TMDB       string `yaml:"tmdb"`
	BaiduPan   string `yaml:"baiduPan"`
	Web        string `yaml:"web"`
	SyncLogDir string `yaml:"syncLogDir"` // 同步任务的日志目录，每个同步任务会生成一个日志文件，文件名为任务ID
}

type PostgresConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	User         string `yaml:"user"`
	Password     string `yaml:"password"`
	Database     string `yaml:"database"`
	MaxOpenConns int    `yaml:"maxOpenConns"`
	MaxIdleConns int    `yaml:"maxIdleConns"`
}

type ConfigDb struct {
	Engine         DbEngine       `yaml:"engine"`         // 使用的数据库引擎，可选值：sqlite, postgres
	SqliteFile     string         `yaml:"sqliteFile"`     // SQLite数据库文件路径
	PostgresType   PostgresType   `yaml:"postgresType"`   // PostgreSQL数据库类型，可选值：embedded, external
	PostgresConfig PostgresConfig `yaml:"postgresConfig"` // PostgreSQL数据库配置
}

type ConfigStrm struct {
	VideoExt     []string `yaml:"videoExt"`
	MinVideoSize int64    `yaml:"minVideoSize"` // 最小视频大小，单位字节
	MetaExt      []string `yaml:"metaExt"`
	Cron         string   `yaml:"cron"` // 定时任务表达式
}

type Config struct {
	Log           ConfigLog  `yaml:"log"`
	Db            ConfigDb   `yaml:"db"`
	CacheSize     int        `yaml:"cacheSize"` // 数据库缓存大小，单位字节
	JwtSecret     string     `yaml:"jwtSecret"`
	HttpHost      string     `yaml:"httpHost"`  // HTTP主机地址
	HttpsHost     string     `yaml:"httpsHost"` // HTTPS主机地址
	Strm          ConfigStrm `yaml:"strm"`
	AuthServer    string     `yaml:"authServer"`
	BaiDuPanAppId string     `yaml:"baiDuPanAppId"`
	AdminUsername string     `yaml:"adminUsername"`
	AdminPassword string     `yaml:"adminPassword"`
}

var GlobalConfig Config
var RootDir string
var ConfigDir string

var DataDir string
var SharePathes string
var AccessiblePathes string
var IsFnOS bool
var IsRelease bool
var Guid string
var FANART_API_KEY = ""
var DEFAULT_TMDB_ACCESS_TOKEN = ""
var DEFAULT_TMDB_API_KEY = ""
var DEFAULT_SC_API_KEY = ""
var ENCRYPTION_KEY = ""

func InitConfig() error {
	configPath := filepath.Join(ConfigDir, "config.yml")
	// 从配置文件加载
	if err := loadYaml(configPath, &GlobalConfig); err != nil {
		return err
	}
	// 给strm填充默认值
	if len(GlobalConfig.Strm.VideoExt) == 0 {
		GlobalConfig.Strm.VideoExt = []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".3gp", ".ts"}
	}
	if len(GlobalConfig.Strm.MetaExt) == 0 {
		GlobalConfig.Strm.MetaExt = []string{".jpg", ".jpeg", ".png", ".webp", ".nfo", ".srt", ".ass", ".svg", ".sup", ".lrc"}
	}
	// if GlobalConfig.Strm.MinVideoSize == 0 {
	GlobalConfig.Strm.MinVideoSize = 100 // 100MB
	// }
	// if GlobalConfig.Strm.Cron == "" {
	GlobalConfig.Strm.Cron = "30 * * * *" // 每小时30分执行
	// }
	return nil
}

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

func loadYaml(configPath string, cfg interface{}) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	return nil
}

func MakeOldConfig() error {
	yamlConfig := MakeDefaultConfig()
	host := os.Getenv("DB_HOST")
	if host != "" {
		yamlConfig.Db.PostgresConfig.Host = host
	}
	port := os.Getenv("DB_PORT")
	if port != "" {
		yamlConfig.Db.PostgresConfig.Port = StringToInt(port)
	}
	user := os.Getenv("DB_USER")
	if user != "" {
		yamlConfig.Db.PostgresConfig.User = user
	}
	password := os.Getenv("DB_PASSWORD")
	if password != "" {
		yamlConfig.Db.PostgresConfig.Password = password
	}
	database := os.Getenv("DB_NAME")
	if database != "" {
		yamlConfig.Db.PostgresConfig.Database = database
	}

	return SaveConfig(yamlConfig)
}

func SaveConfig(config *Config) error {
	configPath := filepath.Join(ConfigDir, "config.yml")
	configData, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return err
	}
	return nil
}

func MakeDefaultConfig() *Config {
	return &Config{
		Log: ConfigLog{
			File:       "logs/app.log",
			V115:       "logs/115.log",
			OpenList:   "logs/openList.log",
			TMDB:       "logs/tmdb.log",
			BaiduPan:   "logs/baidupan.log",
			Web:        "logs/web.log",
			SyncLogDir: "logs/sync",
		},
		Db: ConfigDb{
			Engine:       DbEnginePostgres,
			SqliteFile:   "qmediasync.db",
			PostgresType: PostgresTypeEmbedded,
			PostgresConfig: PostgresConfig{
				Host:         "localhost",
				Port:         5432,
				User:         "qms",
				Password:     "qms123456",
				Database:     "qms",
				MaxOpenConns: 25,
				MaxIdleConns: 25,
			},
		},
		CacheSize:     20971520,
		JwtSecret:     "Q115-STRM-JWT-TOKEN-250706",
		HttpHost:      ":12333",
		HttpsHost:     ":12332",
		AuthServer:    "https://api.mqfamily.top",
		BaiDuPanAppId: "QMediaSync",
		AdminUsername: "admin",
		AdminPassword: "admin123",
		Strm: ConfigStrm{
			VideoExt:     []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".3gp", ".ts"},
			MetaExt:      []string{".jpg", ".jpeg", ".png", ".webp", ".nfo", ".srt", ".ass", ".svg", ".sup", ".lrc"},
			MinVideoSize: 100,          // 100MB
			Cron:         "30 * * * *", // 每小时30分执行
		},
	}
}
