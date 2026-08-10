package handlers

import (
	"encoding/json"
	"net/http"

	"qatest/database"
	"qatest/models"

	"github.com/gin-gonic/gin"
)

// --- 自由电子表格（纯文本网格）---

// spreadsheetInput 用于解析创建/更新请求。cells 在前端契约中是二维数组（create 时直接传数组，
// update 时走了 JSON.stringify 变成字符串），这里用 json.RawMessage 容错两种形态，再规范化为可存储的 JSON 字符串。
type spreadsheetInput struct {
	Name  string          `json:"name"`
	Cells json.RawMessage `json:"cells"`
}

// normalizeCells 将前端传入的 cells（JSON 字符串或二维数组）规范化为可存储的 JSON 字符串。
func normalizeCells(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "[]"
	}
	// 若是 JSON 字符串（update 走了 JSON.stringify），取其内部内容并校验确为合法 JSON 数组
	var asStr string
	if err := json.Unmarshal(raw, &asStr); err == nil {
		var probe [][]string
		if json.Unmarshal([]byte(asStr), &probe) == nil {
			return asStr
		}
		return "[]"
	}
	// 否则视为二维数组，原样存为紧凑 JSON 字符串
	return string(raw)
}

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
	var in spreadsheetInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	s := models.Spreadsheet{
		ID:        generateID("sh"),
		Name:      in.Name,
		Cells:     normalizeCells(in.Cells),
		CreatedAt: models.NowStr(),
		UpdatedAt: models.NowStr(),
	}
	if s.Name == "" {
		s.Name = "工作表"
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
	var in spreadsheetInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	s := models.Spreadsheet{
		ID:        id,
		Name:      in.Name,
		Cells:     normalizeCells(in.Cells),
		UpdatedAt: models.NowStr(),
	}
	_, err := database.DB.Exec("UPDATE spreadsheets SET name=?, cells=?, updated_at=? WHERE id=?",
		s.Name, s.Cells, s.UpdatedAt, id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
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
