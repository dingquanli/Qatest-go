package models

// APIDefinition 接口定义
type APIDefinition struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Method    string `json:"method"`
	URL       string `json:"url"`
	Tags      string `json:"tags"` // JSON
	ModuleID  string `json:"moduleId"`
	Headers   string `json:"headers"` // JSON
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// APIDefModule 接口定义模块
type APIDefModule struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ParentID  string `json:"parentId"`
	SortOrder int    `json:"sortOrder"`
	CreatedAt string `json:"createdAt"`
}

// APIRequest API 请求
type APIRequest struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Method      string `json:"method"`
	URL         string `json:"url"`
	Headers     string `json:"headers"` // JSON
	Params      string `json:"params"`  // JSON
	Body        string `json:"body"`
	Description string `json:"description"`
	Tags        string `json:"tags"` // JSON
	FolderID    string `json:"folderId"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// APIFolder API 文件夹
type APIFolder struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ParentID  string `json:"parentId"`
	SortOrder int    `json:"sortOrder"`
	CreatedAt string `json:"createdAt"`
}

// APIHistory API 请求历史
type APIHistory struct {
	ID         string `json:"id"`
	RequestID  string `json:"requestId"`
	Method     string `json:"method"`
	URL        string `json:"url"`
	Response   string `json:"response"`
	StatusCode int    `json:"statusCode"`
	Duration   int    `json:"duration"`
	CreatedAt  string `json:"createdAt"`
}

// TableCase 表格视图用例
type TableCase struct {
	ID           string `json:"id"`
	ModuleID     string `json:"moduleId"`
	Name         string `json:"name"`
	Priority     string `json:"priority"`
	Type         string `json:"type"`
	Precondition string `json:"precondition"`
	Steps        string `json:"steps"`
	Expected     string `json:"expected"`
	Assignee     string `json:"assignee"`
	Status       string `json:"status"`
	Tags         string `json:"tags"` // JSON
	SortOrder    int    `json:"sortOrder"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

// TableModule 表格视图模块
type TableModule struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ParentID  string `json:"parentId"`
	SortOrder int    `json:"sortOrder"`
	CreatedAt string `json:"createdAt"`
}

// XmindCase XMind 视图用例（纯文本逻辑图节点）
type XmindCase struct {
	ID           string `json:"id"`
	ModuleID     string `json:"moduleId"` // 兼容旧数据，纯文本逻辑图下留空
	ParentID     string `json:"parentId"` // 父节点 ID，空字符串表示中心主题（根）
	Name         string `json:"name"`
	Collapsed    bool   `json:"collapsed"` // 是否折叠（基础功能：折叠/展开子树）
	Priority     string `json:"priority"`
	Type         string `json:"type"`
	Precondition string `json:"precondition"`
	Steps        string `json:"steps"`   // JSON
	Expected     string `json:"expected"`
	Assignee     string `json:"assignee"`
	Status       string `json:"status"`
	Tags         string `json:"tags"`
	// 行业标准测试业务字段（完整版补齐）
	Code         string `json:"code"`         // 用例编号（人工可读，如 TC-LOGIN-001）
	TestData     string `json:"testData"`     // 输入数据
	ActualResult string `json:"actualResult"` // 实际结果（执行阶段填写）
	DefectId     string `json:"defectId"`     // 缺陷编号（关联缺陷单）
	Remark       string `json:"remark"`       // 备注/摘要
	Env          string `json:"env"`          // 测试环境
	Estimate     string `json:"estimate"`     // 预计工时
	SortOrder    int    `json:"sortOrder"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

// XmindModule XMind 视图模块
type XmindModule struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ParentID  string `json:"parentId"`
	SortOrder int    `json:"sortOrder"`
	CreatedAt string `json:"createdAt"`
}

// Spreadsheet 自由电子表格（纯文本网格，cells 为二维字符串数组 JSON）
// 基础格式层（与纯文本内容解耦）：formats 按 "r,c" 存单元格格式；col_widths/row_heights 存像素；merges 存合并区域。
type Spreadsheet struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Cells      string `json:"cells"`      // JSON: [["",""],["",""]]（纯文本内容）
	Formats    string `json:"formats"`    // JSON: {"r,c": {"bold":true,...}}
	ColWidths  string `json:"colWidths"`  // JSON: {"0":120,...}
	RowHeights string `json:"rowHeights"` // JSON: {"0":28,...}
	Merges     string `json:"merges"`     // JSON: [{"r":0,"c":0,"rs":2,"cs":1},...]
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}
