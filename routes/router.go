package routes

import (
	"qatest/handlers"
	"qatest/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes 注册所有 API 路由
func RegisterRoutes(r *gin.Engine) {
	// 全局中间件（静态资源与 SPA fallback 不经过限流，避免浏览器加载 chunk 消耗 API 配额）
	r.Use(middleware.Logger())
	r.Use(middleware.CORS())
	r.Use(middleware.SSRFCheck())

	api := r.Group("/api")
	// 限流仅作用于 API（含登录/上报等公开端点），防暴力破解与脚本灌库
	api.Use(middleware.RateLimit())

	// Auth（白名单，无需 JWT）
	api.POST("/auth/login", handlers.Login)
	api.POST("/auth/refresh", handlers.RefreshToken)

	// 需要认证的路由
	auth := api.Group("")
	auth.Use(middleware.Auth())

	// 设备管理
	auth.GET("/devices", handlers.GetDevices)
	auth.GET("/devices/scan", handlers.ScanDevices)
	auth.GET("/devices/:serial", handlers.GetDevice)
	auth.POST("/devices/:serial/screenshot", handlers.TakeScreenshot)
	auth.POST("/devices/:serial/exec", handlers.ExecDeviceCommand)
	auth.POST("/devices/:serial/install", handlers.InstallAPK)

	// 脚本管理
	auth.GET("/scripts", handlers.GetScripts)
	auth.GET("/scripts/:id", handlers.GetScript)
	auth.POST("/scripts", handlers.CreateScript)
	auth.PUT("/scripts/:id", handlers.UpdateScript)
	auth.DELETE("/scripts/:id", handlers.DeleteScript)

	// 脚本执行
	auth.GET("/executions", handlers.GetExecutions)
	auth.GET("/executions/:id", handlers.GetExecution)
	auth.POST("/executions", handlers.CreateExecution)
	auth.POST("/executions/:id/cancel", handlers.CancelExecution)

	// 缺陷管理
	auth.GET("/bugs", handlers.GetBugs)
	auth.GET("/bugs/stats", handlers.GetBugStats)
	auth.GET("/bugs/:id", handlers.GetBug)
	auth.POST("/bugs", handlers.CreateBug)
	auth.PUT("/bugs/:id", handlers.UpdateBug)
	auth.DELETE("/bugs/:id", handlers.DeleteBug)
	auth.POST("/bugs/:id/sync", handlers.SyncBugToJira)

	// 测试用例
	auth.GET("/cases", handlers.GetTestCases)
	auth.GET("/cases/:id", handlers.GetTestCase)
	auth.POST("/cases", handlers.CreateTestCase)
	auth.PUT("/cases/:id", handlers.UpdateTestCase)
	auth.DELETE("/cases/:id", handlers.DeleteTestCase)
	auth.POST("/cases/batch", handlers.BatchImportCases)

	// 用例模块
	auth.GET("/case-modules", handlers.GetCaseModules)
	auth.POST("/case-modules", handlers.CreateCaseModule)
	auth.PUT("/case-modules/:id", handlers.UpdateCaseModule)
	auth.DELETE("/case-modules/:id", handlers.DeleteCaseModule)

	// 用例执行记录
	auth.GET("/case-executions", handlers.GetCaseExecutions)
	auth.GET("/case-executions/stats", handlers.GetCaseExecutionsStats)
	auth.POST("/case-executions", handlers.CreateCaseExecution)
	auth.PUT("/case-executions/:id", handlers.UpdateCaseExecution)
	auth.DELETE("/case-executions/:id", handlers.DeleteCaseExecution)

	// 测试计划
	auth.GET("/test-plans", handlers.GetTestPlans)
	auth.GET("/test-plans/:id", handlers.GetTestPlan)
	auth.POST("/test-plans", handlers.CreateTestPlan)
	auth.PUT("/test-plans/:id", handlers.UpdateTestPlan)
	auth.DELETE("/test-plans/:id", handlers.DeleteTestPlan)
	auth.POST("/test-plans/:id/execute", handlers.ExecuteTestPlan) // 测试计划执行引擎

	// 计划执行记录
	auth.GET("/plan-executions", handlers.GetPlanExecutions)
	auth.POST("/plan-executions", handlers.CreatePlanExecution)

	// 自动化任务执行记录
	auth.GET("/auto-task-executions", handlers.GetAutoTaskExecutions)
	auth.POST("/auto-task-executions", handlers.CreateAutoTaskExecution)

	// 接口定义
	auth.GET("/api-definitions", handlers.GetAPIDefinitions)
	auth.GET("/api-definitions/:id", handlers.GetAPIDefinition)
	auth.POST("/api-definitions", handlers.CreateAPIDefinition)
	auth.PUT("/api-definitions/:id", handlers.UpdateAPIDefinition)
	auth.DELETE("/api-definitions/:id", handlers.DeleteAPIDefinition)

	// 接口定义模块
	auth.GET("/api-def-modules", handlers.GetAPIDefModules)
	auth.POST("/api-def-modules", handlers.CreateAPIDefModule)
	auth.PUT("/api-def-modules/:id", handlers.UpdateAPIDefModule)
	auth.DELETE("/api-def-modules/:id", handlers.DeleteAPIDefModule)

	// API 请求管理
	auth.GET("/api-requests", handlers.GetAPIRequests)
	auth.GET("/api-requests/:id", handlers.GetAPIRequest)
	auth.POST("/api-requests", handlers.CreateAPIRequest)
	auth.PUT("/api-requests/:id", handlers.UpdateAPIRequest)
	auth.DELETE("/api-requests/:id", handlers.DeleteAPIRequest)

	// API 文件夹
	auth.GET("/api-folders", handlers.GetAPIFolders)
	auth.POST("/api-folders", handlers.CreateAPIFolder)
	auth.PUT("/api-folders/:id", handlers.UpdateAPIFolder)
	auth.DELETE("/api-folders/:id", handlers.DeleteAPIFolder)

	// API 请求历史
	auth.GET("/api-history", handlers.GetAPIHistory)
	auth.POST("/api-history", handlers.CreateAPIHistory)
	auth.DELETE("/api-history", handlers.ClearAPIHistory)

	// 代理控制
	auth.GET("/proxy/status", handlers.GetProxyStatus)
	auth.POST("/proxy/start", handlers.StartProxy)
	auth.POST("/proxy/stop", handlers.StopProxy)
	auth.POST("/proxy/pause", handlers.ToggleProxyPause)
	auth.POST("/proxy/send", handlers.SendProxyRequest)
	auth.POST("/proxy/replay", handlers.ReplayProxy)
	auth.GET("/proxy/logs", handlers.GetProxyLogs)
	auth.GET("/proxy/executions", handlers.GetProxyExecutions)
	auth.DELETE("/proxy/executions", handlers.ClearProxyExecutions)

	// Proto 管理
	auth.GET("/proto/services", handlers.GetProtoServices)
	auth.GET("/proto/describe", handlers.GetProtoDescribe)
	auth.GET("/proto/setdir", handlers.GetProtoDir)
	auth.POST("/proto/setdir", handlers.SetProtoDir)
	auth.POST("/proto/describe-method", handlers.DescribeProtoMethod)

	// 系统设置
	auth.GET("/settings", handlers.GetSettings)
	auth.PUT("/settings", handlers.UpdateSettings)
	auth.GET("/settings/:key", handlers.GetSetting)
	auth.PUT("/settings/:key", handlers.UpdateSetting)

	// 数据迁移
	auth.GET("/migration/status", handlers.GetMigrationStatus)
	auth.POST("/migration/import", handlers.ImportMigration)

	// 日志监听
	auth.GET("/logs", handlers.GetLogEntries)
	auth.GET("/files", handlers.GetLogFiles)
	auth.GET("/file", handlers.GetLogFileContent)

	// SDK 下载
	auth.GET("/config/sdk/list", handlers.GetSDKList)
	auth.GET("/config/sdk/download", handlers.DownloadSDK)

	// Jira 配置状态（脱敏，供前端判断是否启用同步按钮）
	auth.GET("/config/jira/status", handlers.GetJiraStatus)
	// SDK 上报接收（各引擎 SDK 主动上报，携带 Bearer token，服务端校验 report_token）
	api.POST("/qa/report", handlers.ReceiveReport)
	// SDK 上报查询（供「SDK 上报」查看页）
	auth.GET("/qa/reports", handlers.GetQaReports)

	// 表格视图
	auth.GET("/table-cases", handlers.GetTableCases)
	auth.POST("/table-cases", handlers.CreateTableCase)
	auth.PUT("/table-cases/:id", handlers.UpdateTableCase)
	auth.DELETE("/table-cases/:id", handlers.DeleteTableCase)

	auth.GET("/table-modules", handlers.GetTableModules)
	auth.POST("/table-modules", handlers.CreateTableModule)
	auth.PUT("/table-modules/:id", handlers.UpdateTableModule)
	auth.DELETE("/table-modules/:id", handlers.DeleteTableModule)

	// XMind 视图
	auth.GET("/xmind-cases", handlers.GetXmindCases)
	auth.POST("/xmind-cases", handlers.CreateXmindCase)
	auth.PUT("/xmind-cases", handlers.ReplaceXmindCases)
	auth.PUT("/xmind-cases/:id", handlers.UpdateXmindCase)
	auth.DELETE("/xmind-cases/:id", handlers.DeleteXmindCase)

	auth.GET("/xmind-modules", handlers.GetXmindModules)
	auth.POST("/xmind-modules", handlers.CreateXmindModule)
	auth.PUT("/xmind-modules/:id", handlers.UpdateXmindModule)
	auth.DELETE("/xmind-modules/:id", handlers.DeleteXmindModule)

	// 自由电子表格（纯文本网格）
	auth.GET("/spreadsheets", handlers.GetSpreadsheets)
	auth.POST("/spreadsheets", handlers.CreateSpreadsheet)
	auth.GET("/spreadsheets/:id", handlers.GetSpreadsheet)
	auth.PUT("/spreadsheets/:id", handlers.UpdateSpreadsheet)
	auth.DELETE("/spreadsheets/:id", handlers.DeleteSpreadsheet)

	// WebSocket 端点
	auth.GET("/ws", handlers.HandleWebSocket)                 // 执行日志（需 JWT）
	auth.GET("/proxy-ws", handlers.HandleProxyWebSocket)     // 协议录制（需 JWT）

	// 静态文件 + SPA fallback
	r.Static("/assets", "./static/assets")
	// 注意：./docs 仅通过已认证的 DownloadSDK 接口提供，不在此以静态目录公开暴露。
	r.StaticFile("/favicon.ico", "./static/favicon.ico")
	r.NoRoute(func(c *gin.Context) {
		c.File("./static/index.html")
	})
}
