package handlers

import (
	"net/http"

	"qatest/models"
	"qatest/repository"

	"github.com/gin-gonic/gin"
)

// 模块/文件夹 CRUD 公共实现。
// 四张模块表（case_modules / api_def_modules / api_folders / xmind_modules）
// 结构同构（id/name/parent_id/sort_order/created_at）；SQL 已整体迁至 repository/modules.go
// （ListModuleRows / InsertModuleRow / UpdateModuleRow / DeleteModuleRow），
// 本文件只保留表名常量与「请求解析 → 调用 repository → 响应/错误映射」的薄封装，
// 各 handler 退化为「表名常量 + ID 前缀」的一行调用。
// 表名只能使用下方常量（编译期固定，非用户输入），杜绝拼接注入。

const (
	tblCaseModules   = "case_modules"
	tblAPIDefModules = "api_def_modules"
	tblAPIFolders    = "api_folders"
	tblXmindModules  = "xmind_modules"
	// 注：tblTableModules 已随 /table-modules API 移除（前端改用电子表格）；
	// 数据库表 table_modules 保留。
)

// listModules GET /xxx-modules：按 sort_order 返回全量模块
func listModules(c *gin.Context, table string) {
	mods, err := repository.ListModuleRows(table)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: mods})
}

// createModule POST /xxx-modules：绑定请求体并插入
func createModule(c *gin.Context, table, idPrefix string) {
	var m models.ModuleNode
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	m.ID = generateID(idPrefix)
	m.CreatedAt = models.NowStr()
	if err := repository.InsertModuleRow(table, m); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusCreated, models.APIResponse{Success: true, Data: m})
}

// updateModule PUT /xxx-modules/:id
func updateModule(c *gin.Context, table string) {
	id := c.Param("id")
	var m models.ModuleNode
	if err := c.ShouldBindJSON(&m); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	if err := repository.UpdateModuleRow(table, id, m); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	m.ID = id
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: m})
}

// deleteModule DELETE /xxx-modules/:id
func deleteModule(c *gin.Context, table string) {
	id := c.Param("id")
	if err := repository.DeleteModuleRow(table, id); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: nil})
}
