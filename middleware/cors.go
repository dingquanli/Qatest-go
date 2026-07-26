package middleware

import (
	"net/http"

	"qatest/config"

	"github.com/gin-gonic/gin"
)

// CORS 跨域中间件
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowedOrigins := config.AppConfig.AllowedOrigins

		// 计算匹配结果：凭据（Credentials）与通配符 "*" 互斥，且禁止回显任意 Origin。
		hasWildcard := false
		exactMatch := false
		for _, ao := range allowedOrigins {
			if ao == "*" {
				hasWildcard = true
				continue
			}
			if ao == origin {
				exactMatch = true
				break
			}
		}

		allowOrigin := ""
		allowCredentials := false
		if exactMatch {
			// 显式白名单精确匹配：回显该 Origin 并允许凭据
			allowOrigin = origin
			allowCredentials = true
		} else if hasWildcard && origin != "" {
			// 含通配符：回显 "*"，但不得下发 credentials（凭据与通配互斥）
			allowOrigin = "*"
			allowCredentials = false
		}

		if allowOrigin != "" {
			c.Header("Access-Control-Allow-Origin", allowOrigin)
			c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS,PATCH")
			c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Requested-With")
			if allowCredentials {
				c.Header("Access-Control-Allow-Credentials", "true")
			}
			c.Header("Access-Control-Max-Age", "86400")
		}

		// P1-11 修复：添加安全响应头
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// 预检请求
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

