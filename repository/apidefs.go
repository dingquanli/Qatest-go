package repository

import (
	"qatest/database"
	"qatest/models"
)

// —— 接口定义 / API 请求 / 请求历史（SQL 迁自 handlers/api_definitions.go 与 handlers/api_requests.go，语句原样保留） ——

// ListAPIDefinitions 接口定义列表
func ListAPIDefinitions() ([]models.APIDefinition, error) {
	rows, err := database.DB.Query(
		"SELECT id, name, method, url, tags, module_id, headers, body, created_at, updated_at FROM api_definitions ORDER BY updated_at DESC LIMIT 200",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	defs := make([]models.APIDefinition, 0)
	for rows.Next() {
		var d models.APIDefinition
		if err := rows.Scan(&d.ID, &d.Name, &d.Method, &d.URL, &d.Tags, &d.ModuleID, &d.Headers, &d.Body, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		defs = append(defs, d)
	}
	return defs, rows.Err()
}

// GetAPIDefinition 接口定义详情（不存在时返回 sql.ErrNoRows）
func GetAPIDefinition(id string) (models.APIDefinition, error) {
	var d models.APIDefinition
	err := database.DB.QueryRow(
		"SELECT id, name, method, url, tags, module_id, headers, body, created_at, updated_at FROM api_definitions WHERE id = ?", id,
	).Scan(&d.ID, &d.Name, &d.Method, &d.URL, &d.Tags, &d.ModuleID, &d.Headers, &d.Body, &d.CreatedAt, &d.UpdatedAt)
	return d, err
}

// CreateAPIDefinition 插入接口定义（ID/时间戳由调用方填充）
func CreateAPIDefinition(d models.APIDefinition) error {
	_, err := database.DB.Exec(
		"INSERT INTO api_definitions (id, name, method, url, tags, module_id, headers, body, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)",
		d.ID, d.Name, d.Method, d.URL, d.Tags, d.ModuleID, d.Headers, d.Body, d.CreatedAt, d.UpdatedAt,
	)
	return err
}

// UpdateAPIDefinition 更新接口定义
func UpdateAPIDefinition(id string, d models.APIDefinition) error {
	_, err := database.DB.Exec(
		"UPDATE api_definitions SET name=?, method=?, url=?, tags=?, module_id=?, headers=?, body=?, updated_at=? WHERE id=?",
		d.Name, d.Method, d.URL, d.Tags, d.ModuleID, d.Headers, d.Body, d.UpdatedAt, id,
	)
	return err
}

// DeleteAPIDefinition 删除接口定义
func DeleteAPIDefinition(id string) error {
	_, err := database.DB.Exec("DELETE FROM api_definitions WHERE id = ?", id)
	return err
}

// ListAPIRequests API 请求列表
func ListAPIRequests() ([]models.APIRequest, error) {
	rows, err := database.DB.Query(
		"SELECT id, name, method, url, headers, params, body, description, tags, folder_id, created_at, updated_at FROM api_requests ORDER BY updated_at DESC LIMIT 200",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reqs := make([]models.APIRequest, 0)
	for rows.Next() {
		var r models.APIRequest
		if err := rows.Scan(&r.ID, &r.Name, &r.Method, &r.URL, &r.Headers, &r.Params, &r.Body, &r.Description, &r.Tags, &r.FolderID, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		reqs = append(reqs, r)
	}
	return reqs, rows.Err()
}

// GetAPIRequest API 请求详情（不存在时返回 sql.ErrNoRows）
func GetAPIRequest(id string) (models.APIRequest, error) {
	var r models.APIRequest
	err := database.DB.QueryRow(
		"SELECT id, name, method, url, headers, params, body, description, tags, folder_id, created_at, updated_at FROM api_requests WHERE id = ?", id,
	).Scan(&r.ID, &r.Name, &r.Method, &r.URL, &r.Headers, &r.Params, &r.Body, &r.Description, &r.Tags, &r.FolderID, &r.CreatedAt, &r.UpdatedAt)
	return r, err
}

// CreateAPIRequest 插入 API 请求（ID/时间戳由调用方填充）
func CreateAPIRequest(r models.APIRequest) error {
	_, err := database.DB.Exec(
		"INSERT INTO api_requests (id, name, method, url, headers, params, body, description, tags, folder_id, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)",
		r.ID, r.Name, r.Method, r.URL, r.Headers, r.Params, r.Body, r.Description, r.Tags, r.FolderID, r.CreatedAt, r.UpdatedAt,
	)
	return err
}

// UpdateAPIRequest 更新 API 请求
func UpdateAPIRequest(id string, r models.APIRequest) error {
	_, err := database.DB.Exec(
		"UPDATE api_requests SET name=?, method=?, url=?, headers=?, params=?, body=?, description=?, tags=?, folder_id=?, updated_at=? WHERE id=?",
		r.Name, r.Method, r.URL, r.Headers, r.Params, r.Body, r.Description, r.Tags, r.FolderID, r.UpdatedAt, id,
	)
	return err
}

// DeleteAPIRequest 删除 API 请求
func DeleteAPIRequest(id string) error {
	_, err := database.DB.Exec("DELETE FROM api_requests WHERE id = ?", id)
	return err
}

// ListAPIHistory API 请求历史列表
func ListAPIHistory() ([]models.APIHistory, error) {
	rows, err := database.DB.Query(
		"SELECT id, request_id, method, url, response, status_code, duration, created_at FROM api_history ORDER BY created_at DESC LIMIT 100",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hist := make([]models.APIHistory, 0)
	for rows.Next() {
		var h models.APIHistory
		if err := rows.Scan(&h.ID, &h.RequestID, &h.Method, &h.URL, &h.Response, &h.StatusCode, &h.Duration, &h.CreatedAt); err != nil {
			return nil, err
		}
		hist = append(hist, h)
	}
	return hist, rows.Err()
}

// CreateAPIHistory 插入 API 请求历史（ID/时间戳由调用方填充）
func CreateAPIHistory(h models.APIHistory) error {
	_, err := database.DB.Exec(
		"INSERT INTO api_history (id, request_id, method, url, response, status_code, duration, created_at) VALUES (?,?,?,?,?,?,?,?)",
		h.ID, h.RequestID, h.Method, h.URL, h.Response, h.StatusCode, h.Duration, h.CreatedAt,
	)
	return err
}

// ClearAPIHistory 清空 API 请求历史
func ClearAPIHistory() error {
	_, err := database.DB.Exec("DELETE FROM api_history")
	return err
}
