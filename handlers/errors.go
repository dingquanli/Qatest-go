package handlers

import (
	"log"
	"net/http"

	"qatest/models"

	"github.com/gin-gonic/gin"
)

// respondError 向客户端返回脱敏后的错误提示，真实错误记录到服务端日志，
// 避免把数据库结构、文件路径、SQL 等内部实现细节泄露给客户端（P0-3 修复）。
func respondError(c *gin.Context, status int, err error, publicMsg string) {
	if err != nil {
		log.Printf("[ERROR] %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	}
	msg := publicMsg
	if msg == "" {
		msg = "服务器内部错误"
	}
	c.JSON(status, models.APIResponse{Success: false, Error: msg})
}

// validateEnum 校验枚举值是否在允许集合内（P2-8 修复）。
func validateEnum(value string, allowed ...string) bool {
	for _, a := range allowed {
		if value == a {
			return true
		}
	}
	return false
}
