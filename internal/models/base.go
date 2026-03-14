package models

import (
	"Q115-STRM/internal/db"

	"gorm.io/gorm"
)

type BaseModel struct {
	ID        uint  `gorm:"primaryKey" json:"id"`
	CreatedAt int64 `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt int64 `gorm:"autoUpdateTime" json:"updated_at"`
}

func GetTableName(model interface{}) string {
	stmt := &gorm.Statement{DB: db.Db}

	// 解析模型
	if err := stmt.Parse(model); err != nil {
		return ""
	}

	return stmt.Schema.Table
}
