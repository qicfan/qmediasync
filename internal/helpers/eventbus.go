package helpers

import (
	"fmt"
	"sync"
)

// 事件类型
type EventType string

const (
	// 保存115访问凭证的事件，当v115open刷新token后，通知数据库保存
	// Save115TokenEvent EventType = "save_115_token"
	// 115访问凭证无效事件，当v115open返回token无效时，通知数据库清空token
	V115TokenInValidEvent EventType = "115_token_invalid"
	// 保存OpenList访问凭证的事件，当openlist刷新token后，通知数据库保存
	SaveOpenListTokenEvent EventType = "save_open_list_token"
	// 备份任务定时事件，当定时任务触发时，通知备份任务
	BackupCronEevent EventType = "backup_cron_event"
)

// 事件数据
type Event struct {
	Type EventType `json:"type"`
	Data any       `json:"data"`
}

// 同步事件处理结果
type EventResult struct {
	Success bool  `json:"success"`
	Error   error `json:"error"`
	Data    any   `json:"data"`
}

// 事件处理函数类型
type EventHandler func(event Event)

// 同步事件处理函数类型
type SyncEventHandler func(event Event) EventResult

// 事件总线
type EventBus struct {
	handlers     map[EventType][]EventHandler
	syncHandlers map[EventType][]SyncEventHandler
	mutex        sync.RWMutex
}

var globalEventBus *EventBus

// 初始化事件总线
func InitEventBus() {
	globalEventBus = &EventBus{
		handlers:     make(map[EventType][]EventHandler),
		syncHandlers: make(map[EventType][]SyncEventHandler),
	}
	AppLogger.Info("事件总线已初始化")
}

// 订阅事件
func Subscribe(eventType EventType, handler EventHandler) {
	if globalEventBus == nil {
		AppLogger.Error("事件总线未初始化")
		return
	}

	globalEventBus.mutex.Lock()
	defer globalEventBus.mutex.Unlock()

	globalEventBus.handlers[eventType] = append(globalEventBus.handlers[eventType], handler)
	AppLogger.Infof("已订阅事件: %s", eventType)
}

// 发布事件
func Publish(eventType EventType, data interface{}) {
	if globalEventBus == nil {
		AppLogger.Error("事件总线未初始化")
		return
	}

	globalEventBus.mutex.RLock()
	handlers := globalEventBus.handlers[eventType]
	globalEventBus.mutex.RUnlock()

	if len(handlers) == 0 {
		AppLogger.Infof("没有订阅者监听事件: %s", eventType)
		return
	}

	event := Event{
		Type: eventType,
		Data: data,
	}

	AppLogger.Infof("发布事件: %s, 订阅者数量: %d", eventType, len(handlers))

	// 异步处理事件，避免阻塞
	go func() {
		for _, handler := range handlers {
			func() {
				defer func() {
					if r := recover(); r != nil {
						AppLogger.Errorf("事件处理器执行时发生panic: %v", r)
					}
				}()
				handler(event)
			}()
		}
	}()
}

// 订阅同步事件
func SubscribeSync(eventType EventType, handler SyncEventHandler) {
	if globalEventBus == nil {
		AppLogger.Error("事件总线未初始化")
		return
	}

	globalEventBus.mutex.Lock()
	defer globalEventBus.mutex.Unlock()

	globalEventBus.syncHandlers[eventType] = append(globalEventBus.syncHandlers[eventType], handler)
	AppLogger.Infof("已订阅同步事件: %s", eventType)
}

// 发布同步事件（等待所有处理器完成）
func PublishSync(eventType EventType, data any) []EventResult {
	if globalEventBus == nil {
		AppLogger.Error("事件总线未初始化")
		return nil
	}

	globalEventBus.mutex.RLock()
	handlers := globalEventBus.syncHandlers[eventType]
	globalEventBus.mutex.RUnlock()

	if len(handlers) == 0 {
		AppLogger.Infof("没有订阅者监听同步事件: %s", eventType)
		return nil
	}

	event := Event{
		Type: eventType,
		Data: data,
	}

	AppLogger.Infof("发布同步事件: %s, 订阅者数量: %d", eventType, len(handlers))

	results := make([]EventResult, len(handlers))
	var wg sync.WaitGroup

	// 并发处理所有处理器，但等待全部完成
	for i, handler := range handlers {
		wg.Add(1)
		go func(index int, h SyncEventHandler) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					AppLogger.Errorf("同步事件处理器执行时发生panic: %v", r)
					results[index] = EventResult{
						Success: false,
						Error:   fmt.Errorf("panic: %v", r),
					}
				}
			}()

			results[index] = h(event)
		}(i, handler)
	}

	wg.Wait() // 等待所有处理器完成
	AppLogger.Infof("同步事件处理完成: %s", eventType)

	return results
}

// 取消订阅（可选功能）
func Unsubscribe(eventType EventType, handler EventHandler) {
	if globalEventBus == nil {
		AppLogger.Error("事件总线未初始化")
		return
	}

	globalEventBus.mutex.Lock()
	defer globalEventBus.mutex.Unlock()

	handlers := globalEventBus.handlers[eventType]
	for i, h := range handlers {
		// 注意：这里比较函数指针可能不够准确，实际使用中可能需要用ID等方式标识
		if &h == &handler {
			// 移除处理器
			globalEventBus.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
			AppLogger.Infof("已取消订阅事件: %s", eventType)
			break
		}
	}
}

// 获取事件总线状态
func GetEventBusStatus() map[string]interface{} {
	if globalEventBus == nil {
		return map[string]interface{}{
			"initialized": false,
			"handlers":    0,
		}
	}

	globalEventBus.mutex.RLock()
	defer globalEventBus.mutex.RUnlock()

	handlerCount := 0
	eventTypes := make([]string, 0, len(globalEventBus.handlers))

	for eventType, handlers := range globalEventBus.handlers {
		eventTypes = append(eventTypes, string(eventType))
		handlerCount += len(handlers)
	}

	return map[string]interface{}{
		"initialized":   true,
		"event_types":   eventTypes,
		"handler_count": handlerCount,
		"total_events":  len(globalEventBus.handlers),
	}
}
