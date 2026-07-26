package middleware

import (
	"net/http"
	"strings"
	"time"

	"qatest/config"
	"qatest/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// 白名单路径（无需认证）
var authWhitelist = map[string]bool{
	"/api/auth/login":   true,
	"/api/auth/refresh": true,
}

// AuthRequired JWT 认证中间件
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if authWhitelist[c.Request.URL.Path] {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// 也检查 query 参数（WebSocket）
			authHeader = c.Query("token")
			if authHeader != "" {
				authHeader = "Bearer " + authHeader
			}
		}

		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, models.APIResponse{
				Success: false,
				Error:   "缺少认证令牌",
			})
			c.Abort()
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenStr == authHeader {
			c.JSON(http.StatusUnauthorized, models.APIResponse{
				Success: false,
				Error:   "无效的认证格式",
			})
			c.Abort()
			return
		}

		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			// P3 加固：强制 HS256 签名算法，拒绝 none / 非对称算法混淆攻击
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(config.AppConfig.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, models.APIResponse{
				Success: false,
				Error:   "令牌无效或已过期",
			})
			c.Abort()
			return
		}

		if uid, ok := claims["userId"].(string); ok {
			c.Set("userId", uid)
		}
		if uname, ok := claims["username"].(string); ok {
			c.Set("username", uname)
		}
		if role, ok := claims["role"].(string); ok {
			c.Set("role", role)
		}
		c.Next()
	}
}

// GenerateToken 生成 JWT Token
func GenerateToken(user models.User) (string, error) {
	claims := jwt.MapClaims{
		"userId":   user.ID,
		"username": user.Username,
		"role":     user.Role,
		"exp":      time.Now().Add(config.AppConfig.JWTExpiresIn).Unix(),
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(config.AppConfig.JWTSecret))
}

// Auth 别名（兼容路由引用）
func Auth() gin.HandlerFunc {
	return AuthRequired()
}

// ParseToken 解析 Token 并返回 Claims
func ParseToken(tokenStr string) (*models.Claims, error) {
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		// P3 加固：强制 HS256 签名算法，拒绝 none / 非对称算法混淆攻击
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(config.AppConfig.JWTSecret), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}
	uid, _ := claims["userId"].(string)
	uname, _ := claims["username"].(string)
	role, _ := claims["role"].(string)
	return &models.Claims{UserID: uid, Username: uname, Role: role}, nil
}
