package model

import "time"

// DiskAdminUser 管理员账户表
type DiskAdminUser struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Username     string    `gorm:"uniqueIndex;size:64;not null" json:"username"`
	PasswordHash string    `gorm:"column:password_hash;size:128;not null" json:"-"`
	Role         string    `gorm:"size:16;not null;default:admin" json:"role"`
	DisplayName  string    `gorm:"size:128;not null;default:''" json:"displayName"`
	IsActive     bool      `gorm:"default:true" json:"isActive"`
	CreatedBy    string    `gorm:"size:64;not null;default:''" json:"createdBy"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}

func (DiskAdminUser) TableName() string { return "disk_admin_user" }
