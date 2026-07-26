package handlers

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"qatest/database"
	"qatest/models"

	"github.com/gin-gonic/gin"
)

// SDK engines and their files (simplified, mirrors the Node.js SDK module)
var sdkEngines = map[string]map[string]string{
	"unity": {
		"label": "Unity (C#)",
		"dir":   "sdk/unity",
		"files": "QaSDK.cs,QaConfig.cs,QaLogger.cs",
	},
	"unreal": {
		"label": "Unreal Engine (C++)",
		"dir":   "sdk/unreal",
		"files": "QaSDK.h,QaSDK.cpp",
	},
	"cocos": {
		"label": "Cocos Creator (TypeScript)",
		"dir":   "sdk/cocos",
		"files": "QaSDK.ts,QaConfig.ts",
	},
	"android": {
		"label": "Android (Java)",
		"dir":   "sdk/android",
		"files": "QaSDK.java",
	},
	"python": {
		"label": "Python",
		"dir":   "sdk/python",
		"files": "qa_sdk.py,qa_config.py",
	},
	"go": {
		"label": "Go",
		"dir":   "sdk/go",
		"files": "sdk.go,config.go",
	},
	"nodejs": {
		"label": "Node.js (JavaScript)",
		"dir":   "sdk/nodejs",
		"files": "index.js,config.js",
	},
}

// GetSDKList 获取 SDK 引擎和文件列表
func GetSDKList(c *gin.Context) {
	type EngineInfo struct {
		ID         string   `json:"id"`
		Label      string   `json:"label"`
		Files      []string `json:"files"`
		ReportToken string  `json:"reportToken"`
	}

	reportToken := getReportToken()
	engines := make([]EngineInfo, 0)
	for id, info := range sdkEngines {
		files := splitAndTrim(info["files"], ",")
		engines = append(engines, EngineInfo{ID: id, Label: info["label"], Files: files, ReportToken: reportToken})
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: engines})
}

// DownloadSDK 下载 SDK 源文件
func DownloadSDK(c *gin.Context) {
	engine := c.Query("engine")
	filename := c.Query("file")

	// 验证引擎 ID
	info, ok := sdkEngines[engine]
	if !ok {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "未知引擎"})
		return
	}

	// 验证文件名
	files := splitAndTrim(info["files"], ",")
	found := false
	for _, f := range files {
		if f == filename {
			found = true
			break
		}
	}
	if !found {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "未知文件"})
		return
	}

	sdkBase := "sdk"
	filePath := filepath.Join(sdkBase, engine, filename)

	data, err := os.ReadFile(filePath)
	if err != nil {
		// 文件不存在时返回占位内容
		placeholder := "// " + info["label"] + " SDK - " + filename + "\n// This is a placeholder file.\n// Please replace with the actual SDK implementation.\n"
		c.Header("Content-Disposition", "attachment; filename="+filename)
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.String(http.StatusOK, placeholder)
		return
	}

	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Data(http.StatusOK, "text/plain; charset=utf-8", data)
}

// splitAndTrim 辅助函数（使用 stdlib 替代手写 splitString/trimSpace）
func splitAndTrim(s, sep string) []string {
	if s == "" {
		return nil
	}
	parts := make([]string, 0)
	for _, p := range strings.Split(s, sep) {
		p = strings.TrimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

// ReceiveReport 接收各引擎 SDK 上报的数据（POST /api/qa/report）。
//
// 完整协议（与 FileApiLogger.cs 模板对齐）：
//   - 用例结果：event=case_result，字段 { name, result(passed|failed), message, tags }
//   - 自由日志：event=log，字段 { name, message, tags }
//   - API 拦截事件（对应 FileApiLogger 的 REQUEST / RESPONSE / ERROR）：
//       event=request|response|error（也可由 type=REQUEST|RESPONSE|ERROR 映射得出）
//       字段 { method, headers, request, response, error, elapsed_ms, seq, ts, tags }
//
// 所有字段通过 Authorization: Bearer <token> 携带上报令牌，落库 qa_reports。
// request / response / headers 中的敏感字段（token/password/secret/apikey...）
// 在落库前自动脱敏（对应 FileApiLogger.cs 的 MaskSensitiveFields）。
func ReceiveReport(c *gin.Context) {
	var payload struct {
		Event     string         `json:"event"`
		Name      string         `json:"name"`
		Result    string         `json:"result"`
		Message   string         `json:"message"`
		Tags      map[string]any `json:"tags"`
		Timestamp int64          `json:"timestamp"`
		// gRPC / API 拦截事件字段（镜像 FileApiLogger.cs 的 BuildEntry）
		Seq       int64          `json:"seq"`
		Ts        string         `json:"ts"`
		Type      string         `json:"type"`
		Method    string         `json:"method"`
		Headers   json.RawMessage `json:"headers"`
		Request   json.RawMessage `json:"request"`
		Response  json.RawMessage `json:"response"`
		Error     string         `json:"error"`
		ElapsedMs float64        `json:"elapsed_ms"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondError(c, http.StatusBadRequest, err, "请求参数错误")
		return
	}

	// 事件类型归一：type=REQUEST|RESPONSE|ERROR 映射到对应 event 值
	event := strings.ToLower(strings.TrimSpace(payload.Event))
	if payload.Type != "" {
		switch strings.ToUpper(strings.TrimSpace(payload.Type)) {
		case "REQUEST":
			event = "request"
		case "RESPONSE":
			event = "response"
		case "ERROR":
			event = "error"
		}
	}
	if event == "" {
		event = "case_result"
	}

	// 名称：拦截事件以 method 作为标识；用例结果 / 日志以 name 为准
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		name = strings.TrimSpace(payload.Method)
	}
	if name == "" {
		c.JSON(http.StatusBadRequest, models.APIResponse{Success: false, Error: "name 或 method 不能为空"})
		return
	}

	result := strings.TrimSpace(payload.Result)
	if result == "" {
		if event == "error" {
			result = "error"
		} else {
			result = "passed"
		}
	}

	// 解析 Bearer token（SDK 上报携带的鉴权令牌）
	token := ""
	if ah := c.GetHeader("Authorization"); ah != "" {
		if len(ah) > 7 && strings.EqualFold(ah[:7], "Bearer ") {
			token = ah[7:]
		} else {
			token = ah
		}
	}

	// 上报令牌校验：必须与服务端 settings.report_token 一致，否则拒绝。
	// 关闭「任意客户端匿名灌库」的鉴权缺口（P1-1）。
	ensureReportToken()
	if !validReportToken(token) {
		c.JSON(http.StatusUnauthorized, models.APIResponse{Success: false, Error: "上报令牌无效，请使用「下载 SDK」页显示的上报 Token"})
		return
	}

	// tags 序列化（空 tags 落库为 {}）
	tagsJSON := []byte("{}")
	if payload.Tags != nil {
		if b, err := json.Marshal(payload.Tags); err == nil && len(b) > 0 {
			tagsJSON = b
		}
	}

	// 拦截事件字段：原始 JSON 转字符串 + 敏感字段脱敏
	headersStr := maskSensitiveJSON(rawToString(payload.Headers))
	reqStr := maskSensitiveJSON(rawToString(payload.Request))
	respStr := maskSensitiveJSON(rawToString(payload.Response))
	errStr := payload.Error

	ts := payload.Timestamp
	if ts == 0 {
		ts = time.Now().UnixMilli()
	}

	id := generateID("qr")
	_, err := database.DB.Exec(
		`INSERT INTO qa_reports
		 (id, event, name, result, message, tags, token, source, timestamp, created_at,
		  seq, method, headers, req_body, resp_body, err_msg, elapsed_ms, ts)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, event, name, result, payload.Message,
		string(tagsJSON), token, c.ClientIP(), ts, models.NowStr(),
		payload.Seq, strings.TrimSpace(payload.Method), headersStr, reqStr, respStr, errStr, payload.ElapsedMs, payload.Ts,
	)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "写入上报数据失败")
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: gin.H{"id": id}})
}

// rawToString 把 json.RawMessage 转成可存储字符串；空值返回 "null"。
func rawToString(r json.RawMessage) string {
	if len(r) == 0 {
		return "null"
	}
	return string(r)
}

// sensitiveKeySet 敏感字段名（落库前脱敏，对应 FileApiLogger.cs 的 SensitiveFieldNames）。
var sensitiveKeySet = map[string]bool{
	"credential":  true,
	"authtoken":   true,
	"token":       true,
	"password":    true,
	"secret":      true,
	"apikey":      true,
	"key":         true,
	"authorization": true,
}

func isSensitiveKey(k string) bool {
	return sensitiveKeySet[strings.ToLower(k)]
}

// maskRecursive 递归遍历，将敏感键对应的字符串值替换为 "***"（覆盖嵌套对象/数组）。
func maskRecursive(v any) any {
	switch val := v.(type) {
	case map[string]any:
		for k, vv := range val {
			if isSensitiveKey(k) {
				val[k] = "***"
			} else {
				val[k] = maskRecursive(vv)
			}
		}
		return val
	case []any:
		for i := range val {
			val[i] = maskRecursive(val[i])
		}
		return val
	default:
		return v
	}
}

// fallbackMaskRe 在非 JSON（纯文本/畸形）时退化为正则脱敏（仅遮盖字符串值）。
// 预先编译一次，避免循环内重复编译（P2-4）。
var fallbackMaskRe = regexp.MustCompile(`(?i)("(?:credential|authtoken|token|password|secret|apikey|key|authorization)"\s*:\s*")[^"]*(")`)

// maskSensitiveJSON 对 JSON 字符串中的敏感字段值做脱敏。
// 优先解析为对象后递归处理（可覆盖嵌套对象/数组），失败则退化为正则（P2-4）。
func maskSensitiveJSON(s string) string {
	if s == "" || s == "null" {
		return s
	}
	var data any
	if err := json.Unmarshal([]byte(s), &data); err != nil {
		return fallbackMaskRe.ReplaceAllString(s, "$1***$2")
	}
	masked := maskRecursive(data)
	b, err := json.Marshal(masked)
	if err != nil {
		return fallbackMaskRe.ReplaceAllString(s, "$1***$2")
	}
	return string(b)
}

// ============================================================
// 上报令牌（P1-1）：关闭匿名写 qa_reports 的鉴权缺口
// ============================================================

const reportTokenKey = "report_token"

var reportTokenOnce sync.Once

// ensureReportToken 首次调用时若 settings 中无 report_token，则生成随机令牌并持久化。
func ensureReportToken() {
	reportTokenOnce.Do(func() {
		var cnt int
		if err := database.DB.QueryRow("SELECT COUNT(*) FROM settings WHERE key = ?", reportTokenKey).Scan(&cnt); err != nil {
			return
		}
		if cnt > 0 {
			return
		}
		tok := generateSecureToken(32)
		_, _ = database.DB.Exec("INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO NOTHING", reportTokenKey, tok)
	})
}

// getReportToken 读取服务端配置的上报令牌。
func getReportToken() string {
	var v string
	_ = database.DB.QueryRow("SELECT value FROM settings WHERE key = ?", reportTokenKey).Scan(&v)
	return v
}

// validReportToken 以常量时间比较上报令牌与配置是否一致。
func validReportToken(token string) bool {
	expected := getReportToken()
	if expected == "" || token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1
}

// generateSecureToken 生成十六进制安全随机串。
func generateSecureToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// GetQaReports 查询 SDK 上报记录（分页 + 按 event 过滤），供前端「SDK 上报」查看页（P1-2）。
func GetQaReports(c *gin.Context) {
	event := strings.TrimSpace(c.Query("event"))
	limit := atoiDefault(c.Query("limit"), 50)
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := atoiDefault(c.Query("offset"), 0)
	if offset < 0 {
		offset = 0
	}

	where := ""
	args := []any{}
	if event != "" {
		where = "WHERE event = ?"
		args = append(args, event)
	}

	var total int
	if err := database.DB.QueryRow("SELECT COUNT(*) FROM qa_reports "+where, args...).Scan(&total); err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}

	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, limit, offset)
	rows, err := database.DB.Query(
		`SELECT id, event, name, result, message, tags, token, source, timestamp, created_at,
		        seq, method, headers, req_body, resp_body, err_msg, elapsed_ms, ts
		 FROM qa_reports `+where+` ORDER BY timestamp DESC LIMIT ? OFFSET ?`,
		queryArgs...,
	)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
		return
	}
	defer rows.Close()

	type QaReport struct {
		ID        string  `json:"id"`
		Event     string  `json:"event"`
		Name      string  `json:"name"`
		Result    string  `json:"result"`
		Message   string  `json:"message"`
		Tags      string  `json:"tags"`
		Token     string  `json:"token"`
		Source    string  `json:"source"`
		Timestamp int64   `json:"timestamp"`
		CreatedAt string  `json:"createdAt"`
		Seq       int64   `json:"seq"`
		Method    string  `json:"method"`
		Headers   string  `json:"headers"`
		ReqBody   string  `json:"reqBody"`
		RespBody  string  `json:"respBody"`
		ErrMsg    string  `json:"errMsg"`
		ElapsedMs float64 `json:"elapsedMs"`
		Ts        string  `json:"ts"`
	}

	list := make([]QaReport, 0)
	for rows.Next() {
		var r QaReport
		if err := rows.Scan(
			&r.ID, &r.Event, &r.Name, &r.Result, &r.Message, &r.Tags, &r.Token, &r.Source, &r.Timestamp, &r.CreatedAt,
			&r.Seq, &r.Method, &r.Headers, &r.ReqBody, &r.RespBody, &r.ErrMsg, &r.ElapsedMs, &r.Ts,
		); err != nil {
			respondError(c, http.StatusInternalServerError, err, "服务器内部错误,请稍后重试")
			return
		}
		list = append(list, r)
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: gin.H{"total": total, "items": list}})
}

// atoiDefault 解析字符串为整数，失败返回默认值。
func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
