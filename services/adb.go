package services

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// DeviceInfo 设备信息
type DeviceInfo struct {
	Serial       string `json:"serial"`
	Model        string `json:"model"`
	Manufacturer string `json:"manufacturer"`
	AndroidVer   string `json:"androidVer"`
	SDKVersion   string `json:"sdkVersion"`
	Battery      string `json:"battery"`
	Resolution   string `json:"resolution"`
	State        string `json:"state"`
}

// ADBManager ADB 设备管理
type ADBManager struct {
	mu      sync.RWMutex
	devices map[string]*DeviceInfo
}

var ADB = &ADBManager{
	devices: make(map[string]*DeviceInfo),
}

// GetDevices 获取当前设备列表
func (am *ADBManager) GetDevices() []DeviceInfo {
	am.mu.RLock()
	defer am.mu.RUnlock()

	result := make([]DeviceInfo, 0, len(am.devices))
	for _, d := range am.devices {
		result = append(result, *d)
	}
	return result
}

// GetDevice 获取单个设备
func (am *ADBManager) GetDevice(serial string) (*DeviceInfo, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	d, ok := am.devices[serial]
	if !ok {
		return nil, fmt.Errorf("设备未找到: %s", serial)
	}
	return d, nil
}

// Scan 扫描 ADB 设备
func (am *ADBManager) Scan() ([]DeviceInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "adb", "devices", "-l")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ADB 命令失败: %w (请确认 adb 已安装并添加到 PATH)", err)
	}

	am.mu.Lock()
	defer am.mu.Unlock()

	// 清空旧数据
	newDevices := make(map[string]*DeviceInfo)

	lines := strings.Split(string(output), "\n")
	for _, line := range lines[1:] { // 跳过首行 "List of devices attached"
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 格式: serial    device/offline product:xxx model:xxx device:xxx
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		serial := parts[0]
		state := parts[1]

		if state != "device" {
			continue
		}

		// 尝试获取设备详细信息
		di := &DeviceInfo{
			Serial: serial,
			State:  state,
		}

		di.Model = getProp(serial, "ro.product.model")
		di.Manufacturer = getProp(serial, "ro.product.manufacturer")
		di.AndroidVer = getProp(serial, "ro.build.version.release")
		di.SDKVersion = getProp(serial, "ro.build.version.sdk")
		di.Battery = getBattery(serial)
		di.Resolution = getResolution(serial)

		newDevices[serial] = di
	}

	am.devices = newDevices
	log.Printf("[ADB] 扫描完成，发现 %d 台设备", len(am.devices))

	result := make([]DeviceInfo, 0, len(am.devices))
	for _, d := range am.devices {
		result = append(result, *d)
	}
	return result, nil
}

// TakeScreenshot 截图
func (am *ADBManager) TakeScreenshot(serial string) ([]byte, error) {
	cmd := exec.Command("adb", "-s", serial, "exec-out", "screencap", "-p")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("截图失败: %w", err)
	}
	return out.Bytes(), nil
}

// ExecCommand 在设备上执行 Shell 命令
func (am *ADBManager) ExecCommand(serial, command string) (string, error) {
	if !ValidateShellCommand(command) {
		return "", fmt.Errorf("命令被安全策略拦截: %s", command)
	}

	cmd := exec.Command("adb", "-s", serial, "shell", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("执行失败: %w, output: %s", err, string(output))
	}
	return string(output), nil
}

// InstallAPK 安装 APK
func (am *ADBManager) InstallAPK(serial, apkPath string) (string, error) {
	cmd := exec.Command("adb", "-s", serial, "install", "-r", apkPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("安装失败: %w", err)
	}
	return string(output), nil
}

// getProp 获取设备属性
// 优化：添加 5 秒超时，避免每设备 6 个子进程无限等待
func getProp(serial, prop string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "adb", "-s", serial, "shell", "getprop", prop)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// getBattery 获取电池信息
func getBattery(serial string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "adb", "-s", serial, "shell", "dumpsys", "battery")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "level:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "level:"))
		}
	}
	return ""
}

// getResolution 获取分辨率
func getResolution(serial string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "adb", "-s", serial, "shell", "wm", "size")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	line := strings.TrimSpace(string(output))
	if strings.HasPrefix(line, "Physical size:") {
		return strings.TrimSpace(strings.TrimPrefix(line, "Physical size:"))
	}
	// 可能格式为 "Override size: 1080x1920"
	if strings.HasPrefix(line, "Override size:") {
		return strings.TrimSpace(strings.TrimPrefix(line, "Override size:"))
	}
	return ""
}

// StartMonitor 启动设备轮询监控（每 3 秒）

