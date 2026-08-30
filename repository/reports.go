package repository

import (
	"crypto/subtle"
	"sync"

	"qatest/database"
)

// —— SDK 上报（表 qa_reports / settings.report_token；SQL 迁自 handlers/sdk.go，语句原样保留） ——

const ReportTokenKey = "report_token"

var reportTokenOnce sync.Once

// EnsureReportToken 首次调用时若 settings 中无 report_token，则生成随机令牌并持久化。
func EnsureReportToken() {
	reportTokenOnce.Do(func() {
		var cnt int
		if err := database.DB.QueryRow("SELECT COUNT(*) FROM settings WHERE key = ?", ReportTokenKey).Scan(&cnt); err != nil {
			return
		}
		if cnt > 0 {
			return
		}
		tok := NewSecureToken(32)
		_, _ = database.DB.Exec("INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO NOTHING", ReportTokenKey, tok)
	})
}

// GetReportToken 读取服务端配置的上报令牌。
func GetReportToken() string {
	var v string
	_ = database.DB.QueryRow("SELECT value FROM settings WHERE key = ?", ReportTokenKey).Scan(&v)
	return v
}

// ValidReportToken 以常量时间比较上报令牌与配置是否一致。
func ValidReportToken(token string) bool {
	expected := GetReportToken()
	if expected == "" || token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1
}

// ReportInsert 一条上报记录的落库字段（ID/时间戳由调用方填充）
type ReportInsert struct {
	ID        string
	Event     string
	Name      string
	Result    string
	Message   string
	Tags      string
	Token     string
	Source    string
	Timestamp int64
	CreatedAt string
	Seq       int64
	Method    string
	Headers   string
	ReqBody   string
	RespBody  string
	ErrMsg    string
	ElapsedMs float64
	Ts        string
}

// InsertQaReport 落库一条上报记录
func InsertQaReport(r ReportInsert) error {
	_, err := database.DB.Exec(
		`INSERT INTO qa_reports
		 (id, event, name, result, message, tags, token, source, timestamp, created_at,
		  seq, method, headers, req_body, resp_body, err_msg, elapsed_ms, ts)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.ID, r.Event, r.Name, r.Result, r.Message,
		r.Tags, r.Token, r.Source, r.Timestamp, r.CreatedAt,
		r.Seq, r.Method, r.Headers, r.ReqBody, r.RespBody, r.ErrMsg, r.ElapsedMs, r.Ts,
	)
	return err
}

// CountQaReports 统计上报记录数（可按 event 过滤）
func CountQaReports(event string) (int, error) {
	where := ""
	args := []any{}
	if event != "" {
		where = "WHERE event = ?"
		args = append(args, event)
	}
	var total int
	err := database.DB.QueryRow("SELECT COUNT(*) FROM qa_reports "+where, args...).Scan(&total)
	return total, err
}

// QaReportScan 一条上报记录的扫描目标（列顺序与 qa_reports 表查询一致）
type QaReportScan struct {
	ID        string
	Event     string
	Name      string
	Result    string
	Message   string
	Tags      string
	Token     string
	Source    string
	Timestamp int64
	CreatedAt string
	Seq       int64
	Method    string
	Headers   string
	ReqBody   string
	RespBody  string
	ErrMsg    string
	ElapsedMs float64
	Ts        string
}

// ListQaReports 分页查询上报记录（按 event 过滤，时间倒序）
func ListQaReports(event string, limit, offset int) ([]QaReportScan, error) {
	where := ""
	args := []any{}
	if event != "" {
		where = "WHERE event = ?"
		args = append(args, event)
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
		return nil, err
	}
	defer rows.Close()

	list := make([]QaReportScan, 0)
	for rows.Next() {
		var r QaReportScan
		if err := rows.Scan(
			&r.ID, &r.Event, &r.Name, &r.Result, &r.Message, &r.Tags, &r.Token, &r.Source, &r.Timestamp, &r.CreatedAt,
			&r.Seq, &r.Method, &r.Headers, &r.ReqBody, &r.RespBody, &r.ErrMsg, &r.ElapsedMs, &r.Ts,
		); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, rows.Err()
}
