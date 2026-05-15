package model

import "time"

// DiskPermission 智能体权限表
type DiskPermission struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     string    `gorm:"index;size:64;not null" json:"userId"`
	AgentID    string    `gorm:"index;size:64;not null" json:"agentId"`
	ResourceID uint64    `gorm:"index;not null" json:"resourceId"`
	ResType    string    `gorm:"size:16;not null" json:"resType"` // file / folder
	Permission string    `gorm:"size:16;not null" json:"permission"` // owner / read / write / delete
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}

func (DiskPermission) TableName() string { return "disk_permission" }
