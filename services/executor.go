package services

// 包级安全说明（P1-6 最小兜底 / 服务端代码执行设计风险）：
// 本包中的 executePython / executeJS 会先把用户提交的 .py / .js 代码写成临时文件，
// 再用 python / node 在【宿主机】上直接运行，具备任意代码执行（RCE）能力，属高危能力。
// 完整修复需要沙箱 / 容器隔离（本次不做）。此处仅做可见性兜底：
// 服务仅限对受信任用户开放，强烈建议在隔离环境 / 容器中运行，并配合资源、网络、权限限制。
import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"qatest/config"
	"qatest/database"
)

// init 在进程启动时打印一次高危能力警告（P1-6 可见性兜底）。
func init() {
	log.Println("[WARN] 脚本执行功能会在宿主机直接运行用户提交的代码，属高危能力；请确保服务仅对受信任用户开放，并尽量在隔离环境/容器中运行")
	log.Println("[执行器] 脚本执行引擎就绪")
}

// 执行状态
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusSuccess   = "success"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// ExecutionTask 执行任务
type ExecutionTask struct {
	ID           string
	ScriptID     string
	DeviceSerial string
	TaskName     string
	Language     string
	Code         string
	Ctx          context.Context
	Cancel       context.CancelFunc
	LogChan      chan LogEntry
	Pid          int    // 当前命令的进程 PID，用于进程树终止
	mu           sync.Mutex
	OnDone       func(status string) // 任务结束回调（由调用方注入，如计划执行引擎聚合结果）
}

// LogEntry 日志条目
type LogEntry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

// ExecutorManager 执行器管理器
type ExecutorManager struct {
	mu    sync.RWMutex
	tasks map[string]*ExecutionTask
}

var Executor = &ExecutorManager{
	tasks: make(map[string]*ExecutionTask),
}

// BroadcastFunc 类型：日志广播函数（由 handlers 包注入，避免循环依赖）
type BroadcastFunc func([]byte)

// logBroadcastFn 日志广播回调（由 handlers 包通过 SetLogBroadcastFunc 注册）
var logBroadcastFn BroadcastFunc

// SetLogBroadcastFunc 注册日志广播函数（由 handlers 包调用）
// P0-3 修复：services 包不能导入 handlers（循环依赖），通过回调注入
func SetLogBroadcastFunc(fn BroadcastFunc) {
	logBroadcastFn = fn
}

// broadcastLog 如果注册了广播函数则调用它
func broadcastLog(data []byte) {
	if logBroadcastFn != nil {
		logBroadcastFn(data)
	}
}

// Start 启动执行
// P0-3 修复：同时启动 LogChan 消费者 goroutine，将日志通过 BroadcastWS 推送到前端
func (em *ExecutorManager) Start(task *ExecutionTask) {
	em.mu.Lock()
	em.tasks[task.ID] = task
	em.mu.Unlock()

	// P0-3: 启动日志消费者，将 LogChan 中的日志通过 WS 广播给前端
	go em.consumeLogs(task)

	// 执行完成后，若调用方注入 OnDone 回调（如计划执行引擎），读取最终状态并通知。
	// 不影响既有 CreateExecution 路径（其 OnDone 为 nil）。
	go func() {
		em.execute(task)
		if task.OnDone != nil {
			var st string
			if err := database.DB.QueryRow("SELECT status FROM executions WHERE id = ?", task.ID).Scan(&st); err == nil {
				task.OnDone(st)
			}
		}
	}()
}

// consumeLogs 消费任务日志通道并通过 BroadcastWS 推送到前端 WebSocket
// P0-3 修复：之前 LogChan 无人消费，前端收不到实时日志
func (em *ExecutorManager) consumeLogs(task *ExecutionTask) {
	for entry := range task.LogChan {
		data, err := json.Marshal(map[string]interface{}{
			"type":        "log",
			"executionId": task.ID,
			"time":        entry.Time,
			"level":       entry.Level,
			"message":     entry.Message,
		})
		if err == nil {
			broadcastLog(data)
		}
	}
}

// Cancel 取消执行
func (em *ExecutorManager) Cancel(id string) error {
	em.mu.RLock()
	task, exists := em.tasks[id]
	em.mu.RUnlock()

	if !exists {
		return fmt.Errorf("任务不存在: %s", id)
	}

	task.Cancel()

	// 终止进程树（Windows）
	task.mu.Lock()
	pid := task.Pid
	task.mu.Unlock()
	if pid > 0 {
		killProcessTree(pid)
	}

	// 更新数据库状态
	_, err := database.DB.Exec(
		"UPDATE executions SET status = ?, finished_at = ? WHERE id = ?",
		StatusCancelled, time.Now().Format(time.RFC3339), id,
	)
	return err
}

// killProcessTree 终止 Windows 进程树
func killProcessTree(pid int) {
	cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F")
	if err := cmd.Run(); err != nil {
		log.Printf("[执行器] taskkill 失败 (进程可能已退出): %v", err)
	}
}

// GetTask 获取任务
// execute 执行脚本
func (em *ExecutorManager) execute(task *ExecutionTask) {
	// 防止单个任务 panic 拖垮整个进程
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[执行器] 任务 %s 执行发生 panic，已捕获: %v", task.ID, r)
			database.DB.Exec(
				"UPDATE executions SET status = ?, finished_at = ? WHERE id = ?",
				StatusFailed, time.Now().Format(time.RFC3339), task.ID,
			)
		}
	}()
	// 任务结束时释放 context（context.CancelFunc 可安全重复调用）
	defer task.Cancel()

	defer func() {
		em.mu.Lock()
		delete(em.tasks, task.ID)
		em.mu.Unlock()
		close(task.LogChan) // consumeLogs goroutine 会在 range 结束后自动退出
	}()

	// 更新状态为运行中
	database.DB.Exec(
		"UPDATE executions SET status = ?, started_at = ? WHERE id = ?",
		StatusRunning, time.Now().Format(time.RFC3339), task.ID,
	)

	task.emitLog("INFO", fmt.Sprintf("开始执行脚本: %s", task.TaskName))

	tmpDir := filepath.Join(config.AppConfig.LogDir, "tmp")
	os.MkdirAll(tmpDir, 0755)

	switch task.Language {
	case "shell":
		em.executeShell(task)
	case "python":
		em.executePython(task, tmpDir)
	case "javascript":
		em.executeJS(task, tmpDir)
	default:
		task.emitLog("ERROR", fmt.Sprintf("不支持的语言: %s", task.Language))
		database.DB.Exec(
			"UPDATE executions SET status = ?, finished_at = ? WHERE id = ?",
			StatusFailed, time.Now().Format(time.RFC3339), task.ID,
		)
		return
	}
}

// executeShell 逐行执行 Shell 命令（adb shell 模式）
// 每行作为独立的 adb -s <serial> shell <line> 命令执行
func (em *ExecutorManager) executeShell(task *ExecutionTask) {
	lines := strings.Split(task.Code, "\n")
	finishTime := time.Now().Format(time.RFC3339)
	status := StatusSuccess
	cancelled := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 安全校验 — 每行都经 ValidateShellCommand 过滤
		if !ValidateShellCommand(line) {
			task.emitLog("ERROR", "命令被安全策略拦截: "+line)
			status = StatusFailed
			break
		}

		// 检查是否已取消
		select {
		case <-task.Ctx.Done():
			cancelled = true
			task.emitLog("WARN", "任务已被取消")
			status = StatusCancelled
			goto done
		default:
		}

		ctx, cancel := context.WithTimeout(task.Ctx, 300*time.Second)

		// 使用 Start + Wait 模式以便捕获 PID 用于进程树终止
		cmd := exec.CommandContext(ctx, "adb", "-s", task.DeviceSerial, "shell", line)
		cmd.Dir = config.AppConfig.LogDir

		// 创建输出管道
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			cancel()
			status = StatusFailed
			task.emitLog("ERROR", fmt.Sprintf("创建 stdout 管道失败: %v", err))
			goto done
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			cancel()
			status = StatusFailed
			task.emitLog("ERROR", fmt.Sprintf("创建 stderr 管道失败: %v", err))
			goto done
		}

		if err := cmd.Start(); err != nil {
			cancel()
			status = StatusFailed
			task.emitLog("ERROR", fmt.Sprintf("启动命令失败: %v", err))
			goto done
		}

		// 记录 PID 用于进程树终止
		if cmd.Process != nil {
			task.mu.Lock()
			task.Pid = cmd.Process.Pid
			task.mu.Unlock()
		}

		// 并行读取 stdout 和 stderr
		var outputMu sync.Mutex
		var outputLines []string

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				t := strings.TrimSpace(scanner.Text())
				if t != "" {
					outputMu.Lock()
					outputLines = append(outputLines, t)
					outputMu.Unlock()
				}
			}
		}()

		go func() {
			defer wg.Done()
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				t := strings.TrimSpace(scanner.Text())
				if t != "" {
					outputMu.Lock()
					outputLines = append(outputLines, "[STDERR] "+t)
					outputMu.Unlock()
				}
			}
		}()

		wg.Wait()

		err = cmd.Wait()
		cancel()

		// 清除 PID
		task.mu.Lock()
		task.Pid = 0
		task.mu.Unlock()

		finishTime = time.Now().Format(time.RFC3339)

		if err != nil {
			if task.Ctx.Err() != nil {
				cancelled = true
				status = StatusCancelled
				task.emitLog("WARN", "任务已被取消")
			} else {
				status = StatusFailed
				task.emitLog("ERROR", fmt.Sprintf("执行失败: %v", err))
			}
			// 仍然输出已有的输出内容
			for _, outLine := range outputLines {
				task.emitLog("INFO", fmt.Sprintf("[%s] %s", line, outLine))
			}
			goto done
		}

		// 输出执行结果
		for _, outLine := range outputLines {
			task.emitLog("INFO", fmt.Sprintf("[%s] %s", line, outLine))
		}
	}

done:
	database.DB.Exec(
		"UPDATE executions SET status = ?, finished_at = ? WHERE id = ?",
		status, finishTime, task.ID,
	)

	if !cancelled {
		task.emitLog("INFO", fmt.Sprintf("执行完成，状态: %s", status))
	}
}

// executePython 执行 Python 脚本
func (em *ExecutorManager) executePython(task *ExecutionTask, tmpDir string) {
	// 高危：在宿主机直接执行用户提交的代码。
	ctx, cancel := context.WithTimeout(task.Ctx, 300*time.Second)
	defer cancel()

	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("qatest_%s.py", task.ID))
	if err := os.WriteFile(tmpFile, []byte(task.Code), 0644); err != nil {
		task.emitLog("ERROR", fmt.Sprintf("创建临时脚本失败: %v", err))
		database.DB.Exec(
			"UPDATE executions SET status = ?, finished_at = ? WHERE id = ?",
			StatusFailed, time.Now().Format(time.RFC3339), task.ID,
		)
		return
	}

	cmd := exec.CommandContext(ctx, "python", tmpFile)
	cmd.Dir = tmpDir

	// 捕获 PID
	if err := cmd.Start(); err != nil {
		task.emitLog("ERROR", fmt.Sprintf("启动 Python 失败: %v", err))
		os.Remove(tmpFile)
		database.DB.Exec(
			"UPDATE executions SET status = ?, finished_at = ? WHERE id = ?",
			StatusFailed, time.Now().Format(time.RFC3339), task.ID,
		)
		return
	}

	if cmd.Process != nil {
		task.mu.Lock()
		task.Pid = cmd.Process.Pid
		task.mu.Unlock()
	}

	// 后台清理临时文件
	go func() {
		<-ctx.Done()
		time.Sleep(1 * time.Second)
		os.Remove(tmpFile)
	}()

	output, err := cmd.CombinedOutput()

	// 清除 PID
	task.mu.Lock()
	task.Pid = 0
	task.mu.Unlock()

	finishTime := time.Now().Format(time.RFC3339)
	status := StatusSuccess

	if err != nil {
		if task.Ctx.Err() != nil {
			status = StatusCancelled
			task.emitLog("WARN", "任务已被取消")
		} else {
			status = StatusFailed
			task.emitLog("ERROR", fmt.Sprintf("执行失败: %v", err))
		}
	}

	if len(output) > 0 {
		for _, line := range strings.Split(string(output), "\n") {
			if strings.TrimSpace(line) != "" {
				task.emitLog("INFO", line)
			}
		}
	}

	database.DB.Exec(
		"UPDATE executions SET status = ?, finished_at = ? WHERE id = ?",
		status, finishTime, task.ID,
	)
	task.emitLog("INFO", fmt.Sprintf("执行完成，状态: %s", status))
}

// executeJS 执行 JavaScript 脚本
// JS 运行环境预置了 adb() 函数，通过 HTTP 调用后端 /api/devices/:serial/exec 接口执行 ADB 命令
func (em *ExecutorManager) executeJS(task *ExecutionTask, tmpDir string) {
	// 高危：在宿主机直接执行用户提交的代码。
	ctx, cancel := context.WithTimeout(task.Ctx, 300*time.Second)
	defer cancel()

	port := config.AppConfig.Port
	if port == "" {
		port = "3000"
	}
	deviceSerial := task.DeviceSerial

	// P0-4 修复：从配置获取 JWT，不再硬编码空字符串导致 401
	jsToken := config.JSAuthToken
	if jsToken == "" {
		// 未配置 JS_AUTH_TOKEN，输出警告但继续执行（adb 调用会返回 401）
		log.Printf("[WARN] JS_AUTH_TOKEN 未设置，JS 脚本中的 adb() 调用将返回 401；请在 .env 中配置 JS_AUTH_TOKEN")
	}

	// JS preamble: 预置 adb() / log() / sleep() / assert() 函数
	jsPreamble := fmt.Sprintf(`
// Qatest JS 运行环境
const DEVICE_SERIAL = %q;
const PORT = %q;
const TOKEN = %q; // P0-4: 从环境变量 JS_AUTH_TOKEN 获取

const log = (msg) => console.log('[LOG]', msg);
const sleep = (ms) => new Promise(r => setTimeout(r, ms));
const assert = (cond, msg) => { if (!cond) throw new Error(msg || 'Assertion failed'); };

// ADB 函数：通过 HTTP 调用后端 /api/devices/:serial/exec 接口来执行 ADB 命令
const adb = async (cmd) => {
    const r = await fetch('http://localhost:' + PORT + '/api/devices/' + DEVICE_SERIAL + '/exec', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + TOKEN },
        body: JSON.stringify({ command: cmd })
    });
    const d = await r.json();
    log('[ADB] ' + cmd + ' => ' + JSON.stringify(d));
    return d;
};

(async () => {
`, deviceSerial, port, jsToken) + task.Code + `
})().catch(err => { console.error('[ERROR]', err.message); process.exit(1); });
`

	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("qatest_%s.js", task.ID))
	if err := os.WriteFile(tmpFile, []byte(jsPreamble), 0644); err != nil {
		task.emitLog("ERROR", fmt.Sprintf("创建临时脚本失败: %v", err))
		database.DB.Exec(
			"UPDATE executions SET status = ?, finished_at = ? WHERE id = ?",
			StatusFailed, time.Now().Format(time.RFC3339), task.ID,
		)
		return
	}

	cmd := exec.CommandContext(ctx, "node", tmpFile)
	cmd.Dir = tmpDir

	// 捕获 PID
	if err := cmd.Start(); err != nil {
		task.emitLog("ERROR", fmt.Sprintf("启动 Node.js 失败: %v", err))
		os.Remove(tmpFile)
		database.DB.Exec(
			"UPDATE executions SET status = ?, finished_at = ? WHERE id = ?",
			StatusFailed, time.Now().Format(time.RFC3339), task.ID,
		)
		return
	}

	if cmd.Process != nil {
		task.mu.Lock()
		task.Pid = cmd.Process.Pid
		task.mu.Unlock()
	}

	// 后台清理临时文件
	go func() {
		<-ctx.Done()
		time.Sleep(1 * time.Second)
		os.Remove(tmpFile)
	}()

	output, err := cmd.CombinedOutput()

	// 清除 PID
	task.mu.Lock()
	task.Pid = 0
	task.mu.Unlock()

	finishTime := time.Now().Format(time.RFC3339)
	status := StatusSuccess

	if err != nil {
		if task.Ctx.Err() != nil {
			status = StatusCancelled
			task.emitLog("WARN", "任务已被取消")
		} else {
			status = StatusFailed
			task.emitLog("ERROR", fmt.Sprintf("执行失败: %v", err))
		}
	}

	if len(output) > 0 {
		for _, line := range strings.Split(string(output), "\n") {
			if strings.TrimSpace(line) != "" {
				task.emitLog("INFO", line)
			}
		}
	}

	database.DB.Exec(
		"UPDATE executions SET status = ?, finished_at = ? WHERE id = ?",
		status, finishTime, task.ID,
	)
	task.emitLog("INFO", fmt.Sprintf("执行完成，状态: %s", status))
}

// emitLog 发送日志到 LogChan
// P0-3 修复：日志通过 LogChan → consumeLogs → BroadcastWS 推送到前端
func (task *ExecutionTask) emitLog(level, message string) {
	entry := LogEntry{
		Time:    time.Now().Format(time.RFC3339),
		Level:   level,
		Message: message,
	}
	select {
	case task.LogChan <- entry:
	default:
		// 通道满时丢弃（非阻塞）
	}
}
