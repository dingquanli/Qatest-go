package handlers

import (
	"net/http"

	"qatest/database"
	"qatest/models"

	"github.com/gin-gonic/gin"
)

// --- API 请求管理 ---

func GetAPIRequests(c *gin.Context) {
	rows, err := database.DB.Query(
		"SELECT id, name, method, url, headers, params, body, description, tags, folder_id, created_at, updated_at FROM api_requests ORDER BY updated_at DESC LIMIT 200",
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()
	reqs := make([]models.APIRequest, 0)
	for rows.Next() {
		var r models.APIRequest
		if err := rows.Scan(&r.ID, &r.Name, &r.Method, &r.URL, &r.Headers, &r.Params, &r.Body, &r.Description, &r.Tags, &r.FolderID, &r.CreatedAt, &r.UpdatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
			return
		}
		reqs = append(reqs, r)
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: reqs})
}

func GetAPIRequest(c *gin.Context) {
	id := c.Param("id")
	var r models.APIRequest
	err := database.DB.QueryRow(
		"SELECT id, name, method, url, headers, params, body, description, tags, folder_id, created_at, updated_at FROM api_requests WHERE id = ?", id,
	).Scan(&r.ID, &r.Name, &r.Method, &r.URL, &r.Headers, &r.Params, &r.Body, &r.Description, &r.Tags, &r.FolderID, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{Success: false, Error: "请求不存在"})
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: r})
}

func CreateAPIRequest(c *gin.Context) {
	var r models.APIRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	r.ID = generateID("ar")
	r.CreatedAt = models.NowStr()
	r.UpdatedAt = r.CreatedAt
	_, err := database.DB.Exec(
		"INSERT INTO api_requests (id, name, method, url, headers, params, body, description, tags, folder_id, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)",
		r.ID, r.Name, r.Method, r.URL, r.Headers, r.Params, r.Body, r.Description, r.Tags, r.FolderID, r.CreatedAt, r.UpdatedAt,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, models.APIResponse{Success: true, Data: r})
}

func UpdateAPIRequest(c *gin.Context) {
	id := c.Param("id")
	var r models.APIRequest
	if err := c.ShouldBindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	r.UpdatedAt = models.NowStr()
	_, err := database.DB.Exec(
		"UPDATE api_requests SET name=?, method=?, url=?, headers=?, params=?, body=?, description=?, tags=?, folder_id=?, updated_at=? WHERE id=?",
		r.Name, r.Method, r.URL, r.Headers, r.Params, r.Body, r.Description, r.Tags, r.FolderID, r.UpdatedAt, id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	r.ID = id
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: r})
}

func DeleteAPIRequest(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec("DELETE FROM api_requests WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: nil})
}

// --- API 文件夹 ---

func GetAPIFolders(c *gin.Context) {
	rows, err := database.DB.Query("SELECT id, name, parent_id, sort_order, created_at FROM api_folders ORDER BY sort_order")
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()
	folders := make([]models.APIFolder, 0)
	for rows.Next() {
		var f models.APIFolder
		if err := rows.Scan(&f.ID, &f.Name, &f.ParentID, &f.SortOrder, &f.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
			return
		}
		folders = append(folders, f)
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: folders})
}

func CreateAPIFolder(c *gin.Context) {
	var f models.APIFolder
	if err := c.ShouldBindJSON(&f); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	f.ID = generateID("af")
	f.CreatedAt = models.NowStr()
	_, err := database.DB.Exec("INSERT INTO api_folders (id, name, parent_id, sort_order, created_at) VALUES (?,?,?,?,?)",
		f.ID, f.Name, f.ParentID, f.SortOrder, f.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, models.APIResponse{Success: true, Data: f})
}

func UpdateAPIFolder(c *gin.Context) {
	id := c.Param("id")
	var f models.APIFolder
	if err := c.ShouldBindJSON(&f); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	_, err := database.DB.Exec("UPDATE api_folders SET name=?, parent_id=?, sort_order=? WHERE id=?", f.Name, f.ParentID, f.SortOrder, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	f.ID = id
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: f})
}

func DeleteAPIFolder(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec("DELETE FROM api_folders WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: nil})
}

// --- API 请求历史 ---

func GetAPIHistory(c *gin.Context) {
	rows, err := database.DB.Query(
		"SELECT id, request_id, method, url, response, status_code, duration, created_at FROM api_history ORDER BY created_at DESC LIMIT 100",
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()
	hist := make([]models.APIHistory, 0)
	for rows.Next() {
		var h models.APIHistory
		if err := rows.Scan(&h.ID, &h.RequestID, &h.Method, &h.URL, &h.Response, &h.StatusCode, &h.Duration, &h.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
			return
		}
		hist = append(hist, h)
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: hist})
}

func CreateAPIHistory(c *gin.Context) {
	var h models.APIHistory
	if err := c.ShouldBindJSON(&h); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	h.ID = generateID("ah")
	h.CreatedAt = models.NowStr()
	_, err := database.DB.Exec(
		"INSERT INTO api_history (id, request_id, method, url, response, status_code, duration, created_at) VALUES (?,?,?,?,?,?,?,?)",
		h.ID, h.RequestID, h.Method, h.URL, h.Response, h.StatusCode, h.Duration, h.CreatedAt,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, models.APIResponse{Success: true, Data: h})
}

func ClearAPIHistory(c *gin.Context) {
	_, err := database.DB.Exec("DELETE FROM api_history")
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: nil})
}
