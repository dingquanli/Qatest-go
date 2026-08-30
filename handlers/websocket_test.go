package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"qatest/middleware"
	"qatest/models"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// wsTestServer 起一个仅含 WS 路由的 httptest 服务
func wsTestServer(t *testing.T, path string, handler func(*gin.Context)) *httptest.Server {
	t.Helper()
	r := newTestGin(t)
	r.GET(path, handler)
	return httptest.NewServer(r)
}

// dialWS 建立 WS 连接（HTTP 层无认证，认证在首消息）
func dialWS(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(url, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WS 连接失败: %v", err)
	}
	return conn
}

func readWSMsg(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("读取 WS 消息失败: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("解析 WS 消息失败: %v", err)
	}
	return m
}

// TestWebSocketFirstMessageAuth 首消息认证：正确令牌 auth_ok 且可收广播；
// 错误令牌连接被关闭（auth_failed）。
func TestWebSocketFirstMessageAuth(t *testing.T) {
	token := validJWT(t, "admin")

	t.Run("正确令牌通过认证并接收广播", func(t *testing.T) {
		srv := wsTestServer(t, "/api/ws", HandleWebSocket)
		defer srv.Close()
		conn := dialWS(t, srv.URL+"/api/ws")
		defer conn.Close()

		if err := conn.WriteJSON(map[string]string{"type": "auth", "token": token}); err != nil {
			t.Fatal(err)
		}
		if m := readWSMsg(t, conn); m["type"] != "auth_ok" {
			t.Fatalf("want auth_ok, got %v", m)
		}

		// 认证后应能收到广播
		done := make(chan map[string]any, 1)
		go func() { done <- readWSMsg(t, conn) }()
		BroadcastWS([]byte(`{"type":"log","message":"hello"}`))
		select {
		case m := <-done:
			if m["message"] != "hello" {
				t.Fatalf("广播内容不符: %v", m)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("认证后未收到广播")
		}
	})

	t.Run("错误令牌被拒绝并关闭", func(t *testing.T) {
		srv := wsTestServer(t, "/api/ws", HandleWebSocket)
		defer srv.Close()
		conn := dialWS(t, srv.URL+"/api/ws")
		defer conn.Close()

		_ = conn.WriteJSON(map[string]string{"type": "auth", "token": "invalid"})
		if m := readWSMsg(t, conn); m["type"] != "auth_failed" {
			t.Fatalf("want auth_failed, got %v", m)
		}
	})

	t.Run("认证前不发 auth 帧则超时关闭", func(t *testing.T) {
		old := wsAuthTimeout
		wsAuthTimeout = 500 * time.Millisecond
		defer func() { wsAuthTimeout = old }()

		srv := wsTestServer(t, "/api/ws", HandleWebSocket)
		defer srv.Close()
		conn := dialWS(t, srv.URL+"/api/ws")
		defer conn.Close()

		// 发送非 auth 帧：服务端继续等待直至 wsAuthTimeout 超时关闭
		_ = conn.WriteJSON(map[string]string{"type": "log", "message": "sneak"})
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		if _, _, err := conn.ReadMessage(); err == nil {
			t.Fatal("未认证连接应被超时关闭")
		}
	})
}

// validJWT 生成测试用合法 JWT
func validJWT(t *testing.T, username string) string {
	t.Helper()
	tok, err := middleware.GenerateToken(models.User{ID: username, Username: username, Role: "admin"})
	if err != nil {
		t.Fatalf("生成 JWT 失败: %v", err)
	}
	return tok
}

// TestProxyWSFirstMessageAuth 代理 WS：认证通过后才进入决策消息处理
func TestProxyWSFirstMessageAuth(t *testing.T) {
	token := validJWT(t, "admin")
	srv := wsTestServer(t, "/api/proxy-ws", HandleProxyWebSocket)
	defer srv.Close()
	conn := dialWS(t, srv.URL+"/api/proxy-ws")
	defer conn.Close()

	if err := conn.WriteJSON(map[string]string{"type": "auth", "token": token}); err != nil {
		t.Fatal(err)
	}
	if m := readWSMsg(t, conn); m["type"] != "auth_ok" {
		t.Fatalf("want auth_ok, got %v", m)
	}
}
