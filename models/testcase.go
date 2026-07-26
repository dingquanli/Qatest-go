package models

// TestCase 测试用例
type TestCase struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	ModuleID     string `json:"moduleId"`
	Priority     string `json:"priority"` // P0/P1/P2/P3
	Type         string `json:"type"`     // functional/performance/security/compatibility/usability
	Precondition string `json:"precondition"`
	Steps        string `json:"steps"` // JSON: [{action, expected}]
	Assignee     string `json:"assignee"`
	Status       string `json:"status"` // draft/review/approved/archived
	Tags         string `json:"tags"`   // JSON: string[]
	ScriptID     string `json:"scriptId"` // 关联自动化脚本（测试计划自动执行模式使用）
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

// CaseModule 用例模块
type CaseModule struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ParentID  string `json:"parentId"`
	SortOrder int    `json:"sort_order"`
	CreatedAt string `json:"createdAt"`
}

// BatchCaseImport 批量导入用例请求
type BatchCaseImport struct {
	Cases []TestCase `json:"cases" binding:"required"`
}
