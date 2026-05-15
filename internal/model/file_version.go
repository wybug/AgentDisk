package model

import "time"

// DiskFileVersion 版本快照表
type DiskFileVersion struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	FileID     uint64    `gorm:"index;not null" json:"fileId"`
	UserID     string    `gorm:"index;size:64;not null" json:"userId"`
	Version    int       `gorm:"not null" json:"version"`
	OSSKey     string    `gorm:"size:1024;not null" json:"ossKey"`
	FileSize   int64     `gorm:"not null" json:"fileSize"`
	MD5        string    `gorm:"size:32" json:"md5"`
	SnapshotBy string    `gorm:"size:64" json:"snapshotBy"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"createdAt"`
}

func (DiskFileVersion) TableName() string { return "disk_file_version" }
