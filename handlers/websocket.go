package handlers

import (
	"log"
	"net/http"
	"net/url"
	"sync"

	"qatest/config"
	"qatest/services"

	"github.com/gorilla/websocket"

	"github.com/gin-gonic/gin"
)

var upgrader = websocket.Upgrader{
	// P1-5：按来源白名单校验，禁止任意跨站 WebSocket 连接
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
)

// init 在包加载时将广播函数注册到 ProxyServer 和 Executor，接线 P0-1/P0-2/P0-3
func init() {
	// P0-1: 注册代理广播函数 → proxy_server.broadcast → BroadcastProxyWS
	services.ProxyInstance.SetBroadcastFunc(BroadcastProxyWS)
	// P0-3: 注册日志广播函数 → executor.consumeLogs → BroadcastWS
	services.SetLogBroadcastFunc(BroadcastWS)
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
// P0-2 修复：读取前端决策消息并转发给 ProxyServer.HandleProxyWsMessage
func HandleProxyWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("代理 WebSocket 升级失败: %v", err)
		return
	}

	proxyMu.Lock()
	proxyClients[conn] = true
	proxyMu.Unlock()

	// P0-1: 注册 WS 客户端计数
	services.ProxyInstance.RegisterWSClient()
	defer services.ProxyInstance.UnregisterWSClient()

	defer func() {
		proxyMu.Lock()
		delete(proxyClients, conn)
		proxyMu.Unlock()
		conn.Close()
	}()

	// P0-2: 读取前端决策消息并转发给 ProxyServer
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			break
		}
		services.ProxyInstance.HandleProxyWsMessage(string(raw))
	}
}

// BroadcastWS 向所有执行日志 WebSocket 客户端广播消息
// P0-3 修复：executor.go 的 LogChan 消费者通过此函数推送日志到前端
func BroadcastWS(message []byte) {
	wsMu.Lock()
	defer wsMu.Unlock()
	for conn := range wsClients {
		err := conn.WriteMessage(websocket.TextMessage, message)
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
		err := conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			conn.Close()
			delete(proxyClients, conn)
		}
	}
}
