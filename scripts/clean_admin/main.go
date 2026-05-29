package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/agentdisk/agent-disk/config"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
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

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Database.User, cfg.Database.Password,
		cfg.Database.Host, cfg.Database.Port, cfg.Database.Name)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== 清空管理控制台数据 ===")
	db.Exec("SET FOREIGN_KEY_CHECKS = 0")

	var truncated, skipped int
	for _, table := range tables {
		result := db.Exec(fmt.Sprintf("TRUNCATE TABLE %s", table))
		if result.Error != nil {
			fmt.Printf("  SKIP %-30s (%v)\n", table, result.Error)
			skipped++
		} else {
			fmt.Printf("  OK   %-30s\n", table)
			truncated++
		}
	}

	db.Exec("SET FOREIGN_KEY_CHECKS = 1")
	fmt.Printf("\n完成: %d 张表已清空, %d 张表跳过\n", truncated, skipped)
}
