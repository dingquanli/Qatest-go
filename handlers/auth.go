package handlers

import (
	"net/http"

	"qatest/config"
	"qatest/middleware"
	"qatest/models"

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

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data: models.LoginResponse{
			Token: token,
			User:  respUser,
		},
	})
}

// RefreshToken 刷新 Token
func RefreshToken(c *gin.Context) {
	var req models.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	claims, err := middleware.ParseToken(req.Token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.APIResponse{Success: false, Error: "令牌无效"})
		return
	}

	user := models.User{
		ID:       claims.UserID,
		Username: claims.Username,
		Role:     claims.Role,
	}

	token, err := middleware.GenerateToken(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: "刷新失败"})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: gin.H{"token": token}})
}
