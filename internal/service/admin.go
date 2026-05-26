package service

import (
	"errors"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

// adminRepo defines the interface for admin data access.
type adminRepo interface {
	GetByUsername(username string) (*model.DiskAdminUser, error)
	Create(admin *model.DiskAdminUser) error
	Delete(username string) error
	ListAll() ([]model.DiskAdminUser, error)
	UpdatePassword(username, passwordHash string) error
}

// AdminService handles admin operations.
type AdminService struct {
	repo adminRepo
}

// NewAdminService creates a new AdminService.
func NewAdminService(repo *repository.AdminRepo) *AdminService {
	return &AdminService{repo: repo}
}

// Login authenticates an admin user and returns the admin record.
func (s *AdminService) Login(username, password string) (*model.DiskAdminUser, error) {
	admin, err := s.repo.GetByUsername(username)
	if err != nil {
		return nil, errors.New("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("invalid credentials")
	}
	return admin, nil
}

// HashPassword generates a bcrypt hash from a plain password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CreateAdmin creates a new admin user.
func (s *AdminService) CreateAdmin(username, password, role, displayName, createdBy string) (*model.DiskAdminUser, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	admin := &model.DiskAdminUser{
		Username:     username,
		PasswordHash: hash,
		Role:         role,
		DisplayName:  displayName,
		IsActive:     true,
		CreatedBy:    createdBy,
	}
	if err := s.repo.Create(admin); err != nil {
		return nil, err
	}
	return admin, nil
}

// ListAdmins returns all admin users.
func (s *AdminService) ListAdmins() ([]model.DiskAdminUser, error) {
	return s.repo.ListAll()
}

// DeleteAdmin removes an admin user.
func (s *AdminService) DeleteAdmin(username string) error {
	return s.repo.Delete(username)
}

// ChangePassword updates the password for an admin user.
func (s *AdminService) ChangePassword(username, newPassword string) error {
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.repo.UpdatePassword(username, hash)
}
