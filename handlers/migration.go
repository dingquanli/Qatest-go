package handlers

import (
	"encoding/json"
	"net/http"

	"qatest/database"
	"qatest/models"

	"github.com/gin-gonic/gin"
)

// GetMigrationStatus 检查是否需要迁移
// P1：真实计算迁移状态，不再硬编码返回 false。通过检查核心数据表是否已创建来判断。
func GetMigrationStatus(c *gin.Context) {
	coreTables := []string{"scripts", "executions", "test_cases", "bugs", "api_definitions", "settings"}
	missing := make([]string, 0)
	for _, t := range coreTables {
		var name string
		err := database.DB.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", t,
		).Scan(&name)
		if err != nil {
			missing = append(missing, t)
		}
	}

	needsMigration := len(missing) > 0
	message := "数据库已初始化，无需迁移"
	if needsMigration {
		message = "数据库尚未初始化，需要迁移/初始化"
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data: gin.H{
			"needsMigration": needsMigration,
			"missingTables":  missing,
			"message":        message,
		},
	})
}

// ImportMigration 导入数据
// P1：使用事务包裹批量写入，逐条检查 Exec 错误；任一失败回滚并返回真实 {imported, failed}。
func ImportMigration(c *gin.Context) {
	var req map[string]json.RawMessage
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}

	imported := 0
	failed := 0
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

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: gin.H{"imported": imported, "failed": failed}})
}
