package repository

import (
	"github.com/agentdisk/agent-disk/internal/model"
	"gorm.io/gorm"
)

// AdminPasskeyRepo provides data access for admin passkey credentials.
type AdminPasskeyRepo struct {
	db *gorm.DB
}

// NewAdminPasskeyRepo creates a new AdminPasskeyRepo.
func NewAdminPasskeyRepo(db *gorm.DB) *AdminPasskeyRepo {
	return &AdminPasskeyRepo{db: db}
}

// ListByAdmin returns all passkeys for an admin user.
func (r *AdminPasskeyRepo) ListByAdmin(username string) ([]model.DiskAdminPasskey, error) {
	var keys []model.DiskAdminPasskey
	err := r.db.Where("admin_username = ? AND is_active = ?", username, true).
		Order("id ASC").Find(&keys).Error
	return keys, err
}

// GetByCredentialID returns a passkey by its WebAuthn credential ID.
func (r *AdminPasskeyRepo) GetByCredentialID(credID string) (*model.DiskAdminPasskey, error) {
	var key model.DiskAdminPasskey
	if err := r.db.Where("credential_id = ? AND is_active = ?", credID, true).First(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

// GetByID returns a passkey by its primary key.
func (r *AdminPasskeyRepo) GetByID(id uint64) (*model.DiskAdminPasskey, error) {
	var key model.DiskAdminPasskey
	if err := r.db.First(&key, id).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

// Create inserts a new passkey credential.
func (r *AdminPasskeyRepo) Create(passkey *model.DiskAdminPasskey) error {
	return r.db.Create(passkey).Error
}

// UpdateSignCount updates the sign counter for replay protection.
func (r *AdminPasskeyRepo) UpdateSignCount(id uint64, signCount uint32) error {
	return r.db.Model(&model.DiskAdminPasskey{}).Where("id = ?", id).
		Update("sign_count", signCount).Error
}

// UpdateLastUsed sets the last_used_at timestamp.
func (r *AdminPasskeyRepo) UpdateLastUsed(id uint64) error {
	return r.db.Model(&model.DiskAdminPasskey{}).Where("id = ?", id).
		Update("last_used_at", gorm.Expr("NOW()")).Error
}

// Delete soft-deletes a passkey by setting is_active = false.
func (r *AdminPasskeyRepo) Delete(id uint64) error {
	return r.db.Model(&model.DiskAdminPasskey{}).Where("id = ?", id).
		Update("is_active", false).Error
}

// CountByAdmin returns the number of active passkeys for an admin.
func (r *AdminPasskeyRepo) CountByAdmin(username string) (int64, error) {
	var n int64
	err := r.db.Model(&model.DiskAdminPasskey{}).
		Where("admin_username = ? AND is_active = ?", username, true).Count(&n).Error
	return n, err
}

// UpdateName updates the display name of a passkey.
func (r *AdminPasskeyRepo) UpdateName(id uint64, name string) error {
	return r.db.Model(&model.DiskAdminPasskey{}).Where("id = ?", id).
		Update("name", name).Error
}
