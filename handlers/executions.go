package handlers

import (
	"context"
	"net/http"
	"time"

	"qatest/config"
	"qatest/models"
	"qatest/repository"
	"qatest/services"

	"github.com/gin-gonic/gin"
)

// GetExecutions 执行记录列表
func GetExecutions(c *gin.Context) {
	executions, err := repository.ListExecutions()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: executions})
}

// GetExecution 单条执行记录
func GetExecution(c *gin.Context) {
	e, err := repository.GetExecution(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{Success: false, Error: "执行记录不存在"})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: e})
}

// CreateExecution 创建并执行脚本
func CreateExecution(c *gin.Context) {
	// RCE 高危能力熔断：EXECUTOR_ENABLED 未显式开启时拒绝创建任何脚本执行任务（默认关闭）
	if !config.AppConfig.ExecutorEnabled {
		c.JSON(http.StatusForbidden, models.APIResponse{Success: false, Error: "脚本执行引擎已禁用（默认关闭）。如确需执行脚本，请管理员在 .env 中显式设置 EXECUTOR_ENABLED=1 并重启服务"})
		return
	}

	var req models.CreateExecutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	// 查询脚本
	script, err := repository.GetScriptBasic(req.ScriptID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{Success: false, Error: "脚本不存在"})
		return
	}

	now := time.Now().Format(time.RFC3339)
	exec := models.Execution{
		ID:           generateID("exec"),
		ScriptID:     req.ScriptID,
		DeviceSerial: req.DeviceSerial,
		TaskName:     req.TaskName,
		Status:       "running",
		Logs:         "[]",
		Screenshots:  "[]",
		Duration:     0,
		StartedAt:    now,
		FinishedAt:   "",
		CreatedAt:    now,
	}

	if err := repository.CreateExecution(exec); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}

	// 异步执行（通过 executor 运行）
	ctx, cancel := context.WithCancel(context.Background())
	task := &services.ExecutionTask{
		ID:           exec.ID,
		ScriptID:     req.ScriptID,
		DeviceSerial: req.DeviceSerial,
		TaskName:     req.TaskName,
		Language:     script.Language,
		Code:         script.Code,
		LogChan:      make(chan services.LogEntry, 100),
		Ctx:          ctx,
		Cancel:       cancel,
	}
	services.Executor.Start(task)

	c.JSON(http.StatusCreated, models.APIResponse{Success: true, Data: exec})
}

// CancelExecution 取消执行
func CancelExecution(c *gin.Context) {
	id := c.Param("id")

	status, err := repository.GetExecutionStatus(id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{Success: false, Error: "执行记录不存在"})
		return
	}

	if status != "running" {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "任务未在运行中"})
		return
	}

	services.Executor.Cancel(id)

	now := time.Now().Format(time.RFC3339)
	if err := repository.CancelExecution(id, now); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true})
}
