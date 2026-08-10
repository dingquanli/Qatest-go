package handlers

import (
	"net/http"

	"qatest/database"
	"qatest/models"

	"github.com/gin-gonic/gin"
)

// --- 自由电子表格（纯文本网格）---

func GetSpreadsheets(c *gin.Context) {
	rows, err := database.DB.Query("SELECT id, name, cells, created_at, updated_at FROM spreadsheets ORDER BY created_at")
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	defer rows.Close()
	list := make([]models.Spreadsheet, 0)
	for rows.Next() {
		var s models.Spreadsheet
		if err := rows.Scan(&s.ID, &s.Name, &s.Cells, &s.CreatedAt, &s.UpdatedAt); err != nil {
			respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
			return
		}
		list = append(list, s)
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: list})
}

func GetSpreadsheet(c *gin.Context) {
	id := c.Param("id")
	var s models.Spreadsheet
	err := database.DB.QueryRow("SELECT id, name, cells, created_at, updated_at FROM spreadsheets WHERE id = ?", id).
		Scan(&s.ID, &s.Name, &s.Cells, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		respondError(c, http.StatusNotFound, err, "未找到该表")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: s})
}

func CreateSpreadsheet(c *gin.Context) {
	var s models.Spreadsheet
	if err := c.ShouldBindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	s.ID = generateID("sh")
	now := models.NowStr()
	s.CreatedAt = now
	s.UpdatedAt = now
	if s.Name == "" {
		s.Name = "工作表"
	}
	if s.Cells == "" {
		s.Cells = "[]"
	}
	_, err := database.DB.Exec("INSERT INTO spreadsheets (id, name, cells, created_at, updated_at) VALUES (?,?,?,?,?)",
		s.ID, s.Name, s.Cells, s.CreatedAt, s.UpdatedAt)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusCreated, models.APIResponse{Success: true, Data: s})
}

func UpdateSpreadsheet(c *gin.Context) {
	id := c.Param("id")
	var s models.Spreadsheet
	if err := c.ShouldBindJSON(&s); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	s.UpdatedAt = models.NowStr()
	_, err := database.DB.Exec("UPDATE spreadsheets SET name=?, cells=?, updated_at=? WHERE id=?",
		s.Name, s.Cells, s.UpdatedAt, id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	s.ID = id
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: s})
}

func DeleteSpreadsheet(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec("DELETE FROM spreadsheets WHERE id = ?", id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: nil})
}
