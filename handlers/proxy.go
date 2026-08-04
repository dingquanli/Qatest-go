package handlers

import (
	"encoding/json"
	"net/http"

	"qatest/middleware"
	"qatest/models"
	"qatest/services"

	"github.com/gin-gonic/gin"
)

// validateProxyTarget SSRF 校验：复用 middleware.ValidateURL 对代理目标主机做
// 解析与私网拦截。target 为空时由服务侧使用已配置的代理目标，此处不拦截。
func validateProxyTarget(target string) error {
	if target == "" {
		return nil
	}
	// target 通常为 "host" 或 "host:port"，构造完整 URL 交给统一校验函数。
	return middleware.ValidateURL("http://" + target)
}

// GetProxyStatus 获取代理状态
func GetProxyStatus(c *gin.Context) {
	status := services.ProxyInstance.GetStatus()
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: status})
}

// StartProxy 启动代理
func StartProxy(c *gin.Context) {
	var req struct {
		Target string `json:"target"`
	}
	c.ShouldBindJSON(&req)

	if err := services.ProxyInstance.Start(req.Target); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: services.ProxyInstance.GetStatus()})
}

// StopProxy 停止代理
func StopProxy(c *gin.Context) {
	services.ProxyInstance.Stop()
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: services.ProxyInstance.GetStatus()})
}

// ToggleProxyPause 切换暂停
func ToggleProxyPause(c *gin.Context) {
	paused := services.ProxyInstance.TogglePause()
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: gin.H{"paused": paused}})
}

// SendProxyRequest 发送请求。
// - 当携带 url 字段时，按普通 HTTP/REST 语义发送（ApiTest 的"发送"按钮）；
// - 否则按 gRPC 语义发送（ProtocolRecorder / 重放，沿用原逻辑）。
func SendProxyRequest(c *gin.Context) {
	var req struct {
		Method  string            `json:"method"`
		Request string            `json:"request"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers"`
		Params  map[string]string `json:"params"`
		Target  string            `json:"target"`
		Timeout int               `json:"timeout"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	// REST/HTTP 分支：前端 ApiTest 发送普通接口
	if req.URL != "" {
		// SSRF 防护，禁止将请求转发至内网/私网地址
		if err := middleware.ValidateURL(req.URL); err != nil {
			respondError(c, http.StatusBadRequest, err, "目标地址不合法或位于内网,已拒绝")
			return
		}
		result := services.ProxyInstance.SendHTTPRequestSafe(req.Method, req.URL, req.Headers, req.Params, req.Request, req.Timeout)
		c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: result})
		return
	}

	// gRPC 分支：沿用原协议录制逻辑
	// SSRF 防护，禁止将请求转发至内网/私网地址
	if err := validateProxyTarget(req.Target); err != nil {
		respondError(c, http.StatusBadRequest, err, "目标地址不合法或位于内网,已拒绝")
		return
	}

	headersJSON, _ := json.Marshal(req.Headers)
	resp, err := services.ProxyInstance.SendRequest(req.Method, req.Request, string(headersJSON), req.Target, req.Timeout)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: gin.H{"response": resp}})
}

// ReplayProxy 重放 gRPC 帧
func ReplayProxy(c *gin.Context) {
	var req struct {
		Target         string `json:"target"`
		Method         string `json:"method"`
		RawFrameBase64 string `json:"rawFrameBase64"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	// SSRF 防护，禁止将请求转发至内网/私网地址
	if err := validateProxyTarget(req.Target); err != nil {
		respondError(c, http.StatusBadRequest, err, "目标地址不合法或位于内网,已拒绝")
		return
	}

	resp, err := services.ProxyInstance.ReplayFrame(req.Target, req.Method, req.RawFrameBase64)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: gin.H{"response": resp}})
}

// GetProxyLogs 获取代理日志
func GetProxyLogs(c *gin.Context) {
	logs := services.ProxyInstance.GetLogs()
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: logs})
}

// GetProxyExecutions 获取代理执行历史
func GetProxyExecutions(c *gin.Context) {
	execs := services.ProxyInstance.GetExecutions()
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: execs})
}

// ClearProxyExecutions 清空代理执行历史
func ClearProxyExecutions(c *gin.Context) {
	services.ProxyInstance.ClearExecutions()
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: "已清空"})
}
