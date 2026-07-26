package models

// Bug 缺陷
type Bug struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Severity      string `json:"severity"`  // critical/major/minor/trivial
	Priority      string `json:"priority"`  // P0/P1/P2/P3
	Status        string `json:"status"`    // open/in_progress/resolved/closed/reopened
	Assignee      string `json:"assignee"`
	Reporter      string `json:"reporter"`
	Module        string `json:"module"`
	Env           string `json:"env"`
	Description   string `json:"description"`
	Steps         string `json:"steps"`
	Expected      string `json:"expected"`
	Actual        string `json:"actual"`
	Tags          string `json:"tags"` // JSON: string[]
	RelatedCaseID string `json:"relatedCaseId"`
	ExternalID    string `json:"externalId"`
	ExternalURL   string `json:"externalUrl"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

// BugSyncRequest Jira 同步请求
type BugSyncRequest struct {
	Bug      Bug        `json:"bug"`
	Platform string     `json:"platform"` // jira
	Jira     JiraConfig `json:"jira"`
}

// JiraConfig Jira 配置
type JiraConfig struct {
	BaseURL  string `json:"baseUrl"`
	Username string `json:"username"`
	APIToken string `json:"apiToken"`
	Project  string `json:"project"`
}
