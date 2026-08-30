package services

// 包级安全说明（服务端代码执行设计风险）：
// 本包中的 executePython / executeJS 会先把用户提交的 .py / .js 代码写成临时文件，
// 再用 python / node 在【宿主机】上直接运行，具备任意代码执行（RCE）能力，属高危能力。
// 完整修复需要沙箱 / 容器隔离（本次不做）。此处仅做可见性兜底：
// 服务仅限对受信任用户开放，强烈建议在隔离环境 / 容器中运行，并配合资源、网络、权限限制。
import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"qatest/config"
	"qatest/database"
)

// init 在进程启动时按开关状态打印提示：开启时警告宿主机 RCE 风险，关闭时说明已熔断。
// 注意：包 init 时 config 可能尚未加载（如单元测试），需容忍 nil。
func init() {
	if config.AppConfig == nil {
		return
	}
	if !config.AppConfig.ExecutorEnabled {
		log.Println("[执行器] 脚本执行引擎已禁用（EXECUTOR_ENABLED 未开启）；如需执行脚本请在 .env 中显式设置 EXECUTOR_ENABLED=1")
		return
	}
	log.Println("[WARN] 脚本执行功能属高危能力；请确保服务仅对受信任用户开放")
	if config.AppConfig.ExecutorSandbox == "docker" {
		log.Println("[执行器] 脚本执行引擎就绪（沙箱模式: docker，容器隔离 + 资源限制）")
	} else {
		log.Println("[执行器] 脚本执行引擎就绪（沙箱模式: host，宿主机直跑——强烈建议在隔离环境/容器中运行，或设置 EXECUTOR_SANDBOX=docker）")
	}
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
	logMu        sync.Mutex
	logClosed    bool
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
var (
	logBroadcastFn BroadcastFunc
	logBroadcastMu sync.RWMutex
)

// SetLogBroadcastFunc 注册日志广播函数（由 handlers 包调用）
// services 包不能导入 handlers（循环依赖），通过回调注入
func SetLogBroadcastFunc(fn BroadcastFunc) {
	logBroadcastMu.Lock()
	defer logBroadcastMu.Unlock()
	logBroadcastFn = fn
}

// broadcastLog 如果注册了广播函数则调用它
func broadcastLog(data []byte) {
	logBroadcastMu.RLock()
	fn := logBroadcastFn
	logBroadcastMu.RUnlock()
	if fn != nil {
		fn(data)
	}
}

// Start 启动执行
// 同时启动 LogChan 消费者 goroutine，将日志通过 BroadcastWS 推送到前端
func (em *ExecutorManager) Start(task *ExecutionTask) {
	em.mu.Lock()
	em.tasks[task.ID] = task
	em.mu.Unlock()

	// 启动日志消费者，将 LogChan 中的日志通过 WS 广播给前端
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
// 之前 LogChan 无人消费，前端收不到实时日志
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
	// 沙箱模式下按容器名强制清理（杀掉 docker 客户端进程不会停止容器）
	stopSandboxContainer(id)

	// 更新数据库状态
	_, err := database.DB.Exec(
		"UPDATE executions SET status = ?, finished_at = ? WHERE id = ?",
		StatusCancelled, time.Now().Format(time.RFC3339), id,
	)
	return err
}

// killProcessTree 跨平台终止进程及其直接子进程
func killProcessTree(pid int) {
	if pid <= 0 {
		return
	}
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F")
		if err := cmd.Run(); err != nil {
			log.Printf("[执行器] taskkill 失败 (进程可能已退出): %v", err)
		}
	default:
		// Unix-like：先终止直接子进程，再结束主进程
		_ = exec.Command("pkill", "-P", strconv.Itoa(pid)).Run()
		if err := exec.Command("kill", "-9", strconv.Itoa(pid)).Run(); err != nil {
			log.Printf("[执行器] kill 失败 (进程可能已退出): %v", err)
		}
	}
}

// execute 执行脚本
func (em *ExecutorManager) execute(task *ExecutionTask) {
	// 纵深防御：即使调用方（handler）漏检开关，执行层也拒绝运行任何用户提交的代码
	if config.AppConfig == nil || !config.AppConfig.ExecutorEnabled {
		task.emitLog("ERROR", "脚本执行引擎已禁用（EXECUTOR_ENABLED 未开启），任务拒绝执行")
		database.DB.Exec(
			"UPDATE executions SET status = ?, finished_at = ? WHERE id = ?",
			StatusFailed, time.Now().Format(time.RFC3339), task.ID,
		)
		return
	}
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
		task.logMu.Lock()
		if !task.logClosed {
			task.logClosed = true
			close(task.LogChan) // consumeLogs goroutine 会在 range 结束后自动退出
		}
		task.logMu.Unlock()
	}()

	// 更新状态为运行中
	database.DB.Exec(
		"UPDATE executions SET status = ?, started_at = ? WHERE id = ?",
		StatusRunning, time.Now().Format(time.RFC3339), task.ID,
	)

	task.emitLog("INFO", fmt.Sprintf("开始执行脚本: %s", task.TaskName))

	tmpDir := filepath.Join(config.AppConfig.LogDir, "tmp")
	// 修复存量 bug：LogDir 为相对路径（默认 "logs"）时，tmpFile 也是相对路径，
	// 而 cmd.Dir=tmpDir 会让 python/node 再基于工作目录拼接一次，得到
	// "logs/tmp/logs/tmp/xxx.py" 找不到文件。统一先解析为绝对路径。
	if abs, err := filepath.Abs(tmpDir); err == nil {
		tmpDir = abs
	}
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
// 沙箱模式（EXECUTOR_SANDBOX=docker）下在容器内运行（--network none + 资源限制），
// 否则在宿主机直接执行（高危）。
func (em *ExecutorManager) executePython(task *ExecutionTask, tmpDir string) {
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

	// 宿主机模式：python <host路径>；docker 模式：python /task/<文件名>（无网络）
	cmd, err := sandboxCommand(ctx, task.ID, true, config.AppConfig.PythonImage,
		[]string{"python", tmpFile},
		[]string{"python", "/task/" + filepath.Base(tmpFile)},
	)
	if err != nil {
		os.Remove(tmpFile)
		task.emitLog("ERROR", err.Error())
		database.DB.Exec(
			"UPDATE executions SET status = ?, finished_at = ? WHERE id = ?",
			StatusFailed, time.Now().Format(time.RFC3339), task.ID,
		)
		return
	}
	cmd.Dir = tmpDir

	// 修复存量 bug：此前先 Start() 抓 PID 再 CombinedOutput()（内部会二次 Start），
	// 必然报 "exec: already started" 导致 python/js 任务永远 failed。
	// 正确姿势：设置输出缓冲 → Start → Wait。
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf

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

	err = cmd.Wait()
	output := buf.Bytes()
	stopSandboxContainer(task.ID)

	task.mu.Lock()
	task.Pid = 0
	task.mu.Unlock()
	os.Remove(tmpFile)

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
// JS 运行环境预置了 adb() 函数，通过 HTTP 调用后端 /api/devices/:serial/exec 接口执行 ADB 命令。
// 沙箱模式下容器通过 host.docker.internal 回连宿主机 API（需保留容器网络）。
func (em *ExecutorManager) executeJS(task *ExecutionTask, tmpDir string) {
	ctx, cancel := context.WithTimeout(task.Ctx, 300*time.Second)
	defer cancel()

	port := config.AppConfig.Port
	if port == "" {
		port = "3000"
	}
	deviceSerial := task.DeviceSerial

	// 从配置获取 JWT，不再硬编码空字符串导致 401
	jsToken := config.JSAuthToken
	if jsToken == "" {
		// 未配置 JS_AUTH_TOKEN，输出警告但继续执行（adb 调用会返回 401）
		log.Printf("[WARN] JS_AUTH_TOKEN 未设置，JS 脚本中的 adb() 调用将返回 401；请在 .env 中配置 JS_AUTH_TOKEN")
	}

	// 沙箱模式下容器内无法用 localhost 回连宿主机，改用 host.docker.internal
	apiHost := "localhost"
	if sandboxActive() {
		apiHost = sandboxJSHostName
	}

	// JS preamble: 预置 adb() / log() / sleep() / assert() 函数
	jsPreamble := fmt.Sprintf(`
// Qatest JS 运行环境
const DEVICE_SERIAL = %q;
const PORT = %q;
const API_HOST = %q; // 沙箱模式下为 host.docker.internal
const TOKEN = %q; // 从环境变量 JS_AUTH_TOKEN 获取

const log = (msg) => console.log('[LOG]', msg);
const sleep = (ms) => new Promise(r => setTimeout(r, ms));
const assert = (cond, msg) => { if (!cond) throw new Error(msg || 'Assertion failed'); };

// ADB 函数：通过 HTTP 调用后端 /api/devices/:serial/exec 接口来执行 ADB 命令
const adb = async (cmd) => {
    const r = await fetch('http://' + API_HOST + ':' + PORT + '/api/devices/' + DEVICE_SERIAL + '/exec', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + TOKEN },
        body: JSON.stringify({ command: cmd })
    });
    const d = await r.json();
    log('[ADB] ' + cmd + ' => ' + JSON.stringify(d));
    return d;
};

(async () => {
`, deviceSerial, port, apiHost, jsToken) + task.Code + `
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

	// 宿主机模式：node <host路径>；docker 模式：node /task/<文件名>（保留网络以回连宿主机 API）
	cmd, err := sandboxCommand(ctx, task.ID, false, config.AppConfig.NodeImage,
		[]string{"node", tmpFile},
		[]string{"node", "/task/" + filepath.Base(tmpFile)},
	)
	if err != nil {
		os.Remove(tmpFile)
		task.emitLog("ERROR", err.Error())
		database.DB.Exec(
			"UPDATE executions SET status = ?, finished_at = ? WHERE id = ?",
			StatusFailed, time.Now().Format(time.RFC3339), task.ID,
		)
		return
	}
	cmd.Dir = tmpDir

	// 同 executePython：Start + Wait（此前 Start 后接 CombinedOutput 必然 already started）
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf

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

	err = cmd.Wait()
	output := buf.Bytes()
	stopSandboxContainer(task.ID)

	task.mu.Lock()
	task.Pid = 0
	task.mu.Unlock()
	os.Remove(tmpFile)

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
// 日志通过 LogChan → consumeLogs → BroadcastWS 推送到前端
func (task *ExecutionTask) emitLog(level, message string) {
	entry := LogEntry{
		Time:    time.Now().Format(time.RFC3339),
		Level:   level,
		Message: message,
	}
	task.logMu.Lock()
	defer task.logMu.Unlock()
	if task.logClosed {
		return
	}
	select {
	case task.LogChan <- entry:
	default:
		// 通道满时丢弃（非阻塞）
	}
}
