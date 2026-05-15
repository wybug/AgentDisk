package model

import "time"

// DiskTag 标签字典表
type DiskTag struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    string    `gorm:"index;size:64;not null" json:"userId"`
	TagName   string    `gorm:"size:64;not null" json:"tagName"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"createdAt"`
}

func (DiskTag) TableName() string { return "disk_tag" }

// DiskTagRelation 标签-文件关联表
type DiskTagRelation struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	TagID     uint64    `gorm:"index;not null" json:"tagId"`
	FileID    uint64    `gorm:"index;not null" json:"fileId"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"createdAt"`
}

func (DiskTagRelation) TableName() string { return "disk_tag_relation" }
