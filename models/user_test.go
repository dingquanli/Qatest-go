// Package models 单元测试
//
// 覆盖范围：
//   - NowStr() string
//
// 仅使用标准库 testing / time 包，未引入任何外部断言库（go.mod 无 testify）。
package models

import (
	"testing"
	"time"
)

// TestNowStr 校验 NowStr 返回非空且可被 time.RFC3339 解析的时间字符串。
func TestNowStr(t *testing.T) {
	s := NowStr()
	if s == "" {
		t.Fatalf("NowStr() returned empty string")
	}
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Errorf("NowStr() = %q is not valid RFC3339: %v", s, err)
	}
	if parsed.IsZero() {
		t.Errorf("NowStr() parsed to zero time, unexpected")
	}
}
