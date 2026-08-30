package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"qatest/config"
	"qatest/models"

	"github.com/gin-gonic/gin"
)

func init() {
	if config.AppConfig == nil {
		config.AppConfig = &config.Config{
			JWTSecret:      "unit-test-jwt-secret-0123456789abcdef",
			JWTExpiresIn:   time.Hour,
			AllowedOrigins: []string{"http://localhost:3000"},
		}
	}
	gin.SetMode(gin.TestMode)
}

func newAuthTestRouter() (*gin.Engine, *bool) {
	r := gin.New()
	hit := false
	r.GET("/protected", AuthRequired(), func(c *gin.Context) {
		hit = true
		c.JSON(http.StatusOK, models.APIResponse{Success: true})
	})
	return r, &hit
}

func validToken(t *testing.T) string {
	t.Helper()
	tok, err := GenerateToken(models.User{ID: "admin", Username: "admin", Role: "admin"})
	if err != nil {
		t.Fatalf("生成测试令牌失败: %v", err)
	}
	return tok
}

// TestAuthRequiredHeaderOK Authorization 头正常通过
func TestAuthRequiredHeaderOK(t *testing.T) {
	r, hit := newAuthTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+validToken(t))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !*hit {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

// TestAuthRequiredRejectsQueryToken 回归测试：JWT 不再接受 ?token= 传参
// （query 令牌会落入访问日志/浏览器历史，WS 已改为首消息认证）
func TestAuthRequiredRejectsQueryToken(t *testing.T) {
	r, hit := newAuthTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/protected?token="+validToken(t), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("query token 应被拒绝，want 401, got %d", w.Code)
	}
	if *hit {
		t.Fatal("不应放行到业务 handler")
	}
}

// TestAuthRequiredRejectsBadTokens 缺失 / 格式错误 / 篡改令牌
func TestAuthRequiredRejectsBadTokens(t *testing.T) {
	cases := []struct {
		name string
		hdr  string
	}{
		{"缺少令牌", ""},
		{"缺 Bearer 前缀", validToken(t)},
		{"非 Bearer 格式", "Basic " + validToken(t)},
		{"篡改签名", "Bearer " + validToken(t) + "x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := newAuthTestRouter()
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tc.hdr != "" {
				req.Header.Set("Authorization", tc.hdr)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("want 401, got %d", w.Code)
			}
		})
	}
}

// TestAuthRequiredRejectsNoneAlg 算法混淆防护：none/RS256 签名的令牌必须拒绝
func TestAuthRequiredRejectsNoneAlg(t *testing.T) {
	// 手工构造 header 为 alg:none 的未签名 JWT（payload 使用合法 claims）
	unsigned := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJ1c2VySWQiOiJhZG1pbiIsInVzZXJuYW1lIjoiYWRtaW4iLCJyb2xlIjoiYWRtaW4iLCJleHAiOjk5OTk5OTk5OTl9."
	r, _ := newAuthTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+unsigned)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("alg=none 应被拒绝，want 401, got %d", w.Code)
	}
}
