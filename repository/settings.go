package repository

import (
	"qatest/database"
)

// —— 系统设置（SQL 迁自 handlers/settings.go、handlers/bugs.go，语句原样保留） ——

// GetAllSettings 全部设置（按 key 排序）
func GetAllSettings() (map[string]string, error) {
	rows, err := database.DB.Query("SELECT key, value FROM settings ORDER BY key")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		settings[k] = v
	}

	return settings, nil
}

// GetSetting 单项设置（不存在时返回 sql.ErrNoRows）
func GetSetting(key string) (string, error) {
	var value string
	err := database.DB.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	return value, err
}

// UpsertSetting 插入或更新设置项
func UpsertSetting(key, value string) error {
	_, err := database.DB.Exec("INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?", key, value, value)
	return err
}
