package handlers

import (
	"net/http"

	"qatest/models"
	"qatest/repository"

	"github.com/gin-gonic/gin"
)

// 注：旧「表格视图用例」/table-cases、/table-modules 已随 WPS 风格重构
// （表格用例视图改为操作电子表格）于 2026-08-30 移除；数据库表 table_cases /
// table_modules 保留（存量数据不丢）。如需恢复接口，SQL 可参考 git 历史。

// --- XMind 视图用例 ---

func GetXmindCases(c *gin.Context) {
	cases, err := repository.ListXmindCases()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: cases})
}

func CreateXmindCase(c *gin.Context) {
	var x models.XmindCase
	if err := c.ShouldBindJSON(&x); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	x.ID = generateID("xc")
	x.CreatedAt = models.NowStr()
	x.UpdatedAt = x.CreatedAt
	if x.Priority == "" {
		x.Priority = "P2"
	}
	if x.Type == "" {
		x.Type = "functional"
	}
	if x.Status == "" {
		x.Status = "draft"
	}
	if err := repository.CreateXmindCase(x); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusCreated, models.APIResponse{Success: true, Data: x})
}

func UpdateXmindCase(c *gin.Context) {
	id := c.Param("id")
	var x models.XmindCase
	if err := c.ShouldBindJSON(&x); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	x.UpdatedAt = models.NowStr()
	if x.Priority == "" {
		x.Priority = "P2"
	}
	if x.Type == "" {
		x.Type = "functional"
	}
	if x.Status == "" {
		x.Status = "draft"
	}
	if err := repository.UpdateXmindCase(id, x); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	x.ID = id
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: x})
}

// ReplaceXmindCases 整体替换（用于撤销/重做：先清空再按客户端快照全量写回，保留原 id）。
func ReplaceXmindCases(c *gin.Context) {
	var list []models.XmindCase
	if err := c.ShouldBindJSON(&list); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	if len(list) > 2000 {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "节点数量超限"})
		return
	}
	// 填充 ID/默认值/时间戳后交给 repository（事务内清空 + 全量写回）；
	// 与原实现一致，range 拷贝上的填充不回写 list，响应仍返回客户端原始快照。
	now := models.NowStr()
	filled := make([]models.XmindCase, 0, len(list))
	for _, x := range list {
		if x.ID == "" {
			x.ID = generateID("xc")
		}
		if x.Priority == "" {
			x.Priority = "P2"
		}
		if x.Type == "" {
			x.Type = "functional"
		}
		if x.Status == "" {
			x.Status = "draft"
		}
		if x.CreatedAt == "" {
			x.CreatedAt = now
		}
		x.UpdatedAt = now
		filled = append(filled, x)
	}
	if err := repository.ReplaceXmindCases(filled); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: list})
}

func DeleteXmindCase(c *gin.Context) {
	id := c.Param("id")
	if err := repository.DeleteXmindNode(id); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: nil})
}

// --- XMind 视图模块（SQL 收敛于 repository/modules.go；删除前先把模块下用例移至「未分类」） ---

func GetXmindModules(c *gin.Context)  { listModules(c, tblXmindModules) }
func CreateXmindModule(c *gin.Context) { createModule(c, tblXmindModules, "xm") }
func UpdateXmindModule(c *gin.Context) { updateModule(c, tblXmindModules) }

func DeleteXmindModule(c *gin.Context) {
	id := c.Param("id")
	// 删除模块前，先把该模块下的用例移至「未分类」(module_id='')，避免用例被级联丢失
	if err := repository.ClearXmindCasesModuleRef(id); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	deleteModule(c, tblXmindModules)
}
