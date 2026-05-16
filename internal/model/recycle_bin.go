package model

import "time"

// DiskRecycleBin 回收站表
type DiskRecycleBin struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       string    `gorm:"index;size:64;not null" json:"userId"`
	ResourceID   uint64    `gorm:"not null" json:"resourceId"`
	ResType      string    `gorm:"size:16;not null" json:"resType"` // file / folder
	ResName      string    `gorm:"size:255;not null" json:"resName"`
	OriginalPath string    `gorm:"size:1024" json:"originalPath"`
	DeletedBy    string    `gorm:"size:64" json:"deletedBy"`
	ExpireAt     time.Time `gorm:"index" json:"expireAt"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"createdAt"`
}

// TableName handles the TableName endpoint.
func (DiskRecycleBin) TableName() string { return "disk_recycle_bin" }
