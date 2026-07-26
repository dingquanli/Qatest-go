package models

// Script 自动化脚本
type Script struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Language    string `json:"language"` // python / shell / javascript
	Code        string `json:"code"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// Execution 脚本执行记录
type Execution struct {
	ID           string `json:"id"`
	ScriptID     string `json:"scriptId"`
	DeviceSerial string `json:"deviceSerial"`
	TaskName     string `json:"taskName"`
	Status       string `json:"status"` // pending / running / success / failed / cancelled
	Logs         string `json:"logs"`   // JSON 数组
	Screenshots  string `json:"screenshots"` // JSON: string[]
	Duration     int    `json:"duration"`
	StartedAt    string `json:"startedAt"`
	FinishedAt   string `json:"finishedAt,omitempty"`
	CreatedAt    string `json:"createdAt"`
}

// CreateExecutionRequest 创建执行请求
type CreateExecutionRequest struct {
	ScriptID     string `json:"scriptId" binding:"required"`
	DeviceSerial string `json:"deviceSerial"`
	TaskName     string `json:"taskName"`
}

