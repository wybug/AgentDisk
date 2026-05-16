package model

import "time"

// DiskFolder 目录结构表
type DiskFolder struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     string    `gorm:"index;size:64;not null" json:"userId"`
	ParentID   uint64    `gorm:"index;default:0" json:"parentId"`
	FolderName string    `gorm:"size:255;not null" json:"folderName"`
	FullPath   string    `gorm:"size:1024;not null" json:"fullPath"`
	SortOrder  int       `gorm:"default:0" json:"sortOrder"`
	IsDeleted  bool      `gorm:"default:false" json:"isDeleted"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}

// TableName handles the TableName endpoint.
func (DiskFolder) TableName() string { return "disk_folder" }
