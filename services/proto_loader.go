package services

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"qatest/config"

	"github.com/jhump/protoreflect/desc/protoparse"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Proto 目录遍历安全上限（防止目录遍历导致整盘解析的 DoS）
const (
	maxProtoFiles    = 200
	maxProtoFileSize = 5 * 1024 * 1024 // 5MB
)

// ============================================================
// ProtoLoader — Proto 文件加载与类型自省
// ============================================================

// ProtoFieldInfo 字段信息
type ProtoFieldInfo struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Rule    string `json:"rule"`
	ID      int    `json:"id"`
	Comment string `json:"comment"`
}

// ProtoMessageDesc 消息描述符
type ProtoMessageDesc struct {
	Name    string           `json:"name"`
	Fields  []ProtoFieldInfo `json:"fields"`
	RawName string           `json:"-"` // 完整限定名
}

// ProtoMethodInfo 方法信息
type ProtoMethodInfo struct {
	ServiceName  string            `json:"serviceName"`
	MethodName   string            `json:"methodName"`
	RequestType  string            `json:"requestType"`
	ResponseType string            `json:"responseType"`
	RequestDesc  *ProtoMessageDesc `json:"-"`
	ResponseDesc *ProtoMessageDesc `json:"-"`
}

// ProtoServiceInfo 服务信息
type ProtoServiceInfo struct {
	Name    string                    `json:"name"`
	Methods map[string]ProtoMethodInfo `json:"methods"`
}

// ProtoLoaderManager Proto 加载管理器
type ProtoLoaderManager struct {
	mu       sync.RWMutex
	protoDir string
	services map[string]ProtoServiceInfo
	methods  map[string]*ProtoMethodInfo  // key: "package.Service/Method"
	messages map[string]*ProtoMessageDesc // key: 完整限定名

	// protobuf 描述符（优先使用，正则解析为 fallback）
	fileDescriptors map[string]*descriptorpb.FileDescriptorProto // proto file path → descriptor
	useDescriptors  bool
	// 描述符索引：用于快速查找
	descMessages map[string]*descriptorpb.DescriptorProto    // 完整限定名 → 消息描述符
	descEnums    map[string]*descriptorpb.EnumDescriptorProto // 完整限定名 → 枚举描述符
	descMethods  map[string]*methodDescriptorRef              // "full.Service/Method" → ref
}

// methodDescriptorRef 方法描述符引用
type methodDescriptorRef struct {
	serviceName string
	methodName  string
	inputType   string
	outputType  string
}

// ProtoLoader 全局实例
var ProtoLoader = &ProtoLoaderManager{
	services:        make(map[string]ProtoServiceInfo),
	methods:         make(map[string]*ProtoMethodInfo),
	messages:        make(map[string]*ProtoMessageDesc),
	fileDescriptors: make(map[string]*descriptorpb.FileDescriptorProto),
	descMessages:    make(map[string]*descriptorpb.DescriptorProto),
	descEnums:       make(map[string]*descriptorpb.EnumDescriptorProto),
	descMethods:     make(map[string]*methodDescriptorRef),
}

// SetDir 设置并加载 Proto 目录
func (pl *ProtoLoaderManager) SetDir(dir string) error {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	if dir == "" {
		return fmt.Errorf("目录不能为空")
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("目录不存在: %s", dir)
	}

	// 限制为可信基目录，避免任意目录（如 "/"）整盘遍历。
	// 仅当配置了 PROTO_DIR 时才强制基目录约束；未配置时不限制基目录，
	// 但仍由下方文件数量/大小上限提供 DoS 防护。
	if base := config.AppConfig.ProtoDir; base != "" {
		cleanDir := filepath.Clean(dir)
		cleanBase := filepath.Clean(base)
		if cleanDir != cleanBase && !strings.HasPrefix(cleanDir, cleanBase+string(os.PathSeparator)) {
			return fmt.Errorf("proto 目录 %s 不在可信基目录 %s 内，已拒绝", dir, base)
		}
	}

	pl.protoDir = dir
	pl.services = make(map[string]ProtoServiceInfo)
	pl.methods = make(map[string]*ProtoMethodInfo)
	pl.messages = make(map[string]*ProtoMessageDesc)
	pl.fileDescriptors = make(map[string]*descriptorpb.FileDescriptorProto)
	pl.descMessages = make(map[string]*descriptorpb.DescriptorProto)
	pl.descEnums = make(map[string]*descriptorpb.EnumDescriptorProto)
	pl.descMethods = make(map[string]*methodDescriptorRef)
	pl.useDescriptors = false

	// 先尝试 protobuf 描述符解析
	if err := pl.loadProtoDescriptors(dir); err != nil {
		log.Printf("[proto] protobuf 描述符加载失败，将使用正则 fallback: %v", err)
	} else if pl.useDescriptors {
		log.Printf("[proto] protobuf 描述符加载成功，将优先使用类型自省")
	}

	// 始终执行正则解析作为 fallback
	return pl.scanProtos(dir)
}

// GetDir 获取当前 Proto 目录
func (pl *ProtoLoaderManager) GetDir() string {
	pl.mu.RLock()
	defer pl.mu.RUnlock()
	return pl.protoDir
}

// GetServices 获取所有服务列表
func (pl *ProtoLoaderManager) GetServices() map[string]ProtoServiceInfo {
	pl.mu.RLock()
	defer pl.mu.RUnlock()

	result := make(map[string]ProtoServiceInfo, len(pl.services))
	for k, v := range pl.services {
		result[k] = v
	}
	return result
}

// GetDescribe 获取 Proto 总览
func (pl *ProtoLoaderManager) GetDescribe() map[string]interface{} {
	pl.mu.RLock()
	defer pl.mu.RUnlock()

	services := make([]map[string]interface{}, 0)
	for _, svc := range pl.services {
		methods := make([]map[string]interface{}, 0)
		for _, m := range svc.Methods {
			methods = append(methods, map[string]interface{}{
				"name":         m.MethodName,
				"requestType":  m.RequestType,
				"responseType": m.ResponseType,
			})
		}
		services = append(services, map[string]interface{}{
			"name":    svc.Name,
			"methods": methods,
		})
	}

	return map[string]interface{}{
		"dir":      pl.protoDir,
		"services": services,
	}
}

// DescribeMethod 获取方法详情
func (pl *ProtoLoaderManager) DescribeMethod(method string) (map[string]interface{}, error) {
	pl.mu.RLock()
	defer pl.mu.RUnlock()

	info, ok := pl.methods[method]
	if !ok {
		info, ok = pl.methods[strings.TrimPrefix(method, "/")]
		if !ok {
			return nil, fmt.Errorf("方法未找到: %s", method)
		}
	}

	result := map[string]interface{}{
		"serviceName": info.ServiceName,
		"methodName":  info.MethodName,
		"requestType": info.RequestType,
		"responseType": info.ResponseType,
	}

	if info.RequestDesc != nil {
		result["requestFields"] = info.RequestDesc.Fields
		result["requestExample"] = pl.buildJSONExampleForMessage(info.RequestDesc)
	}
	if info.ResponseDesc != nil {
		result["responseFields"] = info.ResponseDesc.Fields
		result["responseExample"] = pl.buildJSONExampleForMessage(info.ResponseDesc)
	}

	return result, nil
}

// GetMethodInfo 获取方法信息（供 proxy_server 使用）
func (pl *ProtoLoaderManager) GetMethodInfo(method string) *ProtoMethodInfo {
	pl.mu.RLock()
	defer pl.mu.RUnlock()

	info, ok := pl.methods[method]
	if ok {
		return info
	}
	info, ok = pl.methods[strings.TrimPrefix(method, "/")]
	if ok {
		return info
	}

	for k, v := range pl.methods {
		if strings.HasSuffix(k, method) || strings.HasSuffix(k, "/"+method) {
			return v
		}
	}

	return nil
}

// ============================================================
// Protobuf 描述符加载（google.golang.org/protobuf）
// ============================================================

// loadProtoDescriptors 使用 protoparse 解析 .proto 文件为 FileDescriptorProto
func (pl *ProtoLoaderManager) loadProtoDescriptors(dir string) error {
	var protoFiles []string
	walkErr := error(nil)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && info.Name() == "google" {
			return filepath.SkipDir
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".proto") {
			// 单文件大小与总文件数上限，防止遍历整盘导致 DoS
			if info.Size() > maxProtoFileSize {
				walkErr = fmt.Errorf("proto 文件 %s 大小 %d 超过上限 %d 字节", path, info.Size(), maxProtoFileSize)
				return walkErr
			}
			protoFiles = append(protoFiles, path)
			if len(protoFiles) > maxProtoFiles {
				walkErr = fmt.Errorf("proto 文件数量 %d 超过上限 %d", len(protoFiles), maxProtoFiles)
				return walkErr
			}
		}
		return nil
	})
	if walkErr != nil {
		return walkErr
	}

	if len(protoFiles) == 0 {
		return fmt.Errorf("目录中未找到 .proto 文件")
	}

	// 配置 protoparse 解析器
	parser := protoparse.Parser{
		ImportPaths:           []string{dir},
		IncludeSourceCodeInfo: false,
	}

	// 尝试解析所有文件
	parsedDescriptors, err := parser.ParseFiles(protoFiles...)
	if err != nil {
		return fmt.Errorf("protoparse 解析失败: %v", err)
	}

	successCount := 0
	for _, fd := range parsedDescriptors {
		if fd == nil {
			continue
		}

		// 将 FileDescriptor 转为 FileDescriptorProto
		fdp := fd.AsProto().(*descriptorpb.FileDescriptorProto)

		filePath := fd.GetName()
		pl.fileDescriptors[filePath] = fdp

		// 提取包名
		pkg := fdp.GetPackage()
		pkgPrefix := ""
		if pkg != "" {
			pkgPrefix = pkg + "."
		}

		// 索引枚举
		for _, enum := range fdp.GetEnumType() {
			fullName := pkgPrefix + enum.GetName()
			pl.descEnums[fullName] = enum
		}

		// 索引消息（递归处理嵌套消息）
		for _, msg := range fdp.GetMessageType() {
			pl.indexMessageDescriptor(msg, pkgPrefix)
		}

		// 索引服务和方法
		for _, svc := range fdp.GetService() {
			serviceName := pkgPrefix + svc.GetName()
			for _, method := range svc.GetMethod() {
				methodKey := serviceName + "/" + method.GetName()
				pl.descMethods[methodKey] = &methodDescriptorRef{
					serviceName: serviceName,
					methodName:  method.GetName(),
					inputType:   method.GetInputType(),
					outputType:  method.GetOutputType(),
				}
			}
		}

		successCount++
	}

	if successCount > 0 {
		pl.useDescriptors = true
		log.Printf("[proto] 描述符索引完成: %d 文件, %d 消息, %d 枚举, %d 方法",
			len(pl.fileDescriptors), len(pl.descMessages), len(pl.descEnums), len(pl.descMethods))
	}

	return nil
}

// indexMessageDescriptor 递归索引消息描述符（包括嵌套消息）
func (pl *ProtoLoaderManager) indexMessageDescriptor(msg *descriptorpb.DescriptorProto, prefix string) {
	fullName := prefix + msg.GetName()
	pl.descMessages[fullName] = msg

	// 索引嵌套枚举
	for _, enum := range msg.GetEnumType() {
		pl.descEnums[fullName+"."+enum.GetName()] = enum
	}

	// 递归索引嵌套消息
	for _, nested := range msg.GetNestedType() {
		pl.indexMessageDescriptor(nested, fullName+".")
	}
}

// ============================================================
// Proto 文件扫描与正则解析（fallback）
// ============================================================

var (
	reSyntax    = regexp.MustCompile(`^\s*syntax\s*=\s*"([^"]+)"`)
	rePackage   = regexp.MustCompile(`^\s*package\s+([a-zA-Z0-9_.]+)`)
	reImport    = regexp.MustCompile(`^\s*import\s+"([^"]+)"`)
	reService   = regexp.MustCompile(`^\s*service\s+(\w+)`)
	reRPC       = regexp.MustCompile(`^\s*rpc\s+(\w+)\s*\(\s*(\w+)\s*\)\s*returns\s*\(\s*(\w+)\s*\)`)
	reMessage   = regexp.MustCompile(`^\s*message\s+(\w+)`)
	reField     = regexp.MustCompile(`^\s*(repeated\s+|optional\s+|required\s+)?(\w+)\s+(\w+)\s*=\s*(\d+)`)
	reEnum      = regexp.MustCompile(`^\s*enum\s+(\w+)`)
	reEnumVal   = regexp.MustCompile(`^\s*(\w+)\s*=\s*(\d+)`)
	reOneof     = regexp.MustCompile(`^\s*oneof\s+(\w+)`)
	reCloseBrace = regexp.MustCompile(`^\s*}`)
)

// scanProtos 扫描并解析目录下的所有 .proto 文件（正则 fallback）
func (pl *ProtoLoaderManager) scanProtos(dir string) error {
	var protoFiles []string
	walkErr := error(nil)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && info.Name() == "google" {
			return filepath.SkipDir
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".proto") {
			// 单文件大小与总文件数上限，防止遍历整盘导致 DoS
			if info.Size() > maxProtoFileSize {
				walkErr = fmt.Errorf("proto 文件 %s 大小 %d 超过上限 %d 字节", path, info.Size(), maxProtoFileSize)
				return walkErr
			}
			protoFiles = append(protoFiles, path)
			if len(protoFiles) > maxProtoFiles {
				walkErr = fmt.Errorf("proto 文件数量 %d 超过上限 %d", len(protoFiles), maxProtoFiles)
				return walkErr
			}
		}
		return nil
	})
	if walkErr != nil {
		return walkErr
	}

	log.Printf("[proto] 发现 %d 个 .proto 文件", len(protoFiles))

	for _, fp := range protoFiles {
		if err := pl.parseProtoFile(fp); err != nil {
			log.Printf("[proto] 解析 %s 失败: %v", fp, err)
		}
	}

	log.Printf("[proto] 正则解析完成: %d 服务, %d 方法, %d 消息",
		len(pl.services), len(pl.methods), len(pl.messages))
	return nil
}

// parseProtoFile 解析单个 .proto 文件
func (pl *ProtoLoaderManager) parseProtoFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")

	cleanLines := make([]string, 0, len(lines))
	for _, line := range lines {
		if idx := strings.Index(line, "//"); idx >= 0 {
			line = line[:idx]
		}
		line = strings.TrimSpace(line)
		if line != "" {
			cleanLines = append(cleanLines, line)
		}
	}

	var packageName string
	var currentService string
	var currentMessage string
	var currentEnum string
	var messageStack []string

	for _, line := range cleanLines {
		if reSyntax.MatchString(line) {
			continue
		}

		if m := rePackage.FindStringSubmatch(line); m != nil {
			packageName = m[1]
			continue
		}

		if reImport.MatchString(line) {
			continue
		}

		if m := reEnum.FindStringSubmatch(line); m != nil {
			currentEnum = m[1]
			continue
		}
		if currentEnum != "" && reEnumVal.MatchString(line) {
			continue
		}

		if reCloseBrace.MatchString(line) {
			if currentEnum != "" {
				currentEnum = ""
			} else if len(messageStack) > 0 {
				messageStack = messageStack[:len(messageStack)-1]
				if len(messageStack) == 0 {
					currentMessage = ""
				}
			} else if currentMessage != "" {
				currentMessage = ""
			} else if currentService != "" {
				currentService = ""
			}
			continue
		}

		if reOneof.MatchString(line) {
			continue
		}

		if m := reService.FindStringSubmatch(line); m != nil {
			currentService = m[1]
			continue
		}

		if m := reRPC.FindStringSubmatch(line); m != nil {
			rpcName := m[1]
			reqType := m[2]
			respType := m[3]

			fullQualifier := packageName
			if currentService != "" {
				if fullQualifier != "" {
					fullQualifier += "."
				}
				fullQualifier += currentService
			}

			serviceKey := fullQualifier
			methodKey := fullQualifier + "/" + rpcName

			reqDesc := pl.findMessageDesc(packageName, reqType)
			respDesc := pl.findMessageDesc(packageName, respType)

			methodInfo := &ProtoMethodInfo{
				ServiceName:  fullQualifier,
				MethodName:   rpcName,
				RequestType:  reqType,
				ResponseType: respType,
				RequestDesc:  reqDesc,
				ResponseDesc: respDesc,
			}

			pl.methods[methodKey] = methodInfo

			if svc, ok := pl.services[serviceKey]; ok {
				svc.Methods[rpcName] = *methodInfo
			} else {
				pl.services[serviceKey] = ProtoServiceInfo{
					Name:    fullQualifier,
					Methods: map[string]ProtoMethodInfo{rpcName: *methodInfo},
				}
			}

			continue
		}

		if m := reMessage.FindStringSubmatch(line); m != nil {
			msgName := m[1]
			if currentMessage != "" {
				messageStack = append(messageStack, currentMessage)
			}
			currentMessage = msgName

			fullName := buildFullName(packageName, append(messageStack, currentMessage))
			if _, exists := pl.messages[fullName]; !exists {
				pl.messages[fullName] = &ProtoMessageDesc{
					Name:    currentMessage,
					RawName: fullName,
					Fields:  make([]ProtoFieldInfo, 0),
				}
			}
			continue
		}

		if m := reField.FindStringSubmatch(line); m != nil {
			rule := strings.TrimSpace(m[1])
			if rule == "" {
				rule = "optional"
			}
			fieldType := m[2]
			fieldName := m[3]
			fieldID := 0
			fmt.Sscanf(m[4], "%d", &fieldID)

			if currentMessage != "" {
				fullName := buildFullName(packageName, append(messageStack, currentMessage))
				if msg, ok := pl.messages[fullName]; ok {
					msg.Fields = append(msg.Fields, ProtoFieldInfo{
						Name: fieldName,
						Type: fieldType,
						Rule: strings.TrimSpace(rule),
						ID:   fieldID,
					})
				}
			}
		}
	}

	return nil
}

// findMessageDesc 查找消息描述符
func (pl *ProtoLoaderManager) findMessageDesc(pkg, name string) *ProtoMessageDesc {
	candidates := []string{
		name,
		pkg + "." + name,
		"." + pkg + "." + name,
	}

	for _, c := range candidates {
		if msg, ok := pl.messages[c]; ok {
			return msg
		}
	}

	for k, v := range pl.messages {
		if strings.HasSuffix(k, "."+name) {
			return v
		}
	}

	return nil
}

// isEnumName 判断给定类型名是否为已注册的 enum 类型
func (pl *ProtoLoaderManager) isEnumName(name string) bool {
	if _, ok := pl.descEnums[name]; ok {
		return true
	}
	// 尝试后缀匹配（与 findMessageDesc 逻辑一致）
	for k := range pl.descEnums {
		if strings.HasSuffix(k, "."+name) {
			return true
		}
	}
	return false
}

// enumValueToString 将 enum 数值转换为可读的名称（如 1 → "RED"）
func (pl *ProtoLoaderManager) enumValueToString(typeName string, value uint64) string {
	// 精确匹配
	if enum, ok := pl.descEnums[typeName]; ok {
		for _, v := range enum.GetValue() {
			if v.GetNumber() == int32(value) {
				return v.GetName()
			}
		}
	}
	// 后缀匹配
	for k, enum := range pl.descEnums {
		if strings.HasSuffix(k, "."+typeName) {
			for _, v := range enum.GetValue() {
				if v.GetNumber() == int32(value) {
					return v.GetName()
				}
			}
		}
	}
	return fmt.Sprintf("%d", value)
}

func buildFullName(pkg string, stack []string) string {
	parts := make([]string, 0)
	if pkg != "" {
		parts = append(parts, pkg)
	}
	parts = append(parts, stack...)
	return strings.Join(parts, ".")
}

// ============================================================
// buildJSONExampleForMessage — 内部辅助：根据消息描述符生成 JSON
// ============================================================

// buildJSONExampleForMessage 根据消息描述符生成 JSON 示例（内部方法）
func (pl *ProtoLoaderManager) buildJSONExampleForMessage(msg *ProtoMessageDesc) map[string]interface{} {
	if msg == nil {
		return map[string]interface{}{}
	}

	result := make(map[string]interface{})
	for _, f := range msg.Fields {
		result[f.Name] = getExampleValue(f)
	}
	return result
}

// getExampleValue 根据字段类型返回示例值
func getExampleValue(f ProtoFieldInfo) interface{} {
	if f.Rule == "repeated" {
		return []interface{}{}
	}
	// 修复：先检查 enum 再检查 message（之前 isEnumType 恒 false）
	if isEnumType(f.Type) || f.Type == "enum" {
		return ""
	}
	if isMessageType(f.Type) {
		return map[string]interface{}{}
	}

	switch f.Type {
	case "int32", "sint32", "sfixed32", "int64", "sint64", "sfixed64",
		"uint32", "fixed32", "uint64", "fixed64":
		return 0
	case "float", "double":
		return 0.0
	case "bool":
		return false
	case "string":
		return ""
	case "bytes":
		return ""
	default:
		return nil
	}
}

// ============================================================
// DecodeToJSON / EncodeFromJSON
// ============================================================

// DecodeToJSON 将 protobuf 二进制解码为 JSON
func (pl *ProtoLoaderManager) DecodeToJSON(data []byte, msgDesc *ProtoMessageDesc) ([]byte, error) {
	if msgDesc == nil || len(data) == 0 {
		return nil, fmt.Errorf("无法解码：消息描述符为空或数据为空")
	}

	decoded := pl.decodeWireFormat(data, msgDesc)
	return json.Marshal(decoded)
}

// EncodeFromJSON 将 JSON 编码为 protobuf 二进制
func (pl *ProtoLoaderManager) EncodeFromJSON(jsonData []byte, msgDesc *ProtoMessageDesc) ([]byte, error) {
	if msgDesc == nil || len(jsonData) == 0 {
		return nil, fmt.Errorf("无法编码：消息描述符为空或数据为空")
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(jsonData, &obj); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %v", err)
	}

	return pl.encodeWireFormat(obj, msgDesc), nil
}

// ============================================================
// Protobuf Wire Format 基础编解码
// ============================================================

const (
	wireVarint          = 0
	wireFixed64         = 1
	wireLengthDelimited = 2
	wireFixed32         = 5
)

func (pl *ProtoLoaderManager) decodeWireFormat(data []byte, msgDesc *ProtoMessageDesc) map[string]interface{} {
	fieldMap := make(map[int]*ProtoFieldInfo)
	for i := range msgDesc.Fields {
		fieldMap[msgDesc.Fields[i].ID] = &msgDesc.Fields[i]
	}

	result := make(map[string]interface{})

	offset := 0
	for offset < len(data) {
		tag, n := decodeVarint(data[offset:])
		if n <= 0 {
			break
		}
		offset += n

		fieldNum := int(tag >> 3)
		wireType := int(tag & 0x7)

		fieldInfo := fieldMap[fieldNum]
		fieldName := fmt.Sprintf("field_%d", fieldNum)
		if fieldInfo != nil {
			fieldName = fieldInfo.Name
		}
		isRepeated := fieldInfo != nil && fieldInfo.Rule == "repeated"

		// 将解码后的值写入结果：repeated 字段聚合为数组，其余直接赋值
		setField := func(v interface{}) {
			if !isRepeated {
				result[fieldName] = v
				return
			}
			if existing, ok := result[fieldName]; ok {
				if arr, ok := existing.([]interface{}); ok {
					result[fieldName] = append(arr, v)
					return
				}
				result[fieldName] = []interface{}{existing, v}
				return
			}
			result[fieldName] = []interface{}{v}
		}

		switch wireType {
		case wireVarint:
			val, nn := decodeVarint(data[offset:])
			if nn <= 0 {
				return result
			}
			offset += nn
			if fieldInfo != nil && fieldInfo.Type == "bool" {
				setField(val != 0)
			} else if fieldInfo != nil && (fieldInfo.Type == "enum" || isEnumType(fieldInfo.Type)) {
				// enum 值转为可读名称
				setField(pl.enumValueToString(fieldInfo.Type, val))
			} else if fieldInfo != nil && (fieldInfo.Type == "sint32" || fieldInfo.Type == "sint64") {
				// sint 用 zigzag 编码，解码时反向还原为有符号数
				setField(zigzagDecode(val))
			} else {
				setField(val)
			}

		case wireFixed64:
			if offset+8 > len(data) {
				return result
			}
			bits := binary.LittleEndian.Uint64(data[offset:])
			offset += 8
			if fieldInfo != nil && fieldInfo.Type == "double" {
				setField(math.Float64frombits(bits))
			} else if fieldInfo != nil && (fieldInfo.Type == "sfixed64" || fieldInfo.Type == "sint64") {
				setField(int64(bits))
			} else {
				setField(bits)
			}

		case wireLengthDelimited:
			length, nn := decodeVarint(data[offset:])
			if nn <= 0 {
				return result
			}
			offset += nn
			l := int(length)
			if offset+l > len(data) {
				return result
			}
			payload := data[offset : offset+l]
			offset += l

			if fieldInfo != nil && fieldInfo.Type == "string" {
				setField(string(payload))
			} else if fieldInfo != nil && isEnumType(fieldInfo.Type) {
				// enum 在 wire format 中是 varint，但走到这里说明被当作 length-delimited 处理
				// 尝试解码为 varint 并查找 enum 值名称
				val, _ := decodeVarint(payload)
				setField(pl.enumValueToString(fieldInfo.Type, val))
			} else if fieldInfo != nil && isMessageType(fieldInfo.Type) {
				nestedDesc := pl.findMessageDesc("", fieldInfo.Type)
				if nestedDesc != nil {
					setField(pl.decodeWireFormat(payload, nestedDesc))
				} else {
					setField(map[string]interface{}{
						"_raw_bytes": fmt.Sprintf("%x", payload),
					})
				}
			} else if fieldInfo != nil && isRepeated && isPackedVarintType(fieldInfo.Type) {
				// packed repeated：length-delimited 载荷内是多个连续 varint（proto3 默认打包标量字段）。
				// 整个 payload 已是一个完整数组，直接赋值，不再经 setField 聚合。
				pOff := 0
				var arr []interface{}
				for pOff < len(payload) {
					v, pn := decodeVarint(payload[pOff:])
					if pn <= 0 {
						break
					}
					pOff += pn
					if fieldInfo.Type == "bool" {
						arr = append(arr, v != 0)
					} else if fieldInfo.Type == "enum" || isEnumType(fieldInfo.Type) {
						arr = append(arr, pl.enumValueToString(fieldInfo.Type, v))
					} else if fieldInfo.Type == "sint32" || fieldInfo.Type == "sint64" {
						arr = append(arr, zigzagDecode(v))
					} else {
						arr = append(arr, v)
					}
				}
				result[fieldName] = arr
			} else {
				setField(string(payload))
			}

		case wireFixed32:
			if offset+4 > len(data) {
				return result
			}
			bits := binary.LittleEndian.Uint32(data[offset:])
			offset += 4
			if fieldInfo != nil && fieldInfo.Type == "float" {
				setField(math.Float32frombits(bits))
			} else if fieldInfo != nil && (fieldInfo.Type == "sfixed32" || fieldInfo.Type == "sint32") {
				setField(int32(bits))
			} else {
				setField(bits)
			}
		}
	}

	return result
}

// isPackedVarintType 判断元素类型是否以 varint 表达（可作为 packed repeated 打包）。
func isPackedVarintType(t string) bool {
	switch t {
	case "int32", "int64", "uint32", "uint64", "sint32", "sint64", "bool", "enum":
		return true
	}
	// 自定义 enum 类型名同样以 varint 表达
	return isEnumType(t)
}

func (pl *ProtoLoaderManager) encodeWireFormat(obj map[string]interface{}, msgDesc *ProtoMessageDesc) []byte {
	var buf []byte

	for i := range msgDesc.Fields {
		f := &msgDesc.Fields[i]
		val, ok := obj[f.Name]
		if !ok {
			continue
		}

		if f.Rule == "repeated" {
			// repeated 字段：逐元素独立编码（非 packed），wire 上与官方解码器兼容；
			// 解码端同时支持 packed 与非 packed 两种形式。
			arr, ok := val.([]interface{})
			if !ok {
				continue
			}
			for _, elem := range arr {
				wireType, data := pl.encodeFieldValue(elem, f)
				if data == nil {
					continue
				}
				tag := uint64(f.ID)<<3 | uint64(wireType)
				buf = append(buf, encodeVarint(tag)...)
				buf = append(buf, data...)
			}
			continue
		}

		wireType, data := pl.encodeFieldValue(val, f)
		if data == nil {
			continue
		}

		tag := uint64(f.ID)<<3 | uint64(wireType)
		buf = append(buf, encodeVarint(tag)...)
		buf = append(buf, data...)
	}

	return buf
}

// zigzag 编码：sint32/sint64 使用（proto2/proto3 规范，负数必须 zigzag 否则编码错误）
func zigzagEncode(v int64) uint64 {
	return uint64(v<<1) ^ uint64(v>>63)
}

// zigzagDecode 反向还原 zigzag 编码的有符号数
func zigzagDecode(v uint64) int64 {
	return int64(v>>1) ^ -int64(v&1)
}

func (pl *ProtoLoaderManager) encodeFieldValue(val interface{}, f *ProtoFieldInfo) (wireType int, data []byte) {
	// 先检查 enum，再检查 message（避免 enum 被 message 分支吞掉）
	if isEnumType(f.Type) || f.Type == "enum" {
		var v uint64
		switch n := val.(type) {
		case float64:
			v = uint64(n)
		case int:
			v = uint64(n)
		case int64:
			v = uint64(n)
		case string:
			// enum 字符串值 → 数字字符串直接转换；否则尝试按 enum 名查数值
			if num, err := strconv.ParseUint(n, 10, 64); err == nil {
				v = num
			} else if num, ok := pl.enumValueToNumber(f.Type, n); ok {
				v = num
			} else {
				return wireVarint, nil
			}
		default:
			return wireVarint, nil
		}
		return wireVarint, encodeVarint(v)
	}

	if isMessageType(f.Type) {
		obj, ok := val.(map[string]interface{})
		if !ok {
			return wireLengthDelimited, nil
		}
		nestedDesc := pl.findMessageDesc("", f.Type)
		if nestedDesc == nil {
			return wireLengthDelimited, nil
		}
		inner := pl.encodeWireFormat(obj, nestedDesc)
		return wireLengthDelimited, append(encodeVarint(uint64(len(inner))), inner...)
	}

	switch f.Type {
	case "int32", "int64", "uint32", "uint64", "bool":
		var v uint64
		switch n := val.(type) {
		case float64:
			v = uint64(n)
		case int:
			v = uint64(n)
		case int64:
			v = uint64(n)
		case bool:
			if n {
				v = 1
			}
		default:
			return wireVarint, nil
		}
		return wireVarint, encodeVarint(v)

	case "sint32", "sint64":
		var v int64
		switch n := val.(type) {
		case float64:
			v = int64(n)
		case int:
			v = int64(n)
		case int64:
			v = n
		case bool:
			if n {
				v = 1
			}
		default:
			return wireVarint, nil
		}
		return wireVarint, encodeVarint(zigzagEncode(v))

	case "fixed32", "sfixed32":
		var v uint64
		switch n := val.(type) {
		case float64:
			v = uint64(n)
		case int:
			v = uint64(n)
		case int64:
			v = uint64(n)
		default:
			return wireFixed32, nil
		}
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(v))
		return wireFixed32, buf

	case "fixed64", "sfixed64":
		var v uint64
		switch n := val.(type) {
		case float64:
			v = uint64(n)
		case int:
			v = uint64(n)
		case int64:
			v = uint64(n)
		default:
			return wireFixed64, nil
		}
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, v)
		return wireFixed64, buf

	case "float":
		var v float32
		switch n := val.(type) {
		case float64:
			v = float32(n)
		default:
			return wireFixed32, nil
		}
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, math.Float32bits(v))
		return wireFixed32, buf

	case "double":
		var v float64
		switch n := val.(type) {
		case float64:
			v = n
		default:
			return wireFixed64, nil
		}
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, math.Float64bits(v))
		return wireFixed64, buf

	case "string", "bytes":
		var s string
		switch v := val.(type) {
		case string:
			s = v
		default:
			b, _ := json.Marshal(val)
			s = string(b)
		}
		return wireLengthDelimited, append(encodeVarint(uint64(len(s))), []byte(s)...)

	default:
		return wireLengthDelimited, nil
	}
}

// enumValueToNumber 将 enum 名称转回数值（与 enumValueToString 互逆）。
func (pl *ProtoLoaderManager) enumValueToNumber(typeName, name string) (uint64, bool) {
	lookup := func(enum *descriptorpb.EnumDescriptorProto) (uint64, bool) {
		if enum == nil {
			return 0, false
		}
		for _, v := range enum.GetValue() {
			if v.GetName() == name {
				return uint64(v.GetNumber()), true
			}
		}
		return 0, false
	}
	if enum, ok := pl.descEnums[typeName]; ok {
		return lookup(enum)
	}
	// 后缀匹配（与 findMessageDesc 一致）
	for k, enum := range pl.descEnums {
		if strings.HasSuffix(k, "."+typeName) {
			return lookup(enum)
		}
	}
	return 0, false
}

func isMessageType(t string) bool {
	primitives := map[string]bool{
		"int32": true, "int64": true, "uint32": true, "uint64": true,
		"sint32": true, "sint64": true, "fixed32": true, "fixed64": true,
		"sfixed32": true, "sfixed64": true, "float": true, "double": true,
		"bool": true, "string": true, "bytes": true, "enum": true,
	}
	return !primitives[t] && len(t) > 0 && t[0] >= 'A' && t[0] <= 'Z'
}

// isEnumType 判断类型是否为 enum 类型
// 修复：之前 isMessageType 对所有大写开头的非标量类型返回 true，
// 导致 isEnumType 的 !isMessageType(t) 恒为 false。
// 现在将 "enum" 纳入 isMessageType 的标量列表，使 isEnumType 能正确区分。
// 注：proto 文件解析时 enum 类型名仍为大写开头（如 Color、Status），
// 需通过 descEnums 查找来区分 enum 和 message。此处提供基于已知 enum 描述符的判断。
func isEnumType(t string) bool {
	primitives := map[string]bool{
		"int32": true, "int64": true, "uint32": true, "uint64": true,
		"sint32": true, "sint64": true, "fixed32": true, "fixed64": true,
		"sfixed32": true, "sfixed64": true, "float": true, "double": true,
		"bool": true, "string": true, "bytes": true, "enum": true,
	}
	// 标量类型或 "enum" 关键字 → 不是 enum 类型名
	if primitives[t] || t == "" {
		return false
	}
	// 大写开头 → 可能是 enum 或 message，需通过 descEnums 查找确认
	if t[0] >= 'A' && t[0] <= 'Z' {
		return ProtoLoader.isEnumName(t)
	}
	return false
}

func decodeVarint(buf []byte) (uint64, int) {
	var x uint64
	var s uint
	for i, b := range buf {
		if i >= 10 {
			return 0, 0
		}
		if b < 0x80 {
			return x | uint64(b)<<s, i + 1
		}
		x |= uint64(b&0x7f) << s
		s += 7
	}
	return 0, 0
}

func encodeVarint(x uint64) []byte {
	var buf [10]byte
	var n int
	for n = 0; n < len(buf); n++ {
		buf[n] = byte(x & 0x7f)
		if x < 0x80 {
			break
		}
		buf[n] |= 0x80
		x >>= 7
	}
	return buf[:n+1]
}

func init() {
	log.Println("[proto] Proto 加载器就绪")
}
