// Package qa — Go SDK 主入口
// 上报地址约定：POST {BaseURL}/api/qa/report
//
// 完整协议（与 FileApiLogger.cs 模板对齐）：
//   - 用例结果：Report(name, result, message, tags)
//   - 自由日志：Log(message, tags)
//   - API 拦截事件：LogRequest / LogResponse / LogError
//       （对应 FileApiLogger 的 REQUEST / RESPONSE / ERROR 三类 gRPC 拦截事件）
//   - 任意原始上报：SendRaw(payload)（可直接转发 FileApiLogger 的 JSONL 行）
// 上报前会对 request / response / headers 中的敏感字段自动脱敏。
package qa

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	mu          sync.Mutex
	initialized bool
)

// sensitiveKeys 敏感字段（落库前脱敏，对应 FileApiLogger.SensitiveFieldNames）
var sensitiveKeys = []string{"credential", "authtoken", "token", "password", "secret", "apikey", "key", "authorization"}

// Init 初始化 SDK 配置（不调用也可直接使用，会使用 config.go 中的默认值）。
func Init(baseURL, token string, enabled bool) {
	mu.Lock()
	defer mu.Unlock()
	if baseURL != "" {
		BaseURL = baseURL
	}
	if token != "" {
		Token = token
	}
	Enabled = enabled
	initialized = true
}

type reportPayload struct {
	Event     string         `json:"event"`
	Name      string         `json:"name"`
	Result    string         `json:"result"`
	Message   string         `json:"message"`
	Tags      map[string]any `json:"tags,omitempty"`
	Timestamp int64          `json:"timestamp"`
	// 拦截事件字段（镜像 FileApiLogger.cs）
	Type      string `json:"type,omitempty"`
	Method    string `json:"method,omitempty"`
	Seq       int64  `json:"seq,omitempty"`
	Headers   any    `json:"headers,omitempty"`
	Request   any    `json:"request,omitempty"`
	Response  any    `json:"response,omitempty"`
	Error     string `json:"error,omitempty"`
	ElapsedMs float64 `json:"elapsed_ms,omitempty"`
	Ts        string `json:"ts,omitempty"` // 与 FileApiLogger.cs 模板对齐（服务端 qa_reports.ts 列）
}

func buildPayload(name, result, message string, tags map[string]any, event string) reportPayload {
	if result == "" {
		result = "passed"
	}
	if event == "" {
		event = "case_result"
	}
	return reportPayload{
		Event:     event,
		Name:      name,
		Result:    result,
		Message:   message,
		Tags:      tags,
		Timestamp: time.Now().UnixMilli(),
	}
}

// redact 递归脱敏：把敏感字段的值替换为 ***。
func redact(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, val := range v {
			if isSensitiveKey(k) {
				out[k] = "***"
			} else {
				out[k] = redact(val)
			}
		}
		return out
	case map[string]string:
		out := make(map[string]any, len(v))
		for k, val := range v {
			if isSensitiveKey(k) {
				out[k] = "***"
			} else {
				out[k] = val
			}
		}
		return out
	case []any:
		for i := range v {
			v[i] = redact(v[i])
		}
		return v
	default:
		return value
	}
}

func isSensitiveKey(k string) bool {
	lk := strings.ToLower(k)
	for _, s := range sensitiveKeys {
		if lk == s {
			return true
		}
	}
	return false
}

func send(p reportPayload) bool {
	if !Enabled {
		return false
	}
	mu.Lock()
	url := BaseURL
	token := Token
	mu.Unlock()

	// 脱敏：request / response / headers
	if p.Headers != nil {
		p.Headers = redact(p.Headers)
	}
	if p.Request != nil {
		p.Request = redact(p.Request)
	}
	if p.Response != nil {
		p.Response = redact(p.Response)
	}

	body, err := json.Marshal(p)
	if err != nil {
		return false
	}
	req, err := http.NewRequest(http.MethodPost, url+"/api/qa/report", bytes.NewReader(body))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// Report 上报一条用例结果。
func Report(name, result, message string, tags map[string]any) bool {
	return send(buildPayload(name, result, message, tags, "case_result"))
}

// Log 上报一条日志。
func Log(message string, tags map[string]any) bool {
	return send(buildPayload("log", "info", message, tags, "log"))
}

// LogRequest 上报一次 API 请求（对应 FileApiLogger REQUEST）。
func LogRequest(method string, headers, request any, seq int64, tags map[string]any) bool {
	p := buildPayload(method, "", "", tags, "request")
	p.Type = "REQUEST"
	p.Method = method
	p.Headers = headers
	p.Request = request
	p.Seq = seq
	return send(p)
}

// LogResponse 上报一次 API 响应（对应 FileApiLogger RESPONSE）。
func LogResponse(method string, headers, response any, elapsedMs float64, seq int64, tags map[string]any) bool {
	p := buildPayload(method, "", "", tags, "response")
	p.Type = "RESPONSE"
	p.Method = method
	p.Headers = headers
	p.Response = response
	p.ElapsedMs = elapsedMs
	p.Seq = seq
	return send(p)
}

// LogError 上报一次 API 错误（对应 FileApiLogger ERROR）。
func LogError(method, errMsg string, elapsedMs float64, headers any, seq int64, tags map[string]any) bool {
	p := buildPayload(method, "error", errMsg, tags, "error")
	p.Type = "ERROR"
	p.Method = method
	p.Headers = headers
	p.Error = errMsg
	p.ElapsedMs = elapsedMs
	p.Seq = seq
	return send(p)
}

// SendRaw 任意原始上报（可直接转发 FileApiLogger 的 JSONL 行）。
func SendRaw(payload map[string]any) bool {
	b, err := json.Marshal(payload)
	if err != nil {
		return false
	}
	var p reportPayload
	if err := json.Unmarshal(b, &p); err != nil {
		return false
	}
	return send(p)
}
