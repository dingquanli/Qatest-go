package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"qatest/database"
	"qatest/models"

	"github.com/gin-gonic/gin"
)

func newTestGin(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return gin.New()
}

// TestMaskSensitiveJSON 敏感字段脱敏：嵌套对象、数组、正则兜底
func TestMaskSensitiveJSON(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string // 输出必须包含的子串
		not  []string // 输出必须不包含的子串
	}{
		{
			name: "顶层敏感键",
			in:   `{"token":"abc","password":"pw","name":"x"}`,
			want: []string{`"token":"***"`, `"password":"***"`, `"name":"x"`},
		},
		{
			name: "嵌套对象与数组",
			in:   `{"a":{"apikey":"K1","arr":[{"authorization":"Bearer x"}]}}`,
			want: []string{`"apikey":"***"`, `"authorization":"***"`},
			not:  []string{"K1", "Bearer x"},
		},
		{
			name: "大小写变体",
			in:   `{"PassWord":"p","SECRET":"s"}`,
			want: []string{`"PassWord":"***"`, `"SECRET":"***"`},
		},
		{
			name: "非 JSON 文本走正则兜底",
			in:   `prefix "token":"leaked-value" suffix`,
			want: []string{`"token":"***"`},
			not:  []string{"leaked-value"},
		},
		{
			name: "空串与 null 原样返回",
			in:   "null",
			want: []string{"null"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := maskSensitiveJSON(tc.in)
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Fatalf("输出 %q 缺少 %q", got, w)
				}
			}
			for _, n := range tc.not {
				if strings.Contains(got, n) {
					t.Fatalf("输出 %q 不应包含 %q", got, n)
				}
			}
		})
	}
}

// TestReceiveReportAuth 上报接口的令牌鉴权：无令牌/错令牌 401，正确令牌 200 且落库脱敏
func TestReceiveReportAuth(t *testing.T) {
	ensureReportToken()
	goodToken := getReportToken()
	if goodToken == "" {
		t.Fatal("ensureReportToken 未生成上报令牌")
	}

	r := newTestGin(t)
	r.POST("/api/qa/report", ReceiveReport)

	post := func(token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/qa/report", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	t.Run("缺少令牌返回401", func(t *testing.T) {
		if w := post("", `{"name":"case1"}`); w.Code != http.StatusUnauthorized {
			t.Fatalf("want 401, got %d", w.Code)
		}
	})
	t.Run("错误令牌返回401", func(t *testing.T) {
		if w := post("wrong-token", `{"name":"case1"}`); w.Code != http.StatusUnauthorized {
			t.Fatalf("want 401, got %d", w.Code)
		}
	})
	t.Run("正确令牌写入并脱敏", func(t *testing.T) {
		body := `{"event":"request","name":"GET /x","method":"GET","headers":{"token":"secret-value","ok":"plain"},"request":{"password":"pw1"}}`
		w := post(goodToken, body)
		if w.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
		}
		var resp models.APIResponse
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		data, _ := resp.Data.(map[string]any)
		id, _ := data["id"].(string)
		if id == "" {
			t.Fatal("响应缺少上报 ID")
		}
		var headers, reqBody string
		if err := database.DB.QueryRow("SELECT headers, req_body FROM qa_reports WHERE id = ?", id).Scan(&headers, &reqBody); err != nil {
			t.Fatalf("上报记录未落库: %v", err)
		}
		if strings.Contains(headers, "secret-value") || strings.Contains(reqBody, "pw1") {
			t.Fatalf("敏感字段未脱敏: headers=%s req_body=%s", headers, reqBody)
		}
		if !strings.Contains(headers, `"token":"***"`) || !strings.Contains(reqBody, `"password":"***"`) {
			t.Fatalf("脱敏标记缺失: headers=%s req_body=%s", headers, reqBody)
		}
		if !strings.Contains(headers, `"ok":"plain"`) {
			t.Fatalf("非敏感字段不应被改写: headers=%s", headers)
		}
	})
	t.Run("缺少name与method返回400", func(t *testing.T) {
		if w := post(goodToken, `{"event":"log","message":"x"}`); w.Code != http.StatusBadRequest {
			t.Fatalf("want 400, got %d", w.Code)
		}
	})
}

// TestGetSDKListRoleVisibility reportToken 仅对 admin 下发
func TestGetSDKListRoleVisibility(t *testing.T) {
	ensureReportToken()
	r := newTestGin(t)
	r.GET("/sdk", func(c *gin.Context) {
		// 模拟 auth 中间件注入的角色
		c.Set("role", c.GetHeader("X-Test-Role"))
		GetSDKList(c)
	})

	fetch := func(role string) string {
		req := httptest.NewRequest(http.MethodGet, "/sdk", nil)
		req.Header.Set("X-Test-Role", role)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var resp models.APIResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}
		items, _ := resp.Data.([]any)
		first, _ := items[0].(map[string]any)
		tok, _ := first["reportToken"].(string)
		return tok
	}

	if tok := fetch("admin"); tok == "" {
		t.Fatal("admin 应能拿到 reportToken")
	}
	if tok := fetch("tester"); tok != "" {
		t.Fatalf("非 admin 不应拿到 reportToken，got %q", tok)
	}
}

// TestValidReportToken 常量时间比较逻辑本身
func TestValidReportToken(t *testing.T) {
	ensureReportToken()
	good := getReportToken()
	if !validReportToken(good) {
		t.Fatal("正确令牌应通过校验")
	}
	if validReportToken(good + "x") {
		t.Fatal("篡改令牌不应通过校验")
	}
	if validReportToken("") {
		t.Fatal("空令牌不应通过校验")
	}
}
