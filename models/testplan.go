package models

// TestPlan 测试计划
type TestPlan struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CaseIDs     string `json:"caseIds"` // JSON: string[]
	Status      string `json:"status"`  // draft/in_progress/completed/cancelled
	StartDate   string `json:"startDate"`
	EndDate     string `json:"endDate"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// CaseExecution 用例执行记录
type CaseExecution struct {
	ID          string `json:"id"`
	CaseID      string `json:"caseId"`
	CaseName    string `json:"caseName"`
	Executor    string `json:"executor"`
	Result      string `json:"result"` // pending/passed/failed/skipped/blocked
	Steps       string `json:"steps"`  // JSON: [{action,expected,actual,status,screenshot}]
	Duration    int    `json:"duration"`
	Remark      string `json:"remark"`
	ExecutedAt  string `json:"executedAt"`
	PlanID      string `json:"planId"`      // 关联测试计划（计划执行引擎）
	ExecutionID string `json:"executionId"` // 关联脚本执行任务（自动模式）
}

// PlanExecution 计划执行记录
type PlanExecution struct {
	ID          string `json:"id"`
	PlanID      string `json:"planId"`
	PlanName    string `json:"planName"`
	Status      string `json:"status"` // pending/running/completed/failed
	Result      string `json:"result"` // JSON
	CasesTotal  int    `json:"casesTotal"`
	CasesPassed int    `json:"casesPassed"`
	CasesFailed int    `json:"casesFailed"`
	ExecutedBy  string `json:"executedBy"`
	FinishedAt  string `json:"finishedAt"`
	CasesDetail string `json:"casesDetail"`         // JSON: [{caseId,caseName,result,remark}]
	Duration    int    `json:"duration"`
	StartedAt   string `json:"startedAt"`
	CreatedAt   string `json:"createdAt"`
}

// AutoTaskExecution 自动化任务执行记录
type AutoTaskExecution struct {
	ID         string `json:"id"`
	TaskID     string `json:"taskId"`
	TaskName   string `json:"taskName"`
	Status     string `json:"status"`
	Result     string `json:"result"`
	Logs       string `json:"logs"` // JSON
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt"`
	CreatedAt  string `json:"createdAt"`
}

