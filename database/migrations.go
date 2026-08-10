package database

import (
	"database/sql"
	"fmt"
	"log"
)

// RunMigrations 执行数据库迁移（建表）
func RunMigrations() error {
	tables := []string{
		// 1. 脚本管理
		`CREATE TABLE IF NOT EXISTS scripts (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			language TEXT NOT NULL DEFAULT 'python',
			code TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT ''
		)`,

		// 2. 脚本执行记录
		`CREATE TABLE IF NOT EXISTS executions (
			id TEXT PRIMARY KEY,
			script_id TEXT NOT NULL DEFAULT '',
			device_serial TEXT NOT NULL DEFAULT '',
			task_name TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending',
			logs TEXT NOT NULL DEFAULT '[]',
			screenshots TEXT NOT NULL DEFAULT '[]',
			duration INTEGER NOT NULL DEFAULT 0,
			started_at TEXT NOT NULL DEFAULT '',
			finished_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT ''
		)`,

		// 3. 测试用例
		`CREATE TABLE IF NOT EXISTS test_cases (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			module_id TEXT NOT NULL DEFAULT '',
			priority TEXT NOT NULL DEFAULT 'P2',
			type TEXT NOT NULL DEFAULT 'functional',
			precondition TEXT NOT NULL DEFAULT '',
			steps TEXT NOT NULL DEFAULT '[]',
			assignee TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'draft',
			tags TEXT NOT NULL DEFAULT '[]',
			created_at TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT ''
		)`,

		// 4. 用例模块
		`CREATE TABLE IF NOT EXISTS case_modules (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			parent_id TEXT NOT NULL DEFAULT '',
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT ''
		)`,

		// 5. 用例执行记录
		`CREATE TABLE IF NOT EXISTS case_executions (
			id TEXT PRIMARY KEY,
			case_id TEXT NOT NULL DEFAULT '',
			case_name TEXT NOT NULL DEFAULT '',
			executor TEXT NOT NULL DEFAULT '',
			result TEXT NOT NULL DEFAULT 'pending',
			steps TEXT NOT NULL DEFAULT '[]',
			duration INTEGER NOT NULL DEFAULT 0,
			remark TEXT NOT NULL DEFAULT '',
			executed_at TEXT NOT NULL DEFAULT ''
		)`,

		// 6. 缺陷管理
		`CREATE TABLE IF NOT EXISTS bugs (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL DEFAULT '',
			severity TEXT NOT NULL DEFAULT 'medium',
			priority TEXT NOT NULL DEFAULT 'P2',
			status TEXT NOT NULL DEFAULT 'open',
			assignee TEXT NOT NULL DEFAULT '',
			reporter TEXT NOT NULL DEFAULT '',
			module TEXT NOT NULL DEFAULT '',
			env TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			steps TEXT NOT NULL DEFAULT '',
			expected TEXT NOT NULL DEFAULT '',
			actual TEXT NOT NULL DEFAULT '',
			tags TEXT NOT NULL DEFAULT '[]',
			related_case_id TEXT NOT NULL DEFAULT '',
			external_id TEXT NOT NULL DEFAULT '',
			external_url TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT ''
		)`,

		// 7. 测试计划
		`CREATE TABLE IF NOT EXISTS test_plans (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			case_ids TEXT NOT NULL DEFAULT '[]',
			status TEXT NOT NULL DEFAULT 'draft',
			start_date TEXT NOT NULL DEFAULT '',
			end_date TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT ''
		)`,

		// 8. 接口定义
		`CREATE TABLE IF NOT EXISTS api_definitions (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			method TEXT NOT NULL DEFAULT 'GET',
			url TEXT NOT NULL DEFAULT '',
			tags TEXT NOT NULL DEFAULT '[]',
			module_id TEXT NOT NULL DEFAULT '',
			headers TEXT NOT NULL DEFAULT '[]',
			body TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT ''
		)`,

		// 9. 接口定义模块
		`CREATE TABLE IF NOT EXISTS api_def_modules (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			parent_id TEXT NOT NULL DEFAULT '',
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT ''
		)`,

		// 10. API 请求集合
		`CREATE TABLE IF NOT EXISTS api_requests (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			method TEXT NOT NULL DEFAULT 'GET',
			url TEXT NOT NULL DEFAULT '',
			headers TEXT NOT NULL DEFAULT '[]',
			params TEXT NOT NULL DEFAULT '[]',
			body TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			tags TEXT NOT NULL DEFAULT '[]',
			folder_id TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT ''
		)`,

		// 11. API 文件夹
		`CREATE TABLE IF NOT EXISTS api_folders (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			parent_id TEXT NOT NULL DEFAULT '',
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT ''
		)`,

		// 12. API 请求历史
		`CREATE TABLE IF NOT EXISTS api_history (
			id TEXT PRIMARY KEY,
			request_id TEXT NOT NULL DEFAULT '',
			method TEXT NOT NULL DEFAULT '',
			url TEXT NOT NULL DEFAULT '',
			response TEXT NOT NULL DEFAULT '',
			status_code INTEGER NOT NULL DEFAULT 0,
			duration INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT ''
		)`,

		// 13. 自动化任务执行记录
		`CREATE TABLE IF NOT EXISTS auto_task_executions (
			id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL DEFAULT '',
			task_name TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending',
			result TEXT NOT NULL DEFAULT '',
			logs TEXT NOT NULL DEFAULT '[]',
			started_at TEXT NOT NULL DEFAULT '',
			finished_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT ''
		)`,

		// 14. 表格视图用例
		`CREATE TABLE IF NOT EXISTS table_cases (
			id TEXT PRIMARY KEY,
			module_id TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',
			priority TEXT NOT NULL DEFAULT 'P2',
			type TEXT NOT NULL DEFAULT 'functional',
			precondition TEXT NOT NULL DEFAULT '',
			steps TEXT NOT NULL DEFAULT '',
			expected TEXT NOT NULL DEFAULT '',
			assignee TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'draft',
			tags TEXT NOT NULL DEFAULT '[]',
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT ''
		)`,

		// 15. 表格视图模块
		`CREATE TABLE IF NOT EXISTS table_modules (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			parent_id TEXT NOT NULL DEFAULT '',
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT ''
		)`,

		// 16. XMind 视图用例（纯文本逻辑图）
		`CREATE TABLE IF NOT EXISTS xmind_cases (
			id TEXT PRIMARY KEY,
			module_id TEXT NOT NULL DEFAULT '',
			parent_id TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',
			collapsed INTEGER NOT NULL DEFAULT 0,
			priority TEXT NOT NULL DEFAULT 'P2',
			type TEXT NOT NULL DEFAULT 'functional',
			precondition TEXT NOT NULL DEFAULT '',
			steps TEXT NOT NULL DEFAULT '[]',
			expected TEXT NOT NULL DEFAULT '',
			assignee TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'draft',
			tags TEXT NOT NULL DEFAULT '',
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT ''
		)`,

		// 17. XMind 视图模块
		`CREATE TABLE IF NOT EXISTS xmind_modules (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			parent_id TEXT NOT NULL DEFAULT '',
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT ''
		)`,

		// 18. 计划执行记录
		`CREATE TABLE IF NOT EXISTS plan_executions (
			id TEXT PRIMARY KEY,
			plan_id TEXT NOT NULL DEFAULT '',
			plan_name TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending',
			result TEXT NOT NULL DEFAULT '{}',
			cases_total INTEGER NOT NULL DEFAULT 0,
			cases_passed INTEGER NOT NULL DEFAULT 0,
			cases_failed INTEGER NOT NULL DEFAULT 0,
			duration INTEGER NOT NULL DEFAULT 0,
			started_at TEXT NOT NULL DEFAULT '',
			finished_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT ''
		)`,

		// 19. 系统设置
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT ''
		)`,

		// 20. SDK 上报数据（各引擎 SDK 通过 POST /api/qa/report 写入）
		`CREATE TABLE IF NOT EXISTS qa_reports (
			id TEXT PRIMARY KEY,
			event TEXT NOT NULL DEFAULT 'case_result',
			name TEXT NOT NULL DEFAULT '',
			result TEXT NOT NULL DEFAULT '',
			message TEXT NOT NULL DEFAULT '',
			tags TEXT NOT NULL DEFAULT '{}',
			token TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT '',
			timestamp INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT ''
		)`,

		// 21. 自由电子表格（纯文本网格，cells 为二维字符串数组 JSON）
		`CREATE TABLE IF NOT EXISTS spreadsheets (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '工作表',
			cells TEXT NOT NULL DEFAULT '[]',
			formats TEXT NOT NULL DEFAULT '{}',
			col_widths TEXT NOT NULL DEFAULT '{}',
			row_heights TEXT NOT NULL DEFAULT '{}',
			merges TEXT NOT NULL DEFAULT '[]',
			created_at TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT ''
		)`,
	}

	for i, ddl := range tables {
		if _, err := DB.Exec(ddl); err != nil {
			return fmt.Errorf("执行第 %d 条 DDL 失败: %w", i+1, err)
		}
	}

	log.Printf("[数据库] 迁移完成，共 %d 张表", len(tables))

	// 为已有数据库补充缺失字段（CREATE TABLE IF NOT EXISTS 不会给旧表加列）
	if err := RunColumnMigrations(); err != nil {
		return err
	}

	return nil
}

// hasColumn 判断表中是否存在某列（表名使用常量拼接，无注入风险）
func hasColumn(table, col string) (bool, error) {
	rows, err := DB.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == col {
			return true, nil
		}
	}
	return false, nil
}

// RunColumnMigrations 为已有的 table_cases / xmind_cases 表补充缺失的字段，
// 确保前端发送的 priority/type/precondition/assignee/status 不会被静默丢弃。
func RunColumnMigrations() error {
	cols := []struct {
		table string
		col   string
		def   string
	}{
		{"table_cases", "priority", "TEXT NOT NULL DEFAULT 'P2'"},
		{"table_cases", "type", "TEXT NOT NULL DEFAULT 'functional'"},
		{"table_cases", "precondition", "TEXT NOT NULL DEFAULT ''"},
		{"table_cases", "assignee", "TEXT NOT NULL DEFAULT ''"},
		{"table_cases", "status", "TEXT NOT NULL DEFAULT 'draft'"},
		{"xmind_cases", "parent_id", "TEXT NOT NULL DEFAULT ''"},
		{"xmind_cases", "collapsed", "INTEGER NOT NULL DEFAULT 0"},
		{"xmind_cases", "priority", "TEXT NOT NULL DEFAULT 'P2'"},
		{"xmind_cases", "type", "TEXT NOT NULL DEFAULT 'functional'"},
		{"xmind_cases", "precondition", "TEXT NOT NULL DEFAULT ''"},
		{"xmind_cases", "assignee", "TEXT NOT NULL DEFAULT ''"},
		{"xmind_cases", "status", "TEXT NOT NULL DEFAULT 'draft'"},
		// XMind 视图用例：完整版补齐行业标准业务字段
		{"xmind_cases", "code", "TEXT NOT NULL DEFAULT ''"},
		{"xmind_cases", "test_data", "TEXT NOT NULL DEFAULT ''"},
		{"xmind_cases", "actual_result", "TEXT NOT NULL DEFAULT ''"},
		{"xmind_cases", "defect_id", "TEXT NOT NULL DEFAULT ''"},
		{"xmind_cases", "remark", "TEXT NOT NULL DEFAULT ''"},
		{"xmind_cases", "env", "TEXT NOT NULL DEFAULT ''"},
		{"xmind_cases", "estimate", "TEXT NOT NULL DEFAULT ''"},
		// 计划执行记录：补充执行人 / 完成时间 / 逐用例明细
		{"plan_executions", "executed_by", "TEXT NOT NULL DEFAULT ''"},
		{"plan_executions", "finished_at", "TEXT NOT NULL DEFAULT ''"},
		{"plan_executions", "cases_detail", "TEXT NOT NULL DEFAULT '[]'"},
		// SDK 上报（qa_reports）：补齐 gRPC / API 拦截事件协议字段
		// 对应 FileApiLogger.cs 的 REQUEST / RESPONSE / ERROR 三类事件
		{"qa_reports", "seq", "INTEGER NOT NULL DEFAULT 0"},
		{"qa_reports", "method", "TEXT NOT NULL DEFAULT ''"},
		{"qa_reports", "headers", "TEXT NOT NULL DEFAULT ''"},
		{"qa_reports", "req_body", "TEXT NOT NULL DEFAULT ''"},
		{"qa_reports", "resp_body", "TEXT NOT NULL DEFAULT ''"},
		{"qa_reports", "err_msg", "TEXT NOT NULL DEFAULT ''"},
		{"qa_reports", "elapsed_ms", "REAL NOT NULL DEFAULT 0"},
		{"qa_reports", "ts", "TEXT NOT NULL DEFAULT ''"},
		// 测试计划执行引擎：用例执行记录关联计划与脚本执行；用例可关联自动化脚本
		{"case_executions", "plan_id", "TEXT NOT NULL DEFAULT ''"},
		{"case_executions", "execution_id", "TEXT NOT NULL DEFAULT ''"},
		{"test_cases", "script_id", "TEXT NOT NULL DEFAULT ''"},
		// 自由电子表格：基础格式层（格式/列宽/行高/合并）
		{"spreadsheets", "formats", "TEXT NOT NULL DEFAULT '{}'"},
		{"spreadsheets", "col_widths", "TEXT NOT NULL DEFAULT '{}'"},
		{"spreadsheets", "row_heights", "TEXT NOT NULL DEFAULT '{}'"},
		{"spreadsheets", "merges", "TEXT NOT NULL DEFAULT '[]'"},
	}
	for _, c := range cols {
		has, err := hasColumn(c.table, c.col)
		if err != nil {
			return fmt.Errorf("检查列 %s.%s 失败: %w", c.table, c.col, err)
		}
		if has {
			continue
		}
		if _, err := DB.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", c.table, c.col, c.def)); err != nil {
			return fmt.Errorf("新增列 %s.%s 失败: %w", c.table, c.col, err)
		}
		log.Printf("[数据库] 已为 %s 新增字段 %s", c.table, c.col)
	}
	return nil
}
