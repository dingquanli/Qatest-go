package repository

import (
	"qatest/database"
	"qatest/models"
)

// 注：旧「表格视图用例」table_cases 的 CRUD 已随 API 一并移除（前端无调用）；
// 数据库表保留。如需恢复，SQL 可参考 git 历史。

// —— XMind 视图用例（SQL 迁自 handlers/table_xmind.go，语句原样保留） ——

// ListXmindCases XMind 用例列表
func ListXmindCases() ([]models.XmindCase, error) {
	rows, err := database.DB.Query(
		"SELECT id, name, module_id, parent_id, collapsed, priority, type, precondition, steps, expected, assignee, status, tags, code, test_data, actual_result, defect_id, remark, env, estimate, sort_order, created_at, updated_at FROM xmind_cases ORDER BY sort_order LIMIT 500",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cases := make([]models.XmindCase, 0)
	for rows.Next() {
		var x models.XmindCase
		if err := rows.Scan(&x.ID, &x.Name, &x.ModuleID, &x.ParentID, &x.Collapsed, &x.Priority, &x.Type, &x.Precondition, &x.Steps, &x.Expected, &x.Assignee, &x.Status, &x.Tags, &x.Code, &x.TestData, &x.ActualResult, &x.DefectId, &x.Remark, &x.Env, &x.Estimate, &x.SortOrder, &x.CreatedAt, &x.UpdatedAt); err != nil {
			return nil, err
		}
		cases = append(cases, x)
	}
	return cases, rows.Err()
}

// CreateXmindCase 插入 XMind 用例（ID/时间戳由调用方填充）
func CreateXmindCase(x models.XmindCase) error {
	_, err := database.DB.Exec(
		"INSERT INTO xmind_cases (id, name, module_id, parent_id, collapsed, priority, type, precondition, steps, expected, assignee, status, tags, code, test_data, actual_result, defect_id, remark, env, estimate, sort_order, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
		x.ID, x.Name, x.ModuleID, x.ParentID, x.Collapsed, x.Priority, x.Type, x.Precondition, x.Steps, x.Expected, x.Assignee, x.Status, x.Tags, x.Code, x.TestData, x.ActualResult, x.DefectId, x.Remark, x.Env, x.Estimate, x.SortOrder, x.CreatedAt, x.UpdatedAt,
	)
	return err
}

// UpdateXmindCase 更新 XMind 用例
func UpdateXmindCase(id string, x models.XmindCase) error {
	_, err := database.DB.Exec(
		"UPDATE xmind_cases SET name=?, module_id=?, parent_id=?, collapsed=?, priority=?, type=?, precondition=?, steps=?, expected=?, assignee=?, status=?, tags=?, code=?, test_data=?, actual_result=?, defect_id=?, remark=?, env=?, estimate=?, sort_order=?, updated_at=? WHERE id=?",
		x.Name, x.ModuleID, x.ParentID, x.Collapsed, x.Priority, x.Type, x.Precondition, x.Steps, x.Expected, x.Assignee, x.Status, x.Tags, x.Code, x.TestData, x.ActualResult, x.DefectId, x.Remark, x.Env, x.Estimate, x.SortOrder, x.UpdatedAt, id,
	)
	return err
}

// ReplaceXmindCases 整体替换 xmind_cases（迁自 handlers/table_xmind.go ReplaceXmindCases）。
// 用于撤销/重做：先清空再按客户端快照全量写回，保留原 id；任一条失败即整体回滚。
// ID/默认值/时间戳填充由调用方完成，本函数原样承接事务（Begin/DELETE/循环 INSERT/Commit）。
func ReplaceXmindCases(list []models.XmindCase) error {
	tx, err := database.DB.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM xmind_cases"); err != nil {
		tx.Rollback()
		return err
	}
	for _, x := range list {
		if _, err := tx.Exec(
			"INSERT INTO xmind_cases (id, name, module_id, parent_id, collapsed, priority, type, precondition, steps, expected, assignee, status, tags, code, test_data, actual_result, defect_id, remark, env, estimate, sort_order, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
			x.ID, x.Name, x.ModuleID, x.ParentID, x.Collapsed, x.Priority, x.Type, x.Precondition, x.Steps, x.Expected, x.Assignee, x.Status, x.Tags, x.Code, x.TestData, x.ActualResult, x.DefectId, x.Remark, x.Env, x.Estimate, x.SortOrder, x.CreatedAt, x.UpdatedAt,
		); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// DeleteXmindNode 删除节点及其全部子孙节点（级联）。
// 整体迁自 handlers/table_xmind.go 的 deleteXmindNode：先逐层收集子孙 ID，再逐条删除。
func DeleteXmindNode(id string) error {
	toDelete := []string{id}
	for i := 0; i < len(toDelete); i++ {
		rows, err := database.DB.Query("SELECT id FROM xmind_cases WHERE parent_id = ?", toDelete[i])
		if err != nil {
			return err
		}
		for rows.Next() {
			var cid string
			if err := rows.Scan(&cid); err != nil {
				rows.Close()
				return err
			}
			toDelete = append(toDelete, cid)
		}
		rows.Close()
	}
	for _, did := range toDelete {
		if _, err := database.DB.Exec("DELETE FROM xmind_cases WHERE id = ?", did); err != nil {
			return err
		}
	}
	return nil
}

// ClearXmindCasesModuleRef 迁自 handlers/table_xmind.go DeleteXmindModule 的前置 UPDATE：
// 删除模块前，先把该模块下的用例移至「未分类」（module_id 置空），避免用例被级联丢失。
func ClearXmindCasesModuleRef(id string) error {
	_, err := database.DB.Exec("UPDATE xmind_cases SET module_id='' WHERE module_id = ?", id)
	return err
}
