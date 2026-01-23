package models

import (
	"Q115-STRM/internal/notification"
)

// NotificationChannel 通知渠道基础配置 - 别名供models包使用
type NotificationChannel = notification.NotificationChannel

// TelegramChannelConfig Telegram渠道配置 - 别名供models包使用
type TelegramChannelConfig = notification.TelegramChannelConfig

// MeoWChannelConfig MeoW渠道配置 - 别名供models包使用
type MeoWChannelConfig = notification.MeoWChannelConfig

// BarkChannelConfig Bark渠道配置 - 别名供models包使用
type BarkChannelConfig = notification.BarkChannelConfig

// ServerChanChannelConfig Server酱渠道配置 - 别名供models包使用
type ServerChanChannelConfig = notification.ServerChanChannelConfig

// NotificationRule 通知规则 - 别名供models包使用
type NotificationRule = notification.NotificationRule

// NotificationType 通知类型枚举 - 从 internal/notification 导入
type NotificationType = notification.NotificationType

const (
	SyncFinished   NotificationType = notification.SyncFinished
	SyncError      NotificationType = notification.SyncError
	ScrapeFinished NotificationType = notification.ScrapeFinished
	SystemAlert    NotificationType = notification.SystemAlert
	MediaAdded     NotificationType = notification.MediaAdded
	MediaRemoved   NotificationType = notification.MediaRemoved
)

// NotificationPriority 通知优先级 - 从 internal/notification 导入
type NotificationPriority = notification.NotificationPriority

const (
	HighPriority   NotificationPriority = notification.HighPriority
	NormalPriority NotificationPriority = notification.NormalPriority
	LowPriority    NotificationPriority = notification.LowPriority
)

// Notification 统一通知对象 - 从 internal/notification 导入
type Notification = notification.Notification

// CustomWebhookChannelConfig 自定义Webhook渠道配置 - 别名供models包使用
type CustomWebhookChannelConfig = notification.CustomWebhookChannelConfig
