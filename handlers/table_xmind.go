package handlers

import (
	"net/http"

	"qatest/database"
	"qatest/models"

	"github.com/gin-gonic/gin"
)

// --- 表格视图用例 ---

func GetTableCases(c *gin.Context) {
	rows, err := database.DB.Query(
		"SELECT id, name, module_id, priority, type, precondition, steps, expected, assignee, status, tags, sort_order, created_at, updated_at FROM table_cases ORDER BY sort_order LIMIT 500",
	)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	defer rows.Close()
	cases := make([]models.TableCase, 0)
	for rows.Next() {
		var e models.TableCase
		if err := rows.Scan(&e.ID, &e.Name, &e.ModuleID, &e.Priority, &e.Type, &e.Precondition, &e.Steps, &e.Expected, &e.Assignee, &e.Status, &e.Tags, &e.SortOrder, &e.CreatedAt, &e.UpdatedAt); err != nil {
			respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
			return
		}
		cases = append(cases, e)
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: cases})
}

func CreateTableCase(c *gin.Context) {
	var e models.TableCase
	if err := c.ShouldBindJSON(&e); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	e.ID = generateID("ec")
	e.CreatedAt = models.NowStr()
	e.UpdatedAt = e.CreatedAt
	if e.Priority == "" {
		e.Priority = "P2"
	}
	if e.Type == "" {
		e.Type = "functional"
	}
	if e.Status == "" {
		e.Status = "draft"
	}
	_, err := database.DB.Exec(
		"INSERT INTO table_cases (id, name, module_id, priority, type, precondition, steps, expected, assignee, status, tags, sort_order, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
		e.ID, e.Name, e.ModuleID, e.Priority, e.Type, e.Precondition, e.Steps, e.Expected, e.Assignee, e.Status, e.Tags, e.SortOrder, e.CreatedAt, e.UpdatedAt,
	)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusCreated, models.APIResponse{Success: true, Data: e})
}

func UpdateTableCase(c *gin.Context) {
	id := c.Param("id")
	var e models.TableCase
	if err := c.ShouldBindJSON(&e); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	e.UpdatedAt = models.NowStr()
	if e.Priority == "" {
		e.Priority = "P2"
	}
	if e.Type == "" {
		e.Type = "functional"
	}
	if e.Status == "" {
		e.Status = "draft"
	}
	_, err := database.DB.Exec(
		"UPDATE table_cases SET name=?, module_id=?, priority=?, type=?, precondition=?, steps=?, expected=?, assignee=?, status=?, tags=?, sort_order=?, updated_at=? WHERE id=?",
		e.Name, e.ModuleID, e.Priority, e.Type, e.Precondition, e.Steps, e.Expected, e.Assignee, e.Status, e.Tags, e.SortOrder, e.UpdatedAt, id,
	)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	e.ID = id
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: e})
}

func DeleteTableCase(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec("DELETE FROM table_cases WHERE id = ?", id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: nil})
}

// --- 表格视图模块 ---

func GetTableModules(c *gin.Context) {
	rows, err := database.DB.Query("SELECT id, name, parent_id, sort_order, created_at FROM table_modules ORDER BY sort_order")
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	defer rows.Close()
	mods := make([]models.TableModule, 0)
	for rows.Next() {
		var m models.TableModule
		if err := rows.Scan(&m.ID, &m.Name, &m.ParentID, &m.SortOrder, &m.CreatedAt); err != nil {
			respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
			return
		}
		mods = append(mods, m)
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: mods})
}

func CreateTableModule(c *gin.Context) {
	var m models.TableModule
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	m.ID = generateID("em")
	m.CreatedAt = models.NowStr()
	_, err := database.DB.Exec("INSERT INTO table_modules (id, name, parent_id, sort_order, created_at) VALUES (?,?,?,?,?)",
		m.ID, m.Name, m.ParentID, m.SortOrder, m.CreatedAt)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusCreated, models.APIResponse{Success: true, Data: m})
}

func UpdateTableModule(c *gin.Context) {
	id := c.Param("id")
	var m models.TableModule
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	_, err := database.DB.Exec("UPDATE table_modules SET name=?, parent_id=?, sort_order=? WHERE id=?", m.Name, m.ParentID, m.SortOrder, id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	m.ID = id
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: m})
}

func DeleteTableModule(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec("DELETE FROM table_modules WHERE id = ?", id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: nil})
}

// --- XMind 视图用例 ---

func GetXmindCases(c *gin.Context) {
	rows, err := database.DB.Query(
		"SELECT id, name, module_id, priority, type, precondition, steps, expected, assignee, status, tags, sort_order, created_at, updated_at FROM xmind_cases ORDER BY sort_order LIMIT 500",
	)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	defer rows.Close()
	cases := make([]models.XmindCase, 0)
	for rows.Next() {
		var x models.XmindCase
		if err := rows.Scan(&x.ID, &x.Name, &x.ModuleID, &x.Priority, &x.Type, &x.Precondition, &x.Steps, &x.Expected, &x.Assignee, &x.Status, &x.Tags, &x.SortOrder, &x.CreatedAt, &x.UpdatedAt); err != nil {
			respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
			return
		}
		cases = append(cases, x)
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: cases})
}

func CreateXmindCase(c *gin.Context) {
	var x models.XmindCase
	if err := c.ShouldBindJSON(&x); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	x.ID = generateID("xc")
	x.CreatedAt = models.NowStr()
	x.UpdatedAt = x.CreatedAt
	if x.Priority == "" {
		x.Priority = "P2"
	}
	if x.Type == "" {
		x.Type = "functional"
	}
	if x.Status == "" {
		x.Status = "draft"
	}
	_, err := database.DB.Exec(
		"INSERT INTO xmind_cases (id, name, module_id, priority, type, precondition, steps, expected, assignee, status, tags, sort_order, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
		x.ID, x.Name, x.ModuleID, x.Priority, x.Type, x.Precondition, x.Steps, x.Expected, x.Assignee, x.Status, x.Tags, x.SortOrder, x.CreatedAt, x.UpdatedAt,
	)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusCreated, models.APIResponse{Success: true, Data: x})
}

func UpdateXmindCase(c *gin.Context) {
	id := c.Param("id")
	var x models.XmindCase
	if err := c.ShouldBindJSON(&x); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	x.UpdatedAt = models.NowStr()
	if x.Priority == "" {
		x.Priority = "P2"
	}
	if x.Type == "" {
		x.Type = "functional"
	}
	if x.Status == "" {
		x.Status = "draft"
	}
	_, err := database.DB.Exec(
		"UPDATE xmind_cases SET name=?, module_id=?, priority=?, type=?, precondition=?, steps=?, expected=?, assignee=?, status=?, tags=?, sort_order=?, updated_at=? WHERE id=?",
		x.Name, x.ModuleID, x.Priority, x.Type, x.Precondition, x.Steps, x.Expected, x.Assignee, x.Status, x.Tags, x.SortOrder, x.UpdatedAt, id,
	)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	x.ID = id
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: x})
}

func DeleteXmindCase(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec("DELETE FROM xmind_cases WHERE id = ?", id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: nil})
}

// --- XMind 视图模块 ---

func GetXmindModules(c *gin.Context) {
	rows, err := database.DB.Query("SELECT id, name, parent_id, sort_order, created_at FROM xmind_modules ORDER BY sort_order")
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	defer rows.Close()
	mods := make([]models.XmindModule, 0)
	for rows.Next() {
		var m models.XmindModule
		if err := rows.Scan(&m.ID, &m.Name, &m.ParentID, &m.SortOrder, &m.CreatedAt); err != nil {
			respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
			return
		}
		mods = append(mods, m)
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: mods})
}

func CreateXmindModule(c *gin.Context) {
	var m models.XmindModule
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	m.ID = generateID("xm")
	m.CreatedAt = models.NowStr()
	_, err := database.DB.Exec("INSERT INTO xmind_modules (id, name, parent_id, sort_order, created_at) VALUES (?,?,?,?,?)",
		m.ID, m.Name, m.ParentID, m.SortOrder, m.CreatedAt)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusCreated, models.APIResponse{Success: true, Data: m})
}

func UpdateXmindModule(c *gin.Context) {
	id := c.Param("id")
	var m models.XmindModule
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	_, err := database.DB.Exec("UPDATE xmind_modules SET name=?, parent_id=?, sort_order=? WHERE id=?", m.Name, m.ParentID, m.SortOrder, id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	m.ID = id
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: m})
}

func DeleteXmindModule(c *gin.Context) {
	id := c.Param("id")
	// 删除模块前，先把该模块下的用例移至「未分类」(module_id='')，避免用例被级联丢失
	if _, err := database.DB.Exec("UPDATE xmind_cases SET module_id='' WHERE module_id = ?", id); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	if _, err := database.DB.Exec("DELETE FROM xmind_modules WHERE id = ?", id); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: nil})
}
