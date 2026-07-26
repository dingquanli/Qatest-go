package handlers

import (
	"net/http"

	"qatest/database"
	"qatest/models"

	"github.com/gin-gonic/gin"
)

// --- 接口定义 ---

func GetAPIDefinitions(c *gin.Context) {
	rows, err := database.DB.Query(
		"SELECT id, name, method, url, tags, module_id, headers, body, created_at, updated_at FROM api_definitions ORDER BY updated_at DESC LIMIT 200",
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()
	defs := make([]models.APIDefinition, 0)
	for rows.Next() {
		var d models.APIDefinition
		if err := rows.Scan(&d.ID, &d.Name, &d.Method, &d.URL, &d.Tags, &d.ModuleID, &d.Headers, &d.Body, &d.CreatedAt, &d.UpdatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
			return
		}
		defs = append(defs, d)
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: defs})
}

func GetAPIDefinition(c *gin.Context) {
	id := c.Param("id")
	var d models.APIDefinition
	err := database.DB.QueryRow(
		"SELECT id, name, method, url, tags, module_id, headers, body, created_at, updated_at FROM api_definitions WHERE id = ?", id,
	).Scan(&d.ID, &d.Name, &d.Method, &d.URL, &d.Tags, &d.ModuleID, &d.Headers, &d.Body, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{Success: false, Error: "接口定义不存在"})
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: d})
}

func CreateAPIDefinition(c *gin.Context) {
	var d models.APIDefinition
	if err := c.ShouldBindJSON(&d); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	d.ID = generateID("ad")
	d.CreatedAt = models.NowStr()
	d.UpdatedAt = d.CreatedAt
	_, err := database.DB.Exec(
		"INSERT INTO api_definitions (id, name, method, url, tags, module_id, headers, body, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)",
		d.ID, d.Name, d.Method, d.URL, d.Tags, d.ModuleID, d.Headers, d.Body, d.CreatedAt, d.UpdatedAt,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, models.APIResponse{Success: true, Data: d})
}

func UpdateAPIDefinition(c *gin.Context) {
	id := c.Param("id")
	var d models.APIDefinition
	if err := c.ShouldBindJSON(&d); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	d.UpdatedAt = models.NowStr()
	_, err := database.DB.Exec(
		"UPDATE api_definitions SET name=?, method=?, url=?, tags=?, module_id=?, headers=?, body=?, updated_at=? WHERE id=?",
		d.Name, d.Method, d.URL, d.Tags, d.ModuleID, d.Headers, d.Body, d.UpdatedAt, id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	d.ID = id
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: d})
}

func DeleteAPIDefinition(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec("DELETE FROM api_definitions WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: nil})
}

// --- 接口定义模块 ---

func GetAPIDefModules(c *gin.Context) {
	rows, err := database.DB.Query("SELECT id, name, parent_id, sort_order, created_at FROM api_def_modules ORDER BY sort_order")
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()
	mods := make([]models.APIDefModule, 0)
	for rows.Next() {
		var m models.APIDefModule
		if err := rows.Scan(&m.ID, &m.Name, &m.ParentID, &m.SortOrder, &m.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
			return
		}
		mods = append(mods, m)
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: mods})
}

func CreateAPIDefModule(c *gin.Context) {
	var m models.APIDefModule
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	m.ID = generateID("adm")
	m.CreatedAt = models.NowStr()
	_, err := database.DB.Exec("INSERT INTO api_def_modules (id, name, parent_id, sort_order, created_at) VALUES (?,?,?,?,?)",
		m.ID, m.Name, m.ParentID, m.SortOrder, m.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, models.APIResponse{Success: true, Data: m})
}

func UpdateAPIDefModule(c *gin.Context) {
	id := c.Param("id")
	var m models.APIDefModule
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	_, err := database.DB.Exec("UPDATE api_def_modules SET name=?, parent_id=?, sort_order=? WHERE id=?", m.Name, m.ParentID, m.SortOrder, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	m.ID = id
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: m})
}

func DeleteAPIDefModule(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec("DELETE FROM api_def_modules WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: nil})
}
