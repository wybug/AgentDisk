package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/agentdisk/agent-disk/config"
	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
	"github.com/agentdisk/agent-disk/internal/service"
	"gorm.io/gorm/logger"
)

func main() {
	cfgPath := flag.String("config", "config.yaml", "config.yaml path")
	username := flag.String("username", "", "Admin username (required)")
	password := flag.String("password", "", "Admin password (required)")
	role := flag.String("role", "admin", "Admin role (admin or super_admin)")
	displayName := flag.String("displayName", "", "Display name")
	flag.Parse()

	if *username == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "Usage: go run ./scripts/add_admin -username <name> -password <pass> [-role <role>] [-displayName <name>]")
		os.Exit(1)
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	db, err := repository.InitDB(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	db.Logger = logger.Default.LogMode(logger.Silent)

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
