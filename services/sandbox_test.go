package services

import (
	"context"
	"strings"
	"testing"

	"qatest/config"
)

// TestBuildDockerRunArgs 容器命令构造：资源限制、网络策略、挂载与镜像
func TestBuildDockerRunArgs(t *testing.T) {
	t.Run("python（无网络）", func(t *testing.T) {
		args := buildDockerRunArgs("qatest-exec-t1", "python:3.11-slim", "C:/logs/tmp", true, "python", "/task/qatest_t1.py")
		joined := strings.Join(args, " ")
		for _, want := range []string{
			"run --rm",
			"--name qatest-exec-t1",
			"--memory 256m",
			"--cpus 1",
			"--pids-limit 64",
			"--network none",
			"-v C:/logs/tmp:/task",
			"-w /task",
			"python:3.11-slim",
			"python /task/qatest_t1.py",
		} {
			if !strings.Contains(joined, want) {
				t.Fatalf("参数 %q 缺少 %q", joined, want)
			}
		}
		if strings.Contains(joined, "host-gateway") {
			t.Fatal("无网络容器不应注入 host-gateway")
		}
	})

	t.Run("js（保留网络回连宿主机）", func(t *testing.T) {
		args := buildDockerRunArgs("qatest-exec-t2", "node:20-alpine", "/var/tmp", false, "node", "/task/qatest_t2.js")
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "--add-host "+sandboxJSHostName+":host-gateway") {
			t.Fatalf("js 容器应注入 host-gateway: %s", joined)
		}
		if strings.Contains(joined, "--network none") {
			t.Fatal("js 容器需访问宿主机 API，不应禁用网络")
		}
	})
}

// TestSandboxCommandHostMode host 模式（默认）使用宿主机命令，行为与历史一致
func TestSandboxCommandHostMode(t *testing.T) {
	old := config.AppConfig
	defer func() { config.AppConfig = old }()
	config.AppConfig = &config.Config{ExecutorSandbox: "host", LogDir: "logs"}

	cmd, err := sandboxCommand(context.Background(), "t1", true, "python:3.11-slim",
		[]string{"python", "logs/tmp/qatest_t1.py"},
		[]string{"python", "/task/qatest_t1.py"})
	if err != nil {
		t.Fatalf("host 模式不应报错: %v", err)
	}
	if cmd.Path == "" || strings.Contains(strings.Join(cmd.Args, " "), "docker") {
		t.Fatalf("host 模式不应调用 docker: %v", cmd.Args)
	}
}

// TestSandboxCommandDockerFailSafe docker 模式：Docker 可用则构造容器命令，
// 不可用则必须报错（fail-safe，绝不静默回退宿主机）。
func TestSandboxCommandDockerFailSafe(t *testing.T) {
	old := config.AppConfig
	defer func() { config.AppConfig = old }()
	config.AppConfig = &config.Config{ExecutorSandbox: "docker", LogDir: "logs"}

	cmd, err := sandboxCommand(context.Background(), "t1", true, "python:3.11-slim",
		[]string{"python", "x.py"},
		[]string{"python", "/task/x.py"})
	if dockerAvailable() {
		if err != nil || cmd == nil {
			t.Fatalf("Docker 可用时应有命令: cmd=%v err=%v", cmd, err)
		}
		return
	}
	// 本机无 Docker（CI/开发机常见）：必须 fail-safe 报错
	if err == nil || cmd != nil {
		t.Fatalf("Docker 不可用时应拒绝执行，got cmd=%v err=%v", cmd, err)
	}
	if !strings.Contains(err.Error(), "不回退宿主机") {
		t.Fatalf("错误信息应说明不回退宿主机: %v", err)
	}
}

// TestSandboxConfigDefaults 非法沙箱配置回退 host
func TestSandboxConfigDefaults(t *testing.T) {
	cases := map[string]string{
		"host":   "host",
		"docker": "docker",
		"DOCKER": "docker",
		"bogus":  "host",
		"":       "host",
	}
	for raw, want := range cases {
		t.Run(raw, func(t *testing.T) {
			t.Setenv("EXECUTOR_SANDBOX", raw)
			t.Setenv("ADMIN_PASSWORD", "x")
			t.Setenv("JWT_SECRET", "unit-test-jwt-secret-0123456789abcdef")
			cfg := config.Load()
			if cfg.ExecutorSandbox != want {
				t.Fatalf("EXECUTOR_SANDBOX=%q want %q got %q", raw, want, cfg.ExecutorSandbox)
			}
			if cfg.PythonImage == "" || cfg.NodeImage == "" {
				t.Fatal("默认镜像不应为空")
			}
		})
	}
}
