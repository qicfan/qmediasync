package db

import (
	"Q115-STRM/internal/db/database"
	"Q115-STRM/internal/helpers"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/glebarez/sqlite"
	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var Db *gorm.DB

// 获取一个数据库连接
func InitSqlite3(dbFile string) *gorm.DB {
	// if !helpers.PathExists(dbFile) {
	// 	return nil
	// }
	sqliteDb, err := gorm.Open(sqlite.Open(dbFile+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		panic(fmt.Sprintf("failed to connect database: %v", err))
	}
	// sqlDB, dbError := sqliteDb.DB()
	// if dbError != nil {
	// 	return nil
	// }

	// 2. 设置 busy_timeout (例如 5000 毫秒)
	if tx := sqliteDb.Exec("PRAGMA busy_timeout = 5000"); tx.Error != nil {
		panic(fmt.Errorf("设置 busy_timeout 失败: %w", tx.Error))
	} else {
		helpers.AppLogger.Infof("设置 busy_timeout 成功: %d 毫秒", 5000)
	}

	// 3. 启用 WAL 模式
	if tx := sqliteDb.Exec("PRAGMA journal_mode = WAL"); tx.Error != nil {
		panic(fmt.Errorf("启用 WAL 失败: %w", tx.Error))
	} else {
		helpers.AppLogger.Infof("启用 WAL 成功: %s", "WAL")
	}

	// 可选：调整同步模式以提升写入性能 (从 FULL 改为 NORMAL)
	// 在 WAL 模式下，NORMAL 是安全且更快的选择[citation:2]
	if tx := sqliteDb.Exec("PRAGMA synchronous = NORMAL"); tx.Error != nil {
		panic(fmt.Errorf("设置 synchronous 失败: %w", tx.Error))
	} else {
		helpers.AppLogger.Infof("设置 synchronous 成功: %s", "NORMAL")
	}
	return sqliteDb
}

// 连接外部PostgreSQL数据库
func ConnectPostgres(dbConfig *database.Config) error {
	// 配置Logger
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             200 * time.Millisecond, // 慢SQL阈值
			LogLevel:                  logger.Warn,            // 日志级别
			IgnoreRecordNotFoundError: true,                   // 忽略ErrRecordNotFound（记录未找到）错误
			Colorful:                  true,                   // 禁用彩色打印
		},
	)

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s",
		dbConfig.Host, dbConfig.Port, dbConfig.User, dbConfig.Password, dbConfig.SSLMode)
	helpers.AppLogger.Infof("连接数据库: %s", connStr)
	sqlDB, cerr := sql.Open("postgres", connStr)
	if cerr != nil {
		helpers.AppLogger.Errorf("连接数据库失败: %v", cerr)
		return cerr
	}
	// 检查数据库是否存在，没有则新建
	// 检查数据库是否已存在
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)"
	eerr := sqlDB.QueryRow(query, dbConfig.DBName).Scan(&exists)
	if eerr != nil {
		return eerr
	}

	// 如果不存在，则创建
	if !exists {
		_, cerr = sqlDB.Exec(fmt.Sprintf("CREATE DATABASE %s", dbConfig.DBName))
		if cerr != nil {
			return fmt.Errorf("创建数据库失败: %v", cerr)
		}
		log.Printf("数据库 %s 创建成功", dbConfig.DBName)
	} else {
		log.Printf("数据库 %s 已存在", dbConfig.DBName)
	}
	// 重新连接数据库
	connStr = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		dbConfig.Host, dbConfig.Port, dbConfig.User, dbConfig.Password, dbConfig.DBName, dbConfig.SSLMode)
	helpers.AppLogger.Infof("重新连接数据库: %s", connStr)
	sqlDB, cerr = sql.Open("postgres", connStr)
	if cerr != nil {
		helpers.AppLogger.Errorf("连接数据库失败: %v", cerr)
		return cerr
	}
	// 配置连接池
	sqlDB.SetMaxOpenConns(dbConfig.MaxOpenConns) // 最多打开25个连接
	sqlDB.SetMaxIdleConns(dbConfig.MaxIdleConns) // 最多5个空闲连接
	sqlDB.SetConnMaxLifetime(60 * time.Minute)   // 连接最多使用60分钟
	sqlDB.SetConnMaxIdleTime(1 * time.Minute)    // 空闲超过1分钟则关闭
	var err error
	pg := postgres.New(postgres.Config{Conn: sqlDB})
	Db, err = gorm.Open(pg, &gorm.Config{})
	if err != nil {
		panic(fmt.Sprintf("failed to connect database: %v", err))
	}
	// 设置全局Logger
	Db.Logger = newLogger
	helpers.AppLogger.Info("成功初始化数据库组件")

	return nil
}

func InitPostgres(sqlDB *sql.DB) {
	if sqlDB == nil {
		panic("数据库连接失败")
	}
	// 配置Logger
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             200 * time.Millisecond, // 慢SQL阈值
			LogLevel:                  logger.Warn,            // 日志级别
			IgnoreRecordNotFoundError: true,                   // 忽略ErrRecordNotFound（记录未找到）错误
			Colorful:                  true,                   // 禁用彩色打印
		},
	)
	// 配置连接池
	sqlDB.SetMaxOpenConns(25)                  // 最多打开25个连接
	sqlDB.SetMaxIdleConns(5)                   // 最多5个空闲连接
	sqlDB.SetConnMaxLifetime(60 * time.Minute) // 连接最多使用5分钟
	sqlDB.SetConnMaxIdleTime(1 * time.Minute)  // 空闲超过10秒则关闭
	var err error
	Db, err = gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{})
	if err != nil {
		panic(fmt.Sprintf("failed to connect database: %v", err))
	}
	// 设置全局Logger
	Db.Logger = newLogger
	// 启动连接保持的goroutine（使用GORM的DB对象）
	go keepGormAlive()
	helpers.AppLogger.Info("成功初始化数据库组件")
}

// keepGormAlive 使用GORM的原始数据库连接进行ping
func keepGormAlive() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		sqlDB, err := Db.DB()
		if err != nil {
			helpers.AppLogger.Errorf("获取数据库连接失败: %v", err)
			continue
		}

		if err := sqlDB.Ping(); err != nil {
			helpers.AppLogger.Errorf("数据库ping失败: %v", err)
		} else {
			// helpers.AppLogger.Debug("数据库ping成功")
		}
	}
}

// IsPostgres 判断当前使用的是否为PostgreSQL数据库
func IsPostgres() bool {
	return helpers.GlobalConfig.Db.Engine == helpers.DbEnginePostgres
}

// getPostgresBinaryPath 获取PostgreSQL二进制路径
func GetPostgresBinaryPath(embeddedBasePath string) string {
	if helpers.IsRunningInDocker() {
		return "" // Docker 容器中的路径
	}
	// 根据平台返回二进制路径
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	var binDir string
	switch goos {
	case "windows":
		binDir = filepath.Join(embeddedBasePath, "windows", goarch, "bin")
	default:
		return ""
	}
	return binDir
}

// func ClearDbLock(configRootPath string) {
// 	// 检查数据库文件是否存在
// 	file1 := filepath.Join(configRootPath, "db.db-shm")
// 	file2 := filepath.Join(configRootPath, "db.db-wal")
// 	file3 := filepath.Join(configRootPath, "db.db-journal")
// 	// 检查文件是否存在
// 	if _, err := os.Stat(file1); err == nil {
// 		os.Remove(file1)
// 	}
// 	if _, err := os.Stat(file2); err == nil {
// 		os.Remove(file2)
// 	}
// 	if _, err := os.Stat(file3); err == nil {
// 		os.Remove(file3)
// 	}
// 	helpers.AppLogger.Info("已清除数据库锁文件")
// }
