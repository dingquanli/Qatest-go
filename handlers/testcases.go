package handlers

import (
	"net/http"

	"qatest/models"
	"qatest/repository"

	"github.com/gin-gonic/gin"
)

// --- 测试用例 ---

func GetTestCases(c *gin.Context) {
	// 优化：添加 LIMIT 分页，默认 100 条
	limit := 100
	cases, err := repository.ListTestCases(limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: cases})
}

func GetTestCase(c *gin.Context) {
	tc, err := repository.GetTestCase(c.Param("id"))
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

	// 服务端校验枚举字段，拒绝非法取值（空值视为未设置，允许）
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

	if err := repository.CreateTestCase(tc); err != nil {
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

	// 服务端校验枚举字段，拒绝非法取值（空值视为未设置，允许）
	validEnum := func(v string, allowed ...string) bool { return v == "" || validateEnum(v, allowed...) }
	if !validEnum(tc.Priority, "P0", "P1", "P2", "P3") ||
		!validEnum(tc.Type, "functional", "performance", "security", "compatibility", "usability") ||
		!validEnum(tc.Status, "draft", "review", "approved", "archived") {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "非法的 priority/type/status 取值"})
		return
	}

	tc.UpdatedAt = models.NowStr()
	if err := repository.UpdateTestCase(id, tc); err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}

	tc.ID = id
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: tc})
}

func DeleteTestCase(c *gin.Context) {
	id := c.Param("id")
	if err := repository.DeleteTestCase(id); err != nil {
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

	// 事务整体收编于 repository（任一条失败整体回滚）
	imported, failed, err := repository.ImportTestCases(req.Cases)
	if failed > 0 {
		// 事务已整体回滚，实际未导入任何用例，响应需与之一致
		c.JSON(http.StatusOK, models.APIResponse{
			Success: false,
			Error:   "部分用例导入失败，已整体回滚",
			Data:    gin.H{"imported": 0, "failed": failed},
		})
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: gin.H{"imported": imported, "failed": failed}})
}

// --- 用例模块（SQL 收敛于 repository/modules.go） ---

func GetCaseModules(c *gin.Context)  { listModules(c, tblCaseModules) }
func CreateCaseModule(c *gin.Context) { createModule(c, tblCaseModules, "cm") }
func UpdateCaseModule(c *gin.Context) { updateModule(c, tblCaseModules) }
func DeleteCaseModule(c *gin.Context) { deleteModule(c, tblCaseModules) }

// --- 用例执行记录 ---

func GetCaseExecutions(c *gin.Context) {
	execs, err := repository.ListCaseExecutions()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: execs})
}

func GetCaseExecutionsStats(c *gin.Context) {
	stats, err := repository.CaseExecutionStats()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
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
	if err := repository.CreateCaseExecution(e); err != nil {
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

	if err := repository.UpdateCaseExecution(id, e); err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}
	e.ID = id

	// 若该用例执行关联了测试计划，更新后重新聚合计划执行结果
	if planID, err := repository.GetCaseExecutionPlanID(id); err == nil && planID != "" {
		aggregatePlanExecution(planID)
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: e})
}

func DeleteCaseExecution(c *gin.Context) {
	id := c.Param("id")
	if err := repository.DeleteCaseExecution(id); err != nil {
		respondError(c, http.StatusInternalServerError, err, "数据库操作失败")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: nil})
}
