package repository

import (
	"qatest/database"
	"qatest/models"
)

// —— 脚本管理（SQL 迁自 handlers/scripts.go，语句原样保留） ——

// ListScripts 脚本列表
func ListScripts() ([]models.Script, error) {
	rows, err := database.DB.Query("SELECT id, name, description, language, code, created_at, updated_at FROM scripts ORDER BY updated_at DESC LIMIT 500")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	scripts := make([]models.Script, 0)
	for rows.Next() {
		var s models.Script
		if err := rows.Scan(&s.ID, &s.Name, &s.Description, &s.Language, &s.Code, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		scripts = append(scripts, s)
	}
	return scripts, rows.Err()
}

// GetScript 脚本详情（不存在时返回 sql.ErrNoRows）
func GetScript(id string) (models.Script, error) {
	var s models.Script
	err := database.DB.QueryRow(
		"SELECT id, name, description, language, code, created_at, updated_at FROM scripts WHERE id = ?",
		id,
	).Scan(&s.ID, &s.Name, &s.Description, &s.Language, &s.Code, &s.CreatedAt, &s.UpdatedAt)
	return s, err
}

// CreateScript 插入脚本（ID/时间戳由调用方填充）
func CreateScript(s models.Script) error {
	_, err := database.DB.Exec(
		"INSERT INTO scripts (id, name, description, language, code, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		s.ID, s.Name, s.Description, s.Language, s.Code, s.CreatedAt, s.UpdatedAt,
	)
	return err
}

// UpdateScript 更新脚本
func UpdateScript(id string, s models.Script) error {
	_, err := database.DB.Exec(
		"UPDATE scripts SET name=?, description=?, language=?, code=?, updated_at=? WHERE id=?",
		s.Name, s.Description, s.Language, s.Code, s.UpdatedAt, id,
	)
	return err
}

// DeleteScript 删除脚本
func DeleteScript(id string) error {
	_, err := database.DB.Exec("DELETE FROM scripts WHERE id = ?", id)
	return err
}
