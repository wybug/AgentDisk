package model

import "time"

// DiskOAuth2Config OAuth2 认证配置表
type DiskOAuth2Config struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name         string    `gorm:"size:64;not null" json:"name"`
	Enabled      bool      `gorm:"default:true" json:"enabled"`
	ClientID     string    `gorm:"column:client_id;size:255;not null;default:''" json:"clientId"`
	ClientSecret string    `gorm:"column:client_secret;size:512;not null;default:''" json:"-"`
	AuthURL      string    `gorm:"size:512;not null;default:''" json:"authUrl"`
	TokenURL     string    `gorm:"size:512;not null;default:''" json:"tokenUrl"`
	UserInfoURL  string    `gorm:"size:512;not null;default:''" json:"userInfoUrl"`
	RedirectURL  string    `gorm:"size:512;not null;default:''" json:"redirectUrl"`
	FrontendURL  string    `gorm:"size:512;not null;default:''" json:"frontendUrl"`
	Scopes       string    `gorm:"size:512;not null;default:''" json:"scopes"`
	UpdatedBy    string    `gorm:"size:64;not null;default:''" json:"updatedBy"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}

func (DiskOAuth2Config) TableName() string { return "disk_oauth2_config" }
