package config

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// loadDotEnv 读取工作目录下的 .env 文件并注入进程环境变量（零第三方依赖）。
// 规则：
//   - 已在真实环境变量中设置的键不会被覆盖（真实环境变量优先级最高）
//   - 支持 # 行注释、空行、KEY=VALUE、以及首尾的成对引号
//   - .env 不存在时静默跳过，不视为错误
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // 文件不存在等情况直接跳过
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		// 去掉成对的引号
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		// 真实环境变量优先，不覆盖
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}

// UserConfig 用户配置结构
type UserConfig struct {
	Username     string `json:"username"`
	PasswordHash string `json:"passwordHash"`
	Name         string `json:"name"`
	Role         string `json:"role"`
}

// Config 全局配置
type Config struct {
	Port           string
	DBPath         string
	JWTSecret      string
	JWTExpiresIn   time.Duration
	AllowedOrigins []string
	// TrustedProxies 可信反向代理网段（CIDR 列表）。
	// 为空表示不信任任何代理：gin 的 c.ClientIP() 将返回直连 IP（连接 IP），
	// 攻击者无法用 X-Forwarded-For 伪造限流/审计 IP；部署在反代后时，
	// 需在 .env 中用 TRUSTED_PROXIES 显式声明代理网段以还原真实客户端 IP。
	TrustedProxies []string
	LogLevel       string
	ProxyTarget    string
	ProtoDir       string
	LogDir         string
	ApkDir         string
	Users          []UserConfig
	// ExecutorEnabled 脚本执行引擎总开关（默认开启，与历史行为一致）。
	// 设为 false 时拒绝创建任何脚本执行任务（RCE 高危能力的部署侧熔断开关）。
	ExecutorEnabled bool
	// Jira 同步配置（环境变量为源，系统设置表可覆盖）
	JiraBaseURL  string
	JiraEmail     string
	JiraAPIToken  string
	JiraProject   string
}

// JSAuthToken JS 脚本执行时使用的 JWT token
var JSAuthToken string

var AppConfig *Config

// Load 加载配置
func Load() *Config {
	// 优先加载 .env（唯一配置源）；真实环境变量仍可覆盖 .env
	loadDotEnv(".env")

	cfg := &Config{
		Port:        getEnv("PORT", "3000"),
		DBPath:      getEnv("DB_PATH", "qatest.db"),
		JWTSecret:   getEnv("JWT_SECRET", "change_me_to_a_random_secret_string"),
		LogLevel:    getEnv("LOG_LEVEL", "INFO"),
		ProxyTarget: getEnv("PROXY_TARGET", ""),
		ProtoDir:    getEnv("PROTO_DIR", ""),
		LogDir:      getEnv("LOG_DIR", "logs"),
		ApkDir:      getEnv("APK_DIR", ""),
		JiraBaseURL:  getEnv("JIRA_BASE_URL", ""),
		JiraEmail:     getEnv("JIRA_EMAIL", ""),
		JiraAPIToken:  getEnv("JIRA_API_TOKEN", ""),
		JiraProject:   getEnv("JIRA_PROJECT", ""),
		Users: []UserConfig{
			// 默认 admin 用户的口令哈希始终由 ADMIN_PASSWORD 覆盖（下方强校验必填），
			// 此处不写死任何 bcrypt 哈希，避免开源仓库中出现「经典 password 哈希」误导读者。
			{Username: "admin", PasswordHash: "", Name: "管理员", Role: "admin"},
		},
	}

	expiresIn := getEnv("JWT_EXPIRES_IN", "24h")
	dur, err := time.ParseDuration(expiresIn)
	if err != nil {
		dur = 24 * time.Hour
	}
	cfg.JWTExpiresIn = dur

	origins := getEnv("ALLOWED_ORIGINS", "http://localhost:3000")
	cfg.AllowedOrigins = strings.Split(origins, ",")
	for i := range cfg.AllowedOrigins {
		cfg.AllowedOrigins[i] = strings.TrimSpace(cfg.AllowedOrigins[i])
	}

	// 可信反向代理网段（逗号分隔 CIDR，如 "10.0.0.0/8,172.16.0.0/12"）
	trusted := getEnv("TRUSTED_PROXIES", "")
	for _, p := range strings.Split(trusted, ",") {
		if p = strings.TrimSpace(p); p != "" {
			cfg.TrustedProxies = append(cfg.TrustedProxies, p)
		}
	}

	// RCE 高危能力熔断开关：默认开启（保持历史行为）；"0"/"false"/"off" 关闭
	cfg.ExecutorEnabled = !strings.EqualFold(getEnv("EXECUTOR_ENABLED", "1"), "0") &&
		!strings.EqualFold(getEnv("EXECUTOR_ENABLED", "1"), "false") &&
		!strings.EqualFold(getEnv("EXECUTOR_ENABLED", "1"), "off")

	if usersJSON := os.Getenv("QATEST_USERS"); usersJSON != "" {
		var users []UserConfig
		if err := json.Unmarshal([]byte(usersJSON), &users); err == nil {
			cfg.Users = users
		}
	}

	// 默认管理员口令处理（未设 ADMIN_PASSWORD 时拒绝启动，而非仅告警）
	if adminPwd := os.Getenv("ADMIN_PASSWORD"); adminPwd != "" {
		hashed, hashErr := bcrypt.GenerateFromPassword([]byte(adminPwd), bcrypt.DefaultCost)
		if hashErr != nil {
			log.Fatalf("生成默认管理员口令哈希失败: %v", hashErr)
		}
		for i := range cfg.Users {
			if cfg.Users[i].Username == "admin" {
				cfg.Users[i].PasswordHash = string(hashed)
			}
		}
		log.Printf("INFO: 已使用环境变量 ADMIN_PASSWORD 覆盖默认管理员口令哈希")
	} else {
		// 不再仅告警，而是拒绝启动
		log.Fatalf("ADMIN_PASSWORD 未设置，拒绝启动；请配置管理员口令环境变量")
	}

	// 读取 JS 脚本执行认证 token
	JSAuthToken = os.Getenv("JS_AUTH_TOKEN")

	AppConfig = cfg

	// 安全校验：禁止使用空密钥或默认弱密钥，否则任何人都可伪造 JWT（含 admin）导致认证绕过
	const defaultJWTSecret = "change_me_to_a_random_secret_string"
	if cfg.JWTSecret == "" || cfg.JWTSecret == defaultJWTSecret {
		log.Fatalf("JWT_SECRET 未设置或使用默认弱密钥，拒绝启动；请配置随机 32+ 字节的强密钥")
	}

	return cfg
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
