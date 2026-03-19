package storage

import (
	"errors"
	"fmt"
)

// DriverType 网盘类型
type DriverType string

const (
	DriverTypeLocal     DriverType = "local"
	DriverType115       DriverType = "115"
	DriverTypeBaiduPan  DriverType = "baidupan"
	DriverTypeOpenList  DriverType = "openlist"
)

// DriverConfig 驱动配置
type DriverConfig struct {
	Type    DriverType
	Account interface{} // 具体的账号信息
	BaseURL string     // 基础URL（某些网盘需要）

	// 115网盘配置
	Client115 interface{} // *v115open.OpenClient

	// 百度网盘配置
	ClientBaidu interface{} // *baidupan.Client

	// OpenList配置
	ClientOpenList interface{} // *openlist.Client
}

// DriverFactory 驱动工厂
type DriverFactory struct {
	constructors map[DriverType]func(config DriverConfig) (CloudStorageDriver, error)
}

// NewDriverFactory 创建驱动工厂
func NewDriverFactory() *DriverFactory {
	factory := &DriverFactory{
		constructors: make(map[DriverType]func(config DriverConfig) (CloudStorageDriver, error)),
	}

	// 注册默认驱动
	factory.Register(DriverTypeLocal, NewLocalDriver)
	factory.Register(DriverType115, New115Driver)
	factory.Register(DriverTypeBaiduPan, NewBaiduPanDriver)
	factory.Register(DriverTypeOpenList, NewOpenListDriver)

	return factory
}

// Register 注册驱动
func (f *DriverFactory) Register(driverType DriverType, constructor func(config DriverConfig) (CloudStorageDriver, error)) {
	f.constructors[driverType] = constructor
}

// Create 创建驱动实例
func (f *DriverFactory) Create(config DriverConfig) (CloudStorageDriver, error) {
	constructor, ok := f.constructors[config.Type]
	if !ok {
		return nil, fmt.Errorf("不支持的驱动类型: %s", config.Type)
	}
	return constructor(config)
}

// GetSupportedTypes 获取支持的驱动类型列表
func (f *DriverFactory) GetSupportedTypes() []DriverType {
	types := make([]DriverType, 0, len(f.constructors))
	for t := range f.constructors {
		types = append(types, t)
	}
	return types
}

// IsSupported 检查是否支持指定的驱动类型
func (f *DriverFactory) IsSupported(driverType DriverType) bool {
	_, ok := f.constructors[driverType]
	return ok
}

// RegisterDriverFromConfig 从配置注册驱动
// 允许在运行时动态注册新的驱动
func (f *DriverFactory) RegisterDriverFromConfig(driverType DriverType, constructor func(config DriverConfig) (CloudStorageDriver, error)) error {
	if driverType == "" {
		return errors.New("驱动类型不能为空")
	}
	if constructor == nil {
		return errors.New("构造函数不能为空")
	}
	f.Register(driverType, constructor)
	return nil
}
