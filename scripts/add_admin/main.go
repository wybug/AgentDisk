package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/agentdisk/agent-disk/internal/service"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	dbPath := flag.String("db", "agentdisk.db", "SQLite database path")
	username := flag.String("username", "", "Admin username (required)")
	password := flag.String("password", "", "Admin password (required)")
	role := flag.String("role", "admin", "Admin role (admin or super_admin)")
	displayName := flag.String("displayName", "", "Display name")
	flag.Parse()

	if *username == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "Usage: add_admin -db <path> -username <name> -password <pass> [-role <role>] [-displayName <name>]")
		os.Exit(1)
	}

	db, err := gorm.Open(sqlite.Open(*dbPath), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
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
