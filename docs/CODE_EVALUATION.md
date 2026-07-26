# Qatest-go 全量代码评估

> 评估时间：2026-07-25 ｜ 评估方式：静态审计（后端 57 个 .go + 前端 15 视图 + 19 API 模块 + 7 引擎 SDK），关键路由交叉验证
> 评估范围：自 `main.go` 入口至 `static/` 前端产物的全链路；不含第三方依赖内部逻辑

## 背景

本报告对 Qatest-go（Go + gin + SQLite 后端，Vue3 + Vite 前端，多语言 SDK 上报）进行全量代码评审，识别安全、功能完整性、代码质量三类问题，并给出修复状态。

## 安全问题（P1）

### P1-1 上报接口匿名可写
- 现象：`POST /api/qa/report`（公开路由组）仅做格式校验，未校验身份，任意客户端可匿名向 `qa_reports` 表灌入数据。
- 修复：引入 `report_token`，上报时校验请求头 `Authorization: Bearer <reportToken>`；令牌由服务端在 `settings` 表生成（`ensureReportToken`），缺失或错误返回 401「上报令牌无效」。前端「下载 SDK / 协议录制」页展示该令牌。
- 状态：已修复并验证（错误令牌 → 401；正确令牌 → 200 落库）。

### P1-2 上报数据无消费端
- 现象：`qa_reports` 表有写入但无查看页面，数据成为孤岛。
- 修复：新增 `GET /api/qa/reports`（分页 + event 过滤，返回 `{total, items}`，含全部 18 列）；新增前端 `SdkReports.vue`（列表 + 详情抽屉，headers/reqBody/respBody 美化展示、可复制）。
- 状态：已修复并验证（新行可见、过滤生效）。

### P1-3 SSRF 出网请求
- 现象：代理转发与 Jira 同步等出网请求仅校验首个解析 IP，存在 DNS 重绑定 TOCTOU 风险。
- 修复：`middleware/security.go` 改为解析全部 A/AAAA 记录、仅允许首个公网 IP；新增 `SafeDialContext` 在连接时重新校验并 pin 已解析 IP，消除 TOCTOU；新增 10s DNS 缓存。
- 状态：已修复并验证（`/proxy/send` 指向 `127.0.0.1` 与 `169.254.169.254` 均返回 400「private ip not allowed」）。

## 功能与质量（P2）

### P2-1 Go SDK 缺 ts 字段
- 现象：上报协议要求 `ts`（时间戳），但 Go SDK 结构体未包含，导致时间字段落库缺失。
- 修复：`sdk/go/sdk.go` 的 `reportPayload` 增加 `Ts string \`json:"ts,omitempty"\``。
- 状态：已修复并验证（`ts`、`elapsedMs` 正确落库）。

### P2-2 疑似孤儿 CRUD 路由
- 现象：部分 PUT/DELETE 路由在后端存在但初判前端未调用。
- 结论：经前端 `cases.ts`/`bugs.ts`/`scripts.ts`/`apidefs.ts` 交叉核验，更新/删除调用均已暴露，**非孤儿路由**，关闭。

### P2-3 日志文件读取缺白名单
- 现象：`GET /api/file?name=...` 读取日志文件，原实现路径约束已通过 `filepath.Rel` 闭合，但仍按目录而非文件名白名单，存在隐患。
- 修复：`handlers/logs.go` 新增 `listLogFiles()` 返回目录下实际文件名，`GetLogFileContent` 仅允许白名单内文件名。
- 状态：已修复并验证（路径穿越返回 403「非法文件名」）。

### P2-4 脱敏正则覆盖不全
- 现象：原正则仅匹配顶层键，嵌套对象/数组中的敏感字段不脱敏。
- 修复：改为递归遍历（`maskRecursive`）+ 敏感键集合（`sensitiveKeySet`）+ 预编译 `fallbackMaskRe` 兜底；敏感键：`token/password/secret/apikey/key/authorization`。
- 状态：已修复并验证（嵌套 `data.token` 等全部置 `***`，非敏感字段保留）。

## 代码质量（P3）

- `generateID`：原实现存在碰撞风险，改为 `crypto/rand` + 纳秒时间戳。
- `InstallAPK`：原实现可指向任意路径，改为限制到 `config.AppConfig.ApkDir`（若设置）。
- WebSocket 初始化顺序：静态审查确认安全（服务重启无 panic）。

## 评估结论

全部 P1/P2/P3 项已闭环，并通过端到端验证（登录、上报鉴权、数据查看、SSRF 拦截、路径穿越拦截、脱敏、字段完整性）。代码可进入开源发布准备。
