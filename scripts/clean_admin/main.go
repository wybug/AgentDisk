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
}

func main() {
	dsn := flag.String("dsn", "", "MySQL DSN (overrides config file)")
	cfgPath := flag.String("config", "", "config.yaml path (optional, uses DSN if empty)")
	flag.Parse()

	connStr, err := resolveDSN(*dsn, *cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Usage: clean_admin [-dsn <mysql-dsn>] [-config <config.yaml>]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Provide either -dsn or -config. If neither is given, defaults to:")
		fmt.Fprintln(os.Stderr, "  root:@tcp(127.0.0.1:3306)/agentdisk?charset=utf8mb4&parseTime=True&loc=Local")
		os.Exit(1)
	}

	db, err := gorm.Open(mysql.Open(connStr), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
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

func resolveDSN(dsn, cfgPath string) (string, error) {
	if dsn != "" {
		return dsn, nil
	}
	if cfgPath != "" {
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return "", fmt.Errorf("failed to load config: %w", err)
		}
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.Database.User, cfg.Database.Password,
			cfg.Database.Host, cfg.Database.Port, cfg.Database.Name), nil
	}
	return "root:@tcp(127.0.0.1:3306)/agentdisk?charset=utf8mb4&parseTime=True&loc=Local", nil
}
