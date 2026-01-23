package notification

import "time"

// NotificationChannel 通知渠道基础配置
type NotificationChannel struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	ChannelType string `json:"channel_type" gorm:"index,uniqueIndex:idx_channel_type"`
	ChannelName string `json:"channel_name"`
	Description string `json:"description"`
	IsEnabled   bool   `json:"is_enabled" gorm:"default:true"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TelegramChannelConfig Telegram渠道配置
type TelegramChannelConfig struct {
	ID        uint   `json:"id" gorm:"primaryKey"`
	ChannelID uint   `json:"channel_id" gorm:"uniqueIndex:idx_telegram_channel"`
	BotToken  string `json:"bot_token"`
	ChatID    string `json:"chat_id"`
	ProxyURL  string `json:"proxy_url"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// MeoWChannelConfig MeoW渠道配置
type MeoWChannelConfig struct {
	ID        uint   `json:"id" gorm:"primaryKey"`
	ChannelID uint   `json:"channel_id" gorm:"uniqueIndex:idx_meow_channel"`
	Nickname  string `json:"nickname"`
	Endpoint  string `json:"endpoint" gorm:"default:http://api.chuckfang.com"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// BarkChannelConfig Bark渠道配置
type BarkChannelConfig struct {
	ID        uint   `json:"id" gorm:"primaryKey"`
	ChannelID uint   `json:"channel_id" gorm:"uniqueIndex:idx_bark_channel"`
	DeviceKey string `json:"device_key"`
	ServerURL string `json:"server_url" gorm:"default:https://api.day.app"`
	Sound     string `json:"sound" gorm:"default:alert"`
	Icon      string `json:"icon"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ServerChanChannelConfig Server酱渠道配置
type ServerChanChannelConfig struct {
	ID        uint   `json:"id" gorm:"primaryKey"`
	ChannelID uint   `json:"channel_id" gorm:"uniqueIndex:idx_serverchan_channel"`
	SCKEY     string `json:"sc_key"`
	Endpoint  string `json:"endpoint" gorm:"default:https://sc.ftqq.com"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NotificationRule 通知规则
type NotificationRule struct {
	ID        uint   `json:"id" gorm:"primaryKey"`
	ChannelID uint   `json:"channel_id" gorm:"index"`
	EventType string `json:"event_type" gorm:"index"`
	IsEnabled bool   `json:"is_enabled" gorm:"default:true"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NotificationType 通知类型枚举
type NotificationType string

const (
	SyncFinished   NotificationType = "sync_finish"
	SyncError      NotificationType = "sync_error"
	ScrapeFinished NotificationType = "scrape_finish"
	SystemAlert    NotificationType = "system_alert"
	MediaAdded     NotificationType = "media_added"
	MediaRemoved   NotificationType = "media_removed"
)

// NotificationPriority 通知优先级
type NotificationPriority string

const (
	HighPriority   NotificationPriority = "high"
	NormalPriority NotificationPriority = "normal"
	LowPriority    NotificationPriority = "low"
)

// Notification 统一通知对象
type Notification struct {
	ID        string                 `json:"id"`
	Type      NotificationType       `json:"type"`
	Title     string                 `json:"title"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata"`
	Timestamp time.Time              `json:"timestamp"`
	Priority  NotificationPriority   `json:"priority"`
	Image     string                 `json:"image"`
}

// CustomWebhookChannelConfig 自定义 Webhook 渠道配置
type CustomWebhookChannelConfig struct {
	ID        uint   `json:"id" gorm:"primaryKey"`
	ChannelID uint   `json:"channel_id" gorm:"uniqueIndex:idx_custom_webhook_channel"`
	Endpoint  string `json:"endpoint"`
	Method    string `json:"method"`   // GET | POST
	Template  string `json:"template"` // 模板字符串，支持 {{title}}, {{content}}, {{timestamp}}, {{image}}
	Format    string `json:"format"`   // json | form | text (POST时必填)
	// 鉴权与扩展
	AuthType      string `json:"auth_type"`                // none|bearer|basic|header|query
	AuthToken     string `json:"auth_token"`               // bearer/header/query 使用
	AuthUser      string `json:"auth_user"`                // basic 用户名
	AuthPass      string `json:"auth_pass"`                // basic 密码
	AuthHeaderKey string `json:"auth_header_key"`          // header 模式下的头名
	AuthQueryKey  string `json:"auth_query_key"`           // query 模式下的查询参数名
	Headers       string `json:"headers" gorm:"type:text"` // 额外头部(JSON对象字符串)
	QueryParam    string `json:"query_param"`              // GET 模式下用于承载模板的参数名，默认 q
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
