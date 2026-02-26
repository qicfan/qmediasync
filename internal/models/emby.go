package models

import (
	"Q115-STRM/internal/db"
	embyclientrestgo "Q115-STRM/internal/embyclient-rest-go"

	"gorm.io/gorm"
)

type EmbyItem struct {
	BaseModel
	Name   string `json:"name"`
	ItemId int64  `json:"item_id"`
}

// EmbyLibrary 媒体库基础表（LibraryId 改为 string 以兼容 Emby 返回的字符串 ID）
type EmbyLibrary struct {
	BaseModel
	Name       string `json:"name"`
	LibraryId  string `json:"library_id"`
	SyncPathId uint   `json:"sync_path_id"` // 媒体库对应的同步目录ID，如果时0则表示没有关联同步目录
}

func (*EmbyLibrary) TableName() string {
	return "emby_libraries"
}

// UpsertEmbyLibraries 更新或创建媒体库记录
func UpsertEmbyLibraries(libs []embyclientrestgo.EmbyLibrary) error {
	for _, lib := range libs {
		existing := &EmbyLibrary{}
		err := db.Db.Where("library_id = ?", lib.ID).First(existing).Error
		switch {
		case err == nil:
			if existing.Name != lib.Name {
				existing.Name = lib.Name
				if uerr := db.Db.Save(existing).Error; uerr != nil {
					return uerr
				}
			}
		case err == gorm.ErrRecordNotFound:
			rec := &EmbyLibrary{Name: lib.Name, LibraryId: lib.ID}
			if cerr := db.Db.Create(rec).Error; cerr != nil {
				return cerr
			}
		default:
			return err
		}
	}
	return nil
}
