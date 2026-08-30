package handlers

import (
	"net/http"
	"time"

	"qatest/models"
	"qatest/repository"
	"qatest/services"

	"github.com/gin-gonic/gin"
)

// GetScripts 脚本列表
func GetScripts(c *gin.Context) {
	scripts, err := repository.ListScripts()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: scripts})
}

// GetScript 脚本详情
func GetScript(c *gin.Context) {
	s, err := repository.GetScript(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{Success: false, Error: "脚本不存在"})
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: s})
}

// CreateScript 创建脚本
func CreateScript(c *gin.Context) {
	var s models.Script
	if err := c.ShouldBindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	s.ID = "s-" + time.Now().Format("20060102150405") + "-s" + services.GenerateSecureID("")[:4]
	s.CreatedAt = models.NowStr()
	s.UpdatedAt = s.CreatedAt

	if err := repository.CreateScript(s); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}

	c.JSON(http.StatusCreated, models.APIResponse{Success: true, Data: s})
}

// UpdateScript 更新脚本
func UpdateScript(c *gin.Context) {
	id := c.Param("id")
	var s models.Script
	if err := c.ShouldBindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	s.UpdatedAt = models.NowStr()

	if err := repository.UpdateScript(id, s); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}

	s.ID = id
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: s})
}

// DeleteScript 删除脚本
func DeleteScript(c *gin.Context) {
	if err := repository.DeleteScript(c.Param("id")); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: nil})
}
