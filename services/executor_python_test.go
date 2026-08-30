package services

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"qatest/config"
	"qatest/database"
)

// TestExecutorPythonEndToEnd host 沙箱模式的端到端回归：
// 回归点为存量 bug「Start 后接 CombinedOutput 导致 exec: already started，
// python/js 任务永远 failed」。修复后应能真实跑通脚本并回传输出。
// 依赖本机 python（不存在则跳过）；不依赖 adb / 网络。
func TestExecutorPythonEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("python"); err != nil {
		t.Skip("本机无 python，跳过")
	}

	dir := t.TempDir()
	old := config.AppConfig
	config.AppConfig = &config.Config{
		DBPath:          filepath.Join(dir, "t.db"),
		JWTSecret:       "unit-test-jwt-secret-0123456789abcdef",
		LogDir:          filepath.Join(dir, "logs"),
		ExecutorEnabled: true,
		ExecutorSandbox: "host",
	}
	t.Cleanup(func() {
		database.Close() // Windows 下须先关连接才能删除临时库文件
		config.AppConfig = old
	})
	if err := database.Init(); err != nil {
		t.Fatal(err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	if _, err := database.DB.Exec(
		`INSERT INTO executions (id, script_id, device_serial, task_name, status, logs, screenshots, duration, started_at, finished_at, created_at)
		 VALUES ('exec-e2e', '', '', 'e2e', 'pending', '[]', '[]', 0, '', '', '')`); err != nil {
		t.Fatal(err)
	}

	// 捕获全部广播日志（consumeLogs 每条日志都会经过广播函数）
	var mu sync.Mutex
	var logs []string
	SetLogBroadcastFunc(func(b []byte) {
		mu.Lock()
		logs = append(logs, string(b))
		mu.Unlock()
	})

	task := &ExecutionTask{
		ID: "exec-e2e", Language: "python", Code: "print('e2e-ok-12345')",
		LogChan: make(chan LogEntry, 100),
		Ctx:     context.Background(),
		Cancel:  func() {},
	}
	Executor.Start(task)

	var status string
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		_ = database.DB.QueryRow("SELECT status FROM executions WHERE id = ?", task.ID).Scan(&status)
		if status == StatusSuccess || status == StatusFailed || status == StatusCancelled {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond) // 等最后的广播落地
	mu.Lock()
	joined := strings.Join(logs, "\n")
	mu.Unlock()

	if status != StatusSuccess {
		t.Fatalf("want success, got %q; logs:\n%s", status, joined)
	}
	if !strings.Contains(joined, "e2e-ok-12345") {
		t.Fatalf("脚本输出未进入日志:\n%s", joined)
	}
}
