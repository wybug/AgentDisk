package repository

import (
	"fmt"

	"github.com/agentdisk/agent-disk/config"
	"github.com/agentdisk/agent-disk/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// DB is the package-level shared database instance.
var DB *gorm.DB

// InitDB opens a GORM database connection from config.
func InitDB(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Database.User, cfg.Database.Password,
		cfg.Database.Host, cfg.Database.Port, cfg.Database.Name)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)

	DB = db
	return db, nil
}

// AutoMigrate runs GORM auto-migration for all project models.
func AutoMigrate(db *gorm.DB) error {
	if err := migratePermissionIndex(db); err != nil {
		return fmt.Errorf("permission index migration: %w", err)
	}
	return db.AutoMigrate(
		&model.UserDisk{},
		&model.DiskFolder{},
		&model.DiskFile{},
		&model.DiskPermission{},
		&model.DiskFileVersion{},
		&model.DiskRecycleBin{},
		&model.DiskTag{},
		&model.DiskTagRelation{},
		&model.DiskShare{},
		&model.ShareAccessLog{},
		&model.DiskAdminUser{},
		&model.DiskAPIKey{},
		&model.DiskPublicDirectory{},
		&model.DiskOAuth2Config{},
		&model.DiskAdminPasskey{},
	)
}

// migratePermissionIndex drops the old uk_agent_resource unique index and creates
// the new composite unique index that includes agent_group_id and resource_path.
func migratePermissionIndex(db *gorm.DB) error {
	if db.Migrator().HasIndex(&model.DiskPermission{}, "uk_agent_resource") {
		if err := db.Migrator().DropIndex(&model.DiskPermission{}, "uk_agent_resource"); err != nil {
			return err
		}
	}
	return nil
}
