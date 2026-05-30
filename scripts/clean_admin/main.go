package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/agentdisk/agent-disk/config"
	"github.com/agentdisk/agent-disk/internal/repository"
	"gorm.io/gorm/logger"
)

var tables = []string{
	"disk_share_access_log",
	"disk_share",
	"disk_tag_relation",
	"disk_tag",
	"disk_recycle_bin",
	"disk_file_version",
	"disk_permission",
	"disk_file",
	"disk_folder",
	"user_disk",
	"disk_api_key",
	"disk_public_directory",
	"disk_oauth2_config",
	"disk_admin_user",
	"disk_admin_passkey",
}

func main() {
	cfgPath := flag.String("config", "config.yaml", "config.yaml path")
	flag.Parse()

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

	fmt.Println("=== 清空管理控制台数据 ===")

	var truncated, skipped int
	for _, table := range tables {
		result := db.Exec(fmt.Sprintf("DELETE FROM %s", table))
		if result.Error != nil {
			fmt.Printf("  SKIP %-30s (%v)\n", table, result.Error)
			skipped++
		} else {
			fmt.Printf("  OK   %-30s\n", table)
			truncated++
		}
	}

	fmt.Printf("\n完成: %d 张表已清空, %d 张表跳过\n", truncated, skipped)
}
