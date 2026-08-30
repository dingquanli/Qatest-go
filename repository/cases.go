package repository

import (
	"database/sql"

	"qatest/database"
	"qatest/models"
)

// —— 测试用例（SQL 迁自 handlers/testcases.go，语句原样保留） ——

// ListTestCases 用例列表（limit 由调用方传入，SQL 保持 LIMIT ? 原样）
func ListTestCases(limit int) ([]models.TestCase, error) {
	rows, err := database.DB.Query(
		`SELECT id, name, module_id, priority, type, precondition, steps, assignee, status, tags, created_at, updated_at
		 FROM test_cases ORDER BY updated_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cases := make([]models.TestCase, 0)
	for rows.Next() {
		var tc models.TestCase
		if err := rows.Scan(&tc.ID, &tc.Name, &tc.ModuleID, &tc.Priority, &tc.Type, &tc.Precondition,
			&tc.Steps, &tc.Assignee, &tc.Status, &tc.Tags, &tc.CreatedAt, &tc.UpdatedAt); err != nil {
			return nil, err
		}
		cases = append(cases, tc)
	}
	return cases, rows.Err()
}

// GetTestCase 用例详情（不存在时返回 sql.ErrNoRows）
func GetTestCase(id string) (models.TestCase, error) {
	var tc models.TestCase
	err := database.DB.QueryRow(
		`SELECT id, name, module_id, priority, type, precondition, steps, assignee, status, tags, created_at, updated_at
		 FROM test_cases WHERE id = ?`, id,
	).Scan(&tc.ID, &tc.Name, &tc.ModuleID, &tc.Priority, &tc.Type, &tc.Precondition,
		&tc.Steps, &tc.Assignee, &tc.Status, &tc.Tags, &tc.CreatedAt, &tc.UpdatedAt)
	return tc, err
}

// CreateTestCase 插入用例（ID/时间戳由调用方填充）
func CreateTestCase(s models.TestCase) error {
	_, err := database.DB.Exec(
		`INSERT INTO test_cases (id, name, module_id, priority, type, precondition, steps, assignee, status, tags, script_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.Name, s.ModuleID, s.Priority, s.Type, s.Precondition,
		s.Steps, s.Assignee, s.Status, s.Tags, s.ScriptID, s.CreatedAt, s.UpdatedAt,
	)
	return err
}

// UpdateTestCase 更新用例
func UpdateTestCase(id string, s models.TestCase) error {
	_, err := database.DB.Exec(
		`UPDATE test_cases SET name=?, module_id=?, priority=?, type=?, precondition=?, steps=?, assignee=?, status=?, tags=?, script_id=?, updated_at=?
		 WHERE id=?`,
		s.Name, s.ModuleID, s.Priority, s.Type, s.Precondition, s.Steps,
		s.Assignee, s.Status, s.Tags, s.ScriptID, s.UpdatedAt, id,
	)
	return err
}

// DeleteTestCase 删除用例
func DeleteTestCase(id string) error {
	_, err := database.DB.Exec("DELETE FROM test_cases WHERE id = ?", id)
	return err
}

// InsertTestCase 批量导入用例的单条 INSERT（迁自 handlers/testcases.go BatchImportCases）。
// 该场景要求整体成功或整体回滚，事务（Begin/Commit/Rollback）由调用方持有，
// 本函数只在传入事务上执行 SQL，循环与失败计数仍留在 handler。
func InsertTestCase(tx *sql.Tx, s models.TestCase) error {
	_, err := tx.Exec(
		`INSERT INTO test_cases (id, name, module_id, priority, type, precondition, steps, assignee, status, tags, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.Name, s.ModuleID, s.Priority, s.Type, s.Precondition,
		s.Steps, s.Assignee, s.Status, s.Tags, s.CreatedAt, s.UpdatedAt,
	)
	return err
}

// —— 用例执行记录（SQL 迁自 handlers/testcases.go，语句原样保留） ——

// ListCaseExecutions 执行记录列表
func ListCaseExecutions() ([]models.CaseExecution, error) {
	rows, err := database.DB.Query(
		"SELECT id, case_id, case_name, executor, result, steps, duration, remark, executed_at, plan_id, execution_id FROM case_executions ORDER BY executed_at DESC LIMIT 100",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	execs := make([]models.CaseExecution, 0)
	for rows.Next() {
		var e models.CaseExecution
		if err := rows.Scan(&e.ID, &e.CaseID, &e.CaseName, &e.Executor, &e.Result, &e.Steps, &e.Duration, &e.Remark, &e.ExecutedAt, &e.PlanID, &e.ExecutionID); err != nil {
			return nil, err
		}
		execs = append(execs, e)
	}
	return execs, rows.Err()
}

// CaseExecutionStats 按 result 聚合执行记录数量
func CaseExecutionStats() (map[string]int, error) {
	rows, err := database.DB.Query("SELECT result, COUNT(*) as cnt FROM case_executions GROUP BY result")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var result string
		var cnt int
		if err := rows.Scan(&result, &cnt); err != nil {
			return nil, err
		}
		stats[result] = cnt
	}
	return stats, rows.Err()
}

// CreateCaseExecution 插入执行记录（ID/时间戳由调用方填充）
func CreateCaseExecution(e models.CaseExecution) error {
	_, err := database.DB.Exec(
		`INSERT INTO case_executions (id, case_id, case_name, executor, result, steps, duration, remark, executed_at, plan_id, execution_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.CaseID, e.CaseName, e.Executor, e.Result, e.Steps, e.Duration, e.Remark, e.ExecutedAt, e.PlanID, e.ExecutionID,
	)
	return err
}

// UpdateCaseExecution 更新执行记录
func UpdateCaseExecution(id string, e models.CaseExecution) error {
	_, err := database.DB.Exec(
		`UPDATE case_executions SET case_id=?, case_name=?, executor=?, result=?, steps=?, duration=?, remark=?
		 WHERE id=?`,
		e.CaseID, e.CaseName, e.Executor, e.Result, e.Steps, e.Duration, e.Remark, id,
	)
	return err
}

// GetCaseExecutionPlanID 查询执行记录关联的测试计划 ID（不存在时返回 sql.ErrNoRows）
func GetCaseExecutionPlanID(id string) (string, error) {
	var planID string
	err := database.DB.QueryRow("SELECT plan_id FROM case_executions WHERE id = ?", id).Scan(&planID)
	return planID, err
}

// DeleteCaseExecution 删除执行记录
func DeleteCaseExecution(id string) error {
	_, err := database.DB.Exec("DELETE FROM case_executions WHERE id = ?", id)
	return err
}
