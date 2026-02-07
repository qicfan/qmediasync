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
	"syscall"
	"time"

	_ "gorm.io/driver/postgres"
)

type EmbeddedManager struct {
	config  *Config
	db      *sql.DB
	process *os.Process
}

func NewEmbeddedManager(config *Config) *EmbeddedManager {
	return &EmbeddedManager{
		config: config,
	}
}

func (m *EmbeddedManager) Start(ctx context.Context) error {
	helpers.AppLogger.Info("启动嵌入式 PostgreSQL...")

	if !m.config.External {
		// 确保 PostgreSQL 二进制文件存在
		if err := m.ensurePostgresBinaries(); err != nil {
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

		// 等待服务启动
		if err := m.waitForPostgres(ctx); err != nil {
			return err
		}
	}
	// 连接数据库
	return m.connectToDB()
}

func (m *EmbeddedManager) Stop() error {
	if m.process != nil {
		helpers.AppLogger.Infof("停止 PostgreSQL 进程 (PID: %d)", m.process.Pid)

		// 使用 pg_ctl 优雅停止
		pgctlPath := filepath.Join(m.config.BinaryPath, "pg_ctl")
		if runtime.GOOS == "windows" {
			pgctlPath += ".exe"
		}

		cmd := exec.Command(pgctlPath, "stop", "-D", m.config.DataDir, "-m", "fast")
		// --- 新增：隐藏退出时的黑框 ---
		if runtime.GOOS == "windows" {
			cmd.SysProcAttr = &syscall.SysProcAttr{
				HideWindow:    true,
				CreationFlags: 0x08000000,
			}
		}
		cmd.Run()

		// 如果优雅停止失败，强制杀死进程
		time.Sleep(2 * time.Second)
		if m.process != nil {
			m.process.Kill()
		}
	}

	if m.db != nil {
		m.db.Close()
	}

	return nil
}

func (m *EmbeddedManager) GetDB() *sql.DB {
	return m.db
}

func (m *EmbeddedManager) HealthCheck() error {
	if m.db == nil {
		return fmt.Errorf("数据库未连接")
	}
	return m.db.Ping()
}

func (m *EmbeddedManager) Backup(ctx context.Context, backupPath string) error {
	pgDumpPath := filepath.Join(m.config.BinaryPath, "pg_dump")
	if runtime.GOOS == "windows" {
		pgDumpPath += ".exe"
	}

	connStr := fmt.Sprintf("postgresql://%s:%s@%s:%d/%s",
		m.config.User, m.config.Password, m.config.Host, m.config.Port, m.config.DBName)

	cmd := exec.CommandContext(ctx, pgDumpPath, "-d", connStr, "-f", backupPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("备份失败: %s, 错误: %v", string(output), err)
	}

	helpers.AppLogger.Infof("数据库已备份到: %s", backupPath)
	return nil
}

func (m *EmbeddedManager) Restore(ctx context.Context, backupPath string) error {
	psqlPath := filepath.Join(m.config.BinaryPath, "psql")
	if runtime.GOOS == "windows" {
		psqlPath += ".exe"
	}

	connStr := fmt.Sprintf("postgresql://%s:%s@%s:%d/%s",
		m.config.User, m.config.Password, m.config.Host, m.config.Port, m.config.DBName)

	cmd := exec.CommandContext(ctx, psqlPath, "-d", connStr, "-f", backupPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("恢复失败: %s, 错误: %v", string(output), err)
	}

	helpers.AppLogger.Infof("数据库已从备份恢复: %s", backupPath)
	return nil
}

func (m *EmbeddedManager) ensurePostgresBinaries() error {
	// 检查必要的二进制文件
	requiredBins := []string{"initdb", "postgres", "pg_ctl", "pg_isready", "psql"}

	for _, bin := range requiredBins {
		binPath := filepath.Join(m.config.BinaryPath, bin)
		if runtime.GOOS == "windows" {
			binPath += ".exe"
		}

		if _, err := os.Stat(binPath); os.IsNotExist(err) {
			return fmt.Errorf("PostgreSQL 二进制文件缺失: %s", binPath)
		}
	}

	return nil
}

func (m *EmbeddedManager) prepareDataDir() error {
	postgresRoot := filepath.Join(helpers.ConfigDir, "postgres")
	if !helpers.PathExists(postgresRoot) {
		if err := os.MkdirAll(postgresRoot, 0755); err != nil {
			return fmt.Errorf("创建数据目录失败 %s: %v", postgresRoot, err)
		}
	}
	logDir := m.config.LogDir
	tmpPath := filepath.Join(postgresRoot, "tmp")
	if helpers.PathExists(tmpPath) {
		os.RemoveAll(tmpPath)
	}
	os.MkdirAll(tmpPath, 0755)
	if helpers.PathExists(logDir) {
		os.RemoveAll(logDir)
	}
	os.MkdirAll(logDir, 0755) // 如果没有log目录则创建
	// 检查是否已经初始化
	pgVersionFile := filepath.Join(m.config.DataDir, "PG_VERSION")
	if _, err := os.Stat(pgVersionFile); os.IsNotExist(err) {
		helpers.AppLogger.Info("初始化 PostgreSQL 数据库...")

		initdbPath := filepath.Join(m.config.BinaryPath, "initdb")
		if runtime.GOOS == "windows" {
			initdbPath += ".exe"
		}

		cmd := exec.Command(initdbPath, "-D", m.config.DataDir, "-U", m.config.User,
			"--encoding=UTF8", "--locale=C", "--auth=trust")
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		if runtime.GOOS == "windows" {
			cmd.SysProcAttr = getSysProcAttr()
		}
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("初始化数据库失败: %s, 错误: %v", string(output), err)
		}
		helpers.AppLogger.Info("数据库初始化完成")
	}

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
log_directory = '%s'
log_filename = 'postgresql.log'
log_file_mode = 0644
log_rotation_age = 1d
log_rotation_size = 100MB
log_truncate_on_rotation = on
log_min_error_statement = error
log_min_duration_statement = -1
log_checkpoints = on
log_connections = on
log_disconnections = on
log_duration = on
log_line_prefix = '%%t [%%p]: [%%l-1] user=%%u,db=%%d,app=%%a,client=%%h '
log_timezone = 'UTC'
log_autovacuum_min_duration = 0

# 性能相关
wal_level = replica
max_wal_senders = 10
checkpoint_timeout = 10min
checkpoint_completion_target = 0.9

# 内存配置
work_mem = 4MB
maintenance_work_mem = 64MB
effective_cache_size = 1GB

# 其他优化
max_worker_processes = 8
max_parallel_workers_per_gather = 2
max_parallel_workers = 8
`, m.config.Host, m.config.Port, sharedMemoryType, m.formatPathForPostgres(m.config.DataDir), m.formatPathForPostgres(m.config.LogDir))

	if err := os.WriteFile(confPath, []byte(strings.TrimSpace(confContent)), 0750); err != nil {
		return fmt.Errorf("写入 postgresql.conf 失败: %v", err)
	}
	// 配置 pg_hba.conf（保持不变）
	hbaPath := filepath.Join(m.config.DataDir, "pg_hba.conf")
	hbaContent := `
# PostgreSQL Client Authentication Configuration File
local   all             all                                     trust
host    all             all             127.0.0.1/32            trust
host    all             all             ::1/128                 trust
host    all             all             0.0.0.0/0               md5
`
	if err := os.WriteFile(hbaPath, []byte(strings.TrimSpace(hbaContent)), 0750); err != nil {
		return fmt.Errorf("写入 pg_hba.conf 失败: %v", err)
	}

	helpers.AppLogger.Infof("PostgreSQL 配置完成，共享内存类型: %s", sharedMemoryType)

	// 创建日志目录
	logDir := filepath.Join(m.config.DataDir, "log")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %v", err)
	}

	return nil
}

func (m *EmbeddedManager) startPostgresProcess() error {
	tmpPath := filepath.Join(filepath.Dir(m.config.DataDir), "tmp")
	postgresPath := filepath.Join(m.config.BinaryPath, "pg_ctl")
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		postgresPath += ".exe"
		cmd = exec.Command(postgresPath, "start", "-D", m.config.DataDir, "-o", fmt.Sprintf("\"-k %s\"", tmpPath))
		cmd.SysProcAttr = getSysProcAttr()
	} else {
		cmd = exec.Command(postgresPath, "start", "-D", m.config.DataDir, "-o", fmt.Sprintf("\"-k %s -c unix_socket_directories=%s\"", tmpPath, tmpPath))
	}

	// 启动 PostgreSQL
	// 设置输出日志
	logFile, err := os.Create(filepath.Join(m.config.DataDir, "postgres.log"))
	if err != nil {
		return fmt.Errorf("创建日志文件失败: %v", err)
	}

	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 PostgreSQL 失败: %v", err)
	}

	m.process = cmd.Process
	helpers.AppLogger.Infof("PostgreSQL 进程已启动 (PID: %d)", m.process.Pid)

	return nil
}

func (m *EmbeddedManager) waitForPostgres(ctx context.Context) error {
	helpers.AppLogger.Info("等待 PostgreSQL 启动...")

	pgIsReadyPath := filepath.Join(m.config.BinaryPath, "pg_isready")
	if runtime.GOOS == "windows" {
		pgIsReadyPath += ".exe"
	}

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
			cmd := exec.Command(pgIsReadyPath, "-h", m.config.Host, "-p",
				fmt.Sprintf("%d", m.config.Port), "-U", m.config.User)
			if err := cmd.Run(); err == nil {
				helpers.AppLogger.Info("PostgreSQL 已就绪")
				return nil
			} else {
				helpers.AppLogger.Infof("PostgreSQL 启动中... 错误: %v", err)
			}
		}
	}
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
