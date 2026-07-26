package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"qatest/config"
	"qatest/models"

	"github.com/gin-gonic/gin"
)

// GetLogEntries 获取当前日志条目
func GetLogEntries(c *gin.Context) {
	dir := config.AppConfig.LogDir
	if dir == "" {
		dir = "logs"
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: []interface{}{}})
		return
	}

	type LogFile struct {
		Name    string `json:"name"`
		Size    int64  `json:"size"`
		ModTime string `json:"modTime"`
	}

	files := make([]LogFile, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, _ := entry.Info()
		files = append(files, LogFile{
			Name:    entry.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format("2006-01-02T15:04:05"),
		})
	}

	sort.Slice(files, func(i, j int) bool { return files[i].ModTime > files[j].ModTime })

	// 读取最新 .jsonl 文件
	// 优化：限制读取的日志条目数量（最多 500 条），避免读取超大日志文件
	const maxLogEntries = 500
	result := make([]map[string]interface{}, 0)
	for _, f := range files {
		if strings.HasSuffix(f.Name, ".jsonl") {
			data, err := os.ReadFile(filepath.Join(dir, f.Name))
			if err == nil {
				lines := strings.Split(string(data), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line != "" {
						result = append(result, map[string]interface{}{"raw": line, "file": f.Name})
						if len(result) >= maxLogEntries {
							break
						}
					}
				}
				if len(result) > 0 {
					break
				}
			}
		}
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: result})
}

// listLogFiles 返回日志目录下全部文件名（用于白名单校验）。
func listLogFiles() ([]string, error) {
	dir := config.AppConfig.LogDir
	if dir == "" {
		dir = "logs"
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		names = append(names, entry.Name())
	}
	return names, nil
}

// GetLogFiles 日志文件列表
func GetLogFiles(c *gin.Context) {
	names, err := listLogFiles()
	if err != nil {
		c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: []interface{}{}})
		return
	}

	dir := config.AppConfig.LogDir
	if dir == "" {
		dir = "logs"
	}

	type LogFile struct {
		Name    string `json:"name"`
		Size    int64  `json:"size"`
		ModTime string `json:"modTime"`
	}

	files := make([]LogFile, 0)
	for _, name := range names {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || info.IsDir() {
			continue
		}
		files = append(files, LogFile{
			Name:    name,
			Size:    info.Size(),
			ModTime: info.ModTime().Format("2006-01-02T15:04:05"),
		})
	}

	sort.Slice(files, func(i, j int) bool { return files[i].ModTime > files[j].ModTime })
	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: files})
}

// GetLogFileContent 读取指定日志文件
func GetLogFileContent(c *gin.Context) {
	name := c.Query("name")

	// 日志目录固定取自配置，禁止用户通过 dir 参数任意指定，防止路径穿越
	base := config.AppConfig.LogDir
	if base == "" {
		base = "logs"
	}

	// 校验文件名：禁止为空、包含路径分隔符或 ".."，避免目录遍历
	if name == "" ||
		strings.Contains(name, "..") ||
		strings.ContainsRune(name, os.PathSeparator) ||
		strings.ContainsRune(name, '/') ||
		strings.ContainsRune(name, '\\') {
		c.JSON(http.StatusForbidden, models.APIResponse{Success: false, Error: "非法文件名"})
		return
	}

	path := filepath.Join(base, name)
	cleaned := filepath.Clean(path)

	// 二次校验：确认拼接后路径仍位于 base 目录内（相对路径不以 ".." 开头且不含分隔符）
	rel, err := filepath.Rel(base, cleaned)
	if err != nil ||
		rel == ".." ||
		strings.HasPrefix(rel, ".."+string(os.PathSeparator)) ||
		strings.ContainsRune(rel, os.PathSeparator) {
		c.JSON(http.StatusForbidden, models.APIResponse{Success: false, Error: "非法路径"})
		return
	}

	// 白名单校验：仅允许读取日志目录中真实存在的文件（P2-3 收窄可读范围）。
	allowed, err := listLogFiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{Success: false, Error: "读取日志目录失败"})
		return
	}
	inWhitelist := false
	for _, a := range allowed {
		if a == name {
			inWhitelist = true
			break
		}
	}
	if !inWhitelist {
		c.JSON(http.StatusForbidden, models.APIResponse{Success: false, Error: "文件不在允许列表内"})
		return
	}

	data, err := os.ReadFile(cleaned)
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{Success: false, Error: "文件不存在"})
		return
	}

	// 优化：限制返回内容大小（最大 1MB），避免读取超大日志文件导致内存溢出
	const maxLogSize = 1 << 20 // 1MB
	content := string(data)
	if len(data) > maxLogSize {
		content = string(data[:maxLogSize]) + "\n\n... [文件过大，已截断显示前 1MB 内容]"
	}

	c.JSON(http.StatusOK, models.APIResponse{Success: true, Data: gin.H{"content": content, "name": name}})
}
