package model

import "time"

// UserDisk 用户云盘配额表
type UserDisk struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     string    `gorm:"uniqueIndex;size:64;not null" json:"userId"`
	TotalQuota int64     `gorm:"not null;default:10737418240" json:"totalQuota"` // 默认10GB
	UsedQuota  int64     `gorm:"not null;default:0" json:"usedQuota"`
	RootFolder string    `gorm:"size:128" json:"rootFolder"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}

// TableName handles the TableName endpoint.
func (UserDisk) TableName() string { return "user_disk" }
