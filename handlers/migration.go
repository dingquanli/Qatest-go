package handlers

import (
	"encoding/json"
	"net/http"

	"qatest/models"
	"qatest/repository"

	"github.com/gin-gonic/gin"
)

// GetMigrationStatus 检查是否需要迁移
// P1：真实计算迁移状态，不再硬编码返回 false。通过检查核心数据表是否已创建来判断。
func GetMigrationStatus(c *gin.Context) {
	coreTables := []string{"scripts", "executions", "test_cases", "bugs", "api_definitions", "settings"}
	missing := make([]string, 0)
	for _, t := range coreTables {
		if err := repository.TableExists(t); err != nil {
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
// P1：事务化批量写入由 repository.ImportMigrationData 完成（逐条检查 Exec 错误，任一失败回滚并返回真实 {imported, failed}）。
func ImportMigration(c *gin.Context) {
	var req map[string]json.RawMessage
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	imported, failed, err := repository.ImportMigrationData(req)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: gin.H{"imported": imported, "failed": failed}})
}
