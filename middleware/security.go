package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"sync"
	"time"

	"qatest/models"

	"github.com/gin-gonic/gin"
)

// 内网 IP 段
var privateIPBlocks []*net.IPNet

func init() {
	for _, cidr := range []string{
		"127.0.0.0/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
		"::1/128",
		"fc00::/7",
	} {
		_, block, _ := net.ParseCIDR(cidr)
		if block != nil {
			privateIPBlocks = append(privateIPBlocks, block)
		}
	}
}

// SSRFCheck SSRF URL 校验中间件
func SSRFCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 仅对包含 URL 参数的请求进行检查
		urlParam := c.Query("url")
		if urlParam == "" {
			// 优化：仅对可能包含 url 字段的 POST/PUT/PATCH 请求读取 body
			method := c.Request.Method
			if method == "POST" || method == "PUT" || method == "PATCH" {
				// P1-5 修复：限制 body 读取大小为 1MB，避免大 payload DoS
				bodyBytes, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
				c.Request.Body.Close()
				c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

				var body map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &body); err == nil {
					if u, ok := body["url"].(string); ok {
						urlParam = u
					}
				}
			}
		}

		if urlParam != "" {
			if err := ValidateURL(urlParam); err != nil {
				c.JSON(400, models.APIResponse{
					Success: false,
					Error:   err.Error(),
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// ValidateURL 校验 URL 是否安全（非内网地址）
func ValidateURL(rawURL string) error {
	_, err := ResolveSafe(rawURL)
	return err
}

// dnsCache 对域名解析结果做短 TTL 缓存，使中间件校验与实际出站连接解析到同一批 IP，
// 缩小 DNS 重绑定（TOCTOU）的时间窗口。
type dnsCacheEntry struct {
	ips     []net.IP
	expires time.Time
}

var (
	dnsMu       sync.Mutex
	dnsCache    = map[string]dnsCacheEntry{}
	dnsCacheTTL = 10 * time.Second
)

// ResolveSafe 解析 URL 主机并校验：IP 字面量直接校验；域名则解析全部 A/AAAA 记录，
// 仅当存在至少一个非内网 IP 时返回该 IP（否则视为不安全）。结果缓存 10s。
func ResolveSafe(rawURL string) (net.IP, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, &url.Error{Op: "parse", URL: rawURL, Err: err}
	}
	host := parsed.Hostname()
	if host == "" {
		return nil, fmt.Errorf("invalid host")
	}

	// IP 字面量：直接校验
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return nil, fmt.Errorf("private ip not allowed")
		}
		return ip, nil
	}

	// 域名：优先命中缓存
	dnsMu.Lock()
	if e, ok := dnsCache[host]; ok && time.Now().Before(e.expires) {
		ips := e.ips
		dnsMu.Unlock()
		return pickPublic(ips)
	}
	dnsMu.Unlock()

	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, &url.Error{Op: "lookup", URL: rawURL, Err: err}
	}
	dnsMu.Lock()
	dnsCache[host] = dnsCacheEntry{ips: ips, expires: time.Now().Add(dnsCacheTTL)}
	dnsMu.Unlock()
	return pickPublic(ips)
}

// pickPublic 从解析结果中选一个非内网 IP；若全部为内网则返回错误。
func pickPublic(ips []net.IP) (net.IP, error) {
	if len(ips) == 0 {
		return nil, fmt.Errorf("no ip resolved")
	}
	for _, ip := range ips {
		if !isPrivateIP(ip) {
			return ip, nil
		}
	}
	return nil, fmt.Errorf("private ip not allowed")
}

// SafeDialContext 在建立 TCP 连接时把主机解析到已校验的非内网 IP（pin），
// 并在连接瞬间再次校验，消除 DNS 重绑定 TOCTOU。TLS SNI 仍由 http.Transport
// 按原始主机名设置，不受 pin 影响。
func SafeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
		port = "80"
	}
	ip, err := ResolveSafe(host)
	if err != nil {
		return nil, err
	}
	dialer := net.Dialer{}
	return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
}

func isPrivateIP(ip net.IP) bool {
	for _, block := range privateIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// IsPrivateIP 导出的内网/保留网段判断，供 services 包在 SSRF 转发前校验解析出的 IP（L2 修复）。
// 复用包内已初始化的 privateIPBlocks，避免重复定义网段。
func IsPrivateIP(ip net.IP) bool {
	return isPrivateIP(ip)
}

