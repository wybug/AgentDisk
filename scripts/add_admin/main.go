package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/service"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	driver := flag.String("driver", "sqlite", "Database driver: mysql or sqlite")
	dsn := flag.String("dsn", "agentdisk.db", "Database DSN (SQLite path or MySQL DSN)")
	username := flag.String("username", "", "Admin username (required)")
	password := flag.String("password", "", "Admin password (required)")
	role := flag.String("role", "admin", "Admin role (admin or super_admin)")
	displayName := flag.String("displayName", "", "Display name")
	flag.Parse()

	if *username == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "Usage: add_admin -driver <mysql|sqlite> -dsn <dsn> -username <name> -password <pass> [-role <role>] [-displayName <name>]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  SQLite:   add_admin -driver sqlite -dsn agentdisk.db -username admin -password admin123")
		fmt.Fprintln(os.Stderr, "  MySQL:    add_admin -driver mysql -dsn 'root:@tcp(172.20.6.37:3306)/agentdisk?parseTime=true' -username admin -password admin123")
		os.Exit(1)
	}

	var dialector gorm.Dialector
	switch *driver {
	case "mysql":
		dialector = mysql.Open(*dsn)
	default:
		dialector = sqlite.Open(*dsn)
	}

	db, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}

	// Auto-migrate admin table
	if err := db.AutoMigrate(&model.DiskAdminUser{}); err != nil {
		fmt.Fprintf(os.Stderr, "Error migrating: %v\n", err)
		os.Exit(1)
	}

	hash, err := service.HashPassword(*password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error hashing password: %v\n", err)
		os.Exit(1)
	}

	result := db.Exec(
		"INSERT INTO disk_admin_user (username, password_hash, role, display_name, is_active, created_by) VALUES (?, ?, ?, ?, 1, 'cli')",
		*username, hash, *role, *displayName,
	)
	if result.Error != nil {
		fmt.Fprintf(os.Stderr, "Error creating admin: %v\n", result.Error)
		os.Exit(1)
	}

	fmt.Printf("Admin user '%s' created successfully (role: %s)\n", *username, *role)
}
