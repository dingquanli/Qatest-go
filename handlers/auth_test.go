package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"qatest/models"

	"github.com/gin-gonic/gin"
)

func newAuthEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/auth/login", Login)
	r.POST("/api/auth/refresh", RefreshToken)
	r.POST("/api/auth/logout", Logout)
	return r
}

func postJSON(r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestLoginWrongPassword 登录失败路径（不暴露用户是否存在）
func TestLoginWrongPassword(t *testing.T) {
	r := newAuthEngine()
	w := postJSON(r, "/api/auth/login", `{"username":"admin","password":"wrong"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

// TestLoginIssuesRefreshToken 登录同时下发刷新令牌
func TestLoginIssuesRefreshToken(t *testing.T) {
	r := newAuthEngine()
	w := postJSON(r, "/api/auth/login", `{"username":"admin","password":"`+testAdminPassword+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp models.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	data, _ := resp.Data.(map[string]any)
	if data["token"].(string) == "" {
		t.Fatal("缺少访问令牌")
	}
	if data["refreshToken"].(string) == "" {
		t.Fatal("缺少刷新令牌")
	}
}

// TestRefreshTokenRotation 轮换式刷新：旧令牌一次性，重放被拒绝；登出后撤销
func TestRefreshTokenRotation(t *testing.T) {
	r := newAuthEngine()

	// 1. 登录拿第一对令牌
	w := postJSON(r, "/api/auth/login", `{"username":"admin","password":"`+testAdminPassword+`"}`)
	var loginResp models.APIResponse
	_ = json.Unmarshal(w.Body.Bytes(), &loginResp)
	loginData, _ := loginResp.Data.(map[string]any)
	refresh1, _ := loginData["refreshToken"].(string)

	// 2. 首次刷新成功，得到新令牌对
	w = postJSON(r, "/api/auth/refresh", `{"token":"`+refresh1+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("首次刷新 want 200, got %d: %s", w.Code, w.Body.String())
	}
	var refreshResp models.APIResponse
	_ = json.Unmarshal(w.Body.Bytes(), &refreshResp)
	refreshData, _ := refreshResp.Data.(map[string]any)
	refresh2, _ := refreshData["refreshToken"].(string)
	if refresh2 == "" || refresh2 == refresh1 {
		t.Fatal("刷新应轮换出新 refreshToken")
	}

	// 3. 重放旧刷新令牌 → 401（轮换的核心防线）
	if w := postJSON(r, "/api/auth/refresh", `{"token":"`+refresh1+`"}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("重放旧令牌 want 401, got %d", w.Code)
	}

	// 4. 登出撤销 refresh2
	if w := postJSON(r, "/api/auth/logout", `{"token":"`+refresh2+`"}`); w.Code != http.StatusOK {
		t.Fatalf("登出 want 200, got %d", w.Code)
	}

	// 5. 已撤销令牌再刷新 → 401；登出幂等
	if w := postJSON(r, "/api/auth/refresh", `{"token":"`+refresh2+`"}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("已撤销令牌刷新 want 401, got %d", w.Code)
	}
	if w := postJSON(r, "/api/auth/logout", `{"token":"`+refresh2+`"}`); w.Code != http.StatusOK {
		t.Fatalf("登出应幂等, got %d", w.Code)
	}
}

// TestRefreshTokenGarbage 非法输入：绑定失败 400，格式合法但无效的令牌 401
// （不区分「不存在」与「已过期」，避免泄露令牌状态）
func TestRefreshTokenGarbage(t *testing.T) {
	r := newAuthEngine()
	cases := []struct {
		body string
		want int
	}{
		{`{"token":"not-a-token"}`, http.StatusUnauthorized},
		{`{"token":""}`, http.StatusBadRequest},  // binding:required 拦截
		{`{}`, http.StatusBadRequest},            // 缺字段
		{`{"token":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}`, http.StatusUnauthorized}, // 格式合法但不存在的令牌
	}
	for _, tc := range cases {
		if w := postJSON(r, "/api/auth/refresh", tc.body); w.Code != tc.want {
			t.Fatalf("body=%s want %d, got %d", tc.body, tc.want, w.Code)
		}
	}
}
