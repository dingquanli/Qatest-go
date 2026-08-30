package services

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"qatest/config"
)

// 脚本执行沙箱：可选 Docker 容器隔离。
//
// 设计约束（fail-safe）：
//   - EXECUTOR_SANDBOX=host（默认）：保持历史行为，直接在宿主机运行（仅建议受信任环境）。
//   - EXECUTOR_SANDBOX=docker：python/js 在容器内运行，带资源限制；Docker 不可用或
//     探测失败时任务直接失败，【绝不静默回退宿主机】——回退等于把高危能力伪装成已隔离。
//   - shell（adb）模式始终在宿主机执行：adb 依赖宿主机 USB/设备连接，无法容器化；
//     该模式本就经过 ValidateShellCommand 命令白名单过滤。
//
// 资源限制：--memory 256m / --cpus 1 / --pids-limit 64；python 容器 --network none
// （用户代码无任何网络）；js 容器需访问宿主机 /api/devices/:serial/exec（adb() 函数），
// 故保留默认网络并通过 host.docker.internal 回连宿主机。
// 首次运行前建议预拉取镜像（docker pull python:3.11-slim 等），否则可能撞上 300s 任务超时。

const (
	sandboxContainerPrefix = "qatest-exec-"
	// js 容器内访问宿主机 API 的主机名（Docker Desktop 原生支持；Linux 由 --add-host 注入）
	sandboxJSHostName = "host.docker.internal"
)

// dockerProbe docker 守护进程可用性探测结果缓存（带 TTL，避免每次执行都探测）
var (
	dockerProbeMu      sync.Mutex
	dockerProbeOK      bool
	dockerProbeAt      time.Time
	dockerProbeTTL     = 1 * time.Minute
	dockerProbeTimeout = 5 * time.Second
)

// dockerAvailable 探测 Docker CLI 与守护进程是否可用（结果缓存 1 分钟）。
func dockerAvailable() bool {
	dockerProbeMu.Lock()
	defer dockerProbeMu.Unlock()
	if time.Since(dockerProbeAt) < dockerProbeTTL {
		return dockerProbeOK
	}
	ctx, cancel := context.WithTimeout(context.Background(), dockerProbeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Version}}")
	err := cmd.Run()
	dockerProbeOK = err == nil
	dockerProbeAt = time.Now()
	if !dockerProbeOK {
		log.Printf("[沙箱] Docker 不可用（未安装或守护进程未运行）")
	}
	return dockerProbeOK
}

// buildDockerRunArgs 构造 docker run 参数（纯函数，便于单测）。
// name 为容器名（--rm 自动清理；取消路径按名 rm -f）；
// mountDir 挂载到容器 /task 并作为工作目录；
// networkNone=true 时容器无网络，否则注入 host-gateway 供容器回连宿主机。
func buildDockerRunArgs(name, image, mountDir string, networkNone bool, inner ...string) []string {
	args := []string{
		"run", "--rm",
		"--name", name,
		"--memory", "256m",
		"--cpus", "1",
		"--pids-limit", "64",
	}
	if networkNone {
		args = append(args, "--network", "none")
	} else {
		args = append(args, "--add-host", sandboxJSHostName+":host-gateway")
	}
	// Windows 路径转正斜杠（Docker Desktop 接受 C:/... 形式）
	args = append(args, "-v", filepath.ToSlash(mountDir)+":/task", "-w", "/task", image)
	args = append(args, inner...)
	return args
}

// sandboxCommand 按沙箱配置构造执行命令（ctx 用于任务取消/超时时终止 docker 客户端）。
// hostInner 为宿主机模式命令（历史行为）；containerInner 为容器内命令（容器内路径以 /task 为根）。
// 返回 (nil, err) 表示配置为 docker 模式但 Docker 不可用（fail-safe：调用方必须让任务失败）。
func sandboxCommand(ctx context.Context, taskID string, networkNone bool, image string, hostInner, containerInner []string) (*exec.Cmd, error) {
	if config.AppConfig == nil || config.AppConfig.ExecutorSandbox != "docker" {
		return exec.CommandContext(ctx, hostInner[0], hostInner[1:]...), nil
	}
	if !dockerAvailable() {
		return nil, fmt.Errorf("EXECUTOR_SANDBOX=docker 但 Docker 不可用，任务拒绝执行（fail-safe，不回退宿主机）")
	}
	mountDir, err := filepath.Abs(containerTmpDir())
	if err != nil {
		return nil, fmt.Errorf("解析沙箱挂载目录失败: %w", err)
	}
	args := buildDockerRunArgs(sandboxContainerName(taskID), image, mountDir, networkNone, containerInner...)
	return exec.CommandContext(ctx, "docker", args...), nil
}

// sandboxActive 报告当前是否处于 docker 沙箱模式（JS 预置代码据此选择回连宿主机的主机名）。
func sandboxActive() bool {
	return config.AppConfig != nil && config.AppConfig.ExecutorSandbox == "docker"
}

// containerTmpDir 脚本临时目录（复用 LogDir/tmp，按任务隔离）。
func containerTmpDir() string {
	if config.AppConfig == nil {
		return "logs/tmp"
	}
	return filepath.Join(config.AppConfig.LogDir, "tmp")
}

// sandboxContainerName 任务对应的沙箱容器名（取消路径按名清理）。
func sandboxContainerName(taskID string) string {
	return sandboxContainerPrefix + taskID
}

// stopSandboxContainer 强制移除任务的沙箱容器（取消/超时路径调用）。
// 正常退出的容器由 --rm 自动清理，此处对已不存在的容器报错属预期，忽略。
func stopSandboxContainer(taskID string) {
	if config.AppConfig == nil || config.AppConfig.ExecutorSandbox != "docker" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, "docker", "rm", "-f", sandboxContainerName(taskID)).Run()
}
