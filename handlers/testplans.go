package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"qatest/config"
	"qatest/models"
	"qatest/repository"
	"qatest/services"

	"github.com/gin-gonic/gin"
)

// --- 测试计划 ---

func GetTestPlans(c *gin.Context) {
	plans, err := repository.ListTestPlans()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: plans})
}

func GetTestPlan(c *gin.Context) {
	p, err := repository.GetTestPlan(c.Param("id"))
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
	if err := repository.CreateTestPlan(p); err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
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
	if err := repository.UpdateTestPlan(id, p); err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}
	p.ID = id
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: p})
}

func DeleteTestPlan(c *gin.Context) {
	id := c.Param("id")
	if err := repository.DeleteTestPlan(id); err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: nil})
}

// --- 计划执行记录 ---

func GetPlanExecutions(c *gin.Context) {
	execs, err := repository.ListPlanExecutions()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: execs})
}

func CreatePlanExecution(c *gin.Context) {
	var e models.PlanExecution
	if err := c.ShouldBindJSON(&e); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	// 若携带逐用例明细，自动聚合通过/失败/总数（登记执行结果）
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
	if err := repository.CreatePlanExecution(e); err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}
	c.JSON(http.StatusCreated, models.APIResponse{Success: true, Data: e})
}

// --- 自动化任务执行记录 ---

func GetAutoTaskExecutions(c *gin.Context) {
	execs, err := repository.ListAutoTaskExecutions()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
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
	if err := repository.CreateAutoTaskExecution(e); err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
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

	p, err := repository.GetTestPlan(id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{Success: false, Error: "计划不存在"})
		return
	}

	var req struct {
		Mode         string `json:"mode"`         // manual | auto
		DeviceSerial string `json:"deviceSerial"` // auto 模式使用的设备序列号
		Executor     string `json:"executor"`
	}
	if err := c.ShouldBindJSON(&req); err != nil { respondError(c, http.StatusBadRequest, err, "请求参数错误"); return }
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
	planExec := models.PlanExecution{
		ID:          planExecID,
		PlanID:      p.ID,
		PlanName:    p.Name,
		Status:      "running",
		Result:      "[]",
		CasesTotal:  len(caseIDs),
		CasesPassed: 0,
		CasesFailed: 0,
		ExecutedBy:  req.Executor,
		FinishedAt:  "",
		CasesDetail: "[]",
		Duration:    0,
		StartedAt:   now,
		CreatedAt:   now,
	}
	if err := repository.CreatePlanExecution(planExec); err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}

	caseExecs := make([]models.CaseExecution, 0, len(caseIDs))
	for _, cid := range caseIDs {
		caseName, steps, scriptID, scanErr := repository.GetTestCaseRunInfo(cid)
		if scanErr != nil {
			continue
		}

		ceID := generateID("ce")
		ce := models.CaseExecution{
			ID: ceID, CaseID: cid, CaseName: caseName, Executor: req.Executor,
			Result: "pending", Steps: steps, Duration: 0, Remark: "", ExecutedAt: now, PlanID: planExecID,
		}
		if err := repository.CreateCaseExecution(ce); err != nil {
			respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
			return
		}

		// auto 模式：若用例关联脚本，派发脚本执行任务
		// RCE 熔断：EXECUTOR_ENABLED=0 时跳过脚本派发（用例保持 pending，不阻断整体计划）
		if mode == "auto" && scriptID != "" {
			if !config.AppConfig.ExecutorEnabled {
				log.Printf("[WARN] 脚本执行引擎已禁用（EXECUTOR_ENABLED=0），跳过用例 %s 的脚本派发", cid)
			} else {
				dispatchCaseScript(c, ceID, planExecID, scriptID, caseName, req.DeviceSerial, req.Executor)
			}
		}

		caseExecs = append(caseExecs, ce)
	}

	// 将逐用例明细写回 plan_executions（初始全 pending）
	detail := buildPlanDetail(caseExecs)
	if err := repository.UpdatePlanExecutionDetail(detail, planExecID); err != nil {
		respondError(c, http.StatusInternalServerError, err, "保存计划执行明细失败")
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: gin.H{
		"planExecution": planExecID,
		"mode":          mode,
		"caseExecutions": caseExecs,
	}})
}

// dispatchCaseScript 派发用例关联的脚本执行任务，并在完成后回写用例结果、聚合计划。
func dispatchCaseScript(c *gin.Context, caseExecID, planID, scriptID, caseName, deviceSerial, executor string) {
	script, err := repository.GetScriptExecFields(scriptID)
	if err != nil {
		// 脚本不存在：保持 pending，不阻断整体计划
		return
	}

	now := models.NowStr()
	execID := generateID("exec")
	exec := models.Execution{
		ID:           execID,
		ScriptID:     script.ID,
		DeviceSerial: deviceSerial,
		TaskName:     caseName,
		Status:       "running",
		Logs:         "[]",
		Screenshots:  "[]",
		Duration:     0,
		StartedAt:    now,
		FinishedAt:   "",
		CreatedAt:    now,
	}
	if err := repository.CreateExecution(exec); err != nil {
		return
	}

	// 记录脚本执行与用例执行的关联
	if err := repository.UpdateCaseExecutionExecID(execID, caseExecID); err != nil {
		log.Printf("[WARN] 关联脚本执行与用例执行失败: %v", err)
	}

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
			repository.UpdateCaseExecutionResult(caseExecID, result)
			aggregatePlanExecution(planID)
		},
	}
	services.Executor.Start(task)
}

// aggregatePlanExecution 根据 plan_id 下所有 case_executions 重新汇总计划执行结果，
// 并回写 plan_executions 与 test_plans 的状态。
func aggregatePlanExecution(planID string) {
	results, err := repository.ListCaseExecutionResults(planID)
	if err != nil {
		return
	}

	total, passed, failed, blocked, skipped := 0, 0, 0, 0, 0
	allDone := true
	for _, r := range results {
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

	repository.UpdatePlanExecutionStats(planID, total, passed, failed, status, finishedAt)

	// 同步更新计划本身的状态
	planStatus := "in_progress"
	if allDone {
		planStatus = "completed"
	}
	if ppid, err := repository.GetPlanExecutionPlanID(planID); err == nil && ppid != "" {
		repository.UpdateTestPlanStatus(ppid, planStatus)
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
