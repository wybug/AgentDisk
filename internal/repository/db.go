package repository

import (
	"fmt"

	"github.com/agentdisk/agent-disk/config"
	"github.com/agentdisk/agent-disk/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// DB is the package-level shared database instance.
var DB *gorm.DB

// InitDB opens a GORM database connection from config.
func InitDB(cfg *config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch cfg.Database.Driver {
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.Database.User, cfg.Database.Password,
			cfg.Database.Host, cfg.Database.Port, cfg.Database.Name)
		dialector = mysql.Open(dsn)
	case "sqlite":
		dialector = sqlite.Open(cfg.Database.Name + ".db")
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Database.Driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{})
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
	)
}
