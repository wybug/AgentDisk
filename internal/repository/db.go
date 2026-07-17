package repository

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	driver := cfg.Database.Driver
	if driver == "" {
		driver = "mysql"
	}

	var db *gorm.DB
	var err error

	switch driver {
	case "sqlite":
		path := cfg.Database.Path
		if path == "" {
			homeDir, _ := os.UserHomeDir()
			path = filepath.Join(homeDir, ".agentdisk", "disk.db")
		}
		if mkErr := os.MkdirAll(filepath.Dir(path), 0o750); mkErr != nil {
			return nil, fmt.Errorf("create db directory: %w", mkErr)
		}
		// DSN params are applied per-connection by mattn/go-sqlite3, which
		// matters once SetMaxOpenConns > 1 — the previous PRAGMA approach
		// only configured whichever connection happened to run the exec.
		//   - _journal_mode=WAL: readers don't block the writer; concurrent
		//     search traffic can flow while WriteMarkdown persists.
		//   - _busy_timeout=5000: serialize waiters for 5s instead of
		//     surfacing "database is locked" the moment a second writer shows
		//     up. Required for pool > 1 to behave under mixed load.
		//   - _foreign_keys=on: enforce FK constraints app-wide.
		dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on", path)
		db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		if err != nil {
			return nil, fmt.Errorf("failed to open sqlite: %w", err)
		}
	default:
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.Database.User, cfg.Database.Password,
			cfg.Database.Host, cfg.Database.Port, cfg.Database.Name)
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err != nil {
			return nil, fmt.Errorf("failed to open database: %w", err)
		}
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	if driver == "sqlite" {
		// WAL + busy_timeout make a small pool safe — readers don't block the
		// writer and lock waiters park up to 5s before erroring. The previous
		// pool of 1 serialized every search behind whatever the writer was
		// doing, which made /okf/search unusable under any concurrent load.
		sqlDB.SetMaxOpenConns(10)
		sqlDB.SetMaxIdleConns(5)
	} else {
		sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
		sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	}

	DB = db
	return db, nil
}

// AutoMigrate runs GORM auto-migration for all project models.
func AutoMigrate(db *gorm.DB) error {
	if err := migratePermissionIndex(db); err != nil {
		return fmt.Errorf("permission index migration: %w", err)
	}
	if err := db.AutoMigrate(
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
		&model.DiskPublicDirectoryGrant{},
		&model.DiskOAuth2Config{},
		&model.DiskAdminPasskey{},
		// OKF v0.1 bundle + materialized node index (P1) + edge graph (P3a).
		&model.OkfBundle{},
		&model.OkfNode{},
		&model.OkfEdge{},
	); err != nil {
		return err
	}
	if err := migrateOkfFullTextIndex(db); err != nil {
		return fmt.Errorf("okf fulltext index migration: %w", err)
	}
	if err := migrateOkfFts5Index(db); err != nil {
		return fmt.Errorf("okf fts5 migration: %w", err)
	}
	return nil
}

// migrateOkfFullTextIndex creates the FULLTEXT index on disk_okf_node(title,
// description) with the ngram parser so CJK queries match by bigram. Only
// MySQL supports FULLTEXT indices; SQLite gets an FTS5 virtual table in
// migrateOkfFts5Index instead. The parser is set via a raw INDEX OPTIONS
// clause because GORM's tag-driven indexes don't surface it.
//
// Idempotent: if the index already exists (name match), the CREATE is
// skipped. The parser choice is mandatory on this index — without it MySQL
// uses the default whitespace tokenizer which gives terrible recall on
// Chinese / Japanese / Korean text.
func migrateOkfFullTextIndex(db *gorm.DB) error {
	if db.Name() != "mysql" {
		return nil
	}
	// Does the FULLTEXT index already cover body? information_schema.statistics
	// has one row per index column, so a body row means it was already migrated.
	var bodyCol int64
	if err := db.Raw(`SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'disk_okf_node' AND index_name = 'idx_okf_node_search' AND column_name = 'body'`).Scan(&bodyCol).Error; err != nil {
		return fmt.Errorf("check fulltext index columns: %w", err)
	}
	if bodyCol > 0 {
		return nil // already includes body
	}
	// Index missing, or pre-body (title, description only): (re)create over body.
	// DROP IF EXISTS covers the upgrade from the old two-column index.
	_ = db.Exec("DROP INDEX idx_okf_node_search ON disk_okf_node").Error
	return db.Exec("CREATE FULLTEXT INDEX idx_okf_node_search ON disk_okf_node (title, description, body) WITH PARSER ngram").Error
}

// migrateOkfFts5Index builds the SQLite FTS5 virtual table that mirrors
// disk_okf_node(title, description) plus triggers that keep it in sync. The
// tokenizer is unicode61 (FTS5's default Unicode-aware tokenizer) — it
// handles ASCII word boundaries and CJK characters by code point, which is
// a noticeable recall improvement over a LIKE substring scan.
//
// Why FTS5 over LIKE:
//   - MATCH is a real tokenized query: ranking, prefix, boolean operators.
//   - LIKE scans every row; FTS5 walks a reverse index. At 10k+ nodes the
//     gap is seconds vs. milliseconds.
//   - WAL mode (set in InitDB) lets FTS5 reads run concurrently with the
//     WriteMarkdown writer, which is the actual reason this exists — the
//     previous LIKE path serialized behind the single writer connection.
//
// Idempotent: skips creation when the virtual table already exists. The
// triggers use INSERT INTO ... VALUES('delete', ...) which is FTS5's
// documented "external content" sync pattern.
func migrateOkfFts5Index(db *gorm.DB) error {
	if db.Name() != "sqlite" {
		return nil
	}
	// Does the FTS5 table already have a body column? Read its CREATE sql from
	// sqlite_master; absence of 'body' means it predates body-indexing (or does
	// not exist yet) and needs (re)creation.
	var createSQL string
	if err := db.Raw("SELECT sql FROM sqlite_master WHERE type='table' AND name='disk_okf_node_fts'").Scan(&createSQL).Error; err != nil {
		return fmt.Errorf("check fts5 schema: %w", err)
	}
	if createSQL != "" && strings.Contains(createSQL, "body") {
		return nil // already migrated to include body
	}
	// Old FTS table lacks body (or is absent): drop triggers + table so the body
	// column is picked up, then recreate with body + repopulate from the base
	// table (body column was added by AutoMigrate before this runs).
	if createSQL != "" {
		for _, trig := range []string{"disk_okf_node_ai", "disk_okf_node_ad", "disk_okf_node_au"} {
			if err := db.Exec("DROP TRIGGER IF EXISTS " + trig).Error; err != nil {
				return fmt.Errorf("drop trigger %s: %w", trig, err)
			}
		}
		if err := db.Exec("DROP TABLE disk_okf_node_fts").Error; err != nil {
			return fmt.Errorf("drop old fts5 table: %w", err)
		}
	}
	stmts := []string{
		`CREATE VIRTUAL TABLE disk_okf_node_fts USING fts5(title, description, body, content='disk_okf_node', content_rowid='id', tokenize='unicode61')`,
		`INSERT INTO disk_okf_node_fts(rowid, title, description, body) SELECT id, title, description, body FROM disk_okf_node`,
		`CREATE TRIGGER disk_okf_node_ai AFTER INSERT ON disk_okf_node BEGIN
			INSERT INTO disk_okf_node_fts(rowid, title, description, body) VALUES (new.id, new.title, new.description, new.body);
		END`,
		`CREATE TRIGGER disk_okf_node_ad AFTER DELETE ON disk_okf_node BEGIN
			INSERT INTO disk_okf_node_fts(disk_okf_node_fts, rowid, title, description, body) VALUES ('delete', old.id, old.title, old.description, old.body);
		END`,
		`CREATE TRIGGER disk_okf_node_au AFTER UPDATE ON disk_okf_node BEGIN
			INSERT INTO disk_okf_node_fts(disk_okf_node_fts, rowid, title, description, body) VALUES ('delete', old.id, old.title, old.description, old.body);
			INSERT INTO disk_okf_node_fts(rowid, title, description, body) VALUES (new.id, new.title, new.description, new.body);
		END`,
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			return fmt.Errorf("apply fts5 statement: %w (stmt=%s)", err, s)
		}
	}
	return nil
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
