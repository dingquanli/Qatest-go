package middleware

import (
	"net/http"
	"sync"
	"time"

	"qatest/models"

	"github.com/gin-gonic/gin"
)

// 速率限制配置
const (
	// maxRequests 单 IP 单窗口最大 API 请求数。
	// 静态资源与 SPA fallback 不走限流（router.go 中限流仅挂 /api 组），
	// 300/60s 可容纳 SPA 正常浏览（登录后一次加载约 10~20 个 API 请求 + 多页面导航），
	// 同时仍能拦截脚本化滥用（单 IP 每分钟 300 次 API 已远超正常操作）。
	maxRequests     = 300
	windowDuration  = 60 * time.Second
	cleanupInterval = 5 * time.Minute
	shardCount      = 32 // 分片数：将全局锁拆分为 32 个分片，降低锁争用
)

type rateLimiter struct {
	shards [shardCount]rateShard
}

type rateShard struct {
	mu       sync.Mutex
	visitors map[string]*visitor
}

type visitor struct {
	count    int
	lastSeen time.Time
}

// FNV-1a 哈希快速分片
func shardIndex(ip string) int {
	var h uint32 = 2166136261
	for i := 0; i < len(ip); i++ {
		h ^= uint32(ip[i])
		h *= 16777619
	}
	return int(h % shardCount)
}

var limiter = &rateLimiter{}

func init() {
	for i := range limiter.shards {
		limiter.shards[i].visitors = make(map[string]*visitor)
	}
	go limiter.cleanup()
}

func (rl *rateLimiter) cleanup() {
	for {
		time.Sleep(cleanupInterval)
		for i := range rl.shards {
			s := &rl.shards[i]
			s.mu.Lock()
			for ip, v := range s.visitors {
				if time.Since(v.lastSeen) > windowDuration {
					delete(s.visitors, ip)
				}
			}
			s.mu.Unlock()
		}
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	s := &rl.shards[shardIndex(ip)]
	s.mu.Lock()
	defer s.mu.Unlock()

	v, exists := s.visitors[ip]
	if !exists || time.Since(v.lastSeen) > windowDuration {
		s.visitors[ip] = &visitor{count: 1, lastSeen: time.Now()}
		return true
	}

	v.lastSeen = time.Now()
	v.count++
	return v.count <= maxRequests
}

// RateLimit 速率限制中间件（maxRequests次/60秒/IP，见上方常量）
// IP 来源：gin 的 c.ClientIP()。main.go 已按 TRUSTED_PROXIES 配置可信代理网段；
// 默认（未配置）时不信任任何代理，直接取直连连接 IP —— 攻击者无法用
// X-Forwarded-For 伪造身份绕过限流（代价是反代场景下按代理 IP 计数，需配置 TRUSTED_PROXIES 还原）。
func RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !limiter.allow(ip) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, models.APIResponse{
				Success: false,
				Error:   "请求过于频繁，请稍后再试",
			})
			return
		}
		c.Next()
	}
}
