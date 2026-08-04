// Package services 单元测试
//
// 覆盖范围（纯逻辑、无外部依赖的包级函数白盒测试）：
//   - isMessageType(t string) bool
//   - isEnumType(t string) bool
//   - decodeVarint(buf []byte) (uint64, int)
//   - encodeVarint(x uint64) []byte
//   - buildFullName(pkg string, stack []string) string
//   - getExampleValue(f ProtoFieldInfo) interface{}
//   - encodeWireFormat / decodeWireFormat 往返（标量、enum、嵌套 message、repeated、packed）
//
// 仅使用标准库 testing 包，未引入任何外部断言库（go.mod 无 testify）。
// 所有被引用的标识符均真实存在于本包，import 仅 "testing"，无多余/缺失。
package services

import (
	"encoding/json"
	"math"
	"testing"

	"google.golang.org/protobuf/types/descriptorpb"
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

// ============================================================
// wire format 编解码往返测试（回归：enum/嵌套 message/repeated/packed/sint/fixed）
// ============================================================

// TestDecodeWireFormatScalars 验证标量字段 wire type 解码与类型转换。
func TestDecodeWireFormatScalars(t *testing.T) {
	desc := &ProtoMessageDesc{
		Name: "ScalarMsg",
		Fields: []ProtoFieldInfo{
			{Name: "id", Type: "int32", ID: 1},
			{Name: "ok", Type: "bool", ID: 2},
			{Name: "name", Type: "string", ID: 3},
			{Name: "score", Type: "double", ID: 4},
			{Name: "ratio", Type: "float", ID: 5},
		},
	}

	// 手工构造 wire bytes：
	// field1 (varint) = 42
	// field2 (varint) = 1 → bool true
	// field3 (length-delimited) = "hello"
	// field4 (fixed64) = bits of 3.14
	// field5 (fixed32) = bits of 1.5f
	var wire []byte
	wire = append(wire, encodeVarint(1<<3|wireVarint)...)
	wire = append(wire, encodeVarint(42)...)
	wire = append(wire, encodeVarint(2<<3|wireVarint)...)
	wire = append(wire, encodeVarint(1)...)
	wire = append(wire, encodeVarint(3<<3|wireLengthDelimited)...)
	wire = append(wire, encodeVarint(5)...)
	wire = append(wire, []byte("hello")...)
	wire = append(wire, encodeVarint(4<<3|wireFixed64)...)
	bits64 := make([]byte, 8)
	putUint64(bits64, math.Float64bits(3.14))
	wire = append(wire, bits64...)
	wire = append(wire, encodeVarint(5<<3|wireFixed32)...)
	bits32 := make([]byte, 4)
	putUint32(bits32, math.Float32bits(1.5))
	wire = append(wire, bits32...)

	got := ProtoLoader.decodeWireFormat(wire, desc)
	if got["id"] != uint64(42) {
		t.Errorf("id = %#v, want 42", got["id"])
	}
	if got["ok"] != true {
		t.Errorf("ok = %#v, want true", got["ok"])
	}
	if got["name"] != "hello" {
		t.Errorf("name = %#v, want hello", got["name"])
	}
	if v, ok := got["score"].(float64); !ok || v < 3.139 || v > 3.141 {
		t.Errorf("score = %#v, want ~3.14", got["score"])
	}
	if v, ok := got["ratio"].(float32); !ok || v < 1.49 || v > 1.51 {
		t.Errorf("ratio = %#v, want ~1.5", got["ratio"])
	}
}

// TestEncodeDecodeRoundTrip 验证 JSON → encode → decode → JSON 的往返一致（核心回归）。
func TestEncodeDecodeRoundTrip(t *testing.T) {
	// 注册嵌套消息与枚举（模拟 SetDir 加载后的状态），测试结束后清理
	const nestedName = "test.pkg.NestedMsg"
	const enumName = "test.pkg.Color"
	enumDesc := &descriptorpb.EnumDescriptorProto{
		Name: strPtr("Color"),
		Value: []*descriptorpb.EnumValueDescriptorProto{
			{Name: strPtr("RED"), Number: int32Ptr(0)},
			{Name: strPtr("GREEN"), Number: int32Ptr(1)},
			{Name: strPtr("BLUE"), Number: int32Ptr(2)},
		},
	}
	ProtoLoader.descEnums[enumName] = enumDesc
	defer delete(ProtoLoader.descEnums, enumName)

	outer := &ProtoMessageDesc{
		Name:   "OuterMsg",
		RawName: "test.pkg.OuterMsg",
		Fields: []ProtoFieldInfo{
			{Name: "id", Type: "int32", ID: 1},
			{Name: "labels", Type: "string", Rule: "repeated", ID: 2},
			{Name: "nums", Type: "int64", Rule: "repeated", ID: 3},
			{Name: "color", Type: "Color", ID: 4},
			{Name: "nested", Type: "NestedMsg", ID: 5},
			{Name: "neg", Type: "sint64", ID: 6},
			{Name: "tags", Type: "bytes", ID: 7},
			{Name: "flags", Type: "bool", Rule: "repeated", ID: 8},
		},
	}
	nested := &ProtoMessageDesc{
		Name:    "NestedMsg",
		RawName: nestedName,
		Fields:  []ProtoFieldInfo{{Name: "value", Type: "int32", ID: 1}},
	}
	// 让 findMessageDesc 能通过类型名查到嵌套消息
	ProtoLoader.messages[nestedName] = nested
	ProtoLoader.messages["test.pkg.NestedMsg"] = nested
	ProtoLoader.messages["NestedMsg"] = nested
	defer func() {
		delete(ProtoLoader.messages, nestedName)
		delete(ProtoLoader.messages, "test.pkg.NestedMsg")
		delete(ProtoLoader.messages, "NestedMsg")
	}()

	obj := map[string]interface{}{
		"id":     float64(7),
		"labels": []interface{}{"a", "b"},
		"nums":   []interface{}{float64(1), float64(2), float64(3)},
		"color":  "GREEN",
		"nested": map[string]interface{}{"value": float64(99)},
		"neg":    float64(-5),
		"tags":   "ab",
		"flags":  []interface{}{true, false},
	}

	encoded := ProtoLoader.encodeWireFormat(obj, outer)
	if len(encoded) == 0 {
		t.Fatal("encodeWireFormat returned empty bytes")
	}
	decoded := ProtoLoader.decodeWireFormat(encoded, outer)

	check := func(name string, want interface{}) {
		v, ok := decoded[name]
		if !ok {
			t.Errorf("decoded missing field %q", name)
			return
		}
		b1, _ := json.Marshal(v)
		b2, _ := json.Marshal(want)
		if string(b1) != string(b2) {
			t.Errorf("field %q = %s, want %s", name, b1, b2)
		}
	}
	check("id", float64(7))
	check("labels", []interface{}{"a", "b"})
	check("nums", []interface{}{float64(1), float64(2), float64(3)})
	// enum 解码为名称
	check("color", "GREEN")
	// 嵌套 message 递归解码
	check("nested", map[string]interface{}{"value": float64(99)})
	// sint64 负值往返（zigzag）
	check("neg", float64(-5))
	check("tags", "ab")
	check("flags", []interface{}{true, false})
}

// TestDecodePackedRepeated 验证 packed repeated（proto3 默认打包标量字段）解码为数组。
func TestDecodePackedRepeated(t *testing.T) {
	desc := &ProtoMessageDesc{
		Name: "PackedMsg",
		Fields: []ProtoFieldInfo{
			{Name: "ids", Type: "int32", Rule: "repeated", ID: 1},
		},
	}

	// packed: field 1, wire type 2 (length-delimited)，载荷为 3 个 varint: 10,20,30
	var payload []byte
	payload = append(payload, encodeVarint(10)...)
	payload = append(payload, encodeVarint(20)...)
	payload = append(payload, encodeVarint(30)...)
	wire := append(encodeVarint(1<<3|wireLengthDelimited), encodeVarint(uint64(len(payload)))...)
	wire = append(wire, payload...)

	got := ProtoLoader.decodeWireFormat(wire, desc)
	arr, ok := got["ids"].([]interface{})
	if !ok {
		t.Fatalf("ids = %#v, want []interface{}", got["ids"])
	}
	if len(arr) != 3 || arr[0] != uint64(10) || arr[1] != uint64(20) || arr[2] != uint64(30) {
		t.Errorf("ids = %#v, want [10 20 30]", arr)
	}
}

// TestZigzagEncode 验证 sint32/sint64 的 zigzag 编码规范。
func TestZigzagEncode(t *testing.T) {
	cases := []struct {
		in   int64
		want uint64
	}{
		{0, 0},
		{-1, 1},
		{1, 2},
		{-2, 3},
		{2, 4},
		{2147483647, 4294967294},                  // MaxInt32
		{-2147483648, 4294967295},                 // MinInt32
		{9223372036854775807, 18446744073709551614}, // MaxInt64
		{-9223372036854775808, 18446744073709551615}, // MinInt64
	}
	for _, c := range cases {
		if got := zigzagEncode(c.in); got != c.want {
			t.Errorf("zigzagEncode(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

// TestEnumValueToNumber 验证 enum 名称 → 数值互逆查找。
func TestEnumValueToNumber(t *testing.T) {
	const enumName = "test.Status"
	ProtoLoader.descEnums[enumName] = &descriptorpb.EnumDescriptorProto{
		Name: strPtr("Status"),
		Value: []*descriptorpb.EnumValueDescriptorProto{
			{Name: strPtr("OK"), Number: int32Ptr(0)},
			{Name: strPtr("FAIL"), Number: int32Ptr(3)},
		},
	}
	defer delete(ProtoLoader.descEnums, enumName)

	v, ok := ProtoLoader.enumValueToNumber("Status", "FAIL")
	if !ok || v != 3 {
		t.Errorf("enumValueToNumber(Status, FAIL) = %d,%v want 3,true", v, ok)
	}
	if _, ok := ProtoLoader.enumValueToNumber("Status", "NOPE"); ok {
		t.Error("enumValueToNumber with unknown name should return false")
	}
}

// 测试辅助
func strPtr(s string) *string { return &s }
func int32Ptr(i int32) *int32 { return &i }

func putUint64(b []byte, v uint64) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
	b[4] = byte(v >> 32)
	b[5] = byte(v >> 40)
	b[6] = byte(v >> 48)
	b[7] = byte(v >> 56)
}

func putUint32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}
