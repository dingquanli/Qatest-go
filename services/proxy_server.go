package services

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"qatest/config"
	"qatest/middleware"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// ============================================================
// ProxyServer — gRPC 拦截代理（拦截-修改-重放）
// ============================================================

const DefaultProxyPort = 18924

// maxExecutions 限制执行历史记录上限，避免无界增长（P1-8 修复）
const maxExecutions = 1000

// ProxyBroadcastMessage WebSocket 广播消息
type ProxyBroadcastMessage struct {
	Type      string      `json:"type"`
	ID        int         `json:"id,omitempty"`
	Method    string      `json:"method,omitempty"`
	Target    string      `json:"target,omitempty"`
	Request   interface{} `json:"request,omitempty"`
	Response  interface{} `json:"response,omitempty"`
	ElapsedMs int64       `json:"elapsed_ms,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp string      `json:"timestamp,omitempty"`
	Dropped   bool        `json:"dropped,omitempty"`
}

// ProxyLogEntry 代理日志条目（JSONL）
type ProxyLogEntry struct {
	Seq        int         `json:"seq"`
	Timestamp  string      `json:"ts"`
	Type       string      `json:"type"`
	Method     string      `json:"method"`
	Target     string      `json:"target"`
	Request    interface{} `json:"request,omitempty"`
	Response   interface{} `json:"response,omitempty"`
	ElapsedMs  int64       `json:"elapsed_ms,omitempty"`
	Error      string      `json:"error,omitempty"`
}

// ProxyExecution 代理执行历史
type ProxyExecution struct {
	ID        int    `json:"id"`
	Method    string `json:"method"`
	Target    string `json:"target"`
	Timestamp string `json:"timestamp"`
	ElapsedMs int64  `json:"elapsed_ms"`
}

// pendingRequest 等待 WebSocket 客户端响应的请求
type pendingRequest struct {
	ID               int
	Method           string
	Target           string
	RawRequest       []byte
	RequestJSON      interface{}
	RawResponse      []byte
	ResponseJSON     interface{}
	State            string // "waiting-request" | "forwarded" | "waiting-response" | "send-response" | "done" | "dropped" | "error"
	ModifiedRequest  interface{}
	ModifiedResponse interface{}
	ResolveRequest   chan struct{}
	ResolveResponse  chan struct{}
	StartTime        time.Time
	ElapsedMs        int64
	ContentType      string
}

// ProxyServer gRPC 代理服务器
type ProxyServer struct {
	mu            sync.Mutex
	running       bool
	paused        bool
	port          int
	target        string
	listener      net.Listener
	httpServer    *http.Server
	reqID         int32
	pending       map[int]*pendingRequest
	logDir        string
	logFile       *os.File
	logSeq        int32
	logCh         chan ProxyLogEntry   // 异步日志 channel
	logWg         sync.WaitGroup       // 日志 writer goroutine 等待（P1-9 修复）
	executions    []ProxyExecution
	executionsMu  sync.Mutex
	broadcastFn   func([]byte) // WebSocket 广播回调
	wsClientCount int32
}

// ProxyInstance 全局代理实例
var ProxyInstance = &ProxyServer{
	port:    DefaultProxyPort,
	pending: make(map[int]*pendingRequest),
}

// SetBroadcastFunc 设置 WebSocket 广播函数（由 handlers 包注册）
func (ps *ProxyServer) SetBroadcastFunc(fn func([]byte)) {
	ps.broadcastFn = fn
}

// broadcast 向所有代理 WebSocket 客户端广播消息
func (ps *ProxyServer) broadcast(msg ProxyBroadcastMessage) {
	if ps.broadcastFn == nil {
		return
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	ps.broadcastFn(data)
}

// GetStatus 获取代理状态
func (ps *ProxyServer) GetStatus() map[string]interface{} {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	pendingCount := 0
	for _, p := range ps.pending {
		if p.State != "done" && p.State != "dropped" && p.State != "error" {
			pendingCount++
		}
	}

	return map[string]interface{}{
		"running":       ps.running,
		"paused":        ps.paused,
		"port":          ps.port,
		"target":        ps.target,
		"pendingCount":  pendingCount,
		"logDir":        ps.logDir,
		"wsClientCount": atomic.LoadInt32(&ps.wsClientCount),
	}
}

// Start 启动代理
func (ps *ProxyServer) Start(target string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.running {
		return nil
	}

	if target != "" {
		ps.target = target
	}
	if ps.target == "" {
		ps.target = config.AppConfig.ProxyTarget
	}

	// 初始化日志
	ps.initLog()

	// 创建 HTTP/2 cleartext (h2c) 服务器
	mux := http.NewServeMux()
	mux.HandleFunc("/", ps.handleGRPC)

	h2s := &http2.Server{}
	h1s := &http.Server{
		// P2 加固：仅绑定本机回环地址，避免 0.0.0.0 无认证暴露到局域网。
		Addr:    fmt.Sprintf("127.0.0.1:%d", ps.port),
		Handler: h2c.NewHandler(mux, h2s),
	}

	ln, err := net.Listen("tcp", h1s.Addr)
	if err != nil {
		return fmt.Errorf("代理端口 %d 监听失败: %v", ps.port, err)
	}

	ps.listener = ln
	ps.httpServer = h1s
	ps.running = true

	go func() {
		if err := h1s.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("[proxy] Server error: %v", err)
			ps.mu.Lock()
			ps.running = false
			ps.mu.Unlock()
		}
	}()

	log.Printf("[proxy] 启动成功，端口: %d → %s", ps.port, ps.target)
	return nil
}

// Stop 停止代理
// P1-9 修复：closeLog 不在持锁状态下调用，避免 writer goroutine 死锁
func (ps *ProxyServer) Stop() {
	ps.mu.Lock()
	if !ps.running {
		ps.mu.Unlock()
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

	if ps.httpServer != nil {
		ps.httpServer.Shutdown(ctx)
	}

	// 清理所有等待中的请求
	for _, p := range ps.pending {
		if p.ResolveRequest != nil {
			select {
			case p.ResolveRequest <- struct{}{}:
			default:
			}
		}
		if p.ResolveResponse != nil {
			select {
			case p.ResolveResponse <- struct{}{}:
			default:
			}
		}
	}

	ps.running = false
	ps.mu.Unlock()
	cancel()

	// 在锁外关闭日志：writer goroutine 可安全获取 ps.mu 写盘
	ps.closeLog()
	log.Println("[proxy] 已停止")
}

// TogglePause 切换暂停状态
func (ps *ProxyServer) TogglePause() bool {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.paused = !ps.paused
	return ps.paused
}

// isPaused 检查是否暂停
func (ps *ProxyServer) isPaused() bool {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.paused
}

// RegisterWSClient WebSocket 客户端注册
func (ps *ProxyServer) RegisterWSClient() {
	atomic.AddInt32(&ps.wsClientCount, 1)
}

// UnregisterWSClient WebSocket 客户端注销
func (ps *ProxyServer) UnregisterWSClient() {
	atomic.AddInt32(&ps.wsClientCount, -1)
}

// ============================================================
// 核心拦截流程：handleGRPC
// ============================================================

// handleGRPC 处理 gRPC 请求（h2c HTTP handler）
// 流程：接收 → 解码 → 广播 proxy-request → 等待 WS 客户端决策 →
//
//	proxy-forward: 转发 → 解码响应 → 广播 proxy-response → 等待修改 → 返回
//	proxy-send-response: 直接编码返回（跳过转发）
//	proxy-drop: 返回 gRPC 错误
func (ps *ProxyServer) handleGRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method Not Allowed", 405)
		return
	}

	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/grpc") {
		http.Error(w, "Unsupported Media Type", 415)
		return
	}

	method := r.URL.Path
	targetHost := r.Host
	if targetHost == fmt.Sprintf("localhost:%d", ps.port) || targetHost == fmt.Sprintf("127.0.0.1:%d", ps.port) {
		ps.mu.Lock()
		targetHost = ps.target
		ps.mu.Unlock()
	}

	// 读取 gRPC 数据帧
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[proxy] 读取请求体失败: %v", err)
		http.Error(w, "Bad Request", 400)
		return
	}

	// 分配请求 ID
	id := int(atomic.AddInt32(&ps.reqID, 1))

	// 尝试解码为 JSON
	requestJSON := ps.tryDecodeGrpcFrame(body, method, "request")

	entry := &pendingRequest{
		ID:             id,
		Method:         method,
		Target:         targetHost,
		RawRequest:     body,
		RequestJSON:    requestJSON,
		State:          "waiting-request",
		ResolveRequest: make(chan struct{}),
		StartTime:      time.Now(),
		ContentType:    ct,
	}

	ps.mu.Lock()
	ps.pending[id] = entry
	ps.mu.Unlock()

	log.Printf("[proxy] REQUEST id=%d method=%s target=%s", id, method, targetHost)

	// === 阶段一：广播 proxy-request 并等待 WS 客户端决策 ===
	ps.broadcast(ProxyBroadcastMessage{
		Type:      "proxy-request",
		ID:        id,
		Method:    method,
		Target:    targetHost,
		Request:   requestJSON,
		Timestamp: time.Now().Format(time.RFC3339),
	})

	// 等待 WebSocket 客户端回复（5 分钟超时）
	if !ps.isPaused() {
		select {
		case <-entry.ResolveRequest:
		case <-time.After(5 * time.Minute):
			log.Printf("[proxy] 等待 WS 客户端超时 id=%d", id)
			entry.State = "error"
		}
	}

	// 检查是否需要丢弃
	if entry.State == "dropped" {
		ps.writeLog(ProxyLogEntry{
			Type:    "DROPPED",
			Method:  method,
			Target:  targetHost,
			Request: requestJSON,
			Error:   "Request dropped by WebSocket client",
		})
		ps.mu.Lock()
		delete(ps.pending, id)
		ps.mu.Unlock()

		ps.broadcast(ProxyBroadcastMessage{
			Type:    "proxy-done",
			ID:      id,
			Dropped: true,
		})
		ps.writeGrpcError(w, 1, "Request dropped by proxy")
		return
	}

	// === proxy-send-response：跳过转发，直接编码返回 ===
	if entry.State == "send-response" {
		elapsed := time.Since(entry.StartTime).Milliseconds()

		if entry.ModifiedResponse != nil {
			encoded := ps.tryEncodeGrpcFrame(entry.ModifiedResponse, method, "response")
			if encoded != nil {
				w.Header().Set("Content-Type", ct)
				w.Header().Set("Trailer", "grpc-status")
				w.WriteHeader(200)
				w.Write(encoded)
				w.Header().Set("grpc-status", "0")

				ps.writeLog(ProxyLogEntry{
					Type:      "DIRECT_RESPONSE",
					Method:    method,
					Target:    targetHost,
					Request:   requestJSON,
					Response:  entry.ModifiedResponse,
					ElapsedMs: elapsed,
				})
			} else {
				ps.writeLog(ProxyLogEntry{
					Type:      "ENCODE_ERROR",
					Method:    method,
					Target:    targetHost,
					Request:   requestJSON,
					Error:     "编码响应失败",
					ElapsedMs: elapsed,
				})
				ps.writeGrpcError(w, 13, "Failed to encode response")
			}
		} else {
			ps.writeLog(ProxyLogEntry{
				Type:      "DIRECT_RESPONSE",
				Method:    method,
				Target:    targetHost,
				Request:   requestJSON,
				Response:  map[string]interface{}{},
				ElapsedMs: elapsed,
			})
			w.Header().Set("Content-Type", ct)
			w.Header().Set("Trailer", "grpc-status")
			w.WriteHeader(200)
			w.Write(buildGRPCFrame([]byte{}))
			w.Header().Set("grpc-status", "0")
		}

		ps.broadcast(ProxyBroadcastMessage{
			Type: "proxy-done",
			ID:   id,
		})

		ps.recordExecution(id, method, targetHost, elapsed)
		ps.mu.Lock()
		delete(ps.pending, id)
		ps.mu.Unlock()
		return
	}

	// 错误状态（如超时）
	if entry.State == "error" {
		ps.writeLog(ProxyLogEntry{
			Type:    "ERROR",
			Method:  method,
			Target:  targetHost,
			Request: requestJSON,
			Error:   "等待 WebSocket 客户端超时或出错",
		})
		ps.mu.Lock()
		delete(ps.pending, id)
		ps.mu.Unlock()
		ps.writeGrpcError(w, 4, "Deadline exceeded waiting for proxy client")
		return
	}

	// === proxy-forward：转发到目标服务器 ===
	entry.State = "forwarded"
	forwardStart := time.Now()

	// 构建转发请求体（可能已被 WS 客户端修改）
	forwardBody := entry.RawRequest
	if entry.ModifiedRequest != nil {
		if encoded := ps.tryEncodeGrpcFrame(entry.ModifiedRequest, method, "request"); encoded != nil {
			forwardBody = encoded
			log.Printf("[proxy] 使用修改后的请求 id=%d", id)
		}
	}

	respBody, respStatus, respErr := ps.forwardToTarget(targetHost, method, ct, forwardBody)
	forwardElapsed := time.Since(forwardStart).Milliseconds()

	if respErr != nil {
		log.Printf("[proxy] 转发失败 id=%d: %v", id, respErr)
		entry.State = "error"
		ps.writeLog(ProxyLogEntry{
			Type:      "FORWARD_ERROR",
			Method:    method,
			Target:    targetHost,
			Request:   coalesceJSON(entry.ModifiedRequest, requestJSON),
			Error:     respErr.Error(),
			ElapsedMs: forwardElapsed,
		})
		ps.broadcast(ProxyBroadcastMessage{
			Type:  "proxy-error",
			ID:    id,
			Error: respErr.Error(),
		})
		ps.mu.Lock()
		delete(ps.pending, id)
		ps.mu.Unlock()
		ps.writeGrpcError(w, 14, "Forward to target failed")
		return
	}

	entry.RawResponse = respBody

	// === 阶段二：解码响应，广播 proxy-response ===
	responseJSON := ps.tryDecodeGrpcFrame(respBody, method, "response")
	entry.ResponseJSON = responseJSON
	entry.State = "waiting-response"
	entry.ResolveResponse = make(chan struct{})

	ps.broadcast(ProxyBroadcastMessage{
		Type:      "proxy-response",
		ID:        id,
		Method:    method,
		Target:    targetHost,
		Response:  responseJSON,
		ElapsedMs: forwardElapsed,
		Timestamp: time.Now().Format(time.RFC3339),
	})

	// 等待 WebSocket 客户端修改响应
	if !ps.isPaused() {
		select {
		case <-entry.ResolveResponse:
		case <-time.After(5 * time.Minute):
			log.Printf("[proxy] 等待响应修改超时 id=%d", id)
		}
	}

	if entry.State == "dropped" {
		ps.writeLog(ProxyLogEntry{
			Type:      "DROPPED_RESPONSE",
			Method:    method,
			Target:    targetHost,
			Request:   coalesceJSON(entry.ModifiedRequest, requestJSON),
			Response:  responseJSON,
			ElapsedMs: forwardElapsed,
			Error:     "Response dropped by WebSocket client",
		})
		ps.mu.Lock()
		delete(ps.pending, id)
		ps.mu.Unlock()

		ps.broadcast(ProxyBroadcastMessage{
			Type:    "proxy-done",
			ID:      id,
			Dropped: true,
		})
		ps.writeGrpcError(w, 1, "Response dropped by proxy")
		return
	}

	// 构建最终响应（可能已被修改）
	finalResp := entry.RawResponse
	if entry.ModifiedResponse != nil {
		if encoded := ps.tryEncodeGrpcFrame(entry.ModifiedResponse, method, "response"); encoded != nil {
			finalResp = encoded
		}
	}

	// 返回响应给原始 gRPC 客户端
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Trailer", "grpc-status")
	w.WriteHeader(respStatus)
	w.Write(finalResp)
	w.Header().Set("grpc-status", "0")

	entry.State = "done"
	totalElapsed := time.Since(entry.StartTime).Milliseconds()

	ps.broadcast(ProxyBroadcastMessage{
		Type: "proxy-done",
		ID:   id,
	})

	// 写入 JSONL 日志
	ps.writeLog(ProxyLogEntry{
		Type:      "REQUEST_RESPONSE",
		Method:    method,
		Target:    targetHost,
		Request:   coalesceJSON(entry.ModifiedRequest, requestJSON),
		Response:  coalesceJSON(entry.ModifiedResponse, responseJSON),
		ElapsedMs: totalElapsed,
	})

	// 记录执行历史
	ps.recordExecution(id, method, targetHost, totalElapsed)

	ps.mu.Lock()
	delete(ps.pending, id)
	ps.mu.Unlock()
}

// ============================================================
// WebSocket 消息处理
// ============================================================

// HandleProxyWsMessage 处理前端 WebSocket 消息
// 支持三种消息类型：
//   - proxy-forward: 转发请求（可附带修改） / 修改响应
//   - proxy-send-response: 跳过转发，直接编码返回
//   - proxy-drop: 丢弃请求
func (ps *ProxyServer) HandleProxyWsMessage(raw string) {
	var msg struct {
		Type     string      `json:"type"`
		ID       int         `json:"id"`
		Request  interface{} `json:"request"`
		Response interface{} `json:"response"`
	}
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		return
	}

	ps.mu.Lock()
	entry, ok := ps.pending[msg.ID]
	ps.mu.Unlock()

	if !ok {
		log.Printf("[proxy] HandleProxyWsMessage: 未找到请求 id=%d", msg.ID)
		return
	}

	switch msg.Type {

	// === proxy-forward ===
	// waiting-request: 附带修改后的请求 → 转发到目标
	// waiting-response: 附带修改后的响应 → 返回给客户端
	case "proxy-forward":
		ps.mu.Lock()
		switch entry.State {
		case "waiting-request":
			entry.ModifiedRequest = msg.Request
			entry.State = "forwarded"
			if entry.ResolveRequest != nil {
				select {
				case entry.ResolveRequest <- struct{}{}:
				default:
				}
			}
			log.Printf("[proxy] proxy-forward (request) id=%d", msg.ID)
		case "waiting-response":
			entry.ModifiedResponse = msg.Response
			if entry.ResolveResponse != nil {
				select {
				case entry.ResolveResponse <- struct{}{}:
				default:
				}
			}
			log.Printf("[proxy] proxy-forward (response) id=%d", msg.ID)
		default:
			log.Printf("[proxy] proxy-forward 忽略：请求 id=%d 状态为 %s", msg.ID, entry.State)
		}
		ps.mu.Unlock()

	// === proxy-send-response ===
	// 仅在 waiting-request 状态有效：跳过转发，直接编码返回
	case "proxy-send-response":
		ps.mu.Lock()
		if entry.State == "waiting-request" {
			entry.ModifiedResponse = msg.Response
			entry.State = "send-response"
			if entry.ResolveRequest != nil {
				select {
				case entry.ResolveRequest <- struct{}{}:
				default:
				}
			}
			log.Printf("[proxy] proxy-send-response id=%d", msg.ID)
		} else {
			log.Printf("[proxy] proxy-send-response 忽略：请求 id=%d 状态为 %s", msg.ID, entry.State)
		}
		ps.mu.Unlock()

	// === proxy-drop ===
	// 丢弃请求，返回 gRPC 错误
	case "proxy-drop":
		ps.mu.Lock()
		entry.State = "dropped"
		if entry.ResolveRequest != nil {
			select {
			case entry.ResolveRequest <- struct{}{}:
			default:
			}
		}
		if entry.ResolveResponse != nil {
			select {
			case entry.ResolveResponse <- struct{}{}:
			default:
			}
		}
		ps.mu.Unlock()
		log.Printf("[proxy] proxy-drop id=%d", msg.ID)

	default:
		log.Printf("[proxy] 未知消息类型: %s", msg.Type)
	}
}

// ============================================================
// 目标转发
// ============================================================

// getSharedForwardClient 构建带 SSRF 防护的转发 client（按 target 缓存复用 5 分钟）。
//  1. 预先解析 target 主机，校验每个解析出的 IP 均非私网/保留网段，任一为私网则拒绝；
//  2. 通过 DialTLSContext 将连接固定到已校验的 IP，避免转发时二次解析被重绑定劫持；
//  3. 禁用自动重定向（CheckRedirect 返回 http.ErrUseLastResponse），防止 3xx 跳转到内网绕过校验；
//  4. 按 target 缓存 client，避免每请求都 DNS 解析 + IP 校验 + 新建 Transport。
//
// 注：本代理使用 h2c（HTTP/2 cleartext）转发，:authority 取自原始 Host，
// 连接层仅把地址替换为已校验 IP，不影响上层目标主机标识（SNI/:authority 仍用原 host）。
// clientEntry 缓存的 HTTP 客户端条目（按 target 复用连接）
type clientEntry struct {
	client  *http.Client
	expires time.Time
}

var (
	clientCache   = make(map[string]*clientEntry)
	clientCacheMu sync.Mutex
	clientTTL     = 5 * time.Minute
)

// getSharedForwardClient 获取或创建复用的 HTTP 客户端（按 target 缓存 5 分钟）
// 避免每次请求都创建新的 http2.Transport + DNS 解析 + IP 校验
func getSharedForwardClient(target string, timeout time.Duration) (*http.Client, error) {
	clientCacheMu.Lock()
	defer clientCacheMu.Unlock()

	key := target
	if entry, ok := clientCache[key]; ok && time.Now().Before(entry.expires) {
		return entry.client, nil
	}

	// 创建新客户端
	hostname := target
	if h, _, err := net.SplitHostPort(target); err == nil {
		hostname = h
	}

	ips, err := net.LookupIP(hostname)
	if err != nil {
		return nil, fmt.Errorf("解析目标主机 %q 失败: %v", hostname, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("目标主机 %q 无可用 IP", hostname)
	}

	var pinnedIP net.IP
	for _, ip := range ips {
		if middleware.IsPrivateIP(ip) {
			return nil, fmt.Errorf("目标 %q 解析到私网/保留地址 %s，已拒绝（防 SSRF）", hostname, ip.String())
		}
		if pinnedIP == nil {
			pinnedIP = ip
		}
	}

	dial := func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
		_, port, perr := net.SplitHostPort(addr)
		if perr != nil {
			port = "80"
		}
		return net.Dial(network, net.JoinHostPort(pinnedIP.String(), port))
	}

	c := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP:      true,
			DialTLSContext: dial,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: timeout,
	}

	clientCache[key] = &clientEntry{client: c, expires: time.Now().Add(clientTTL)}
	return c, nil
}

// forwardToTarget 转发请求到目标 gRPC 服务器
func (ps *ProxyServer) forwardToTarget(target, method, ct string, body []byte) ([]byte, int, error) {
	client, err := getSharedForwardClient(target, 30*time.Second)
	if err != nil {
		return nil, 0, err
	}

	url := fmt.Sprintf("http://%s%s", target, method)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", ct)
	req.Header.Set("TE", "trailers")

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	return respBody, resp.StatusCode, nil
}

// ============================================================
// gRPC 帧编解码
// ============================================================

// tryDecodeGrpcFrame 尝试将 gRPC 帧解码为 JSON
// 如果 protobuf 解码成功返回 JSON 对象，失败则返回 base64 包装的原始数据
func (ps *ProxyServer) tryDecodeGrpcFrame(data []byte, method, direction string) interface{} {
	frames := parseGRPCFrames(data)
	if len(frames) == 0 {
		return nil
	}

	methodName := strings.TrimPrefix(method, "/")
	methodInfo := ProtoLoader.GetMethodInfo(methodName)
	if methodInfo == nil {
		// 无方法信息，返回 base64 原始数据
		return map[string]interface{}{
			"_raw_base64":  base64.StdEncoding.EncodeToString(frames[0]),
			"_frame_count": len(frames),
		}
	}

	var msgDesc *ProtoMessageDesc
	if direction == "request" {
		msgDesc = methodInfo.RequestDesc
	} else {
		msgDesc = methodInfo.ResponseDesc
	}

	if msgDesc != nil {
		if jsonBytes, err := ProtoLoader.DecodeToJSON(frames[0], msgDesc); err == nil {
			var result interface{}
			if json.Unmarshal(jsonBytes, &result) == nil {
				return result
			}
		}
	}

	// protobuf 解码失败 → base64 兜底
	return map[string]interface{}{
		"_raw_base64":  base64.StdEncoding.EncodeToString(frames[0]),
		"_frame_count": len(frames),
	}
}

// tryEncodeGrpcFrame 尝试将 JSON 编码为 gRPC 帧
func (ps *ProxyServer) tryEncodeGrpcFrame(jsonData interface{}, method, direction string) []byte {
	methodName := strings.TrimPrefix(method, "/")
	methodInfo := ProtoLoader.GetMethodInfo(methodName)
	if methodInfo == nil {
		return nil
	}

	var msgDesc *ProtoMessageDesc
	if direction == "request" {
		msgDesc = methodInfo.RequestDesc
	} else {
		msgDesc = methodInfo.ResponseDesc
	}

	if msgDesc == nil {
		return nil
	}

	jsonBytes, err := json.Marshal(jsonData)
	if err != nil {
		log.Printf("[proxy] JSON 序列化失败: %v", err)
		return nil
	}

	encoded, err := ProtoLoader.EncodeFromJSON(jsonBytes, msgDesc)
	if err != nil {
		log.Printf("[proxy] protobuf 编码失败: %v", err)
		return nil
	}

	return buildGRPCFrame(encoded)
}

// parseGRPCFrames 解析 gRPC 长度前缀帧
func parseGRPCFrames(data []byte) [][]byte {
	var frames [][]byte
	offset := 0
	for offset+5 <= len(data) {
		length := int(data[offset+1])<<24 | int(data[offset+2])<<16 | int(data[offset+3])<<8 | int(data[offset+4])
		offset += 5
		if offset+length > len(data) {
			break
		}
		frames = append(frames, data[offset:offset+length])
		offset += length
	}
	return frames
}

// buildGRPCFrame 构建 gRPC 长度前缀帧
func buildGRPCFrame(data []byte) []byte {
	buf := make([]byte, 5+len(data))
	buf[0] = 0 // uncompressed
	buf[1] = byte(len(data) >> 24)
	buf[2] = byte(len(data) >> 16)
	buf[3] = byte(len(data) >> 8)
	buf[4] = byte(len(data))
	copy(buf[5:], data)
	return buf
}

// ============================================================
// gRPC 错误响应
// ============================================================

// writeGrpcError 向客户端返回 gRPC 标准错误
// statusCode 为 gRPC 状态码（0=OK, 1=Cancelled, 2=Unknown, 4=DeadlineExceeded, 13=Internal, 14=Unavailable）
func (ps *ProxyServer) writeGrpcError(w http.ResponseWriter, grpcStatusCode int, message string) {
	w.Header().Set("Content-Type", "application/grpc")
	w.Header().Set("Trailer", "grpc-status")
	w.Header().Set("Trailer", "grpc-message")
	w.WriteHeader(200)
	w.Write([]byte{})
	// h2c 下 trailer 设置
	w.Header().Set("grpc-status", fmt.Sprintf("%d", grpcStatusCode))
	w.Header().Set("grpc-message", message)
}

// ============================================================
// 直接发送与重放（供前端调用）
// ============================================================

// SendRequest 发送 gRPC 请求（供前端直接调用，不走拦截流程）
func (ps *ProxyServer) SendRequest(method, requestJSON, headersJSON, targetHost string, timeout int) (string, error) {
	if targetHost == "" {
		ps.mu.Lock()
		targetHost = ps.target
		ps.mu.Unlock()
	}

	methodName := strings.TrimPrefix(method, "/")
	methodInfo := ProtoLoader.GetMethodInfo(methodName)

	var body []byte
	if methodInfo != nil && methodInfo.RequestDesc != nil {
		encoded, err := ProtoLoader.EncodeFromJSON([]byte(requestJSON), methodInfo.RequestDesc)
		if err == nil {
			body = buildGRPCFrame(encoded)
		}
	}

	if body == nil {
		body = []byte(requestJSON)
	}

	if timeout <= 0 {
		timeout = 30
	}

	client, err := getSharedForwardClient(targetHost, time.Duration(timeout)*time.Second)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("http://%s%s", targetHost, method)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/grpc")
	req.Header.Set("TE", "trailers")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if methodInfo != nil && methodInfo.ResponseDesc != nil {
		frames := parseGRPCFrames(respBody)
		if len(frames) > 0 {
			if jsonBytes, err := ProtoLoader.DecodeToJSON(frames[0], methodInfo.ResponseDesc); err == nil {
				return string(jsonBytes), nil
			}
		}
	}

	return base64.StdEncoding.EncodeToString(respBody), nil
}

// HTTPResult 普通 HTTP/REST 请求的结构化响应（供前端 ApiTest 渲染）
type HTTPResult struct {
	Status     int               `json:"status"`
	StatusText string            `json:"statusText"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	Duration   int64             `json:"duration"` // 毫秒
	Error      string            `json:"error,omitempty"`
}

// SendHTTPRequest 发送普通 HTTP/REST 请求（供受信任的服务器侧调用，如 Jira 同步）。
// 与 SendRequest（gRPC 专用）区分：这里按标准 HTTP 语义发包并回传状态码/头/体。
func (ps *ProxyServer) SendHTTPRequest(method, rawURL string, headers, params map[string]string, body string, timeout int) HTTPResult {
	return ps.sendHTTP(method, rawURL, headers, params, body, timeout, nil)
}

// SendHTTPRequestSafe 与 SendHTTPRequest 行为一致，但出站连接经 SSRF 安全拨号
// （解析并校验所有 IP 非内网、pin 到已校验 IP、连接瞬间再校验，消除 DNS 重绑定 TOCTOU）。
// 供用户主动发起的任意 URL 请求（代理发送 / ApiTest 发送）使用。
func (ps *ProxyServer) SendHTTPRequestSafe(method, rawURL string, headers, params map[string]string, body string, timeout int) HTTPResult {
	if timeout <= 0 {
		timeout = 30
	}
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
		Transport: &http.Transport{
			DialContext:    middleware.SafeDialContext,
			DialTLSContext: middleware.SafeDialContext,
		},
	}
	return ps.sendHTTP(method, rawURL, headers, params, body, timeout, client)
}

// sendHTTP 核心实现：构建请求并用给定 client 发送（client 为 nil 时使用默认客户端）。
func (ps *ProxyServer) sendHTTP(method, rawURL string, headers, params map[string]string, body string, timeout int, client *http.Client) HTTPResult {
	if timeout <= 0 {
		timeout = 30
	}
	start := time.Now()

	// 拼接 query 参数到 URL
	if len(params) > 0 {
		u, err := url.Parse(rawURL)
		if err == nil {
			q := u.Query()
			for k, v := range params {
				q.Set(k, v)
			}
			u.RawQuery = q.Encode()
			rawURL = u.String()
		}
	}

	var bodyReader io.Reader
	if body != "" {
		bodyReader = bytes.NewReader([]byte(body))
	}

	req, err := http.NewRequest(strings.ToUpper(method), rawURL, bodyReader)
	if err != nil {
		return HTTPResult{Status: 0, StatusText: "", Error: "构造请求失败: " + err.Error(), Duration: time.Since(start).Milliseconds()}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	if client == nil {
		client = &http.Client{Timeout: time.Duration(timeout) * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return HTTPResult{Status: 0, StatusText: "", Error: err.Error(), Duration: time.Since(start).Milliseconds()}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return HTTPResult{Status: resp.StatusCode, StatusText: resp.Status, Error: "读取响应体失败: " + err.Error(), Duration: time.Since(start).Milliseconds()}
	}

	respHeaders := make(map[string]string)
	for k, vs := range resp.Header {
		if len(vs) > 0 {
			respHeaders[k] = vs[0]
		}
	}

	// 尝试把 JSON 美化输出；失败则原样返回
	bodyStr := string(respBody)
	if len(respBody) > 0 && (strings.Contains(resp.Header.Get("Content-Type"), "json")) {
		var pretty any
		if json.Unmarshal(respBody, &pretty) == nil {
			if b, err := json.MarshalIndent(pretty, "", "  "); err == nil {
				bodyStr = string(b)
			}
		}
	}

	return HTTPResult{
		Status:     resp.StatusCode,
		StatusText: resp.Status,
		Headers:    respHeaders,
		Body:       bodyStr,
		Duration:   time.Since(start).Milliseconds(),
	}
}

// ReplayFrame 重放 gRPC 原始帧
func (ps *ProxyServer) ReplayFrame(targetHost, method, rawFrameBase64 string) (string, error) {
	frameData, err := base64.StdEncoding.DecodeString(rawFrameBase64)
	if err != nil {
		return "", fmt.Errorf("Base64 解码失败: %v", err)
	}

	if targetHost == "" {
		ps.mu.Lock()
		targetHost = ps.target
		ps.mu.Unlock()
	}

	client, err := getSharedForwardClient(targetHost, 30*time.Second)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("http://%s%s", targetHost, method)
	req, err := http.NewRequest("POST", url, bytes.NewReader(frameData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/grpc")
	req.Header.Set("TE", "trailers")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(respBody), nil
}

// ============================================================
// 执行历史与日志
// ============================================================

// GetLogs 获取代理日志
func (ps *ProxyServer) GetLogs() []ProxyLogEntry {
	ps.executionsMu.Lock()
	defer ps.executionsMu.Unlock()

	logs := make([]ProxyLogEntry, 0, len(ps.executions))
	for _, e := range ps.executions {
		logs = append(logs, ProxyLogEntry{
			Seq:       e.ID,
			Method:    e.Method,
			Target:    e.Target,
			ElapsedMs: e.ElapsedMs,
			Timestamp: e.Timestamp,
		})
	}
	return logs
}

// GetExecutions 获取代理执行历史
func (ps *ProxyServer) GetExecutions() []ProxyExecution {
	ps.executionsMu.Lock()
	defer ps.executionsMu.Unlock()

	result := make([]ProxyExecution, len(ps.executions))
	copy(result, ps.executions)
	return result
}

// ClearExecutions 清空代理执行历史
func (ps *ProxyServer) ClearExecutions() {
	ps.executionsMu.Lock()
	defer ps.executionsMu.Unlock()
	ps.executions = nil
}

// recordExecution 记录一条执行历史（P1-8 修复：超过上限时裁剪旧记录）
func (ps *ProxyServer) recordExecution(id int, method, target string, elapsed int64) {
	ps.executionsMu.Lock()
	defer ps.executionsMu.Unlock()

	ps.executions = append(ps.executions, ProxyExecution{
		ID:        id,
		Method:    method,
		Target:    target,
		Timestamp: time.Now().Format(time.RFC3339),
		ElapsedMs: elapsed,
	})

	// 超过上限时裁剪旧记录
	if len(ps.executions) > maxExecutions {
		ps.executions = ps.executions[len(ps.executions)-maxExecutions:]
	}
}

// ============================================================
// JSONL 日志
// ============================================================

// initLog 初始化 JSONL 日志文件
func (ps *ProxyServer) initLog() {
	exePath, _ := os.Executable()
	logDir := filepath.Join(filepath.Dir(exePath), "ProxyLogs")

	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		logDir = "ProxyLogs"
	}

	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("[proxy] 创建日志目录失败: %v", err)
		return
	}

	ps.logDir = logDir

	timestamp := time.Now().Format("20060102_150405")
	logPath := filepath.Join(logDir, fmt.Sprintf("proxy_%s.jsonl", timestamp))

	f, err := os.Create(logPath)
	if err != nil {
		log.Printf("[proxy] 创建日志文件失败: %v", err)
		return
	}

	// 写入元数据行
	meta := fmt.Sprintf(`{"_meta":true,"ver":"1.0","source":"proxy","created_at":"%s"}`+"\n", time.Now().Format(time.RFC3339))
	f.WriteString(meta)

	ps.logFile = f
	ps.logSeq = 0
	ps.startLogWriter() // 启动后台日志 goroutine
	log.Printf("[proxy] 日志文件: %s", logPath)
}

// writeLog 异步写入一条 JSONL 日志（通过 channel 解耦，不阻塞主锁）
func (ps *ProxyServer) writeLog(entry ProxyLogEntry) {
	if ps.logFile == nil {
		return
	}

	seq := atomic.AddInt32(&ps.logSeq, 1)
	entry.Seq = int(seq)
	entry.Timestamp = time.Now().Format(time.RFC3339)
	if entry.Type == "" {
		if entry.Error != "" {
			entry.Type = "ERROR"
		} else {
			entry.Type = "REQUEST_RESPONSE"
		}
	}

	// 非序列化到 channel，由后台 goroutine 持锁写盘
	select {
	case ps.logCh <- entry:
	default:
		// channel 满时丢弃，避免阻塞转发路径
	}
}

// startLogWriter 启动后台日志写入 goroutine
func (ps *ProxyServer) startLogWriter() {
	ps.logCh = make(chan ProxyLogEntry, 256)
	ps.logWg.Add(1)
	go func() {
		defer ps.logWg.Done()
		for entry := range ps.logCh {
			data, _ := json.Marshal(entry)
			ps.mu.Lock()
			if ps.logFile != nil {
				ps.logFile.Write(append(data, '\n'))
			}
			ps.mu.Unlock()
		}
	}()
}

// stopLogWriter 停止后台日志写入 goroutine 并刷盘（P1-9 修复：先 close channel，再 WaitGroup 等待 goroutine 退出）
func (ps *ProxyServer) stopLogWriter() {
	if ps.logCh != nil {
		close(ps.logCh)
		ps.logCh = nil
	}
	ps.logWg.Wait() // 等待 writer goroutine 处理完剩余日志后退出
}

// closeLog 关闭日志文件
func (ps *ProxyServer) closeLog() {
	ps.stopLogWriter() // 先停止写入 goroutine 并刷盘
	if ps.logFile != nil {
		ps.logFile.Close()
		ps.logFile = nil
	}
}

// ============================================================
// 工具函数
// ============================================================

// coalesceJSON 返回第一个非 nil 的值
func coalesceJSON(vals ...interface{}) interface{} {
	for _, v := range vals {
		if v != nil {
			return v
		}
	}
	return nil
}
