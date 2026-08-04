package handlers

import (
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"qatest/config"
	"qatest/services"

	"github.com/gorilla/websocket"

	"github.com/gin-gonic/gin"
)

// WebSocket 读写参数：服务端定时发 ping，客户端（浏览器）按规范自动回 pong；
// 若超过 pongWait 未收到 pong，读超时触发，及时释放连接资源。
const (
	wsPongWait   = 60 * time.Second
	wsPingPeriod = wsPongWait * 9 / 10
	wsWriteWait  = 10 * time.Second
)

var upgrader = websocket.Upgrader{
	// 按来源白名单校验，禁止任意跨站 WebSocket 连接
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		// 无 Origin 头（同源/非浏览器）允许
		if origin == "" {
			return true
		}
		allowedOrigins := config.AppConfig.AllowedOrigins
		hasWildcard := false
		for _, ao := range allowedOrigins {
			if ao == "*" {
				hasWildcard = true
				continue
			}
			if ao == origin {
				return true
			}
		}
		// 白名单含通配符 "*" 或为空时收紧策略：仅允许同源
		// （Origin 的 host 与请求 Host 相同），禁止任意跨站连接（防 CSWSH）。
		// 不再因通配而接受任意 Origin。
		if hasWildcard || len(allowedOrigins) == 0 {
			if u, err := url.Parse(origin); err == nil && u.Host == r.Host {
				return true
			}
			return false
		}
		// 非通配且未精确匹配：拒绝
		return false
	},
}

// WebSocket 连接管理
var (
	wsClients    = make(map[*websocket.Conn]bool)
	wsMu         sync.Mutex
	proxyClients = make(map[*websocket.Conn]bool)
	proxyMu      sync.Mutex
	// wsWriteMu 串行化所有 WebSocket 写操作，避免广播与心跳 ping 并发写同一连接
	wsWriteMu sync.Mutex
)

// init 在包加载时将广播函数注册到 ProxyServer 和 Executor
func init() {
	// 注册代理广播函数 → proxy_server.broadcast → BroadcastProxyWS
	services.ProxyInstance.SetBroadcastFunc(BroadcastProxyWS)
	// 注册日志广播函数 → executor.consumeLogs → BroadcastWS
	services.SetLogBroadcastFunc(BroadcastWS)
}

// setupWSHeartbeat 配置连接的读超时与 pong 处理，并返回用于发送心跳 ping 的 ticker。
// 心跳 goroutine 由调用方通过 defer ticker.Stop() 停止。
func setupWSHeartbeat(conn *websocket.Conn) *time.Ticker {
	conn.SetReadLimit(1 << 20)
	conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})
	ticker := time.NewTicker(wsPingPeriod)
	go func() {
		for range ticker.C {
			wsWriteMu.Lock()
			conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			err := conn.WriteMessage(websocket.PingMessage, nil)
			wsWriteMu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	return ticker
}

// HandleWebSocket 执行日志 WebSocket
func HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket 升级失败: %v", err)
		return
	}

	wsMu.Lock()
	wsClients[conn] = true
	wsMu.Unlock()

	ticker := setupWSHeartbeat(conn)
	defer ticker.Stop()

	defer func() {
		wsMu.Lock()
		delete(wsClients, conn)
		wsMu.Unlock()
		conn.Close()
	}()

	// 保持连接，读取消息（处理 ping/pong）
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// HandleProxyWebSocket 代理 WebSocket
// 读取前端决策消息并转发给 ProxyServer.HandleProxyWsMessage
func HandleProxyWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("代理 WebSocket 升级失败: %v", err)
		return
	}

	proxyMu.Lock()
	proxyClients[conn] = true
	proxyMu.Unlock()

	// 注册 WS 客户端计数
	services.ProxyInstance.RegisterWSClient()
	defer func() {
		services.ProxyInstance.UnregisterWSClient()
		// 边界加固：全部代理 WS 客户端断开后，立即中止等待中的 pending 请求，
		// 避免它们残留到 5 分钟超时；页面刷新/断线不会阻塞后续代理流程。
		services.ProxyInstance.NotifyNoWSClient()
	}()

	ticker := setupWSHeartbeat(conn)
	defer ticker.Stop()

	defer func() {
		proxyMu.Lock()
		delete(proxyClients, conn)
		proxyMu.Unlock()
		conn.Close()
	}()

	// 读取前端决策消息并转发给 ProxyServer
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			break
		}
		services.ProxyInstance.HandleProxyWsMessage(string(raw))
	}
}

// BroadcastWS 向所有执行日志 WebSocket 客户端广播消息
// executor.go 的 LogChan 消费者通过此函数推送日志到前端
func BroadcastWS(message []byte) {
	wsMu.Lock()
	defer wsMu.Unlock()
	for conn := range wsClients {
		wsWriteMu.Lock()
		err := conn.WriteMessage(websocket.TextMessage, message)
		wsWriteMu.Unlock()
		if err != nil {
			conn.Close()
			delete(wsClients, conn)
		}
	}
}

// BroadcastProxyWS 向所有代理 WebSocket 客户端广播消息
func BroadcastProxyWS(message []byte) {
	proxyMu.Lock()
	defer proxyMu.Unlock()
	for conn := range proxyClients {
		wsWriteMu.Lock()
		err := conn.WriteMessage(websocket.TextMessage, message)
		wsWriteMu.Unlock()
		if err != nil {
			conn.Close()
			delete(proxyClients, conn)
		}
	}
}
