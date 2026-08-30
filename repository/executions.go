package repository

import (
	"qatest/database"
	"qatest/models"
)

// —— 脚本执行记录（SQL 迁自 handlers/executions.go，语句原样保留） ——

// ListExecutions 执行记录列表
func ListExecutions() ([]models.Execution, error) {
	rows, err := database.DB.Query(
		`SELECT id, script_id, device_serial, task_name, status, logs, screenshots, duration, started_at, finished_at, created_at
		 FROM executions ORDER BY started_at DESC LIMIT 100`,
	)
	if err != nil {
		return nil, err
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
			return nil, err
		}
		executions = append(executions, e)
	}
	return executions, rows.Err()
}

// GetExecution 单条执行记录（不存在时返回 sql.ErrNoRows）
func GetExecution(id string) (models.Execution, error) {
	var e models.Execution
	err := database.DB.QueryRow(
		`SELECT id, script_id, device_serial, task_name, status, logs, screenshots, duration, started_at, finished_at, created_at
		 FROM executions WHERE id = ?`, id,
	).Scan(&e.ID, &e.ScriptID, &e.DeviceSerial, &e.TaskName, &e.Status, &e.Logs,
		&e.Screenshots, &e.Duration, &e.StartedAt, &e.FinishedAt, &e.CreatedAt)
	return e, err
}

// GetScriptBasic 查询脚本基本字段（迁自 handlers/executions.go CreateExecution，语句原样保留；
// 不存在时返回 sql.ErrNoRows）
func GetScriptBasic(id string) (models.Script, error) {
	var s models.Script
	err := database.DB.QueryRow(
		"SELECT id, name, description, language, code FROM scripts WHERE id = ?", id,
	).Scan(&s.ID, &s.Name, &s.Description, &s.Language, &s.Code)
	return s, err
}

// CreateExecution 插入执行记录（ID/时间戳由调用方填充）。
// handlers/testplans.go 的计划执行引擎（dispatchCaseScript）亦使用同一条语句，一并复用本函数。
func CreateExecution(e models.Execution) error {
	_, err := database.DB.Exec(
		`INSERT INTO executions (id, script_id, device_serial, task_name, status, logs, screenshots, duration, started_at, finished_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.ScriptID, e.DeviceSerial, e.TaskName, e.Status, e.Logs,
		e.Screenshots, e.Duration, e.StartedAt, e.FinishedAt, e.CreatedAt,
	)
	return err
}

// GetExecutionStatus 查询执行记录的当前状态（迁自 handlers/executions.go CancelExecution，语句原样保留；
// 不存在时返回 sql.ErrNoRows）
func GetExecutionStatus(id string) (string, error) {
	var execID, status string
	err := database.DB.QueryRow("SELECT id, status FROM executions WHERE id = ?", id).Scan(&execID, &status)
	return status, err
}

// CancelExecution 将执行记录置为 cancelled 并记录结束时间（迁自 handlers/executions.go CancelExecution）
func CancelExecution(id, finishedAt string) error {
	_, err := database.DB.Exec("UPDATE executions SET status='cancelled', finished_at=? WHERE id=?", finishedAt, id)
	return err
}
