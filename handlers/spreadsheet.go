package handlers

import (
	"encoding/json"
	"net/http"

	"qatest/models"
	"qatest/repository"

	"github.com/gin-gonic/gin"
)

// --- 自由电子表格（纯文本网格）---

// spreadsheetInput 用于解析创建/更新请求。cells 在前端契约中是二维数组（create 时直接传数组，
// update 时走了 JSON.stringify 变成字符串），这里用 json.RawMessage 容错两种形态，再规范化为可存储的 JSON 字符串。
// 格式层（formats/col_widths/row_heights/merges）同样用 RawMessage 容错对象/字符串，规范化为 JSON 字符串。
type spreadsheetInput struct {
	Name       string          `json:"name"`
	Cells      json.RawMessage `json:"cells"`
	Formats    json.RawMessage `json:"formats"`
	ColWidths  json.RawMessage `json:"colWidths"`
	RowHeights json.RawMessage `json:"rowHeights"`
	Merges     json.RawMessage `json:"merges"`
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

// normalizeJSON 将对象/数组形态的输入规范化为紧凑 JSON 字符串；空则回退 def。
func normalizeJSON(raw json.RawMessage, def string) string {
	if len(raw) == 0 {
		return def
	}
	var probe interface{}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return def
	}
	b, err := json.Marshal(probe)
	if err != nil {
		return def
	}
	return string(b)
}

func GetSpreadsheets(c *gin.Context) {
	list, err := repository.ListSpreadsheets()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: list})
}

func GetSpreadsheet(c *gin.Context) {
	id := c.Param("id")
	s, err := repository.GetSpreadsheet(id)
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
		ID:         generateID("sh"),
		Name:       in.Name,
		Cells:      normalizeCells(in.Cells),
		Formats:    normalizeJSON(in.Formats, "{}"),
		ColWidths:  normalizeJSON(in.ColWidths, "{}"),
		RowHeights: normalizeJSON(in.RowHeights, "{}"),
		Merges:     normalizeJSON(in.Merges, "[]"),
		CreatedAt:  models.NowStr(),
		UpdatedAt:  models.NowStr(),
	}
	if s.Name == "" {
		s.Name = "工作表"
	}
	if err := repository.CreateSpreadsheet(s); err != nil {
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
		ID:         id,
		Name:       in.Name,
		Cells:      normalizeCells(in.Cells),
		Formats:    normalizeJSON(in.Formats, "{}"),
		ColWidths:  normalizeJSON(in.ColWidths, "{}"),
		RowHeights: normalizeJSON(in.RowHeights, "{}"),
		Merges:     normalizeJSON(in.Merges, "[]"),
		UpdatedAt:  models.NowStr(),
	}
	if err := repository.UpdateSpreadsheet(id, s); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: s})
}

func DeleteSpreadsheet(c *gin.Context) {
	id := c.Param("id")
	if err := repository.DeleteSpreadsheet(id); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: nil})
}
