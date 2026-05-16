package model

import "time"

// DiskShare 外链分享表
type DiskShare struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      string    `gorm:"index;size:64;not null" json:"userId"`
	ResourceID  uint64    `gorm:"not null" json:"resourceId"`
	ResType     string    `gorm:"size:16;not null" json:"resType"` // file / folder
	ShareCode   string    `gorm:"uniqueIndex;size:32;not null" json:"shareCode"`
	ExtractCode string    `gorm:"size:8" json:"extractCode"`
	MaxVisit    int       `gorm:"default:-1" json:"maxVisit"` // -1 means unlimited
	VisitCount  int       `gorm:"default:0" json:"visitCount"`
	ExpireAt    time.Time `gorm:"index" json:"expireAt"`
	IsActive    bool      `gorm:"default:true;index" json:"isActive"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"createdAt"`
}

// TableName handles the TableName endpoint.
func (DiskShare) TableName() string { return "disk_share" }

// ShareAccessLog 分享访问日志
type ShareAccessLog struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ShareID   uint64    `gorm:"index;not null" json:"shareId"`
	VisitorIP string    `gorm:"size:45" json:"visitorIP"`
	UserAgent string    `gorm:"size:512" json:"userAgent"`
	Action    string    `gorm:"size:32" json:"action"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"createdAt"`
}

// TableName handles the TableName endpoint.
func (ShareAccessLog) TableName() string { return "disk_share_access_log" }
