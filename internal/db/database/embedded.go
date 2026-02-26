package database

import (
	"Q115-STRM/internal/helpers"
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "gorm.io/driver/postgres"
)

type Config struct {
	Mode         helpers.PostgresType // "embedded" 或 "external"
	Host         string
	Port         int
	User         string
	Password     string
	DBName       string
	SSLMode      string
	LogDir       string
	DataDir      string
	BinaryPath   string
	MaxOpenConns int
	MaxIdleConns int
}

type EmbeddedManager struct {
	config       *Config
	db           *sql.DB
	process      *os.Process
	userSwitcher *UserSwitcher
	UserName     string
	GroupName    string
}

func NewEmbeddedManager(config *Config) *EmbeddedManager {
	manager := &EmbeddedManager{
		config: config,
	}
	if helpers.Guid == "" || helpers.Guid == "0" {
		manager.UserName = "qms"
		manager.GroupName = "qms"
		manager.userSwitcher = NewUserSwitcher(manager.UserName)
	} else {
		manager.userSwitcher = NewUserSwitcher("")
	}
	return manager
}

func (m *EmbeddedManager) Start(ctx context.Context) error {
	helpers.AppLogger.Info("启动内嵌 PostgreSQL...")
	// 初始化目录和权限
	if err := m.InitDataDir(); err != nil {
		return err
	}

	// 准备数据目录
	if err := m.prepareDataDir(); err != nil {
		return err
	}

	// 初始化数据库
	if err := m.initDatabase(); err != nil {
		return err
	}

	// 启动 PostgreSQL 进程
	if err := m.startPostgresProcess(); err != nil {
		return err
	}
	// 等待数据库可用
	if err := m.waitForPostgres(ctx); err != nil {
		return err
	}
	// 连接数据库
	return m.connectToDB()
}

func (m *EmbeddedManager) Stop() error {
	helpers.AppLogger.Info("停止内嵌的 PostgreSQL...")

	if m.process != nil {
		postgresPath := "pg_ctl"
		if runtime.GOOS == "windows" {
			postgresPath = filepath.Join(m.config.BinaryPath, "pg_ctl.exe")
		}
		// 使用 pg_ctl 优雅停止，使用qms用户执行
		output, err := m.userSwitcher.RunCommandAsUser(postgresPath, "stop", "-D", m.config.DataDir, "-m", "fast")
		if err != nil {
			helpers.AppLogger.Errorf("pg_ctl stop 执行失败: %v", err)
			return err
		}
		helpers.AppLogger.Infof("pg_ctl stop 输出: %s", output)

		time.Sleep(2 * time.Second)
	}

	return nil
}

func (m *EmbeddedManager) InitDataDir() error {
	// 先重建目录
	postgresRoot := filepath.Dir(m.config.DataDir)
	if !helpers.PathExists(postgresRoot) {
		// 如果没有config/postgres目录则创建
		if err := os.MkdirAll(postgresRoot, 4750); err != nil {
			return err
		}
		helpers.AppLogger.Infof("创建Postgres目录 %s 成功", postgresRoot)
	}
	if !helpers.PathExists(m.config.DataDir) {
		if err := os.MkdirAll(m.config.DataDir, 4750); err != nil {
			return err
		}
		helpers.AppLogger.Infof("创建Postgres数据目录 %s 成功", m.config.DataDir)
	}
	postmasterFile := filepath.Join(m.config.DataDir, "postmaster.pid")
	if helpers.PathExists(postmasterFile) {
		if err := os.Remove(postmasterFile); err != nil {
			return err
		}
		helpers.AppLogger.Infof("删除Postgres postmaster.pid 文件 %s 成功", postmasterFile)
	}
	logDir := filepath.Join(postgresRoot, "log")
	if helpers.PathExists(logDir) {
		// 删除掉日志文件
		if err := os.RemoveAll(logDir); err != nil {
			return err
		}
		helpers.AppLogger.Infof("删除Postgres日志目录 %s 成功", logDir)
		if err := os.MkdirAll(logDir, 4750); err != nil {
			return err
		}
		helpers.AppLogger.Infof("创建Postgres日志目录 %s 成功", logDir)
	}
	tmpDir := filepath.Join(postgresRoot, "tmp")
	if !helpers.PathExists(tmpDir) {
		if err := os.MkdirAll(tmpDir, 4750); err != nil {
			return err
		}
		helpers.AppLogger.Infof("创建Postgres临时目录 %s 成功", tmpDir)
	}
	// 再修改权限
	if (helpers.Guid != "" && helpers.Guid != "0") || runtime.GOOS == "windows" {
		// 如果是非root用户启动，postgres用guid启动即可，无需修改权限
		// windows无需修改权限
		return nil
	}
	exec.Command("chown", "-R", fmt.Sprintf("%s:%s", m.UserName, m.GroupName), postgresRoot).Run() // 设置目录所有者为qms:qms
	helpers.AppLogger.Infof("设置Postgres目录 %s 所有者为%s:%s成功", postgresRoot, m.UserName, m.GroupName)
	exec.Command("chown", "-R", fmt.Sprintf("%s:%s", m.UserName, m.GroupName), m.config.DataDir).Run()
	helpers.AppLogger.Infof("设置Postgres数据目录 %s 所有者为%s:%s成功", m.config.DataDir, m.UserName, m.GroupName)
	exec.Command("chown", "-R", fmt.Sprintf("%s:%s", m.UserName, m.GroupName), logDir).Run()
	helpers.AppLogger.Infof("设置Postgres日志目录 %s 所有者为%s:%s成功", logDir, m.UserName, m.GroupName)
	exec.Command("chown", "-R", fmt.Sprintf("%s:%s", m.UserName, m.GroupName), tmpDir).Run()
	helpers.AppLogger.Infof("设置Postgres临时目录 %s 所有者为%s:%s成功", tmpDir, m.UserName, m.GroupName)
	return nil
}

func (m *EmbeddedManager) prepareDataDir() error {
	// 检查是否已经初始化
	pgVersionFile := filepath.Join(m.config.DataDir, "PG_VERSION")
	if helpers.PathExists(pgVersionFile) {
		return nil
	}

	// 使用 qms 用户初始化数据库
	initdbPath := "initdb"
	if runtime.GOOS == "windows" {
		initdbPath = filepath.Join(m.config.BinaryPath, "initdb.exe")
	}
	output, err := m.userSwitcher.RunCommandAsUser(initdbPath, "-D", m.config.DataDir, "-U", m.config.User, "--encoding=UTF8", "--locale=C", "--auth=trust")
	if err != nil {
		return fmt.Errorf("数据库用户初始化失败: %v, 输出: %s", err, output)
	}
	helpers.AppLogger.Info("数据库初始化完成")
	return nil
}

// 添加路径处理函数
func (m *EmbeddedManager) formatPathForPostgres(path string) string {
	if runtime.GOOS == "windows" {
		// Windows 中 PostgreSQL 配置需要正斜杠或双反斜杠
		// 将路径转换为 Windows 可识别的格式
		path = filepath.Clean(path)

		// 方法1: 使用正斜杠（推荐，跨平台兼容）
		path = strings.ReplaceAll(path, "\\", "/")

		// 或者方法2: 使用双反斜杠
		// path = strings.ReplaceAll(path, "\\", "\\\\")

		// 如果路径包含空格，确保正确转义
		if strings.Contains(path, " ") {
			path = "\"" + path + "\""
		}
	}
	return path
}

func (m *EmbeddedManager) initDatabase() error {
	// 检测操作系统并选择合适的共享内存类型
	sharedMemoryType := m.getSharedMemoryType()
	// 配置 postgresql.conf
	confPath := filepath.Join(m.config.DataDir, "postgresql.conf")
	confContent := fmt.Sprintf(`
# 基本配置
listen_addresses = '%s'
port = %d
max_connections = 100
shared_buffers = 128MB
dynamic_shared_memory_type = %s
unix_socket_directories = '%s'

# 日志配置
log_destination = 'stderr'
logging_collector = on
log_file_mode = 0644
log_rotation_age = 1d
log_rotation_size = 10MB
log_truncate_on_rotation = on
log_min_error_statement = error
log_min_duration_statement = -1

# 性能相关
wal_level = replica
max_wal_senders = 10
checkpoint_timeout = 10min
checkpoint_completion_target = 0.9

# 内存配置
work_mem = 16MB
maintenance_work_mem = 64MB
effective_cache_size = 1GB

# 其他优化
max_worker_processes = 8
max_parallel_workers_per_gather = 2
max_parallel_workers = 8
`, m.config.Host, m.config.Port, sharedMemoryType, m.formatPathForPostgres(m.config.DataDir))

	if err := os.WriteFile(confPath, []byte(strings.TrimSpace(confContent)), 4750); err != nil {
		return fmt.Errorf("写入 postgresql.conf 失败: %v", err)
	}
	if runtime.GOOS != "windows" && m.UserName != "" {
		// 改变所有者
		exec.Command("chown", "-R", fmt.Sprintf("%s:%s", m.UserName, m.GroupName), confPath).Run()
		helpers.AppLogger.Infof("设置Postgres配置文件 %s 所有者为%s:%s成功", confPath, m.UserName, m.GroupName)
	}
	// 配置 pg_hba.conf（保持不变）
	hbaPath := filepath.Join(m.config.DataDir, "pg_hba.conf")
	hbaContent := `
# PostgreSQL Client Authentication Configuration File
local   all             all                                     trust
host    all             all             127.0.0.1/32            trust
host    all             all             ::1/128                 trust
`
	if err := os.WriteFile(hbaPath, []byte(strings.TrimSpace(hbaContent)), 4750); err != nil {
		return fmt.Errorf("写入 pg_hba.conf 失败: %v", err)
	}
	if runtime.GOOS != "windows" && m.UserName != "" {
		// 改变所有者
		exec.Command("chown", "-R", fmt.Sprintf("%s:%s", m.UserName, m.GroupName), hbaPath).Run()
		helpers.AppLogger.Infof("设置Postgres HBA文件 %s 所有者为%s:%s成功", hbaPath, m.UserName, m.GroupName)
	}

	helpers.AppLogger.Infof("PostgreSQL 配置完成，共享内存类型: %s", sharedMemoryType)
	return nil
}

func (m *EmbeddedManager) startPostgresProcess() error {
	tmpPath := filepath.Join(filepath.Dir(m.config.DataDir), "tmp")
	var cmd *exec.Cmd
	postgresPath := "pg_ctl"
	if runtime.GOOS == "windows" {
		postgresPath = filepath.Join(m.config.BinaryPath, "pg_ctl.exe")
	}
	command := fmt.Sprintf("%s start -D %s -o '-k %s'", postgresPath, m.config.DataDir, tmpPath)
	cmd, err := m.userSwitcher.RunCommandAsUserWithEnv(
		map[string]string{
			"PGDATA": m.config.DataDir,
			"PGPORT": fmt.Sprintf("%d", m.config.Port),
		},
		command,
	)
	// pg_ctl start -D /app/config/postgres/data -o "-k /app/config/postgres/tmp -c unix_socket_directories='/app/config/postgres/tmp'"
	// su - qms -c "postgres -D /app/config/postgres/data -k /app/config/postgres/tmp -c unix_socket_directories='/app/config/postgres/tmp'"
	if err != nil {
		helpers.AppLogger.Errorf("启动 PostgreSQL 失败: %v", err)
		return fmt.Errorf("启动 PostgreSQL 失败: %v", err)
	}
	m.process = cmd.Process
	helpers.AppLogger.Infof("PostgreSQL 进程已启动 (PID: %d)", m.process.Pid)

	return nil
}

func (m *EmbeddedManager) waitForPostgres(ctx context.Context) error {
	helpers.AppLogger.Infof("等待 PostgreSQL 在 %s:%d 启动...", m.config.Host, m.config.Port)

	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("等待 PostgreSQL 启动超时")
		case <-ticker.C:
			pgIsReadyPath := "pg_isready"
			if runtime.GOOS == "windows" {
				pgIsReadyPath = filepath.Join(m.config.BinaryPath, "pg_isready.exe")
			}
			cmd := exec.Command(pgIsReadyPath, "-h", m.config.Host, "-p",
				fmt.Sprintf("%d", m.config.Port), "-U", m.config.User)
			helpers.AppLogger.Infof("执行命令: %s", cmd.String())
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err == nil {
				helpers.AppLogger.Info("PostgreSQL 已就绪")
				return nil
			} else {
				helpers.AppLogger.Infof("PostgreSQL 启动中... 错误: %v", err)
			}
		}
	}
}

// 根据操作系统选择合适的共享内存类型
func (m *EmbeddedManager) getSharedMemoryType() string {
	// 检查是否在 Alpine Linux 中运行
	if m.isAlpineLinux() {
		helpers.AppLogger.Info("检测到 Alpine Linux，使用 sysv 共享内存")
		return "sysv"
	}

	// 检查是否在 Docker 中运行
	if helpers.IsRunningInDocker() {
		helpers.AppLogger.Info("检测到 Docker 环境，使用 mmap 共享内存")
		return "mmap"
	}

	// 默认情况下，根据操作系统选择
	switch runtime.GOOS {
	case "linux":
		// 检查是否是 musl libc (Alpine)
		if m.isMuslLibc() {
			return "sysv"
		}
		return "posix"
	case "darwin":
		return "posix"
	case "windows":
		return "windows"
	default:
		return "sysv"
	}
}

// 检测是否是 Alpine Linux
func (m *EmbeddedManager) isAlpineLinux() bool {
	// 检查 /etc/alpine-release 文件
	if _, err := os.Stat("/etc/alpine-release"); err == nil {
		return true
	}

	// 检查 os-release 文件
	if content, err := os.ReadFile("/etc/os-release"); err == nil {
		if strings.Contains(strings.ToLower(string(content)), "alpine") {
			return true
		}
	}

	return false
}

// 检测是否是 musl libc
func (m *EmbeddedManager) isMuslLibc() bool {
	// 尝试执行 ldd 命令来检测
	cmd := exec.Command("ldd", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}

	return strings.Contains(strings.ToLower(string(output)), "musl")
}

func (m *EmbeddedManager) connectToDB() error {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s",
		m.config.Host, m.config.Port, m.config.User, m.config.Password, m.config.SSLMode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("连接数据库失败: %v", err)
	}

	// 测试连接
	if derr := db.Ping(); derr != nil {
		db.Close()
		return fmt.Errorf("数据库连接测试失败: %v", derr)
	}

	m.db = db

	// 创建应用数据库
	if cerr := m.createAppDatabase(); cerr != nil {
		return cerr
	}

	// 重新连接到应用数据库
	connStr = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		m.config.Host, m.config.Port, m.config.User, m.config.Password, m.config.DBName, m.config.SSLMode)

	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("连接到应用数据库失败: %v", err)
	}

	m.db = db
	helpers.AppLogger.Info("成功连接到嵌入式数据库")

	return nil
}

func (m *EmbeddedManager) createAppDatabase() error {
	var exists bool
	err := m.db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM pg_database WHERE datname = $1
		)`, m.config.DBName).Scan(&exists)

	if err != nil {
		return fmt.Errorf("检查数据库存在性失败: %v", err)
	}

	if !exists {
		helpers.AppLogger.Infof("创建数据库: %s", m.config.DBName)
		_, err = m.db.Exec(fmt.Sprintf("CREATE DATABASE %s", m.config.DBName))
		if err != nil {
			helpers.AppLogger.Errorf("创建数据库失败: %v\n", err)
		}
		helpers.AppLogger.Info("数据库创建成功")
	}

	return nil
}

func (m *EmbeddedManager) GetDB() *sql.DB {
	return m.db
}
