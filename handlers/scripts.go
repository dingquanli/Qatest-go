package handlers

import (
	"net/http"
	"time"

	"qatest/database"
	"qatest/models"
	"qatest/services"

	"github.com/gin-gonic/gin"
)

// GetScripts 脚本列表
func GetScripts(c *gin.Context) {
	rows, err := database.DB.Query("SELECT id, name, description, language, code, created_at, updated_at FROM scripts ORDER BY updated_at DESC LIMIT 500")
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()

	scripts := make([]models.Script, 0)
	for rows.Next() {
		var s models.Script
		if err := rows.Scan(&s.ID, &s.Name, &s.Description, &s.Language, &s.Code, &s.CreatedAt, &s.UpdatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
			return
		}
		scripts = append(scripts, s)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: scripts})
}

// GetScript 脚本详情
func GetScript(c *gin.Context) {
	id := c.Param("id")
	var s models.Script
	err := database.DB.QueryRow(
		"SELECT id, name, description, language, code, created_at, updated_at FROM scripts WHERE id = ?",
		id,
	).Scan(&s.ID, &s.Name, &s.Description, &s.Language, &s.Code, &s.CreatedAt, &s.UpdatedAt)

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

	_, err := database.DB.Exec(
		"INSERT INTO scripts (id, name, description, language, code, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		s.ID, s.Name, s.Description, s.Language, s.Code, s.CreatedAt, s.UpdatedAt,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
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

	_, err := database.DB.Exec(
		"UPDATE scripts SET name=?, description=?, language=?, code=?, updated_at=? WHERE id=?",
		s.Name, s.Description, s.Language, s.Code, s.UpdatedAt, id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}

	s.ID = id
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: s})
}

// DeleteScript 删除脚本
func DeleteScript(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec("DELETE FROM scripts WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: nil})
}
