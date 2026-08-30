package handlers

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"qatest/config"
	"qatest/database"

	"golang.org/x/crypto/bcrypt"
)

const testAdminPassword = "test-admin-pass-123"

// TestMain 全包一次性初始化：测试配置 + 临时 SQLite + 迁移。
// handlers 包的 handler 直接读写全局 database.DB，测试用临时库隔离，
// 不触碰开发者本地的 qatest.db。
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "qatest-handlers-test-*")
	if err != nil {
		panic("创建临时目录失败: " + err.Error())
	}
	defer os.RemoveAll(dir)

	hash, err := bcrypt.GenerateFromPassword([]byte(testAdminPassword), bcrypt.MinCost)
	if err != nil {
		panic("生成测试口令哈希失败: " + err.Error())
	}

	config.AppConfig = &config.Config{
		DBPath:          filepath.Join(dir, "test.db"),
		JWTSecret:       "unit-test-jwt-secret-0123456789abcdef",
		JWTExpiresIn:    time.Hour,
		AllowedOrigins:  []string{"http://localhost:3000"},
		LogDir:          filepath.Join(dir, "logs"),
		ExecutorEnabled: false, // 默认关闭；需要验证执行路径的测试自行显式打开
		Users: []config.UserConfig{
			{Username: "admin", PasswordHash: string(hash), Name: "管理员", Role: "admin"},
			{Username: "tester", PasswordHash: string(hash), Name: "测试员", Role: "tester"},
		},
	}

	if err := database.Init(); err != nil {
		panic("初始化测试数据库失败: " + err.Error())
	}
	if err := database.RunMigrations(); err != nil {
		panic("执行测试迁移失败: " + err.Error())
	}

	code := m.Run()
	database.Close()
	os.Exit(code)
}
