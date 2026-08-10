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
	Priority     string `json:"priority"`
	Type         string `json:"type"`
	Precondition string `json:"precondition"`
	Steps        string `json:"steps"`   // JSON
	Expected     string `json:"expected"`
	Assignee     string `json:"assignee"`
	Status       string `json:"status"`
	Tags         string `json:"tags"`
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
type Spreadsheet struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Cells     string `json:"cells"` // JSON: [["",""],["",""]]
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
