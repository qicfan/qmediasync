package config

import (
	"Q115-STRM/internal/helpers"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

type Config struct {
	App struct {
		Mode string // "binary" 或 "docker"
	}
	DB struct {
		Host           string
		Port           int
		User           string
		Password       string
		Name           string
		SSLMode        string
		MaxOpenConns   int
		MaxIdleConns   int
		LogDir         string
		DataDir        string
		BinaryBasePath string
		BinaryPath     string // 嵌入式 PostgreSQL 二进制路径
		External       bool   // 是否连接外部数据库
	}
}

func Load() *Config {
	var cfg Config
	cfg.DB.BinaryBasePath = helpers.DataDir
	// 应用模式检测
	cfg.App.Mode = getEnv("APP_MODE", "binary")
	if helpers.IsRunningInDocker() {
		cfg.App.Mode = "docker"
	}

	// 数据库配置
	cfg.DB.Host = getEnv("DB_HOST", "localhost")
	cfg.DB.Port = getEnvInt("DB_PORT", 5432)
	cfg.DB.User = getEnv("DB_USER", "qms")
	cfg.DB.Password = getEnv("DB_PASSWORD", "qms123456")
	cfg.DB.Name = getEnv("DB_NAME", "qms")
	cfg.DB.SSLMode = getEnv("DB_SSLMODE", "disable")
	cfg.DB.MaxOpenConns = getEnvInt("DB_MAX_OPEN_CONNS", 25)
	cfg.DB.MaxIdleConns = getEnvInt("DB_MAX_IDLE_CONNS", 25)
	cfg.DB.LogDir = filepath.Join(helpers.ConfigDir, "postgres", "log")
	cfg.DB.DataDir = filepath.Join(helpers.ConfigDir, "postgres", "data")

	if cfg.DB.Password != "qms123456" || cfg.DB.Host != "localhost" || cfg.DB.User != "qms" {
		cfg.DB.External = true
		fmt.Printf("\n检测到外部数据库配置，将连接外部PostgreSQL 数据库连接信息：postgres://%s:%s@%s:%d/%s\n", cfg.DB.User, "******", cfg.DB.Host, cfg.DB.Port, cfg.DB.Name)
	} else {
		fmt.Printf("\n使用默认数据库配置，将使用嵌入式PostgreSQL\n")
	}
	// 数据目录配置
	// if cfg.App.Mode == "docker" {

	dataDir := getEnv("DB_DATA_DIR", "")
	if dataDir != "" {
		cfg.DB.DataDir = dataDir
	}
	// 二进制路径配置
	cfg.DB.BinaryPath = getPostgresBinaryPath(cfg.App.Mode, cfg.DB.BinaryBasePath)
	return &cfg
}

func getPostgresBinaryPath(mode string, embeddedBasePath string) string {
	if mode == "docker" {
		return "/usr/lib/postgresql/15/bin" // Docker 容器中的路径
	}

	// 二进制发行模式，使用嵌入的 PostgreSQL
	baseDir := "./postgres"
	if mode == "binary" {
		baseDir = embeddedBasePath
	}

	// 根据平台返回二进制路径
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	var binDir string
	switch goos {
	case "windows":
		binDir = filepath.Join(baseDir, "windows", goarch, "bin")
	case "darwin":
		binDir = filepath.Join(baseDir, "darwin", goarch, "bin")
	case "linux":
		binDir = filepath.Join(baseDir, "linux", goarch, "bin")
	default:
		binDir = filepath.Join(baseDir, goos, goarch, "bin")
	}

	return binDir
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
