package model

import "time"

// DiskAPIKey API 密钥表
type DiskAPIKey struct {
	ID         uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	KeyName    string     `gorm:"size:128;not null" json:"keyName"`
	KeyHash    string     `gorm:"uniqueIndex;size:64;not null" json:"-"`
	KeyPrefix  string     `gorm:"size:8;not null" json:"keyPrefix"`
	Scope      string     `gorm:"size:16;not null;default:public_read" json:"scope"`
	Department string     `gorm:"size:64;not null;default:''" json:"department"`
	CreatedBy  string     `gorm:"size:64;not null;default:''" json:"createdBy"`
	IsRevoked  bool       `gorm:"default:false" json:"isRevoked"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
	ExpiresAt  *time.Time `json:"expiresAt"`
	CreatedAt  time.Time  `gorm:"autoCreateTime" json:"createdAt"`
}

func (DiskAPIKey) TableName() string { return "disk_api_key" }
