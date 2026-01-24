package models

import (
	"Q115-STRM/internal/db"
	"Q115-STRM/internal/helpers"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// ApiKey API密钥模型
type ApiKey struct {
	BaseModel
	UserID     uint   `gorm:"index;not null" json:"user_id"`           // 关联的用户ID
	Name       string `gorm:"not null" json:"name"`                    // API Key名称/描述
	KeyHash    string `gorm:"unique;not null;index" json:"-"`          // API Key的SHA256哈希值（不返回给前端）
	KeyPrefix  string `gorm:"not null" json:"key_prefix"`              // Key前缀（前8位明文，用于显示）
	LastUsedAt int64  `gorm:"default:0" json:"last_used_at"`           // 最后使用时间
	IsActive   bool   `gorm:"default:true" json:"is_active"`           // 是否启用
	User       *User  `gorm:"foreignKey:UserID" json:"user,omitempty"` // 关联的用户对象
}

// TableName 表名
func (ApiKey) TableName() string {
	return "api_keys"
}

// GenerateAPIKey 生成API Key
// 格式: qms_ + 24位随机字符 (a-zA-Z0-9)
func GenerateAPIKey() (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const randomLength = 24
	const prefix = "qms_"

	randomBytes := make([]byte, randomLength)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", fmt.Errorf("生成随机字节失败: %w", err)
	}

	// 将随机字节映射到字符集
	for i := 0; i < randomLength; i++ {
		randomBytes[i] = charset[int(randomBytes[i])%len(charset)]
	}

	return prefix + string(randomBytes), nil
}

// HashAPIKey 对API Key进行SHA256哈希
func HashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// CreateAPIKey 创建新的API Key
func CreateAPIKey(userID uint, name string) (*ApiKey, string, error) {
	// 生成原始密钥
	rawKey, err := GenerateAPIKey()
	if err != nil {
		helpers.AppLogger.Errorf("生成API Key失败: %v", err)
		return nil, "", err
	}

	// 哈希密钥
	keyHash := HashAPIKey(rawKey)

	// 提取前缀（前8位）
	keyPrefix := rawKey
	if len(rawKey) > 8 {
		keyPrefix = rawKey[:8]
	}

	apiKey := &ApiKey{
		UserID:    userID,
		Name:      name,
		KeyHash:   keyHash,
		KeyPrefix: keyPrefix,
		IsActive:  true,
	}

	if err := db.Db.Create(apiKey).Error; err != nil {
		helpers.AppLogger.Errorf("创建API Key失败: %v", err)
		return nil, "", err
	}

	helpers.AppLogger.Infof("用户 %d 创建了API Key: %s (ID: %d)", userID, name, apiKey.ID)

	// 返回完整的原始密钥（仅此一次）
	return apiKey, rawKey, nil
}

// ValidateAPIKey 验证API Key
func ValidateAPIKey(key string) (*ApiKey, error) {
	keyHash := HashAPIKey(key)

	var apiKey ApiKey
	result := db.Db.Where("key_hash = ? AND is_active = ?", keyHash, true).First(&apiKey)
	if result.Error != nil {
		return nil, result.Error
	}

	return &apiKey, nil
}

// UpdateLastUsedAt 更新最后使用时间
func (apiKey *ApiKey) UpdateLastUsedAt() error {
	apiKey.LastUsedAt = time.Now().Unix()
	if err := db.Db.Model(apiKey).Update("last_used_at", apiKey.LastUsedAt).Error; err != nil {
		helpers.AppLogger.Errorf("更新API Key最后使用时间失败: %v", err)
		return err
	}
	return nil
}

// GetAPIKeysByUserID 获取用户的所有API Keys
func GetAPIKeysByUserID(userID uint) ([]ApiKey, error) {
	var apiKeys []ApiKey
	result := db.Db.Where("user_id = ?", userID).Order("created_at DESC").Find(&apiKeys)
	if result.Error != nil {
		helpers.AppLogger.Errorf("查询用户API Keys失败: %v", result.Error)
		return nil, result.Error
	}
	return apiKeys, nil
}

// GetAPIKeyByID 根据ID获取API Key
func GetAPIKeyByID(id uint) (*ApiKey, error) {
	var apiKey ApiKey
	result := db.Db.First(&apiKey, id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &apiKey, nil
}

// DeleteAPIKey 删除API Key
func DeleteAPIKey(id uint, userID uint) error {
	// 确保只能删除自己的API Key
	result := db.Db.Where("id = ? AND user_id = ?", id, userID).Delete(&ApiKey{})
	if result.Error != nil {
		helpers.AppLogger.Errorf("删除API Key失败: %v", result.Error)
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("API Key不存在或无权限删除")
	}
	helpers.AppLogger.Infof("用户 %d 删除了API Key ID: %d", userID, id)
	return nil
}

// UpdateAPIKeyStatus 更新API Key的启用/禁用状态
func UpdateAPIKeyStatus(id uint, userID uint, isActive bool) error {
	// 确保只能更新自己的API Key
	result := db.Db.Model(&ApiKey{}).Where("id = ? AND user_id = ?", id, userID).Update("is_active", isActive)
	if result.Error != nil {
		helpers.AppLogger.Errorf("更新API Key状态失败: %v", result.Error)
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("API Key不存在或无权限更新")
	}
	statusText := "禁用"
	if isActive {
		statusText = "启用"
	}
	helpers.AppLogger.Infof("用户 %d %s了API Key ID: %d", userID, statusText, id)
	return nil
}
