package repository

import (
	"github.com/agentdisk/agent-disk/internal/model"
	"gorm.io/gorm"
)

// AdminRepo provides data access for admin user operations.
type AdminRepo struct {
	db *gorm.DB
}

// NewAdminRepo creates a new AdminRepo.
func NewAdminRepo(db *gorm.DB) *AdminRepo {
	return &AdminRepo{db: db}
}

// GetByUsername returns an admin user by username.
func (r *AdminRepo) GetByUsername(username string) (*model.DiskAdminUser, error) {
	var admin model.DiskAdminUser
	if err := r.db.Where("username = ? AND is_active = ?", username, true).First(&admin).Error; err != nil {
		return nil, err
	}
	return &admin, nil
}

// Create inserts a new admin user.
func (r *AdminRepo) Create(admin *model.DiskAdminUser) error {
	return r.db.Create(admin).Error
}

// Delete removes an admin user by username.
func (r *AdminRepo) Delete(username string) error {
	return r.db.Where("username = ?", username).Delete(&model.DiskAdminUser{}).Error
}

// ListAll returns all admin users.
func (r *AdminRepo) ListAll() ([]model.DiskAdminUser, error) {
	var admins []model.DiskAdminUser
	err := r.db.Order("id ASC").Find(&admins).Error
	return admins, err
}

// Count returns the number of admin users.
func (r *AdminRepo) Count() (int64, error) {
	var n int64
	err := r.db.Model(&model.DiskAdminUser{}).Count(&n).Error
	return n, err
}

// UpdatePassword updates the password hash for an admin user.
func (r *AdminRepo) UpdatePassword(username, passwordHash string) error {
	return r.db.Model(&model.DiskAdminUser{}).
		Where("username = ?", username).
		Update("password_hash", passwordHash).Error
}

// UpdateMFAEnabled updates the MFA enabled flag for an admin user.
func (r *AdminRepo) UpdateMFAEnabled(username string, enabled bool) error {
	return r.db.Model(&model.DiskAdminUser{}).
		Where("username = ?", username).
		Update("mfa_enabled", enabled).Error
}

// GetMFAEnabled returns the MFA enabled flag for an admin user.
func (r *AdminRepo) GetMFAEnabled(username string) (bool, error) {
	var admin model.DiskAdminUser
	if err := r.db.Select("mfa_enabled").Where("username = ?", username).First(&admin).Error; err != nil {
		return false, err
	}
	return admin.MfaEnabled, nil
}
