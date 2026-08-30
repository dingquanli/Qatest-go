package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"qatest/config"
	"qatest/models"
	"qatest/repository"
	"qatest/services"

	"github.com/gin-gonic/gin"
)

// GetBugs 缺陷列表
func GetBugs(c *gin.Context) {
	bugs, err := repository.ListBugs()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: bugs})
}

// GetBugStats 缺陷统计
func GetBugStats(c *gin.Context) {
	stats, err := repository.GetBugStats()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: stats})
}

// GetBug 缺陷详情
func GetBug(c *gin.Context) {
	id := c.Param("id")
	b, err := repository.GetBug(id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{Success: false, Error: "缺陷不存在"})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: b})
}

// CreateBug 创建缺陷
func CreateBug(c *gin.Context) {
	var b models.Bug
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	b.ID = generateID("bug")
	b.CreatedAt = models.NowStr()
	b.UpdatedAt = b.CreatedAt

	if err := repository.CreateBug(b); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}

	c.JSON(http.StatusCreated, models.APIResponse{Success: true, Data: b})
}

// UpdateBug 更新缺陷
func UpdateBug(c *gin.Context) {
	id := c.Param("id")
	var b models.Bug
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}

	b.UpdatedAt = models.NowStr()
	if err := repository.UpdateBug(id, b); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}

	b.ID = id
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: b})
}

// DeleteBug 删除缺陷
func DeleteBug(c *gin.Context) {
	id := c.Param("id")
	if err := repository.DeleteBug(id); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: nil})
}

// jiraRuntimeConfig Jira 运行期配置（env 为源，系统设置表可覆盖）
type jiraRuntimeConfig struct {
	BaseURL  string
	Email    string
	APIToken string
	Project  string
	Enabled  bool
}

// loadJiraConfig 读取 Jira 配置：先取环境变量，再由 settings 表覆盖（允许 UI 配置）。
func loadJiraConfig() jiraRuntimeConfig {
	cfg := jiraRuntimeConfig{
		BaseURL:  config.AppConfig.JiraBaseURL,
		Email:    config.AppConfig.JiraEmail,
		APIToken: config.AppConfig.JiraAPIToken,
		Project:  config.AppConfig.JiraProject,
	}
	overrides := map[string]*string{
		"jira_base_url":  &cfg.BaseURL,
		"jira_email":     &cfg.Email,
		"jira_api_token": &cfg.APIToken,
		"jira_project":   &cfg.Project,
	}
	for key, ptr := range overrides {
		if v, err := repository.GetSetting(key); err == nil && v != "" {
			*ptr = v
		}
	}
	cfg.Enabled = cfg.BaseURL != "" && cfg.Email != "" && cfg.APIToken != "" && cfg.Project != ""
	return cfg
}

// GetJiraStatus 返回 Jira 是否已配置（脱敏，不暴露 token）
func GetJiraStatus(c *gin.Context) {
	cfg := loadJiraConfig()
	masked := cfg.BaseURL
	if masked != "" {
		if u, err := url.Parse(cfg.BaseURL); err == nil {
			masked = u.Scheme + "://" + u.Host
		}
	}
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: gin.H{
		"configured": cfg.Enabled,
		"baseUrl":    masked,
		"project":    cfg.Project,
	}})
}

// mapJiraPriority 将内部优先级映射到 Jira 优先级名
func mapJiraPriority(p string) string {
	switch p {
	case "P0":
		return "Highest"
	case "P1":
		return "High"
	case "P2":
		return "Medium"
	case "P3":
		return "Low"
	default:
		return "Medium"
	}
}

// buildJiraDescription 拼装 Jira 缺陷描述（Atlassian 纯文本格式，兼容 /rest/api/2/issue）
func buildJiraDescription(b models.Bug) string {
	var sb strings.Builder
	if b.Description != "" {
		sb.WriteString(b.Description + "\n\n")
	}
	if b.Steps != "" {
		sb.WriteString("重现步骤:\n" + b.Steps + "\n\n")
	}
	if b.Expected != "" {
		sb.WriteString("期望结果:\n" + b.Expected + "\n\n")
	}
	if b.Actual != "" {
		sb.WriteString("实际结果:\n" + b.Actual + "\n\n")
	}
	if b.Module != "" {
		sb.WriteString("模块: " + b.Module)
	}
	return sb.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// SyncBugToJira 同步缺陷到 Jira（真实 REST 调用，未配置时返回 409 明确提示）
func SyncBugToJira(c *gin.Context) {
	var req models.BugSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "参数错误"})
		return
	}
	// 优先使用 URL 路径中的 id（前端以 /bugs/:id/sync 调用），回退到请求体
	bugID := c.Param("id")
	if bugID == "" {
		bugID = req.Bug.ID
	}
	if bugID == "" {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "缺陷 ID 缺失"})
		return
	}

	jc := loadJiraConfig()
	if !jc.Enabled {
		c.JSON(http.StatusConflict, models.APIResponse{
			Success: false,
			Error:   "Jira 未配置：请在「系统设置 → Jira 同步」填写地址、账号、API Token 与项目 Key",
		})
		return
	}

	// 从库读取最新缺陷
	b, err := repository.GetBugForSync(bugID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{Success: false, Error: "缺陷不存在"})
		return
	}

	// 构建 Jira issue 报文
	fields := map[string]any{
		"project":     map[string]any{"key": jc.Project},
		"summary":     b.Title,
		"description": buildJiraDescription(b),
		"issuetype":   map[string]any{"name": "Bug"},
		"priority":    map[string]any{"name": mapJiraPriority(b.Priority)},
	}
	if b.Tags != "" {
		var tags []string
		if json.Unmarshal([]byte(b.Tags), &tags) == nil && len(tags) > 0 {
			fields["labels"] = tags
		}
	}
	issue := map[string]any{"fields": fields}
	bodyBytes, err := json.Marshal(issue)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: "序列化失败"})
		return
	}

	endpoint := strings.TrimRight(jc.BaseURL, "/") + "/rest/api/2/issue"
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(jc.Email+":"+jc.APIToken))
	headers := map[string]string{
		"Authorization": auth,
		"Content-Type":  "application/json",
		"Accept":        "application/json",
	}

	res := services.ProxyInstance.SendHTTPRequest("POST", endpoint, headers, nil, string(bodyBytes), 30)
	if res.Error != "" {
		c.JSON(http.StatusBadGateway, models.APIResponse{Success: false, Error: "Jira 请求失败: " + res.Error})
		return
	}
	if res.Status < 200 || res.Status >= 300 {
		c.JSON(http.StatusBadGateway, models.APIResponse{
			Success: false,
			Error:   fmt.Sprintf("Jira 返回 %d: %s", res.Status, truncate(res.Body, 500)),
		})
		return
	}

	var jr struct {
		ID   string `json:"id"`
		Key  string `json:"key"`
		Self string `json:"self"`
	}
	if umErr := json.Unmarshal([]byte(res.Body), &jr); umErr != nil {
		c.JSON(http.StatusBadGateway, models.APIResponse{Success: false, Error: "Jira 响应解析失败: " + umErr.Error()})
		return
	}

	externalURL := jr.Self
	if jr.Key != "" {
		externalURL = strings.TrimRight(jc.BaseURL, "/") + "/browse/" + jr.Key
	}
	if err := repository.UpdateBugExternal(bugID, jr.Key, externalURL, models.NowStr()); err != nil {
		respondError(c, http.StatusInternalServerError, err, "回写外部链接失败")
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: gin.H{
		"externalId":  jr.Key,
		"externalUrl": externalURL,
	}})
}
