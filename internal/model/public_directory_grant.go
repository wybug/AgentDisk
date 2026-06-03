package model

import "time"

// DiskPublicDirectoryGrant 公共目录用户授权表
type DiskPublicDirectoryGrant struct {
	ID                uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	PublicDirectoryID uint64    `gorm:"uniqueIndex:idx_pd_user;not null" json:"publicDirectoryId"`
	UserID            string    `gorm:"uniqueIndex:idx_pd_user;size:64;not null" json:"userId"`
	GrantedBy         string    `gorm:"size:64;not null;default:''" json:"grantedBy"`
	CreatedAt         time.Time `gorm:"autoCreateTime" json:"createdAt"`
}

// TableName handles the TableName endpoint.
func (DiskPublicDirectoryGrant) TableName() string { return "disk_public_directory_grant" }
