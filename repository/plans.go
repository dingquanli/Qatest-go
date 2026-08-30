package repository

import (
	"qatest/database"
	"qatest/models"
)

// —— 测试计划（SQL 迁自 handlers/testplans.go，语句原样保留） ——

// ListTestPlans 测试计划列表
func ListTestPlans() ([]models.TestPlan, error) {
	rows, err := database.DB.Query(
		"SELECT id, name, description, case_ids, status, start_date, end_date, created_at, updated_at FROM test_plans ORDER BY updated_at DESC LIMIT 100",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	plans := make([]models.TestPlan, 0)
	for rows.Next() {
		var p models.TestPlan
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CaseIDs, &p.Status, &p.StartDate, &p.EndDate, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}

// GetTestPlan 计划详情（不存在时返回 sql.ErrNoRows）
func GetTestPlan(id string) (models.TestPlan, error) {
	var p models.TestPlan
	err := database.DB.QueryRow(
		"SELECT id, name, description, case_ids, status, start_date, end_date, created_at, updated_at FROM test_plans WHERE id = ?", id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.CaseIDs, &p.Status, &p.StartDate, &p.EndDate, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

// CreateTestPlan 插入测试计划（ID/时间戳由调用方填充）
func CreateTestPlan(p models.TestPlan) error {
	_, err := database.DB.Exec(
		"INSERT INTO test_plans (id, name, description, case_ids, status, start_date, end_date, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?)",
		p.ID, p.Name, p.Description, p.CaseIDs, p.Status, p.StartDate, p.EndDate, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

// UpdateTestPlan 更新测试计划
func UpdateTestPlan(id string, p models.TestPlan) error {
	_, err := database.DB.Exec(
		"UPDATE test_plans SET name=?, description=?, case_ids=?, status=?, start_date=?, end_date=?, updated_at=? WHERE id=?",
		p.Name, p.Description, p.CaseIDs, p.Status, p.StartDate, p.EndDate, p.UpdatedAt, id,
	)
	return err
}

// DeleteTestPlan 删除测试计划
func DeleteTestPlan(id string) error {
	_, err := database.DB.Exec("DELETE FROM test_plans WHERE id = ?", id)
	return err
}

// UpdateTestPlanStatus 同步更新测试计划本身的状态（迁自 handlers/testplans.go aggregatePlanExecution）
func UpdateTestPlanStatus(id, status string) error {
	_, err := database.DB.Exec("UPDATE test_plans SET status=? WHERE id=?", status, id)
	return err
}

// —— 计划执行记录（SQL 迁自 handlers/testplans.go，语句原样保留） ——

// ListPlanExecutions 计划执行记录列表
func ListPlanExecutions() ([]models.PlanExecution, error) {
	rows, err := database.DB.Query("SELECT id, plan_id, plan_name, status, result, cases_total, cases_passed, cases_failed, executed_by, finished_at, cases_detail, duration, started_at, created_at FROM plan_executions ORDER BY created_at DESC LIMIT 100")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	execs := make([]models.PlanExecution, 0)
	for rows.Next() {
		var e models.PlanExecution
		if err := rows.Scan(&e.ID, &e.PlanID, &e.PlanName, &e.Status, &e.Result, &e.CasesTotal, &e.CasesPassed, &e.CasesFailed, &e.ExecutedBy, &e.FinishedAt, &e.CasesDetail, &e.Duration, &e.StartedAt, &e.CreatedAt); err != nil {
			return nil, err
		}
		execs = append(execs, e)
	}
	return execs, rows.Err()
}

// CreatePlanExecution 插入计划执行记录（ID/时间戳由调用方填充）。
// handlers/testplans.go 的执行引擎（ExecuteTestPlan）亦使用同一条语句，一并复用本函数。
func CreatePlanExecution(e models.PlanExecution) error {
	_, err := database.DB.Exec(
		`INSERT INTO plan_executions
		 (id, plan_id, plan_name, status, result, cases_total, cases_passed, cases_failed, executed_by, finished_at, cases_detail, duration, started_at, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		e.ID, e.PlanID, e.PlanName, e.Status, e.Result, e.CasesTotal, e.CasesPassed, e.CasesFailed,
		e.ExecutedBy, e.FinishedAt, e.CasesDetail, e.Duration, e.StartedAt, e.CreatedAt,
	)
	return err
}

// UpdatePlanExecutionDetail 回写计划执行的逐用例明细（迁自 handlers/testplans.go ExecuteTestPlan）
func UpdatePlanExecutionDetail(detail, id string) error {
	_, err := database.DB.Exec("UPDATE plan_executions SET cases_detail=? WHERE id=?", detail, id)
	return err
}

// UpdatePlanExecutionStats 聚合回写计划执行的用例统计与状态（迁自 handlers/testplans.go aggregatePlanExecution）
func UpdatePlanExecutionStats(planID string, total, passed, failed int, status, finishedAt string) error {
	_, err := database.DB.Exec(
		`UPDATE plan_executions SET cases_total=?, cases_passed=?, cases_failed=?, status=?, finished_at=? WHERE id=?`,
		total, passed, failed, status, finishedAt, planID,
	)
	return err
}

// GetPlanExecutionPlanID 查询计划执行记录所属的测试计划 ID（不存在时返回 sql.ErrNoRows）
func GetPlanExecutionPlanID(id string) (string, error) {
	var planID string
	err := database.DB.QueryRow("SELECT plan_id FROM plan_executions WHERE id = ?", id).Scan(&planID)
	return planID, err
}

// —— 自动化任务执行记录（SQL 迁自 handlers/testplans.go，语句原样保留） ——

// ListAutoTaskExecutions 自动化任务执行记录列表
func ListAutoTaskExecutions() ([]models.AutoTaskExecution, error) {
	rows, err := database.DB.Query("SELECT id, task_id, task_name, status, result, logs, started_at, finished_at, created_at FROM auto_task_executions ORDER BY created_at DESC LIMIT 100")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	execs := make([]models.AutoTaskExecution, 0)
	for rows.Next() {
		var e models.AutoTaskExecution
		if err := rows.Scan(&e.ID, &e.TaskID, &e.TaskName, &e.Status, &e.Result, &e.Logs, &e.StartedAt, &e.FinishedAt, &e.CreatedAt); err != nil {
			return nil, err
		}
		execs = append(execs, e)
	}
	return execs, rows.Err()
}

// CreateAutoTaskExecution 插入自动化任务执行记录（ID/时间戳由调用方填充）
func CreateAutoTaskExecution(e models.AutoTaskExecution) error {
	_, err := database.DB.Exec("INSERT INTO auto_task_executions (id, task_id, task_name, status, result, logs, created_at) VALUES (?,?,?,?,?,?,?)",
		e.ID, e.TaskID, e.TaskName, e.Status, e.Result, e.Logs, e.CreatedAt)
	return err
}

// —— 测试计划执行引擎辅助查询/回写（SQL 迁自 handlers/testplans.go，语句原样保留） ——

// ScriptExecInfo 脚本执行所需的最小字段（对应 handlers/testplans.go dispatchCaseScript 的匿名结构体）
type ScriptExecInfo struct {
	ID       string
	Name     string
	Language string
	Code     string
}

// GetScriptExecFields 查询脚本执行所需字段（迁自 handlers/testplans.go dispatchCaseScript，语句原样保留；
// 不存在时返回 sql.ErrNoRows）
func GetScriptExecFields(id string) (ScriptExecInfo, error) {
	var s ScriptExecInfo
	err := database.DB.QueryRow("SELECT id, name, language, code FROM scripts WHERE id = ?", id).
		Scan(&s.ID, &s.Name, &s.Language, &s.Code)
	return s, err
}

// GetTestCaseRunInfo 查询用例名称、步骤与关联脚本 ID（迁自 handlers/testplans.go ExecuteTestPlan，
// 语句原样保留；不存在时返回 sql.ErrNoRows）
func GetTestCaseRunInfo(id string) (caseName, steps, scriptID string, err error) {
	err = database.DB.QueryRow("SELECT name, steps, script_id FROM test_cases WHERE id = ?", id).
		Scan(&caseName, &steps, &scriptID)
	return
}

// UpdateCaseExecutionExecID 记录脚本执行与用例执行的关联（迁自 handlers/testplans.go dispatchCaseScript）
func UpdateCaseExecutionExecID(executionID, caseExecID string) error {
	_, err := database.DB.Exec("UPDATE case_executions SET execution_id=? WHERE id=?", executionID, caseExecID)
	return err
}

// UpdateCaseExecutionResult 回写用例执行结果（迁自 handlers/testplans.go dispatchCaseScript 的 OnDone 回调）
func UpdateCaseExecutionResult(caseExecID, result string) error {
	_, err := database.DB.Exec("UPDATE case_executions SET result=? WHERE id=?", result, caseExecID)
	return err
}

// ListCaseExecutionResults 查询某计划下全部用例执行结果（迁自 handlers/testplans.go aggregatePlanExecution）。
// 注：原实现未检查 rows.Err()，此处保持一致，避免改变聚合行为。
func ListCaseExecutionResults(planID string) ([]string, error) {
	rows, err := database.DB.Query("SELECT result FROM case_executions WHERE plan_id = ?", planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]string, 0)
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}
