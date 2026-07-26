package middleware

import (
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// sensitiveQueryKeys 记录请求日志时需脱敏的 query 参数名
var sensitiveQueryKeys = []string{"token", "password", "passwd", "secret", "apikey", "api_key", "credential", "auth", "authorization", "key", "privatekey", "private_key"}

// isSensitiveQueryKey 判断 query 参数名是否为敏感字段
func isSensitiveQueryKey(name string) bool {
	n := strings.ToLower(name)
	for _, kw := range sensitiveQueryKeys {
		if strings.Contains(n, kw) {
			return true
		}
	}
	return false
}

// redactQuery 对 URL query 中的敏感参数值进行脱敏，避免 token/password 落入日志
func redactQuery(raw string) string {
	if raw == "" {
		return ""
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		// 无法解析时整体脱敏，避免泄露原始内容
		return "***"
	}
	for k := range values {
		if isSensitiveQueryKey(k) {
			for i := range values[k] {
				values[k][i] = "***"
			}
		}
	}
	return values.Encode()
}

// Logger 请求日志中间件
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := redactQuery(c.Request.URL.RawQuery)

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()
		method := c.Request.Method
		clientIP := c.ClientIP()

		if query != "" {
			path = path + "?" + query
		}

		log.Printf("[HTTP] %s | %3d | %13v | %s | %s",
			method,
			statusCode,
			latency,
			clientIP,
			path,
		)
	}
}
