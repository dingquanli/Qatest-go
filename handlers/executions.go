package handlers

import (
	"context"
	"net/http"
	"time"

	"qatest/database"
	"qatest/models"
	"qatest/services"

	"github.com/gin-gonic/gin"
)

// GetExecutions 执行记录列表
func GetExecutions(c *gin.Context) {
	rows, err := database.DB.Query(
		`SELECT id, script_id, device_serial, task_name, status, logs, screenshots, duration, started_at, finished_at, created_at
		 FROM executions ORDER BY started_at DESC LIMIT 100`,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}
	defer rows.Close()

	executions := make([]models.Execution, 0)
	for rows.Next() {
		var e models.Execution
		// SELECT 列与 Scan 目标数量必须一致：id, script_id, device_serial, task_name,
		// status, logs, screenshots, duration, started_at, finished_at, created_at
		if err := rows.Scan(
			&e.ID, &e.ScriptID, &e.DeviceSerial, &e.TaskName, &e.Status, &e.Logs,
			&e.Screenshots, &e.Duration, &e.StartedAt, &e.FinishedAt, &e.CreatedAt,
		); err != nil {
			c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
			return
		}
		executions = append(executions, e)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: executions})
}

// GetExecution 单条执行记录
func GetExecution(c *gin.Context) {
	id := c.Param("id")
	var e models.Execution
	err := database.DB.QueryRow(
		`SELECT id, script_id, device_serial, task_name, status, logs, screenshots, duration, started_at, finished_at, created_at
		 FROM executions WHERE id = ?`, id,
	).Scan(&e.ID, &e.ScriptID, &e.DeviceSerial, &e.TaskName, &e.Status, &e.Logs,
		&e.Screenshots, &e.Duration, &e.StartedAt, &e.FinishedAt, &e.CreatedAt)

	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{Success: false, Error: "执行记录不存在"})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: e})
}

// CreateExecution 创建并执行脚本
func CreateExecution(c *gin.Context) {
	var req models.CreateExecutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	// 查询脚本
	var script models.Script
	err := database.DB.QueryRow(
		"SELECT id, name, description, language, code FROM scripts WHERE id = ?", req.ScriptID,
	).Scan(&script.ID, &script.Name, &script.Description, &script.Language, &script.Code)
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

	_, err = database.DB.Exec(
		`INSERT INTO executions (id, script_id, device_serial, task_name, status, logs, screenshots, duration, started_at, finished_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		exec.ID, exec.ScriptID, exec.DeviceSerial, exec.TaskName, exec.Status, exec.Logs,
		exec.Screenshots, exec.Duration, exec.StartedAt, exec.FinishedAt, exec.CreatedAt,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
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

	var exec models.Execution
	err := database.DB.QueryRow("SELECT id, status FROM executions WHERE id = ?", id).Scan(&exec.ID, &exec.Status)
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{Success: false, Error: "执行记录不存在"})
		return
	}

	if exec.Status != "running" {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "任务未在运行中"})
		return
	}

	services.Executor.Cancel(id)

	now := time.Now().Format(time.RFC3339)
	if _, err := database.DB.Exec("UPDATE executions SET status='cancelled', finished_at=? WHERE id=?", now, id); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true})
}
