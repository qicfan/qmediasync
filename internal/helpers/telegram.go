package helpers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TelegramBot 结构体用于处理Telegram机器人操作
type TelegramBot struct {
	Token  string
	ChatID string
	Client *tgbotapi.BotAPI
}

// TelegramResponse Telegram API响应结构
type TelegramResponse struct {
	OK          bool        `json:"ok"`
	Result      interface{} `json:"result"`
	ErrorCode   int         `json:"error_code"`
	Description string      `json:"description"`
}

// TelegramMessage 发送消息的结构
type TelegramMessage struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

// maskToken 掩码token用于日志输出
func maskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "***" + token[len(token)-4:]
}

// NewTelegramBot 创建新的Telegram机器人实例
func NewTelegramBot(token, chatID string) *TelegramBot {
	if token == "" {
		AppLogger.Errorf("Telegram token为空")
		return nil
	}
	if chatID == "" {
		AppLogger.Errorf("Telegram ChatID为空")
		return nil
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		AppLogger.Errorf("创建Telegram机器人失败 (token: %s, chatID: %s): %v", maskToken(token), chatID, err)
		return nil
	}
	return &TelegramBot{
		Token:  token,
		ChatID: chatID,
		Client: bot,
	}
}

// NewTelegramBotWithProxy 创建带代理的Telegram机器人实例
func NewTelegramBotWithProxy(token, chatID, proxyURL string) (*TelegramBot, error) {
	// 增加超时时间以适配代理连接
	client := &http.Client{
		Timeout: 120 * time.Second, // 增加总超时时间
	}

	// 如果提供了代理URL，配置代理
	if proxyURL != "" {
		transport, err := createProxyTransport(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("创建代理传输失败: %v", err)
		}
		client.Transport = transport
	}
	bot, err := tgbotapi.NewBotAPIWithClient(token, "https://api.telegram.org/bot%s/%s", client)
	if err != nil {
		return nil, err
	}

	return &TelegramBot{
		Token:  token,
		ChatID: chatID,
		Client: bot,
	}, nil
}

// SendMessage 发送消息到Telegram
func (bot *TelegramBot) SendMessage(text string) error {
	if bot == nil {
		return fmt.Errorf("telegram bot 实例不能为空")
	}
	if bot.Client == nil {
		return fmt.Errorf("telegram bot client 不能为空")
	}
	if bot.Token == "" {
		return fmt.Errorf("telegram bot token不能为空")
	}
	if bot.ChatID == "" {
		return fmt.Errorf("telegram chat ID不能为空")
	}

	msg := tgbotapi.NewMessage(StringToInt64(bot.ChatID), text)
	msg.ParseMode = "HTML"
	_, err := bot.Client.Send(msg)
	if err != nil {
		return fmt.Errorf("发送消息失败: %v", err)
	}
	return nil

	// url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", bot.Token)

	// message := TelegramMessage{
	// 	ChatID:    bot.ChatID,
	// 	Text:      text,
	// 	ParseMode: "HTML",
	// }

	// jsonData, err := json.Marshal(message)
	// if err != nil {
	// 	return fmt.Errorf("序列化消息失败: %v", err)
	// }

	// req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	// if err != nil {
	// 	return fmt.Errorf("创建请求失败: %v", err)
	// }

	// req.Header.Set("Content-Type", "application/json")

	// resp, err := bot.Client.Do(req)
	// if err != nil {
	// 	return fmt.Errorf("发送请求失败: %v", err)
	// }
	// defer resp.Body.Close()

	// body, err := io.ReadAll(resp.Body)
	// if err != nil {
	// 	return fmt.Errorf("读取响应失败: %v", err)
	// }

	// var telegramResp TelegramResponse
	// if err := json.Unmarshal(body, &telegramResp); err != nil {
	// 	return fmt.Errorf("解析响应失败: %v", err)
	// }

	// if !telegramResp.OK {
	// 	return fmt.Errorf("telegram API错误 [%d]: %s", telegramResp.ErrorCode, telegramResp.Description)
	// }

	// return nil
}

// SendPhoto 发送图片到Telegram，支持本地文件路径或网络URL
func (bot *TelegramBot) SendPhoto(image string, caption string) error {
	if bot == nil {
		return fmt.Errorf("telegram bot 实例不能为空")
	}
	if bot.Client == nil {
		return fmt.Errorf("telegram bot client 不能为空")
	}
	if bot.Token == "" {
		return fmt.Errorf("telegram bot token不能为空")
	}
	if bot.ChatID == "" {
		return fmt.Errorf("telegram chat ID不能为空")
	}

	var file tgbotapi.RequestFileData
	// 判断是否为URL
	if strings.HasPrefix(strings.ToLower(image), "http://") || strings.HasPrefix(strings.ToLower(image), "https://") {
		file = tgbotapi.FileURL(image)
	} else {
		file = tgbotapi.FilePath(image)
	}

	msg := tgbotapi.NewPhoto(StringToInt64(bot.ChatID), file)
	if caption != "" {
		// Telegram 照片caption上限约为1024字符，这里做简单截断
		if len([]rune(caption)) > 1024 {
			// 保留前1024个字符
			runes := []rune(caption)
			caption = string(runes[:1024])
		}
		msg.Caption = caption
		msg.ParseMode = "HTML"
	}

	_, err := bot.Client.Send(msg)
	if err != nil {
		return fmt.Errorf("发送图片失败: %v", err)
	}
	return nil
}

// SendMessageWithRetry 带重试机制的发送消息
func (bot *TelegramBot) SendMessageWithRetry(text string, maxRetries int) error {
	var lastError error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// 重试前等待，使用指数退避
			waitTime := time.Duration(attempt*attempt) * time.Second
			AppLogger.Infof("Telegram消息发送失败，%d秒后重试 (第%d次尝试)", waitTime/time.Second, attempt)
			time.Sleep(waitTime)
		}

		err := bot.SendMessage(text)
		if err == nil {
			if attempt > 0 {
				AppLogger.Infof("Telegram消息重试发送成功 (第%d次尝试)", attempt)
			}
			return nil
		}

		lastError = err
		AppLogger.Warnf("Telegram消息发送失败 (第%d次尝试): %v", attempt+1, err)

		// 如果是超时错误，继续重试
		if isTimeoutError(err) {
			continue
		}

		// 如果是其他类型的错误，立即返回
		break
	}

	return fmt.Errorf("经过%d次重试后仍然失败: %v", maxRetries+1, lastError)
}

// isTimeoutError 检查是否是超时错误
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	timeoutKeywords := []string{
		"timeout",
		"tls handshake timeout",
		"context deadline exceeded",
		"connection timeout",
		"dial timeout",
	}

	for _, keyword := range timeoutKeywords {
		if strings.Contains(errStr, keyword) {
			return true
		}
	}

	return false
}

// TestConnection 测试Telegram机器人连接
func (bot *TelegramBot) TestConnection() error {
	if bot.Token == "" {
		return fmt.Errorf("telegram bot token不能为空")
	}

	// 如果提供了ChatID，测试发送消息
	if bot.ChatID == "" {
		return fmt.Errorf("telegram chat ID不能为空")
	}
	return bot.SendMessage("🤖 Telegram机器人连接测试成功！\n\n这是一条测试消息，表明您的机器人配置正确。")
}

// TestTelegramBot 测试Telegram机器人连接的便捷函数
func TestTelegramBot(token, chatID, httpProxy string) error {
	if httpProxy == "" {
		bot := NewTelegramBot(token, chatID)
		if bot == nil {
			return fmt.Errorf("创建Telegram机器人失败")
		}
		return bot.TestConnection()
	} else {
		bot, err := NewTelegramBotWithProxy(token, chatID, httpProxy)
		if err != nil {
			return fmt.Errorf("创建带代理的Telegram机器人失败: %v", err)
		}
		return bot.TestConnection()
	}
}
func (bot *TelegramBot) StartListening(ctx context.Context, handleCommand map[string]func([]string) string) {
	if bot.Client == nil {
		AppLogger.Errorf("Bot Client 未初始化")
		return
	}

	// 配置轮询参数
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	// 获取更新通道
	updates := bot.Client.GetUpdatesChan(u)

	AppLogger.Infof("Telegram Bot 控制监听已启动...")

	// 监听 Context 取消信号 ---
	go func() {
		<-ctx.Done()
		bot.Client.StopReceivingUpdates() // 这会导致 updates channel 被关闭，从而结束下面的 for 循环
	}()

	for update := range updates {
		var cmd string
		var args []string
		var chatID int64

		if update.Message != nil && update.Message.IsCommand() {
			// 处理文字命令 /xxxx
			cmd = update.Message.Command()
			args = strings.Fields(update.Message.CommandArguments())
			chatID = update.Message.Chat.ID
		} else if update.CallbackQuery != nil {
			// 处理按钮点击
			bot.Client.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, ""))
			cmd = update.CallbackQuery.Data
			args = []string{}
			chatID = update.CallbackQuery.Message.Chat.ID
		} else {
			continue
		}

		// --- 权限检查 ---
		// 重点：只响应你在配置中指定的 ChatID，防止其他人控制你的程序
		if bot.ChatID != "" && fmt.Sprintf("%d", chatID) != bot.ChatID {
			continue
		}

		// --- 处理命令 ---
		var responseText string
		if logic, ok := handleCommand[cmd]; ok {
			responseText = logic(args)
		} else {
			switch cmd {
			case "start", "help":
				responseText = `👋 <b>欢迎使用 QMediaSync Bot</b>  

					📋 <b>命令列表：</b>  
					📊/status - <b>查看系统运行状态</b>  
					🚀/strm_sync - <b>执行全量 STRM 同步</b>  
					🔄/strm_inc - <b>执行增量 STRM 同步</b>  
					🎬/scrape - <b>执行刮削任务</b>  
					🔄🎬/scrape_strm - <b>先刮削后同步</b>  
					🎬🔄/strm_scrape - <b>先同步后刮削</b>
					 
					  
					⚡ <b>同步模式说明：</b>  
					• <b>全量模式：</b> "全量同步"操作会删除所有缓存数据（不会删除本地文件），然后执行同步，可以处理所有网盘文件变更  
					• <b>增量模式：</b> "增量同步"仅会处理新增的文件，无法感知文件夹重命名等操作
					
					⚡ <b>同步/刮削命令：</b>  
					• 不加任何参数执行默认对所有同步/刮削路径执行
					• 可在命令后增加序号指定执行目录, 序号见同步/刮削目录设置。格式: /scrape #序号
					
					⚡ <b>任务序列命令：</b>  
					• 格式: /scrape_strm #刮削目录序号 #同步目录序号
					• 格式: /strm_scrape #同步目录序号 #刮削目录序号
					• 若参数为 #0，则对所有目录执行任务
					`
			case "status":
				responseText = "📊 <b>系统状态</b>\n运行中: OK\n时间: " + time.Now().Format("2006-01-02 15:04:05")
			default:
				responseText = "❓ 未知命令，输入 /help 查看帮助"
			}
		}

		// 回复结果
		if responseText != "" {
			reply := tgbotapi.NewMessage(chatID, responseText)
			reply.ParseMode = "HTML"
			if cmd == "start" || cmd == "help" {
				keyboard := tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("📊 系统状态", "status"),
						tgbotapi.NewInlineKeyboardButtonData("🚀 全量同步", "strm_sync"),
						tgbotapi.NewInlineKeyboardButtonData("🔄 增量同步", "strm_inc"),
					),
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("🎬 刮削任务", "scrape"),
						tgbotapi.NewInlineKeyboardButtonData("🎬🔄 刮削后同步", "scrape_strm"),
						tgbotapi.NewInlineKeyboardButtonData("🔄🎬 同步后刮削", "strm_scrape"),
					),
				)
				reply.ReplyMarkup = keyboard
			}
			bot.Client.Send(reply)
		}

	}
}

func (bot *TelegramBot) SetMenuContent() {
	menu := map[string]string{
		"status":      "📊 查看系统运行状态",
		"strm_sync":   "🚀 执行 STRM 全量同步",
		"strm_inc":    "🔄 执行 STRM 增量同步",
		"scrape":      "🎬 执行刮削任务",
		"scrape_strm": "🎬🔄 先刮削后同步",
		"strm_scrape": "🔄🎬 先同步后刮削",
		"help":        "📋 显示功能操作指南",
	}

	var tgCommands []tgbotapi.BotCommand
	for cmd, desc := range menu {
		tgCommands = append(tgCommands, tgbotapi.BotCommand{
			Command:     cmd,
			Description: desc,
		})
	}
	scope := tgbotapi.NewBotCommandScopeAllPrivateChats()
	cfg := tgbotapi.NewSetMyCommandsWithScope(scope, tgCommands...)
	_, err := bot.Client.Request(cfg)
	if err != nil {
		AppLogger.Errorf("设置Bot菜单失败: %v", err)
	}
}
