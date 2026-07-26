package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// ADB Shell 命令白名单（23 个安全命令）
var safeADBCommands = []string{
	"am",
	"pm",
	"input",
	"dumpsys",
	"cmd",
	"settings",
	"getprop",
	"setprop",
	"content",
	"monkey",
	"uiautomator",
	"screencap",
	"screenrecord",
	"wm",
	"svc",
	"logcat",
	"dmesg",
	"ls",
	"cat",
	"head",
	"tail",
	"wc",
	"list packages",
}

// 危险字符/模式黑名单（13 个正则）
var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`[;&|]`),                              // 命令链
	regexp.MustCompile("`"),                                   // 命令替换（反引号）
	regexp.MustCompile(`\$\(`),                                // 命令替换 $(...)
	regexp.MustCompile(`\$\{`),                                // 变量替换 ${...}
	regexp.MustCompile(`>`),                                   // 重定向
	regexp.MustCompile(`rm\s+-rf`),                            // 强制删除
	regexp.MustCompile(`/dev/null`),                           // 设备操作
	regexp.MustCompile(`mkfs`),                                // 格式化
	regexp.MustCompile(`dd\s+if=`),                            // 磁盘操作
	regexp.MustCompile(`chmod\s+777`),                         // 危险权限
	regexp.MustCompile(`\.\./`),                               // 路径遍历
	regexp.MustCompile(`\\x[0-9a-fA-F]{2}`),                   // 十六进制编码绕过
	regexp.MustCompile(`(curl|wget|nc|python|perl|ruby)\s`),   // 危险命令
}

// ValidateShellCommand 校验 ADB Shell 命令安全性
// 白名单优先：命令必须以安全命令开头
// 黑名单兜底：检测危险字符/模式
func ValidateShellCommand(command string) bool {
	cmd := strings.TrimSpace(command)

	// 空命令不通过
	if cmd == "" {
		return false
	}

	// 白名单检查：命令必须以安全命令开头
	allowed := false
	cmdLower := strings.ToLower(cmd)
	for _, safe := range safeADBCommands {
		if strings.HasPrefix(cmdLower, safe) {
			// 确保是完整前缀匹配（如下一个字符是空格或命令结束）
			rest := cmdLower[len(safe):]
			if rest == "" || rest[0] == ' ' || rest[0] == '\t' {
				allowed = true
				break
			}
		}
	}
	if !allowed {
		return false
	}

	// 黑名单检查：检测危险字符/模式
	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(cmd) {
			return false
		}
	}

	return true
}


// GenerateSecureID 生成安全 ID
func GenerateSecureID(prefix string) string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(b))
}
