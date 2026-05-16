package model

import "time"

// DiskFile 文件元数据表
type DiskFile struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      string    `gorm:"index;size:64;not null" json:"userId"`
	FolderID    uint64    `gorm:"index;not null" json:"folderId"`
	FileName    string    `gorm:"size:255;not null" json:"fileName"`
	FileSize    int64     `gorm:"not null;default:0" json:"fileSize"`
	FileType    string    `gorm:"size:32" json:"fileType"`
	OSSKey      string    `gorm:"size:1024;not null" json:"ossKey"`
	MD5         string    `gorm:"size:32" json:"md5"`
	Version     int       `gorm:"not null;default:1" json:"version"`
	IsDeleted   bool      `gorm:"default:false;index" json:"isDeleted"`
	SourceAgent string    `gorm:"size:64" json:"sourceAgent"`
	IsArtifact  bool      `gorm:"default:false" json:"isArtifact"`
	Tags        string    `gorm:"size:1024" json:"tags"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}

// TableName handles the TableName endpoint.
func (DiskFile) TableName() string { return "disk_file" }
