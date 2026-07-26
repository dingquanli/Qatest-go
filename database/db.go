package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"qatest/config"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

// Init 初始化数据库连接（使用配置中的 DBPath）
func Init() error {
	dbPath := config.AppConfig.DBPath
	if dbPath == "" {
		dbPath = "qatest.db" // 配置未设置时的默认值
	}
	// 确保目录存在
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建数据库目录失败: %w", err)
		}
	}

	var err error
	DB, err = sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}

	// 连接池配置
	// WAL 模式允许读写并发：读连接可多路并行，写连接仍串行
	// 使用 1 写 + 多读策略：MaxOpenConns 提升到 4，配合 _busy_timeout 处理写冲突
	DB.SetMaxOpenConns(4)
	DB.SetMaxIdleConns(2)
	DB.SetConnMaxLifetime(0) // 长连接复用

	if err := DB.Ping(); err != nil {
		return fmt.Errorf("数据库连接失败: %w", err)
	}

	if _, err := DB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("开启WAL模式失败: %w", err)
	}
	// WAL 检查点频率优化：NORMAL 模式在每次提交后自动检查点
	if _, err := DB.Exec("PRAGMA wal_autocheckpoint=1000"); err != nil {
		log.Printf("[数据库] 设置 wal_autocheckpoint 失败: %v", err)
	}
	if _, err := DB.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return fmt.Errorf("开启外键约束失败: %w", err)
	}

	log.Println("[数据库] SQLite 连接成功，WAL 模式已开启")
	return nil
}

// Close 关闭数据库连接
func Close() {
	if DB != nil {
		DB.Close()
		log.Println("[数据库] 连接已关闭")
	}
}
