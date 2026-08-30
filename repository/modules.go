package repository

import (
	"qatest/database"
	"qatest/models"
)

// —— 模块/文件夹 CRUD（SQL 迁自 handlers/module_crud.go，语句原样保留） ——
//
// 四张模块表（case_modules / api_def_modules / api_folders / xmind_modules；
// table_modules 已随 /table-modules API 移除，表保留）
// 结构同构（id/name/parent_id/sort_order/created_at），共用以下函数。
// table 参数由 handlers 以包内固定常量传入（编译期固定，非用户输入），杜绝拼接注入。

// ListModuleRows 按 sort_order 返回全量模块
func ListModuleRows(table string) ([]models.ModuleNode, error) {
	rows, err := database.DB.Query(
		"SELECT id, name, parent_id, sort_order, created_at FROM " + table + " ORDER BY sort_order",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	mods := make([]models.ModuleNode, 0)
	for rows.Next() {
		var m models.ModuleNode
		if err := rows.Scan(&m.ID, &m.Name, &m.ParentID, &m.SortOrder, &m.CreatedAt); err != nil {
			return nil, err
		}
		mods = append(mods, m)
	}
	return mods, rows.Err()
}

// InsertModuleRow 插入模块（ID/时间戳由调用方填充）
func InsertModuleRow(table string, m models.ModuleNode) error {
	_, err := database.DB.Exec(
		"INSERT INTO "+table+" (id, name, parent_id, sort_order, created_at) VALUES (?,?,?,?,?)",
		m.ID, m.Name, m.ParentID, m.SortOrder, m.CreatedAt,
	)
	return err
}

// UpdateModuleRow 按 id 更新模块
func UpdateModuleRow(table string, id string, m models.ModuleNode) error {
	_, err := database.DB.Exec(
		"UPDATE "+table+" SET name=?, parent_id=?, sort_order=? WHERE id=?",
		m.Name, m.ParentID, m.SortOrder, id,
	)
	return err
}

// DeleteModuleRow 按 id 删除模块
func DeleteModuleRow(table string, id string) error {
	_, err := database.DB.Exec("DELETE FROM "+table+" WHERE id = ?", id)
	return err
}
