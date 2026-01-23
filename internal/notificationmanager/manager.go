package notificationmanager

import (
	"context"
	"fmt"
	"sync"
	"time"

	"Q115-STRM/internal/helpers"
	"Q115-STRM/internal/notification"

	"gorm.io/gorm"
)

// ============ 增强的通知管理器 ============

// EnhancedNotificationManager 增强的通知管理器
type EnhancedNotificationManager struct {
	handlers    map[uint]*channelInfo // key: ChannelID, value: handler + config
	rules       map[string][]uint     // key: EventType, value: ChannelIDs
	mu          sync.RWMutex
	db          *gorm.DB
	getProxyURL func() string // 获取代理URL的回调函数
}

type channelInfo struct {
	handler ChannelHandler
	config  *notification.NotificationChannel
}

// GlobalEnhancedNotificationManager 全局增强的通知管理器
var GlobalEnhancedNotificationManager *EnhancedNotificationManager

// NewEnhancedNotificationManager 创建增强的通知管理器
// getProxyURL: 获取系统代理URL的回调函数，返回空字符串表示不使用代理
func NewEnhancedNotificationManager(db *gorm.DB, getProxyURL func() string) *EnhancedNotificationManager {
	return &EnhancedNotificationManager{
		handlers:    make(map[uint]*channelInfo),
		rules:       make(map[string][]uint),
		db:          db,
		getProxyURL: getProxyURL,
	}
}

// LoadChannels 从数据库加载启用的渠道
func (m *EnhancedNotificationManager) LoadChannels() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.handlers = make(map[uint]*channelInfo)
	m.rules = make(map[string][]uint)

	// 加载所有启用的通知渠道
	var channels []notification.NotificationChannel
	if err := m.db.Where("is_enabled = ?", true).Find(&channels).Error; err != nil {
		helpers.AppLogger.Errorf("加载通知渠道失败: %v", err)
		return err
	}

	// 为每个渠道创建处理器
	for _, channel := range channels {
		handler, err := m.createChannelHandler(&channel)
		if err != nil {
			helpers.AppLogger.Warnf("创建渠道处理器失败 [%s]: %v", channel.ChannelType, err)
			continue
		}

		m.handlers[channel.ID] = &channelInfo{
			handler: handler,
			config:  &channel,
		}
	}

	// 加载通知规则
	var rules []notification.NotificationRule
	if err := m.db.Where("is_enabled = ?", true).Find(&rules).Error; err != nil {
		helpers.AppLogger.Warnf("加载通知规则失败: %v", err)
	} else {
		for _, rule := range rules {
			m.rules[rule.EventType] = append(m.rules[rule.EventType], rule.ChannelID)
		}
	}

	helpers.AppLogger.Infof("已加载 %d 个通知渠道", len(m.handlers))
	return nil
}

func (m *EnhancedNotificationManager) createChannelHandler(channel *notification.NotificationChannel) (ChannelHandler, error) {
	switch channel.ChannelType {
	case "telegram":
		var config notification.TelegramChannelConfig
		if err := m.db.Where("channel_id = ?", channel.ID).First(&config).Error; err != nil {
			return nil, fmt.Errorf("Telegram配置不存在: %v", err)
		}
		// 通过回调函数获取代理URL
		var proxyURL string
		if m.getProxyURL != nil {
			proxyURL = m.getProxyURL()
		}
		helpers.AppLogger.Infof("为Telegram渠道使用代理: %s", proxyURL)
		if proxyURL != "" {
			return NewTelegramChannelHandlerWithProxy(&config, proxyURL), nil
		}
		return NewTelegramChannelHandler(&config), nil

	case "meow":
		var config notification.MeoWChannelConfig
		if err := m.db.Where("channel_id = ?", channel.ID).First(&config).Error; err != nil {
			return nil, fmt.Errorf("MeoW配置不存在: %v", err)
		}
		return NewMeoWChannelHandler(&config), nil

	case "bark":
		var config notification.BarkChannelConfig
		if err := m.db.Where("channel_id = ?", channel.ID).First(&config).Error; err != nil {
			return nil, fmt.Errorf("Bark配置不存在: %v", err)
		}
		return NewBarkChannelHandler(&config), nil

	case "serverchan":
		var config notification.ServerChanChannelConfig
		if err := m.db.Where("channel_id = ?", channel.ID).First(&config).Error; err != nil {
			return nil, fmt.Errorf("Server酱配置不存在: %v", err)
		}
		return NewServerChanChannelHandler(&config), nil

	case "webhook":
		var config notification.CustomWebhookChannelConfig
		if err := m.db.Where("channel_id = ?", channel.ID).First(&config).Error; err != nil {
			return nil, fmt.Errorf("Webhook配置不存在: %v", err)
		}
		return NewCustomWebhookChannelHandler(&config), nil

	default:
		return nil, fmt.Errorf("未知的渠道类型: %s", channel.ChannelType)
	}
}

// SendNotification 发送通知到所有相关渠道
func (m *EnhancedNotificationManager) SendNotification(ctx context.Context, notification *notification.Notification) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 获取此事件类型启用的渠道
	channelIDs, exists := m.rules[string(notification.Type)]
	if !exists {
		helpers.AppLogger.Warnf("未找到事件类型 %s 的通知规则", notification.Type)
		return nil
	}

	var errs []error
	for _, channelID := range channelIDs {
		info, ok := m.handlers[channelID]
		if !ok {
			continue
		}

		// 为每个通知发送创建子context，超时15秒
		sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()

		if err := info.handler.Send(sendCtx, notification); err != nil {
			helpers.AppLogger.Errorf("渠道 [%s] 发送失败: %v", info.config.ChannelType, err)
			errs = append(errs, err)
		} else {
			helpers.AppLogger.Debugf("渠道 [%s] 发送成功", info.config.ChannelType)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("部分渠道发送失败: %v", errs)
	}
	return nil
}

// ReloadChannel 重新加载单个渠道
func (m *EnhancedNotificationManager) ReloadChannel(channelID uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var channel notification.NotificationChannel
	if err := m.db.Where("id = ?", channelID).First(&channel).Error; err != nil {
		return fmt.Errorf("渠道不存在: %v", err)
	}

	if !channel.IsEnabled {
		delete(m.handlers, channelID)
		return nil
	}

	handler, err := m.createChannelHandler(&channel)
	if err != nil {
		return err
	}

	m.handlers[channelID] = &channelInfo{
		handler: handler,
		config:  &channel,
	}

	helpers.AppLogger.Infof("已重新加载渠道: %s", channel.ChannelName)
	return nil
}

// GetChannels 获取所有渠道
func (m *EnhancedNotificationManager) GetChannels() []notification.NotificationChannel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var channels []notification.NotificationChannel
	for _, info := range m.handlers {
		channels = append(channels, *info.config)
	}
	return channels
}
