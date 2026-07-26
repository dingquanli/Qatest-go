package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"qatest/database"
	"qatest/models"
	"qatest/services"

	"github.com/gin-gonic/gin"
)

// --- 测试计划 ---

func GetTestPlans(c *gin.Context) {
	rows, err := database.DB.Query(
		"SELECT id, name, description, case_ids, status, start_date, end_date, created_at, updated_at FROM test_plans ORDER BY updated_at DESC LIMIT 100",
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()

	plans := make([]models.TestPlan, 0)
	for rows.Next() {
		var p models.TestPlan
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.CaseIDs, &p.Status, &p.StartDate, &p.EndDate, &p.CreatedAt, &p.UpdatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
			return
		}
		plans = append(plans, p)
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: plans})
}

func GetTestPlan(c *gin.Context) {
	id := c.Param("id")
	var p models.TestPlan
	err := database.DB.QueryRow(
		"SELECT id, name, description, case_ids, status, start_date, end_date, created_at, updated_at FROM test_plans WHERE id = ?", id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.CaseIDs, &p.Status, &p.StartDate, &p.EndDate, &p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{Success: false, Error: "计划不存在"})
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: p})
}

func CreateTestPlan(c *gin.Context) {
	var p models.TestPlan
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	p.ID = generateID("tp")
	p.CreatedAt = models.NowStr()
	p.UpdatedAt = p.CreatedAt
	_, err := database.DB.Exec(
		"INSERT INTO test_plans (id, name, description, case_ids, status, start_date, end_date, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?)",
		p.ID, p.Name, p.Description, p.CaseIDs, p.Status, p.StartDate, p.EndDate, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, models.APIResponse{Success: true, Data: p})
}

func UpdateTestPlan(c *gin.Context) {
	id := c.Param("id")
	var p models.TestPlan
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	p.UpdatedAt = models.NowStr()
	_, err := database.DB.Exec(
		"UPDATE test_plans SET name=?, description=?, case_ids=?, status=?, start_date=?, end_date=?, updated_at=? WHERE id=?",
		p.Name, p.Description, p.CaseIDs, p.Status, p.StartDate, p.EndDate, p.UpdatedAt, id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	p.ID = id
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: p})
}

func DeleteTestPlan(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec("DELETE FROM test_plans WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: nil})
}

// --- 计划执行记录 ---

func GetPlanExecutions(c *gin.Context) {
	rows, err := database.DB.Query("SELECT id, plan_id, plan_name, status, result, cases_total, cases_passed, cases_failed, executed_by, finished_at, cases_detail, duration, started_at, created_at FROM plan_executions ORDER BY created_at DESC LIMIT 100")
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()
	execs := make([]models.PlanExecution, 0)
	for rows.Next() {
		var e models.PlanExecution
		if err := rows.Scan(&e.ID, &e.PlanID, &e.PlanName, &e.Status, &e.Result, &e.CasesTotal, &e.CasesPassed, &e.CasesFailed, &e.ExecutedBy, &e.FinishedAt, &e.CasesDetail, &e.Duration, &e.StartedAt, &e.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
			return
		}
		execs = append(execs, e)
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: execs})
}

func CreatePlanExecution(c *gin.Context) {
	var e models.PlanExecution
	if err := c.ShouldBindJSON(&e); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	// 若携带逐用例明细，自动聚合通过/失败/总数（P1-2：登记执行结果）
	if e.CasesDetail != "" {
		var details []struct {
			CaseID   string `json:"caseId"`
			CaseName string `json:"caseName"`
			Result   string `json:"result"`
			Remark   string `json:"remark"`
		}
		if err := json.Unmarshal([]byte(e.CasesDetail), &details); err == nil {
			total, passed, failed := 0, 0, 0
			for _, d := range details {
				total++
				switch d.Result {
				case "passed":
					passed++
				case "failed":
					failed++
				}
			}
			e.CasesTotal = total
			e.CasesPassed = passed
			e.CasesFailed = failed
		}
	}

	if e.Status == "" {
		e.Status = "completed"
	}
	e.ID = generateID("pe")
	e.CreatedAt = models.NowStr()
	_, err := database.DB.Exec(
		`INSERT INTO plan_executions
		 (id, plan_id, plan_name, status, result, cases_total, cases_passed, cases_failed, executed_by, finished_at, cases_detail, duration, started_at, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		e.ID, e.PlanID, e.PlanName, e.Status, e.Result, e.CasesTotal, e.CasesPassed, e.CasesFailed,
		e.ExecutedBy, e.FinishedAt, e.CasesDetail, e.Duration, e.StartedAt, e.CreatedAt,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, models.APIResponse{Success: true, Data: e})
}

// --- 自动化任务执行记录 ---

func GetAutoTaskExecutions(c *gin.Context) {
	rows, err := database.DB.Query("SELECT id, task_id, task_name, status, result, logs, started_at, finished_at, created_at FROM auto_task_executions ORDER BY created_at DESC LIMIT 100")
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()
	execs := make([]models.AutoTaskExecution, 0)
	for rows.Next() {
		var e models.AutoTaskExecution
		if err := rows.Scan(&e.ID, &e.TaskID, &e.TaskName, &e.Status, &e.Result, &e.Logs, &e.StartedAt, &e.FinishedAt, &e.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
			return
		}
		execs = append(execs, e)
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: execs})
}

func CreateAutoTaskExecution(c *gin.Context) {
	var e models.AutoTaskExecution
	if err := c.ShouldBindJSON(&e); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	e.ID = generateID("at")
	e.CreatedAt = models.NowStr()
	_, err := database.DB.Exec("INSERT INTO auto_task_executions (id, task_id, task_name, status, result, logs, created_at) VALUES (?,?,?,?,?,?,?)",
		e.ID, e.TaskID, e.TaskName, e.Status, e.Result, e.Logs, e.CreatedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, models.APIResponse{Success: true, Data: e})
}

// --- 测试计划执行引擎 ---

// ExecuteTestPlan 启动一个测试计划的执行：
//   - 读取计划的 case_ids；
//   - 创建一条 plan_executions（running）与逐用例的 case_executions（pending，关联 plan_id）；
//   - manual 模式：仅登记骨架，等待前端逐条回写结果；
//   - auto 模式：若用例关联了脚本(script_id)，则派发脚本执行任务，完成后通过 OnDone 回写用例结果并聚合。
func ExecuteTestPlan(c *gin.Context) {
	id := c.Param("id")

	var p models.TestPlan
	err := database.DB.QueryRow(
		"SELECT id, name, description, case_ids, status, start_date, end_date, created_at, updated_at FROM test_plans WHERE id = ?", id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.CaseIDs, &p.Status, &p.StartDate, &p.EndDate, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{Success: false, Error: "计划不存在"})
		return
	}

	var req struct {
		Mode         string `json:"mode"`         // manual | auto
		DeviceSerial string `json:"deviceSerial"` // auto 模式使用的设备序列号
		Executor     string `json:"executor"`     // 执行人
	}
	_ = c.ShouldBindJSON(&req)
	mode := req.Mode
	if mode != "auto" {
		mode = "manual"
	}
	if req.Executor == "" {
		if u, ok := c.Get("username"); ok {
			if s, ok := u.(string); ok {
				req.Executor = s
			}
		}
	}

	caseIDs := parseCaseIDs(p.CaseIDs)
	if len(caseIDs) == 0 {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "计划未关联任何用例"})
		return
	}

	now := models.NowStr()
	planExecID := generateID("pe")
	_, err = database.DB.Exec(
		`INSERT INTO plan_executions
		 (id, plan_id, plan_name, status, result, cases_total, cases_passed, cases_failed, executed_by, finished_at, cases_detail, duration, started_at, created_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		planExecID, p.ID, p.Name, "running", "[]", len(caseIDs), 0, 0,
		req.Executor, "", "[]", 0, now, now,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}

	caseExecs := make([]models.CaseExecution, 0, len(caseIDs))
	for _, cid := range caseIDs {
		var caseName, steps, scriptID string
		_ = database.DB.QueryRow("SELECT name, steps, script_id FROM test_cases WHERE id = ?", cid).
			Scan(&caseName, &steps, &scriptID)

		ceID := generateID("ce")
		_, err := database.DB.Exec(
			`INSERT INTO case_executions
			 (id, case_id, case_name, executor, result, steps, duration, remark, executed_at, plan_id, execution_id)
			 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			ceID, cid, caseName, req.Executor, "pending", steps, 0, "", now, planExecID, "",
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
			return
		}
		ce := models.CaseExecution{
			ID: ceID, CaseID: cid, CaseName: caseName, Executor: req.Executor,
			Result: "pending", Steps: steps, Duration: 0, Remark: "", ExecutedAt: now, PlanID: planExecID,
		}

		// auto 模式：若用例关联脚本，派发脚本执行任务
		if mode == "auto" && scriptID != "" {
			dispatchCaseScript(c, ceID, planExecID, scriptID, caseName, req.DeviceSerial, req.Executor)
		}

		caseExecs = append(caseExecs, ce)
	}

	// 将逐用例明细写回 plan_executions（初始全 pending）
	detail := buildPlanDetail(caseExecs)
	database.DB.Exec("UPDATE plan_executions SET cases_detail=? WHERE id=?", detail, planExecID)

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: gin.H{
		"planExecution": planExecID,
		"mode":          mode,
		"caseExecutions": caseExecs,
	}})
}

// dispatchCaseScript 派发用例关联的脚本执行任务，并在完成后回写用例结果、聚合计划。
func dispatchCaseScript(c *gin.Context, caseExecID, planID, scriptID, caseName, deviceSerial, executor string) {
	var script struct {
		ID       string
		Name     string
		Language string
		Code     string
	}
	err := database.DB.QueryRow("SELECT id, name, language, code FROM scripts WHERE id = ?", scriptID).
		Scan(&script.ID, &script.Name, &script.Language, &script.Code)
	if err != nil {
		// 脚本不存在：保持 pending，不阻断整体计划
		return
	}

	now := models.NowStr()
	execID := generateID("exec")
	_, err = database.DB.Exec(
		`INSERT INTO executions (id, script_id, device_serial, task_name, status, logs, screenshots, duration, started_at, finished_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		execID, script.ID, deviceSerial, caseName, "running", "[]", "[]", 0, now, "", now,
	)
	if err != nil {
		return
	}

	// 记录脚本执行与用例执行的关联
	database.DB.Exec("UPDATE case_executions SET execution_id=? WHERE id=?", execID, caseExecID)

	ctx, cancel := context.WithCancel(context.Background())
	task := &services.ExecutionTask{
		ID:           execID,
		ScriptID:     script.ID,
		DeviceSerial: deviceSerial,
		TaskName:     caseName,
		Language:     script.Language,
		Code:         script.Code,
		LogChan:      make(chan services.LogEntry, 100),
		Ctx:          ctx,
		Cancel:       cancel,
		OnDone: func(status string) {
			// 脚本成功(success)→用例通过；失败/取消→失败
			result := "failed"
			if status == "success" {
				result = "passed"
			} else if status == "cancelled" {
				result = "failed"
			}
			database.DB.Exec("UPDATE case_executions SET result=? WHERE id=?", result, caseExecID)
			aggregatePlanExecution(planID)
		},
	}
	services.Executor.Start(task)
}

// aggregatePlanExecution 根据 plan_id 下所有 case_executions 重新汇总计划执行结果，
// 并回写 plan_executions 与 test_plans 的状态。
func aggregatePlanExecution(planID string) {
	rows, err := database.DB.Query("SELECT result FROM case_executions WHERE plan_id = ?", planID)
	if err != nil {
		return
	}
	defer rows.Close()

	total, passed, failed, blocked, skipped := 0, 0, 0, 0, 0
	allDone := true
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return
		}
		total++
		switch r {
		case "passed":
			passed++
		case "failed":
			failed++
		case "blocked":
			blocked++
		case "skipped":
			skipped++
		default:
			allDone = false // pending 视为未完成
		}
	}

	status := "running"
	if allDone {
		status = "completed"
	}
	finishedAt := ""
	if allDone {
		finishedAt = models.NowStr()
	}

	database.DB.Exec(
		`UPDATE plan_executions SET cases_total=?, cases_passed=?, cases_failed=?, status=?, finished_at=? WHERE id=?`,
		total, passed, failed, status, finishedAt, planID,
	)

	// 同步更新计划本身的状态
	planStatus := "in_progress"
	if allDone {
		planStatus = "completed"
	}
	var ppid string
	if err := database.DB.QueryRow("SELECT plan_id FROM plan_executions WHERE id = ?", planID).Scan(&ppid); err == nil && ppid != "" {
		database.DB.Exec("UPDATE test_plans SET status=? WHERE id=?", planStatus, ppid)
	}
}

// parseCaseIDs 解析计划中的 case_ids（JSON 字符串数组）。
func parseCaseIDs(raw string) []string {
	var ids []string
	if raw == "" {
		return ids
	}
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return ids
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			out = append(out, id)
		}
	}
	return out
}

// buildPlanDetail 由逐用例执行记录构造 plan_executions.cases_detail JSON。
func buildPlanDetail(cases []models.CaseExecution) string {
	details := make([]map[string]string, 0, len(cases))
	for _, ce := range cases {
		details = append(details, map[string]string{
			"caseId":   ce.CaseID,
			"caseName": ce.CaseName,
			"result":   ce.Result,
			"remark":   ce.Remark,
		})
	}
	b, _ := json.Marshal(details)
	return string(b)
}
