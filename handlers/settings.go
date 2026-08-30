package handlers

import (
	"net/http"
	"strings"

	"qatest/models"
	"qatest/repository"

	"github.com/gin-gonic/gin"
)

// maskedSecretValue 敏感字段脱敏后返回的占位符（前端不应将其写回）
const maskedSecretValue = "********"

// isSecretSettingKey 判断 settings 键是否为敏感字段，返回给前端时需脱敏
func isSecretSettingKey(key string) bool {
	k := strings.ToLower(key)
	for _, kw := range []string{"token", "secret", "password", "passwd", "apikey", "api_key", "credential", "auth", "privatekey", "private_key"} {
		if strings.Contains(k, kw) {
			return true
		}
	}
	return false
}

// requireAdmin 校验当前登录用户是否为管理员
func requireAdmin(c *gin.Context) bool {
	return c.GetString("role") == "admin"
}

// GetSettings 获取全部设置（敏感字段脱敏）
func GetSettings(c *gin.Context) {
	raw, err := repository.GetAllSettings()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}

	settings := make(map[string]string)
	for k, v := range raw {
		if isSecretSettingKey(k) {
			v = maskedSecretValue
		}
		settings[k] = v
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: settings})
}

// UpdateSettings 批量更新设置（仅管理员，且跳过脱敏占位符）
func UpdateSettings(c *gin.Context) {
	if !requireAdmin(c) {
		c.JSON(http.StatusForbidden, models.APIResponse{Success: false, Error: "仅管理员可修改系统设置"})
		return
	}

	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	for k, v := range req {
		// 跳过脱敏占位符，避免覆盖真实密钥
		if v == maskedSecretValue {
			continue
		}
		if err := repository.UpsertSetting(k, v); err != nil {
			respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
			return
		}
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: req})
}

// GetSetting 获取单项设置（敏感字段脱敏）
func GetSetting(c *gin.Context) {
	key := c.Param("key")
	value, err := repository.GetSetting(key)
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{Success: false, Error: "设置项不存在"})
		return
	}
	if isSecretSettingKey(key) {
		value = maskedSecretValue
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: gin.H{"key": key, "value": value}})
}

// UpdateSetting 更新单项设置（仅管理员，且跳过脱敏占位符）
func UpdateSetting(c *gin.Context) {
	if !requireAdmin(c) {
		c.JSON(http.StatusForbidden, models.APIResponse{Success: false, Error: "仅管理员可修改系统设置"})
		return
	}

	key := c.Param("key")
	var req struct {
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	// 跳过脱敏占位符，避免覆盖真实密钥
	if req.Value == maskedSecretValue {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "请勿提交脱敏占位符，需提供真实值"})
		return
	}

	if err := repository.UpsertSetting(key, req.Value); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: gin.H{"key": key, "value": req.Value}})
}
