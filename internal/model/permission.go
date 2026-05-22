package model

import "time"

// DiskPermission 智能体权限表
type DiskPermission struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       string    `gorm:"index;size:64;not null" json:"userId"`
	AgentID      string    `gorm:"index:idx_agent;size:64;not null;default:''" json:"agentId"`
	AgentGroupID string    `gorm:"index:idx_agent;size:64;not null;default:''" json:"agentGroupId"`
	ResourceID   uint64    `gorm:"index;not null;default:0" json:"resourceId"`
	ResType      string    `gorm:"size:16;not null;default:''" json:"resType"`
	ResourcePath string    `gorm:"size:1024;not null;default:''" json:"resourcePath"`
	Permission   string    `gorm:"size:16;not null" json:"permission"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}

// TableName handles the TableName endpoint.
func (DiskPermission) TableName() string { return "disk_permission" }
