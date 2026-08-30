package handlers

import (
	"net/http"

	"qatest/config"
	"qatest/middleware"
	"qatest/models"
	"qatest/repository"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// Login 用户登录
func Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	var user *config.UserConfig
	for _, u := range config.AppConfig.Users {
		if u.Username == req.Username {
			user = &u
			break
		}
	}

	if user == nil {
		c.JSON(http.StatusUnauthorized, models.APIResponse{Success: false, Error: "用户名或密码错误"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, models.APIResponse{Success: false, Error: "用户名或密码错误"})
		return
	}

	respUser := models.User{
		ID:       user.Username,
		Username: user.Username,
		Name:     user.Name,
		Role:     user.Role,
	}

	token, err := middleware.GenerateToken(respUser)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: "生成令牌失败"})
		return
	}

	refreshToken, err := repository.IssueRefreshToken(user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: "生成刷新令牌失败"})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data: models.LoginResponse{
			Token:        token,
			RefreshToken: refreshToken,
			User:         respUser,
		},
	})
}

// RefreshToken 刷新 Token（轮换式）：
// 旧刷新令牌校验通过后立即作废并签发新令牌对；已作废/过期的刷新令牌一律拒绝。
// 数据库只存 SHA-256 哈希，明文令牌泄露无法反查，且被重放的旧令牌会被识别拒绝。
func RefreshToken(c *gin.Context) {
	var req models.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	userID, err := repository.RotateRefreshToken(req.Token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.APIResponse{Success: false, Error: "刷新令牌无效或已过期"})
		return
	}

	var role, name string
	for _, u := range config.AppConfig.Users {
		if u.Username == userID {
			role, name = u.Role, u.Name
			break
		}
	}

	user := models.User{ID: userID, Username: userID, Name: name, Role: role}

	token, err := middleware.GenerateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: "刷新失败"})
		return
	}

	newRefresh, err := repository.IssueRefreshToken(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: "刷新失败"})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: gin.H{"token": token, "refreshToken": newRefresh}})
}

// Logout 登出：撤销客户端携带的刷新令牌（幂等，令牌已无效也返回成功）。
func Logout(c *gin.Context) {
	var req models.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Token == "" {
		c.JSON(http.StatusOK, models.APIResponse{Success: true})
		return
	}
	_ = repository.RevokeRefreshToken(req.Token)
	c.JSON(http.StatusOK, models.APIResponse{Success: true})
}
