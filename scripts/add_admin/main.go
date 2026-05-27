package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/service"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	dsn := flag.String("dsn", "root:@tcp(127.0.0.1:3306)/agentdisk?charset=utf8mb4&parseTime=True&loc=Local", "MySQL DSN")
	username := flag.String("username", "", "Admin username (required)")
	password := flag.String("password", "", "Admin password (required)")
	role := flag.String("role", "admin", "Admin role (admin or super_admin)")
	displayName := flag.String("displayName", "", "Display name")
	flag.Parse()

	if *username == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "Usage: add_admin -dsn <mysql-dsn> -username <name> -password <pass> [-role <role>] [-displayName <name>]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Example:")
		fmt.Fprintln(os.Stderr, "  add_admin -dsn 'root:@tcp(172.20.6.37:3306)/agentdisk?parseTime=true' -username admin -password admin123")
		os.Exit(1)
	}

	db, err := gorm.Open(mysql.Open(*dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}

	// Auto-migrate admin table
	if err = db.AutoMigrate(&model.DiskAdminUser{}); err != nil {
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
