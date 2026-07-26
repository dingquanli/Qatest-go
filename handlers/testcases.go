package handlers

import (
	"log"
	"net/http"

	"qatest/database"
	"qatest/models"

	"github.com/gin-gonic/gin"
)

// --- 测试用例 ---

func GetTestCases(c *gin.Context) {
	// 优化：添加 LIMIT 分页，默认 100 条
	limit := 100
	rows, err := database.DB.Query(
		`SELECT id, name, module_id, priority, type, precondition, steps, assignee, status, tags, created_at, updated_at
		 FROM test_cases ORDER BY updated_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}
	defer rows.Close()

	cases := make([]models.TestCase, 0)
	for rows.Next() {
		var tc models.TestCase
		if err := rows.Scan(&tc.ID, &tc.Name, &tc.ModuleID, &tc.Priority, &tc.Type, &tc.Precondition,
			&tc.Steps, &tc.Assignee, &tc.Status, &tc.Tags, &tc.CreatedAt, &tc.UpdatedAt); err != nil {
			respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
			return
		}
		cases = append(cases, tc)
	}
	if err := rows.Err(); err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: cases})
}

func GetTestCase(c *gin.Context) {
	id := c.Param("id")
	var tc models.TestCase
	err := database.DB.QueryRow(
		`SELECT id, name, module_id, priority, type, precondition, steps, assignee, status, tags, created_at, updated_at
		 FROM test_cases WHERE id = ?`, id,
	).Scan(&tc.ID, &tc.Name, &tc.ModuleID, &tc.Priority, &tc.Type, &tc.Precondition,
		&tc.Steps, &tc.Assignee, &tc.Status, &tc.Tags, &tc.CreatedAt, &tc.UpdatedAt)

	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{Success: false, Error: "用例不存在"})
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: tc})
}

func CreateTestCase(c *gin.Context) {
	var tc models.TestCase
	if err := c.ShouldBindJSON(&tc); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	// P2-8 修复：服务端校验枚举字段，拒绝非法取值（空值视为未设置，允许）
	validEnum := func(v string, allowed ...string) bool { return v == "" || validateEnum(v, allowed...) }
	if !validEnum(tc.Priority, "P0", "P1", "P2", "P3") ||
		!validEnum(tc.Type, "functional", "performance", "security", "compatibility", "usability") ||
		!validEnum(tc.Status, "draft", "review", "approved", "archived") {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "非法的 priority/type/status 取值"})
		return
	}

	tc.ID = generateID("tc")
	tc.CreatedAt = models.NowStr()
	tc.UpdatedAt = tc.CreatedAt

	_, err := database.DB.Exec(
		`INSERT INTO test_cases (id, name, module_id, priority, type, precondition, steps, assignee, status, tags, script_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tc.ID, tc.Name, tc.ModuleID, tc.Priority, tc.Type, tc.Precondition,
		tc.Steps, tc.Assignee, tc.Status, tc.Tags, tc.ScriptID, tc.CreatedAt, tc.UpdatedAt,
	)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}

	c.JSON(http.StatusCreated, models.APIResponse{Success: true, Data: tc})
}

func UpdateTestCase(c *gin.Context) {
	id := c.Param("id")
	var tc models.TestCase
	if err := c.ShouldBindJSON(&tc); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	// P2-8 修复：服务端校验枚举字段，拒绝非法取值（空值视为未设置，允许）
	validEnum := func(v string, allowed ...string) bool { return v == "" || validateEnum(v, allowed...) }
	if !validEnum(tc.Priority, "P0", "P1", "P2", "P3") ||
		!validEnum(tc.Type, "functional", "performance", "security", "compatibility", "usability") ||
		!validEnum(tc.Status, "draft", "review", "approved", "archived") {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "非法的 priority/type/status 取值"})
		return
	}

	tc.UpdatedAt = models.NowStr()
	_, err := database.DB.Exec(
		`UPDATE test_cases SET name=?, module_id=?, priority=?, type=?, precondition=?, steps=?, assignee=?, status=?, tags=?, script_id=?, updated_at=?
		 WHERE id=?`,
		tc.Name, tc.ModuleID, tc.Priority, tc.Type, tc.Precondition, tc.Steps,
		tc.Assignee, tc.Status, tc.Tags, tc.ScriptID, tc.UpdatedAt, id,
	)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}

	tc.ID = id
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: tc})
}

func DeleteTestCase(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec("DELETE FROM test_cases WHERE id = ?", id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: nil})
}

func BatchImportCases(c *gin.Context) {
	var req models.BatchCaseImport
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	tx, err := database.DB.Begin()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}

	imported := 0
	failed := 0
	for _, tc := range req.Cases {
		tc.ID = generateID("tc")
		tc.CreatedAt = models.NowStr()
		tc.UpdatedAt = tc.CreatedAt
		if _, e := tx.Exec(
			`INSERT INTO test_cases (id, name, module_id, priority, type, precondition, steps, assignee, status, tags, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			tc.ID, tc.Name, tc.ModuleID, tc.Priority, tc.Type, tc.Precondition,
			tc.Steps, tc.Assignee, tc.Status, tc.Tags, tc.CreatedAt, tc.UpdatedAt,
		); e != nil {
			failed++
			continue // P2-4 修复：不 break，继续处理剩余条目
		}
		imported++
	}

	if failed > 0 {
		if rbErr := tx.Rollback(); rbErr != nil {
			log.Printf("[ERROR] BatchImportCases 回滚失败: %v", rbErr)
		}
		// 事务已整体回滚，实际未导入任何用例，响应需与之一致
		c.JSON(http.StatusOK, models.APIResponse{
			Success: false,
			Error:   "部分用例导入失败，已整体回滚",
			Data:    gin.H{"imported": 0, "failed": failed},
		})
		return
	}
	if err := tx.Commit(); err != nil {
		respondError(c, http.StatusInternalServerError, err, "提交用例导入失败")
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: gin.H{"imported": imported, "failed": failed}})
}

// --- 用例模块 ---

func GetCaseModules(c *gin.Context) {
	rows, err := database.DB.Query("SELECT id, name, parent_id, sort_order, created_at FROM case_modules ORDER BY sort_order")
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}
	defer rows.Close()

	modules := make([]models.CaseModule, 0)
	for rows.Next() {
		var m models.CaseModule
		if err := rows.Scan(&m.ID, &m.Name, &m.ParentID, &m.SortOrder, &m.CreatedAt); err != nil {
			respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
			return
		}
		modules = append(modules, m)
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: modules})
}

func CreateCaseModule(c *gin.Context) {
	var m models.CaseModule
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	m.ID = generateID("cm")
	m.CreatedAt = models.NowStr()
	_, err := database.DB.Exec("INSERT INTO case_modules (id, name, parent_id, sort_order, created_at) VALUES (?, ?, ?, ?, ?)",
		m.ID, m.Name, m.ParentID, m.SortOrder, m.CreatedAt)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}

	c.JSON(http.StatusCreated, models.APIResponse{Success: true, Data: m})
}

func UpdateCaseModule(c *gin.Context) {
	id := c.Param("id")
	var m models.CaseModule
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	_, err := database.DB.Exec("UPDATE case_modules SET name=?, parent_id=?, sort_order=? WHERE id=?",
		m.Name, m.ParentID, m.SortOrder, id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}
	m.ID = id
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: m})
}

func DeleteCaseModule(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec("DELETE FROM case_modules WHERE id = ?", id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: nil})
}

// --- 用例执行记录 ---

func GetCaseExecutions(c *gin.Context) {
	rows, err := database.DB.Query(
		"SELECT id, case_id, case_name, executor, result, steps, duration, remark, executed_at, plan_id, execution_id FROM case_executions ORDER BY executed_at DESC LIMIT 100",
	)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}
	defer rows.Close()

	execs := make([]models.CaseExecution, 0)
	for rows.Next() {
		var e models.CaseExecution
		if err := rows.Scan(&e.ID, &e.CaseID, &e.CaseName, &e.Executor, &e.Result, &e.Steps, &e.Duration, &e.Remark, &e.ExecutedAt, &e.PlanID, &e.ExecutionID); err != nil {
			respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
			return
		}
		execs = append(execs, e)
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: execs})
}

func GetCaseExecutionsStats(c *gin.Context) {
	rows, err := database.DB.Query("SELECT result, COUNT(*) as cnt FROM case_executions GROUP BY result")
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var result string
		var cnt int
		if err := rows.Scan(&result, &cnt); err != nil {
			respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
			return
		}
		stats[result] = cnt
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: stats})
}

func CreateCaseExecution(c *gin.Context) {
	var e models.CaseExecution
	if err := c.ShouldBindJSON(&e); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	e.ID = generateID("ce")
	e.ExecutedAt = models.NowStr()
	_, err := database.DB.Exec(
		`INSERT INTO case_executions (id, case_id, case_name, executor, result, steps, duration, remark, executed_at, plan_id, execution_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.CaseID, e.CaseName, e.Executor, e.Result, e.Steps, e.Duration, e.Remark, e.ExecutedAt, e.PlanID, e.ExecutionID,
	)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}

	c.JSON(http.StatusCreated, models.APIResponse{Success: true, Data: e})
}

func UpdateCaseExecution(c *gin.Context) {
	id := c.Param("id")
	var e models.CaseExecution
	if err := c.ShouldBindJSON(&e); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	_, err := database.DB.Exec(
		`UPDATE case_executions SET case_id=?, case_name=?, executor=?, result=?, steps=?, duration=?, remark=?
		 WHERE id=?`,
		e.CaseID, e.CaseName, e.Executor, e.Result, e.Steps, e.Duration, e.Remark, id,
	)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}
	e.ID = id

	// 若该用例执行关联了测试计划，更新后重新聚合计划执行结果
	var planID string
	if err := database.DB.QueryRow("SELECT plan_id FROM case_executions WHERE id = ?", id).Scan(&planID); err == nil && planID != "" {
		aggregatePlanExecution(planID)
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: e})
}

func DeleteCaseExecution(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec("DELETE FROM case_executions WHERE id = ?", id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: nil})
}
