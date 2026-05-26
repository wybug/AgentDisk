package model

import "time"

// DiskPublicDirectory 公共目录映射表
type DiskPublicDirectory struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	FolderID    uint64    `gorm:"uniqueIndex;not null" json:"folderId"`
	Scope       string    `gorm:"size:16;not null" json:"scope"`
	Department  string    `gorm:"size:64;not null;default:''" json:"department"`
	DisplayName string    `gorm:"size:255;not null" json:"displayName"`
	FixedPath   string    `gorm:"size:512;not null;default:''" json:"fixedPath"`
	CreatedBy   string    `gorm:"size:64;not null;default:''" json:"createdBy"`
	IsActive    bool      `gorm:"default:true" json:"isActive"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}

func (DiskPublicDirectory) TableName() string { return "disk_public_directory" }
