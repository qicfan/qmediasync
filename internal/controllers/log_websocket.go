package controllers

import (
	"Q115-STRM/internal/helpers"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// LogEntry 日志条目结构
type LogEntry struct {
	Level     string `json:"level"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"` // 时间字符串，格式为 "2025-11-29 12:33:09.530499"
}

// parseLogLine 解析日志行，提取级别、消息和时间戳
func parseLogLine(line string) LogEntry {
	// 初始化默认值
	entry := LogEntry{
		Level:     "info",
		Message:   line,
		Timestamp: time.Now().Format("2006-01-02 15:04:05.000000"),
	}

	// 解析日志格式：2025/11/29 12:33:09.530499 [INFO] 开始处理同步任务: 类型=刮削整理, ID=2
	// 正则表达式匹配日志格式
	pattern := `^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d{6}) \[(\w+)\] (.+)$`
	regex := regexp.MustCompile(pattern)
	matches := regex.FindStringSubmatch(line)

	if len(matches) == 4 {
		entry.Timestamp = matches[1]
		// 解析日志级别
		level := strings.ToLower(matches[2])
		switch level {
		case "info":
			entry.Level = "info"
		case "warn", "warning":
			entry.Level = "warn"
		case "error", "err":
			entry.Level = "error"
		case "debug":
			entry.Level = "debug"
		default:
			entry.Level = "info"
		}
		// 解析日志消息
		entry.Message = matches[3]
	}

	return entry
}

type OldLogsRequest struct {
	Path      string `json:"path" form:"path"`
	Pos       int64  `json:"pos" form:"pos"`
	Limit     int    `json:"limit" form:"limit"`
	Direction string `json:"direction" form:"direction"` // 可选值：forward（默认）或 backward
}

type OldLogsResponse struct {
	Entries  []LogEntry `json:"entries"`
	Pos      int64      `json:"pos"`
	StartPos int64      `json:"start_pos"`
}

// GetOldLogs 通过HTTP接口获取旧日志，返回JSON格式
func GetOldLogs(c *gin.Context) {
	var req *OldLogsRequest
	err := c.ShouldBind(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}
	pos := req.Pos
	limit := req.Limit
	direction := "forward"
	logPath := req.Path
	if req.Direction == "backward" {
		// 从后往前读取
		direction = "backward"
	}

	if pos == 0 && direction == "forward" {
		// 已经到了文件开头
		// 返回JSON结果
		c.JSON(http.StatusOK, OldLogsResponse{
			Entries:  make([]LogEntry, 0),
			Pos:      0,
			StartPos: 0,
		})
		return
	}

	// 拼接完整日志文件路径
	fullLogPath := filepath.Join(helpers.ConfigDir, "logs", logPath)

	// 检查文件是否存在
	if _, serr := os.Stat(fullLogPath); os.IsNotExist(serr) {
		c.JSON(http.StatusNotFound, gin.H{"error": "日志文件不存在"})
		return
	}
	resultLines, newPos, err := helpers.ReadLines(fullLogPath, int64(pos), limit, direction)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("读取日志文件失败: %v", err)})
		return
	}
	// 解析日志行，转换为LogEntry结构
	entries := make([]LogEntry, 0, len(resultLines))
	for _, line := range resultLines {
		entries = append(entries, parseLogLine(line))
	}

	// 返回JSON结果
	c.JSON(http.StatusOK, OldLogsResponse{
		Entries:  entries,
		Pos:      newPos,
		StartPos: int64(pos),
	})
}

// DownloadLogFile 下载日志文件
func DownloadLogFile(c *gin.Context) {
	// 获取日志文件路径参数
	logPath := c.Query("path")
	if logPath == "" || strings.Contains(logPath, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未提供日志文件路径或路径包含不合法字符"})
		return
	}

	// 拼接完整日志文件路径
	fullLogPath, err := helpers.SafeJoin(filepath.Join(helpers.ConfigDir, "logs"), logPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("路径遍历攻击 detected: %v", err)})
		return
	}

	// 检查文件是否存在
	if _, err := os.Stat(fullLogPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "日志文件不存在"})
		return
	}

	// 设置响应头，支持文件下载
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(fullLogPath)))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Expires", "0")
	c.Header("Cache-Control", "must-revalidate")
	c.Header("Pragma", "public")

	// 发送文件
	c.File(fullLogPath)
}

// LogWebSocket 通过websocket查看日志
func LogWebSocket(c *gin.Context) {
	// 升级HTTP连接为WebSocket连接
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		helpers.AppLogger.Errorf("升级WebSocket连接失败: %v", err)
		return
	}
	defer conn.Close()

	// 获取日志文件路径参数
	logPath := c.Query("path")
	if logPath == "" {
		entry := LogEntry{
			Level:     "error",
			Message:   "错误: 未提供日志文件路径",
			Timestamp: time.Now().Format("2006-01-02 15:04:05.000000"),
		}
		if werr := conn.WriteJSON(entry); werr != nil {
			helpers.AppLogger.Errorf("发送错误消息失败: %v", werr)
		}
		return
	}

	// 拼接完整日志文件路径
	fullLogPath := filepath.Join(helpers.ConfigDir, "logs", logPath)

	// 检查文件是否存在
	if _, serr := os.Stat(fullLogPath); os.IsNotExist(serr) {
		entry := LogEntry{
			Level:     "error",
			Message:   fmt.Sprintf("错误: 日志文件不存在: %s", fullLogPath),
			Timestamp: time.Now().Format("2006-01-02 15:04:05.000000"),
		}
		if werr := conn.WriteJSON(entry); werr != nil {
			helpers.AppLogger.Errorf("发送错误消息失败: %v", werr)
		}
		return
	}

	// 打开日志文件
	file, err := os.Open(fullLogPath)
	if err != nil {
		entry := LogEntry{
			Level:     "error",
			Message:   fmt.Sprintf("错误: 打开日志文件失败: %v", err),
			Timestamp: time.Now().Format("2006-01-02 15:04:05.000000"),
		}
		if werr := conn.WriteJSON(entry); werr != nil {
			helpers.AppLogger.Errorf("发送错误消息失败: %v", werr)
		}
		return
	}
	defer file.Close()

	// 获取文件信息（仅用于检查文件是否存在，不需要文件大小）
	_, err = file.Stat()
	if err != nil {
		entry := LogEntry{
			Level:     "error",
			Message:   fmt.Sprintf("错误: 获取文件信息失败: %v", err),
			Timestamp: time.Now().Format("2006-01-02 15:04:05.000000"),
		}
		if werr := conn.WriteJSON(entry); werr != nil {
			helpers.AppLogger.Errorf("发送错误消息失败: %v", werr)
		}
		return
	}

	// 发送初始消息
	entry := LogEntry{
		Level:     "info",
		Message:   fmt.Sprintf("开始监控日志文件: %s", fullLogPath),
		Timestamp: time.Now().Format("2006-01-02 15:04:05.000000"),
	}
	if werr := conn.WriteJSON(entry); werr != nil {
		helpers.AppLogger.Errorf("发送初始消息失败: %v", werr)
		return
	}

	// 创建文件监听器
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		entry := LogEntry{
			Level:     "error",
			Message:   fmt.Sprintf("错误: 创建文件监听器失败: %v", err),
			Timestamp: time.Now().Format("2006-01-02 15:04:05.000000"),
		}
		if werr := conn.WriteJSON(entry); werr != nil {
			helpers.AppLogger.Errorf("发送错误消息失败: %v", werr)
		}
		return
	}
	defer watcher.Close()

	// 添加文件到监听器
	if err := watcher.Add(fullLogPath); err != nil {
		entry := LogEntry{
			Level:     "error",
			Message:   fmt.Sprintf("错误: 添加文件到监听器失败: %v", err),
			Timestamp: time.Now().Format("2006-01-02 15:04:05.000000"),
		}
		if werr := conn.WriteJSON(entry); werr != nil {
			helpers.AppLogger.Errorf("发送错误消息失败: %v", werr)
		}
		return
	}

	// 关闭信号通道
	closed := make(chan struct{})
	defer close(closed)

	// 启动协程处理客户端消息
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				// 客户端断开连接，这是正常情况，不需要记录错误
				return
			}
			// 不再处理客户端消息，因为旧日志已经通过HTTP接口获取
		}
	}()

	// 移动到文件末尾，只获取新内容
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		if werr := conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("错误: 定位文件末尾失败: %v", err))); werr != nil {
			helpers.AppLogger.Errorf("发送错误消息失败: %v", werr)
		}
		return
	}

	// 启动协程读取新内容
	go func() {
		buffer := make([]byte, 4096)
		var leftover []byte // 保存上次读取的不完整行
		for {
			select {
			case <-closed:
				// 连接已关闭，退出协程
				return
			default:
				// 读取文件新内容
				n, err := file.Read(buffer)
				if err != nil {
					if err != io.EOF {
						// 简单处理，直接转换为字符串
						errMsg := fmt.Sprintf("错误: 读取日志文件失败: %v", err)
						// 发送错误消息
						entry := LogEntry{
							Level:     "error",
							Message:   errMsg,
							Timestamp: time.Now().Format("2006-01-02 15:04:05.000000"),
						}
						if err := conn.WriteJSON(entry); err != nil {
							// 连接已关闭，退出协程
							return
						}
						return
					}
					// 没有新内容，等待文件变化
					time.Sleep(100 * time.Millisecond)
					continue
				}

				if n > 0 {
					// 合并上次的剩余内容和本次读取的内容
					content := append(leftover, buffer[:n]...)

					// 按换行符分割
					lines := strings.Split(string(content), "\n")

					// 保存最后一行可能不完整的内容
					if len(lines) > 0 {
						leftover = []byte(lines[len(lines)-1])
						// 移除最后一行（可能不完整）
						lines = lines[:len(lines)-1]
					}

					// 只处理有内容的行
					var validLines []string
					for _, line := range lines {
						if line != "" {
							validLines = append(validLines, line)
						}
					}

					// 如果有有效行，反转顺序后发送
					if len(validLines) > 0 {
						// 反转行顺序，使最新的行在最前面
						for i, j := 0, len(validLines)-1; i < j; i, j = i+1, j-1 {
							validLines[i], validLines[j] = validLines[j], validLines[i]
						}

						// 解析日志行，转换为LogEntry结构
						for _, line := range validLines {
							entry := parseLogLine(line)
							// 发送日志内容（JSON格式）
							if err := conn.WriteJSON(entry); err != nil {
								// 连接已关闭，退出协程
								return
							}
						}
					}
				}
			}
		}
	}()

	// 处理文件变化事件
	for {
		select {
		case <-closed:
			// 连接已关闭，退出主循环
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			// 只处理写入事件
			if event.Op&fsnotify.Write == fsnotify.Write {
				// 唤醒读取协程
				continue
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			entry := LogEntry{
				Level:     "error",
				Message:   fmt.Sprintf("错误: 文件监控失败: %v", err),
				Timestamp: time.Now().Format("2006-01-02 15:04:05.000000"),
			}
			if werr := conn.WriteJSON(entry); werr != nil {
				helpers.AppLogger.Errorf("发送错误消息失败: %v", werr)
				// 连接已关闭，退出主循环
				return
			}
			return
		case <-c.Request.Context().Done():
			// 客户端断开连接
			return
		}
	}
}
