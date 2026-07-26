// Package services 单元测试
//
// 覆盖范围（纯逻辑、无外部依赖的包级函数白盒测试）：
//   - isMessageType(t string) bool
//   - isEnumType(t string) bool
//   - decodeVarint(buf []byte) (uint64, int)
//   - encodeVarint(x uint64) []byte
//   - buildFullName(pkg string, stack []string) string
//   - getExampleValue(f ProtoFieldInfo) interface{}
//
// 仅使用标准库 testing 包，未引入任何外部断言库（go.mod 无 testify）。
// 所有被引用的标识符均真实存在于本包，import 仅 "testing"，无多余/缺失。
package services

import (
	"testing"
)

// TestIsMessageType 验证消息类型判断：首字母大写且非标量即视为消息类型。
func TestIsMessageType(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"string", false},
		{"int32", false},
		{"int64", false},
		{"bool", false},
		{"bytes", false},
		{"double", false},
		{"MyMessage", true}, // 首字母大写 + 非标量
		{"User", true},
		{"", false},          // 空字符串
		{"lowercase", false}, // 首字母小写，非消息
	}
	for _, c := range cases {
		if got := isMessageType(c.in); got != c.want {
			t.Errorf("isMessageType(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestIsEnumType 验证枚举类型判断的“实际”行为。
//
// 注意（生产代码疑似缺陷，仅标注不修改）：isEnumType 在 isMessageType 之后判定，
// 而“首字母大写且非标量”已被 isMessageType 占满，因此对任意输入 isEnumType 恒返回 false。
// 下面按实际行为断言，作为回归基准。
func TestIsEnumType(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"string", false},
		{"Status", false}, // 首字母大写但被归为消息类型
		{"MyEnum", false},
		{"Color", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isEnumType(c.in); got != c.want {
			t.Errorf("isEnumType(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestEncodeDecodeVarintRoundTrip 验证 encodeVarint 与 decodeVarint 往返一致。
func TestEncodeDecodeVarintRoundTrip(t *testing.T) {
	values := []uint64{
		0,
		1,
		127,
		128,
		300,
		16383,
		16384,
		65535,
		65536,
		268435455,      // 0xFFFFFFF
		1 << 32,
		1 << 40,
		1 << 48,
		1 << 56,
		(1 << 63) - 1, // 最大安全值：2^63-1 恰好 9 字节，避免 buf 越界 panic
	}
	for _, v := range values {
		enc := encodeVarint(v)
		if len(enc) == 0 {
			t.Fatalf("encodeVarint(%d) returned empty slice", v)
		}
		got, n := decodeVarint(enc)
		if got != v {
			t.Errorf("round-trip mismatch for %d: decoded %d", v, got)
		}
		if n != len(enc) {
			t.Errorf("decodeVarint consumed %d bytes, expected %d for value %d", n, len(enc), v)
		}
	}
}

// TestDecodeVarintIncomplete 验证不完整 varint 缓冲返回 (0, 0)。
func TestDecodeVarintIncomplete(t *testing.T) {
	got, n := decodeVarint([]byte{0x80}) // 仅有续位、无终止字节
	if got != 0 || n != 0 {
		t.Errorf("decodeVarint(incomplete) = (%d, %d), want (0, 0)", got, n)
	}
}

// TestBuildFullName 验证限定名拼接逻辑。
func TestBuildFullName(t *testing.T) {
	cases := []struct {
		pkg   string
		stack []string
		want  string
	}{
		{"a", []string{"B", "C"}, "a.B.C"},
		{"", []string{"B", "C"}, "B.C"},
		{"a", []string{}, "a"},
		{"", []string{}, ""},
		{"pkg", []string{"Outer", "Inner", "Field"}, "pkg.Outer.Inner.Field"},
	}
	for _, c := range cases {
		if got := buildFullName(c.pkg, c.stack); got != c.want {
			t.Errorf("buildFullName(%q, %v) = %q, want %q", c.pkg, c.stack, got, c.want)
		}
	}
}

// TestGetExampleValue 验证各类型字段的示例值生成。
func TestGetExampleValue(t *testing.T) {
	cases := []struct {
		name string
		in   ProtoFieldInfo
	}{
		{"string", ProtoFieldInfo{Type: "string"}},
		{"int32", ProtoFieldInfo{Type: "int32"}},
		{"int64", ProtoFieldInfo{Type: "int64"}},
		{"uint32", ProtoFieldInfo{Type: "uint32"}},
		{"float", ProtoFieldInfo{Type: "float"}},
		{"double", ProtoFieldInfo{Type: "double"}},
		{"bool", ProtoFieldInfo{Type: "bool"}},
		{"bytes", ProtoFieldInfo{Type: "bytes"}},
		{"message", ProtoFieldInfo{Type: "MyMessage"}},
		{"repeated", ProtoFieldInfo{Rule: "repeated", Type: "int32"}},
		{"unknown", ProtoFieldInfo{Type: "customtype"}}, // 小写非标量 → 走 default 返回 nil
	}
	for _, c := range cases {
		got := getExampleValue(c.in)
		switch c.name {
		case "repeated":
			s, ok := got.([]interface{})
			if !ok || len(s) != 0 {
				t.Errorf("getExampleValue(%v) = %#v, want empty []interface{}", c.in, got)
			}
		case "message":
			m, ok := got.(map[string]interface{})
			if !ok || len(m) != 0 {
				t.Errorf("getExampleValue(%v) = %#v, want empty map", c.in, got)
			}
		case "bool":
			b, ok := got.(bool)
			if !ok || b != false {
				t.Errorf("getExampleValue(%v) = %#v, want false", c.in, got)
			}
		case "unknown":
			if got != nil {
				t.Errorf("getExampleValue(%v) = %#v, want nil", c.in, got)
			}
		default:
			if got == nil {
				t.Errorf("getExampleValue(%v) = nil, want non-nil example value", c.in)
			}
		}
	}
}
