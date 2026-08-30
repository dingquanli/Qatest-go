package repository

import (
	"encoding/json"

	"qatest/database"
	"qatest/models"
)

// —— 数据迁移（SQL 迁自 handlers/migration.go，事务体原样保留，不做改写） ——

// TableExists 检查数据表是否存在（SQL 迁自 handlers/migration.go GetMigrationStatus）。
// 返回原始 Scan error：sql.ErrNoRows 表示表不存在，其余错误原样上抛，归类由 handler 决定。
func TableExists(name string) error {
	var tableName string
	return database.DB.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name=?", name,
	).Scan(&tableName)
}

// ImportMigrationData 事务化导入迁移数据（整体迁自 handlers/migration.go ImportMigration 的
// database.DB.Begin() 事务块：INSERT OR REPLACE 语句、计数与回滚逻辑逐字保留）。
// 返回成功/失败条数；err 仅在开启事务失败时非空。
func ImportMigrationData(req map[string]json.RawMessage) (imported int, failed int, err error) {
	tx, err := database.DB.Begin()
	if err != nil {
		return 0, 0, err
	}

	hasErr := false
	run := func(query string, args ...interface{}) {
		if hasErr {
			return
		}
		if _, e := tx.Exec(query, args...); e != nil {
			failed++
			hasErr = true
		} else {
			imported++
		}
	}

	// 导入脚本
	if scripts, ok := req["scripts"]; ok {
		var list []models.Script
		if json.Unmarshal(scripts, &list) == nil {
			for _, s := range list {
				run(
					"INSERT OR REPLACE INTO scripts (id, name, description, language, code, created_at, updated_at) VALUES (?,?,?,?,?,?,?)",
					s.ID, s.Name, s.Description, s.Language, s.Code, s.CreatedAt, s.UpdatedAt,
				)
			}
		} else {
			// L3：解析失败计入 failed，避免静默跳过造成计数偏差
			failed++
		}
	}

	// 导入测试用例
	if cases, ok := req["test_cases"]; ok {
		var list []models.TestCase
		if json.Unmarshal(cases, &list) == nil {
			for _, tc := range list {
				run(
					"INSERT OR REPLACE INTO test_cases (id, name, module_id, priority, type, precondition, steps, assignee, status, tags, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)",
					tc.ID, tc.Name, tc.ModuleID, tc.Priority, tc.Type, tc.Precondition, tc.Steps, tc.Assignee, tc.Status, tc.Tags, tc.CreatedAt, tc.UpdatedAt,
				)
			}
		} else {
			// L3：解析失败计入 failed，避免静默跳过造成计数偏差
			failed++
		}
	}

	// 导入缺陷
	if bugs, ok := req["bugs"]; ok {
		var list []models.Bug
		if json.Unmarshal(bugs, &list) == nil {
			for _, b := range list {
				run(
					`INSERT OR REPLACE INTO bugs (id, title, severity, priority, status, assignee, reporter, module, env, description, steps, expected, actual, tags, related_case_id, external_id, external_url, created_at, updated_at)
					 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
					b.ID, b.Title, b.Severity, b.Priority, b.Status, b.Assignee, b.Reporter, b.Module, b.Env,
					b.Description, b.Steps, b.Expected, b.Actual, b.Tags, b.RelatedCaseID, b.ExternalID, b.ExternalURL, b.CreatedAt, b.UpdatedAt,
				)
			}
		} else {
			// L3：解析失败计入 failed，避免静默跳过造成计数偏差
			failed++
		}
	}

	// 导入接口定义
	if apiDefs, ok := req["api_definitions"]; ok {
		var list []models.APIDefinition
		if json.Unmarshal(apiDefs, &list) == nil {
			for _, d := range list {
				run(
					"INSERT OR REPLACE INTO api_definitions (id, name, method, url, module_id, headers, body, tags, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)",
					d.ID, d.Name, d.Method, d.URL, d.ModuleID, d.Headers, d.Body, d.Tags, d.CreatedAt, d.UpdatedAt,
				)
			}
		} else {
			// L3：解析失败计入 failed，避免静默跳过造成计数偏差
			failed++
		}
	}

	// 导入设置
	if settings, ok := req["settings"]; ok {
		var m map[string]string
		if json.Unmarshal(settings, &m) == nil {
			for k, v := range m {
				run("INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)", k, v)
			}
		} else {
			// L3：解析失败计入 failed，避免静默跳过造成计数偏差
			failed++
		}
	}

	if hasErr {
		tx.Rollback()
	} else {
		tx.Commit()
	}

	return imported, failed, nil
}
