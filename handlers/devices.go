package handlers

import (
	"encoding/base64"
	"net/http"
	"path/filepath"
	"strings"

	"qatest/config"
	"qatest/models"
	"qatest/services"

	"github.com/gin-gonic/gin"
)

// GetDevices 获取全部设备列表
func GetDevices(c *gin.Context) {
	devices := services.ADB.GetDevices()
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: devices})
}

// ScanDevices 扫描 ADB 设备
func ScanDevices(c *gin.Context) {
	devices, err := services.ADB.Scan()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: devices})
}

// GetDevice 获取单个设备详情
func GetDevice(c *gin.Context) {
	serial := c.Param("serial")
	device, err := services.ADB.GetDevice(serial)
	if err != nil {
		respondError(c, http.StatusNotFound, err, "未找到请求的资源")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: device})
}

// TakeScreenshot 设备截图
func TakeScreenshot(c *gin.Context) {
	serial := c.Param("serial")
	imgBytes, err := services.ADB.TakeScreenshot(serial)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}

	base64Img := base64.StdEncoding.EncodeToString(imgBytes)
	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    gin.H{"image": "data:image/png;base64," + base64Img},
	})
}

// ExecDeviceCommand 在设备上执行 Shell 命令
func ExecDeviceCommand(c *gin.Context) {
	serial := c.Param("serial")

	var req struct {
		Command string `json:"command" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	output, err := services.ADB.ExecCommand(serial, req.Command)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: gin.H{"output": output}})
}

// InstallAPK 安装 APK
// P2-5 修复：校验 APK 路径，防止路径穿越
func InstallAPK(c *gin.Context) {
	serial := c.Param("serial")

	var req struct {
		APKPath string `json:"apkPath" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	// P2-5 修复：校验 APK 路径安全
	apkPath := filepath.Clean(req.APKPath)
	if strings.Contains(apkPath, "..") {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "APK 路径不合法"})
		return
	}
	if !strings.HasSuffix(strings.ToLower(apkPath), ".apk") {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "仅允许 .apk 文件"})
		return
	}

	// P2 加固：限制可安装的 APK 来源路径，防止任意位置（如 /etc/x.apk）被安装。
	if dir := config.AppConfig.ApkDir; dir != "" {
		// 已配置 APK_DIR：要求 APK 必须位于该目录内。
		allowed := filepath.Clean(dir)
		rel, err := filepath.Rel(allowed, apkPath)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "APK 路径不在允许的目录内"})
			return
		}
	} else if filepath.IsAbs(apkPath) {
		// 未配置 APK_DIR 时，禁止安装绝对路径的 APK（失败闭合，避免任意系统路径被安装）。
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "未配置 APK_DIR，禁止安装绝对路径的 APK"})
		return
	}

	output, err := services.ADB.InstallAPK(serial, apkPath)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: gin.H{"output": output}})
}
