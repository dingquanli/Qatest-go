// Package middleware 单元测试
//
// 覆盖范围（白盒，同包可测未导出函数）：
//   - IsPrivateIP(ip net.IP) bool   （导出）
//   - isPrivateIP(ip net.IP) bool   （未导出）
//   - ValidateURL(rawURL string) error （导出）
//
// 仅使用标准库 net / testing 包，未引入任何外部断言库（go.mod 无 testify）。
// ValidateURL 用例全部使用 IP 字面量或空 host 的 URL，避免触发 net.LookupIP 网络调用。
package middleware

import (
	"net"
	"testing"
)

// TestIsPrivateIP 验证内网/保留网段判断（导出与未导出函数行为一致）。
func TestIsPrivateIP(t *testing.T) {
	privateCases := []string{
		"127.0.0.1",
		"10.0.0.1",
		"172.16.5.4",
		"192.168.1.1",
		"169.254.1.1",
		"::1",
	}
	for _, s := range privateCases {
		ip := net.ParseIP(s)
		if ip == nil {
			t.Fatalf("net.ParseIP(%q) returned nil, test setup error", s)
		}
		if !IsPrivateIP(ip) {
			t.Errorf("IsPrivateIP(%q) = false, want true", s)
		}
		if !isPrivateIP(ip) {
			t.Errorf("isPrivateIP(%q) = false, want true", s)
		}
	}

	publicCases := []string{
		"8.8.8.8",
		"1.1.1.1",
		"203.0.113.5",
	}
	for _, s := range publicCases {
		ip := net.ParseIP(s)
		if ip == nil {
			t.Fatalf("net.ParseIP(%q) returned nil, test setup error", s)
		}
		if IsPrivateIP(ip) {
			t.Errorf("IsPrivateIP(%q) = true, want false", s)
		}
		if isPrivateIP(ip) {
			t.Errorf("isPrivateIP(%q) = true, want false", s)
		}
	}
}

// TestValidateURL 验证 URL SSRF 校验。
//
// 说明：原始用例中的 "ftp://x" 其 host 实际为 "x"（非空），会进入 DNS 解析分支触发
// net.LookupIP 网络调用，与“避免网络依赖”约束冲突；故改用 "garbage://"（host 为空，
// 命中空 host 拒绝分支，无网络调用）来覆盖该路径。这样既能验证拒绝逻辑，又保持稳定。
func TestValidateURL(t *testing.T) {
	cases := []struct {
		rawURL string
		wantOK bool
	}{
		{"http://127.0.0.1", false}, // 内网 IPv4，拒绝
		{"http://8.8.8.8", true},    // 公网 IP 字面量，不触发 DNS
		{"http://[::1]", false},     // IPv6 环回，拒绝
		{"garbage://", false},       // host 为空，拒绝（无 DNS）
		{"not-a-url", false},        // 无 host，拒绝（无 DNS）
	}
	for _, c := range cases {
		err := ValidateURL(c.rawURL)
		if c.wantOK && err != nil {
			t.Errorf("ValidateURL(%q) returned err = %v, want nil", c.rawURL, err)
		}
		if !c.wantOK && err == nil {
			t.Errorf("ValidateURL(%q) returned nil, want error", c.rawURL)
		}
	}
}
