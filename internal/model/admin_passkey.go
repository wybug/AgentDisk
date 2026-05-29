package model

import "time"

// DiskAdminPasskey stores a WebAuthn credential for admin MFA.
type DiskAdminPasskey struct {
	ID              uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	AdminUsername   string     `gorm:"size:64;not null;index:idx_admin_username" json:"adminUsername"`
	CredentialID    string     `gorm:"size:256;not null;uniqueIndex" json:"credentialId"`
	PublicKey       []byte     `gorm:"type:longblob;not null" json:"-"`
	AttestationType string     `gorm:"size:32;not null;default:''" json:"attestationType"`
	Transport       string     `gorm:"size:128;not null;default:''" json:"transport"`
	SignCount       uint64     `gorm:"not null;default:0" json:"signCount"`
	Name            string     `gorm:"size:128;not null;default:''" json:"name"`
	AAGUID          []byte     `gorm:"type:blob" json:"-"`
	BackupEligible  bool       `gorm:"default:false" json:"backupEligible"`
	BackupState     bool       `gorm:"default:false" json:"backupState"`
	IsActive        bool       `gorm:"default:true" json:"isActive"`
	LastUsedAt      *time.Time `json:"lastUsedAt"`
	CreatedAt       time.Time  `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt       time.Time  `gorm:"autoUpdateTime" json:"updatedAt"`
}

// TableName handles the TableName endpoint.
func (DiskAdminPasskey) TableName() string { return "disk_admin_passkey" }
