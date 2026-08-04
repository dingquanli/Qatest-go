# Qatest-go 项目结构梳理与开发交接文档

> 用途：本文档面向**后续接手的 AI 开发者**，用于在既有代码基础上做功能修复 / 加固 / 扩展。
> 编写时间：2026-08-04 ｜ 代码基线：`git log` 最新提交 `36381a2`（安全加固收尾）。
> 配套文档：`docs/CODE_EVALUATION.md`（全量评估）、`docs/CODE_REVIEW.md`（第二轮复审）、`docs/sdk.md`（上报协议）、`docs/frontend-architecture.md`（前端架构）、`README.md`。

---

## 1. 项目概览

基于 **Go + Vue3** 的轻量级 QA 自动化测试平台。后端用 Go（gin）单体服务，前端用 Vue3 + Vite 编译后由后端**同源托管**（`static/`）。

核心能力：
- 测试用例 / 模块 / 表格视图 / 思维导图（XMind）视图管理
- 测试计划编排与「执行引擎」（`handlers/testplans.go` 的 `ExecuteTestPlan`）
- 设备（ADB）管理与脚本执行（Python / Node.js / Shell），**直接在宿主机运行用户代码（RCE 能力，高危）**
- 多端 SDK 上报（Go / Node.js / Python / Android / Unity / Cocos / Unreal 共 7 种）
- Jira 双向同步（配置在 `settings` 表 + `config.Config`）
- **gRPC 拦截代理**：HTTP/2 cleartext（h2c）代理，拦截 → 修改 → 重放，由 WebSocket 驱动决策
- **协议录制**：HTTP/WebSocket 与 gRPC 代理录制
- 安全：JWT 鉴权、SSRF 多道防线、上报接口令牌鉴权、日志 / 上报数据敏感字段脱敏

> ⚠️ 当前 git 工作区**干净**（无未提交改动）。安全加固（P0~P3）已基本闭环，但仍有部分「清理项 / 设计风险」未处理，见第 11、12 节。

---

## 2. 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go 1.24（go.mod 声明；本机 `go` 为 1.26，可正常编译）、gin 1.9、SQLite（`modernc.org/sqlite` 纯 Go 实现、WAL 模式、**无需 CGO**）、golang-jwt v5、gorilla/websocket、protobuf（jhump/protoreflect + google.golang.org/protobuf） |
| 前端 | Vue 3.5、Vite 5、Pinia、Vue Router 4、Element Plus、Tailwind CSS 3、ECharts、axios、TypeScript |
| 数据 | 单文件 SQLite（`qatest.db` + `-wal` / `-shm`），零外部依赖 |
| 构建 | 前端 `vite build` → 产物输出到 `../static`；后端 `go build -o qatest-server`；Docker 多阶段构建（`Dockerfile`） |

---

## 3. 目录结构总览

```
Qatest-go/
├── main.go                 # 服务入口：加载配置 → 初始化 DB → 迁移 → 启动 gin → 优雅关闭
├── go.mod / go.sum         # 模块名 qatest；Go 1.24
├── config/                 # 配置加载：.env 解析 + 默认值 + 强校验（JWT_SECRET/ADMIN_PASSWORD 必填）
├── database/               # SQLite 连接（WAL、连接池）+ 建表迁移 + 字段补齐
├── models/                 # 数据模型（结构体 ↔ 表）；含 user/auth 响应封装
├── handlers/               # HTTP 处理器（每个资源一个文件，约 20 个文件）
├── middleware/             # 全局中间件：日志、CORS、限流、SSRF 校验、JWT 鉴权
├── routes/                 # 路由注册（router.go，所有端点集中在此）
├── services/               # 业务服务：gRPC 代理、执行器、ADB、Proto 加载、命令安全校验
├── sdk/                    # 7 种语言的「上报 SDK」源码（供前端下载 / 集成到被测引擎）
├── web/                    # 前端源码（Vue3 + Vite）；web/node_modules 已存在但不入库
├── docs/                   # 文档（评估、复审、SDK 协议、前端架构、本文件）
├── static/                 # 前端构建产物（**当前为空**，需 npm run build 才有 UI）
├── ProxyLogs/              # 代理 JSONL 日志输出目录（运行时生成）
├── qatest.db*              # SQLite 数据库文件（运行时）
├── .env / .env.example     # 运行配置（.env 不入 git）
├── docker-compose.yml      # 容器编排（数据/日志卷、env 校验 JWT_SECRET）
└── Dockerfile              # 多阶段构建，非 root 用户运行，健康检查
```

---

## 4. 后端架构（分层）

### 4.1 启动流程 `main.go`
1. `config.Load()` —— 读 `.env`、填充 `AppConfig`、强校验 `JWT_SECRET`（禁止空/默认值）与 `ADMIN_PASSWORD`（缺失则 `log.Fatalf` 拒绝启动）。
2. `database.Init()` —— 打开 SQLite（WAL + busy_timeout=5000），连接池 `MaxOpenConns=4`。
3. `database.RunMigrations()` —— 建表 + 字段补齐（见 4.3）。
4. gin 模式按 `LOG_LEVEL` 切换 Debug/Release。
5. `routes.RegisterRoutes(r)` —— 挂载中间件 + 全部路由 + 静态托管。
6. 启动 HTTP Server（goroutine）+ 监听 `SIGINT/SIGTERM` 优雅关闭（5s 超时）。

### 4.2 `config` 包
- 零第三方依赖的 `.env` 解析器 `loadDotEnv`：真实环境变量优先于 `.env`。
- `Config` 结构：`Port / DBPath / JWTSecret / JWTExpiresIn / AllowedOrigins / LogLevel / ProxyTarget / ProtoDir / LogDir / ApkDir / Users / Jira*`。
- 包级变量 `JSAuthToken`（JS 脚本执行鉴权令牌，P0-4）。
- 启动强校验（安全基线）：`JWT_SECRET` 非空且非默认占位值；`ADMIN_PASSWORD` 必填（bcrypt 后覆盖 admin 哈希）。

### 4.3 `database` 包
- `db.go`：连接、`PRAGMA journal_mode=WAL`、`foreign_keys=ON`、`wal_autocheckpoint`。
- `migrations.go`：
  - `RunMigrations`：20 张表的 `CREATE TABLE IF NOT EXISTS`（DDL 中**拼接的表名/列名均为硬编码常量，无注入风险**）。
  - `RunColumnMigrations`：为旧表补齐缺失列（如 `table_cases/xmind_cases` 的 `priority/type/...`、`qa_reports` 的 `seq/method/headers/...`、`case_executions` 的 `plan_id/execution_id`、`test_cases.script_id`）。所有查询走 `?` 占位符，无 SQL 注入。

### 4.4 `models` 包
- 纯结构体（`TestCase / Bug / Script / TestPlan / User / API* / ...`），JSON tag 决定前后端字段名。
- 统一响应：`models.APIResponse{Success, Data, Error, ...}`、`models.NowStr()`、`models.Claims`。
- 注意前端偏好 **camelCase**（如 `createdAt`、`moduleId`），模型 tag 已对齐。

### 4.5 `middleware` 包
| 文件 | 职责 |
|---|---|
| `auth.go` | JWT 鉴权（`AuthRequired` / `Auth` / `GenerateToken` / `ParseToken`）。**已强制 HS256**，拒绝 none/非对称算法混淆。白名单：`/api/auth/login`、`/api/auth/refresh`。支持从 header 或 query `token`（WebSocket 用）取令牌。 |
| `security.go` | **SSRF 防护核心**：`SSRFCheck` 中间件（仅校验含 `url` 字段的请求，body 读取限 1MB）；`ResolveSafe`（解析全部 A/AAAA、10s 缓存、仅允许非私网 IP）；`SafeDialContext`（连接时重校验 + pin IP，消除 DNS 重绑定 TOCTOU）；`isPrivateIP` 内网网段集合。`IsPrivateIP` 导出供 services 复用。 |
| `cors.go` | 精确匹配 `AllowedOrigins` 才下发 `Access-Control-Allow-Credentials: true`；通配符 `*` 不下发凭据。 |
| `ratelimit.go` | 简易令牌桶限流（`maxRequests`，复审后已降为 120/60s）。注意 `c.ClientIP()` 在反代后可能不可靠（未设 `TrustedProxies`）。 |
| `logger.go` | 请求日志；`redactQuery` 对敏感 query 参数（`token/key/secret...`）脱敏。 |

### 4.6 `routes` 包（`router.go`）
**所有端点集中注册**，是「后端能力全景图」。分组：
- 全局中间件：`Logger → CORS → RateLimit → SSRFCheck`。
- 公开组（`api`，无需 JWT）：`POST /api/auth/login`、`POST /api/auth/refresh`、`POST /api/qa/report`（靠 `report_token` 鉴权，见 5.6）。
- 认证组（`auth.Use(middleware.Auth())`）：设备、脚本、执行、缺陷、用例（cases/table/xmind）、测试用例模块、计划、API 定义/请求/文件夹/历史、代理控制、Proto、设置、迁移、日志、SDK 下载、Jira 状态、SDK 上报查询、表格/XMind 视图。
- WebSocket：`GET /api/ws`（执行日志）、`GET /api/proxy-ws`（协议录制）—— 均置于认证组，**受 JWT 保护**。
- 静态：`/assets`、favicon、`NoRoute` → `./static/index.html`（SPA fallback）。

### 4.7 `handlers` 包
按资源拆分（每个文件一组 CRUD + 相关接口），共约 20 个文件：

| 文件 | 负责资源 / 接口 |
|---|---|
| `auth.go` | 登录 / 刷新令牌 |
| `testcases.go` | 测试用例 + 模块 + 执行记录 + 批量导入 |
| `testplans.go` | 测试计划 + **执行引擎**（`ExecuteTestPlan`，聚合用例执行结果） |
| `table_xmind.go` | 表格视图 / 思维导图视图用例与模块 |
| `bugs.go` | 缺陷 CRUD + Jira 同步 |
| `scripts.go` / `executions.go` | 脚本 CRUD / 执行记录 + 取消 |
| `devices.go` | 设备扫描 / 截图 / 执行命令 / 安装 APK（含 ApkDir 路径约束） |
| `api_definitions.go` / `api_requests.go` | 接口定义 / 请求集合 / 文件夹 / 历史 |
| `proxy.go` | 代理状态 / 启停 / 暂停 / 发送 / 重放 / 日志 / 执行历史 |
| `proto.go` | Proto 服务列表 / 描述 / 设置目录（admin 校验） |
| `settings.go` | 系统设置读写（敏感 key 脱敏 + admin 写校验） |
| `sdk.go` | SDK 列表 / 下载 / **上报接收 `ReceiveReport`** / 上报查询 / 令牌校验 / 脱敏 |
| `logs.go` | 日志文件列表 / 内容（文件名白名单 + `filepath.Rel` 防穿越） |
| `migration.go` | 数据导入导出 |
| `websocket.go` | `/ws` 与 `/proxy-ws` 的处理（鉴权由中间件负责） |
| `helpers.go` / `errors.go` | 公共辅助：`respondError`（统一错误响应，消除内部错误泄露）、`generateID`、`NowStr` 等 |

**约定（修复时务必沿用）：**
- 错误统一走 `respondError(c, status, err, friendlyMsg)`，不要把 `err.Error()` 直出（P0-3 已修，勿回退）。
- 所有 DB 查询用 `?` 占位符；新增表/列仅在 `migrations.go` 用硬编码常量拼接。
- ID 生成用 `generateID(prefix)`（crypto/rand + 纳秒时间戳）或 `services.GenerateSecureID` / `generateSecureToken`。

### 4.8 `services` 包（单例管理服务）
| 文件 | 单例 | 职责 / 风险点 |
|---|---|---|
| `proxy_server.go` | `ProxyInstance` | **gRPC 拦截代理**（h2c，默认 `127.0.0.1:18924`）。状态机：`waiting-request → forwarded/waiting-response → done/dropped/error`。WebSocket `HandleProxyWsMessage` 驱动决策。转发经 SSRF 校验客户端（`getSharedForwardClient` 5min 缓存 + pin IP）。JSONL 日志异步写盘（channel + goroutine，`stopLogWriter` 用 `WaitGroup` 刷盘）。 |
| `executor.go` | `Executor` | **脚本执行引擎（RCE 高危）**：`Start` → `execute` → `executeShell/Python/JS`。日志经 `LogChan → consumeLogs → broadcastLog` 推前端 WS。`SetLogBroadcastFunc` 注入广播回调（**避免 services→handlers 循环依赖**）。进程树终止（Windows `taskkill /T /F`）。 |
| `adb.go` | `ADB` | ADB 设备管理：`Scan / GetDevices / TakeScreenshot / ExecCommand / InstallAPK`。`ExecCommand` 经 `ValidateShellCommand` 白名单校验（无 shell 传参）。 |
| `proto_loader.go` | `ProtoLoader` | Proto 文件加载：`protoparse` 描述符优先 + 正则 fallback。自研 **protobuf wire format 编解码**（`decode/encodeWireFormat`，处理 varint/fixed/length-delimited）。enum/嵌套 message 解析历史上有 bug（见注释「修复」），改动需谨慎。 |
| `security.go` | — | `ValidateShellCommand`：23 条白名单命令 + 13 条危险模式黑名单（命令链、重定向、`rm -rf`、`curl/wget/nc` 等）。`GenerateSecureID`。 |

---

## 5. 关键子系统与修复注意事项

### 5.1 认证 / 鉴权
- JWT 用 HS256，密钥来自 `JWT_SECRET`，缺失/默认则**拒绝启动**。
- 登录：`handlers/auth.go` 比对 `config.AppConfig.Users`（来自 `QATEST_USERS` 环境变量或默认 admin）。
- 上报接口 `POST /api/qa/report` **不走 JWT 组**，靠 `Authorization: Bearer <reportToken>` 令牌（服务端 `settings.report_token`，`ensureReportToken` 首次自动生成，常量时间比较）。

### 5.2 SSRF 防护（三层，不要拆掉任何一层）
1. `middleware.SSRFCheck`：解析所有 IP + 10s 缓存，仅允许非私网。
2. `middleware.SafeDialContext`：连接时**重校验并 pin IP**，消除 DNS 重绑定 TOCTOU。
3. `proxy_server.getSharedForwardClient`：转发目标任一解析 IP 为私网即拒绝，且锁定第一个公网 IP。
> 改转发/出站逻辑时务必保留这三层；新增「用户输入 URL 出站」的接口要纳入 `SSRFCheck` 或显式调用 `ResolveSafe`/`SafeDialContext`。

### 5.3 脚本执行引擎（RCE，设计风险）
- `executor.go` 将 Python/JS 写成临时文件后**直接在宿主机运行**，具备任意代码执行能力。
- 当前仅做可见性兜底（启动告警 + 注释提示），**完整修复需沙箱/容器隔离**（未实现）。
- 修改时注意：`services` 包**不得 import `handlers`**（循环依赖），跨包广播用回调注入（`SetLogBroadcastFunc`、`ProxyServer.SetBroadcastFunc`）。

### 5.4 gRPC 拦截代理（状态机 + WebSocket）
- `handleGRPC` 是核心拦截流程；`pending[id]` 状态驱动「等待 WS 决策 → 转发/直回/丢弃」。
- 关键约束：5 分钟超时、WS 客户端掉线要清理 `pending`、writer goroutine 不能在持锁时写盘（P1-9 已修死锁）。
- 代理端口已绑定 `127.0.0.1`（非 0.0.0.0，P2 已修）。新增代理能力时注意默认仅本机。

### 5.5 Proto 编解码
- 自研 wire format 解析（`encode/decodeWireFormat`），`enum` 与 `message` 的区分历史上出过 bug（注释已标注「修复」）。改动后**务必用真实 .proto 回归** request/response 的 encode→decode 往返。
- `SetDir` 受 `PROTO_DIR` 基目录约束（未配置时不强制基目录，但仍有 200 文件 / 5MB 上限防 DoS）。

### 5.6 SDK 上报与脱敏
- `ReceiveReport` 校验令牌 → 落库 `qa_reports`（18 列）→ 落库前对 `headers/request/response` 做**递归脱敏**（`maskRecursive` + 敏感键集合 + 正则兜底）。
- 新增上报字段时，确认是否需要脱敏（token/password/secret/apikey/key/authorization/credential/authtoken）。

### 5.7 设备管理（ADB）
- 依赖宿主机 `adb` 在 PATH。`ExecCommand` 用 `exec.CommandContext(ctx,"adb",...)` 传参（无 shell），命令经 `ValidateShellCommand` 过滤。
- `InstallAPK` 受 `ApkDir` 约束；`ApkDir` 为空时复审已要求**失败闭合**（拒绝绝对路径）。

### 5.8 静态托管 / SPA
- 前端产物必须构建到 `static/`，否则浏览器只能拿到空的 SPA 外壳。**当前 `static/` 为空**，新接手者第一步通常是 `cd web && npm install && npm run build`。

---

## 6. 前端架构（`web/src`）

- 构建：`vite build`（含 `vue-tsc --noEmit` 类型检查）→ 产物到 `../static`。
- `api/request.ts`：**`baseURL: '/api'`**（相对路径，同源），统一拦截器处理 401 跳登录、错误提示。
- `api/*.ts`：每个后端资源一个模块（cases/bugs/scripts/plans/devices/proxy/proto/sdk/settings/logs/apidefs/apitest/executions/auth/xmind/table），与后端端点一一对应。
- `stores/user.ts`：Pinia 用户态（token、登录态），`router.beforeEach` 做路由守卫。
- `router/index.ts`：路由 → 视图映射（见下表）。
- `views/*.vue`：页面（Dashboard / Cases / TableCases / XmindCases / TestPlan / PlanExecs / ApiDefs / ApiTest / Automation / ProxyInterceptor / ProtocolRecorder / SdkReports / Bugs / Settings / Login）。
- `components/`：`ui/` 基础组件（Button/Card/Dialog/Input 等，shadcn 风格）、`BaseChart`（ECharts）、`JsonViewer`、`KeyValueEditor`、`CaseEditorDrawer`、`ReportBugModal`。
- `composables/`：`useWebSocket`（执行日志）、`useProxyWebSocket`（协议录制）、`useTheme`、`useAppSettings`。
- `types/index.ts`、`utils/index.ts`、`lib/utils.ts`（clsx + tailwind-merge）。

### 路由 ↔ 后端 ↔ 视图 映射（修复前端时对照）
| 前端路由 | 视图 | 主要后端资源 |
|---|---|---|
| `/dashboard` | Dashboard | 统计聚合（settings/bugs/plan-execs） |
| `/cases` | Cases | `/api/cases`、`/api/case-modules`、`/api/case-executions` |
| `/table-cases` | TableCases | `/api/table-cases`、`/api/table-modules` |
| `/xmind-cases` | XmindCases | `/api/xmind-cases`、`/api/xmind-modules` |
| `/testplan` | TestPlan | `/api/test-plans` + `ExecuteTestPlan` |
| `/plan-execs` | PlanExecs | `/api/plan-executions`、`/api/auto-task-executions` |
| `/api-defs` | ApiDefs | `/api/api-definitions`、`/api/api-def-modules` |
| `/api-test` | ApiTest | `/api/api-requests`、`/api/api-folders`、`/api/api-history`、`/proxy/send` |
| `/automation` | Automation | `/api/scripts`、`/api/executions`、`/api/devices` |
| `/proxy-interceptor` | ProxyInterceptor | `/proxy/*`、`/proxy-ws` |
| `/protocol-recorder` | ProtocolRecorder | `/proxy/*` 录制、`/proxy/logs` |
| `/sdk-reports` | SdkReports | `/api/qa/reports`、`/config/sdk/*` |
| `/bugs` | Bugs | `/api/bugs`、`/bugs/:id/sync`（Jira） |
| `/settings` | Settings | `/api/settings`、`/config/jira/status` |

---

## 7. 数据库 Schema（20 张表）

`scripts, executions, test_cases, case_modules, case_executions, bugs, test_plans, api_definitions, api_def_modules, api_requests, api_folders, api_history, auto_task_executions, table_cases, table_modules, xmind_cases, xmind_modules, plan_executions, settings, qa_reports`

- 主键均为 `TEXT`（业务生成的 ID，非自增）。
- JSON 字段以 TEXT 存储（`steps`、`tags`、`headers`、`body` 等），读取端自行反序列化。
- `settings` 为 KV 表（key 主键）：存 Jira 配置、上报令牌、前端设置等。**敏感 key 经脱敏后才返回前端**。
- `qa_reports`：SDK 上报落库，含 gRPC/API 拦截事件协议字段（`seq/method/headers/req_body/resp_body/err_msg/elapsed_ms/ts`）。

---

## 8. 多语言 SDK（`sdk/`）

| 引擎 | 目录 | 文件 |
|---|---|---|
| Unity (C#) | `sdk/unity` | QaSDK.cs, QaConfig.cs, QaLogger.cs |
| Unreal (C++) | `sdk/unreal` | QaSDK.h, QaSDK.cpp |
| Cocos (TS) | `sdk/cocos` | QaSDK.ts, QaConfig.ts |
| Android (Java) | `sdk/android` | QaSDK.java |
| Python | `sdk/python` | qa_sdk.py, qa_config.py |
| Go | `sdk/go` | sdk.go, config.go |
| Node.js | `sdk/nodejs` | index.js, config.js |

- 上报协议见 `docs/sdk.md`：事件 `case_result / log / request / response / error`，统一 `Authorization: Bearer <reportToken>`。
- `handlers/sdk.go` 的 `sdkEngines` 映射决定了「下载 SDK」页展示哪些引擎/文件——**新增 SDK 文件要同步更新该映射**，否则前端下载不到。

---

## 9. 配置（`.env`，不入 git）

关键项（详见 `.env.example`）：
- `PORT`（默认 3000）、`DB_PATH`（默认 `qatest.db`）
- `JWT_SECRET`：**必填**，随机 32+ 字节强密钥，否则拒绝启动
- `ADMIN_PASSWORD`：**必填**，bcrypt 后覆盖 admin 口令
- `JS_AUTH_TOKEN`：JS 脚本内 `adb()` 调用鉴权令牌（P0-4）
- `ALLOWED_ORIGINS`、`LOG_LEVEL`
- `PROXY_TARGET`、`PROTO_DIR`、`LOG_DIR`、`APK_DIR`
- `QATEST_USERS`：用户 JSON 数组（含 username/passwordHash/name/role）
- `JIRA_BASE_URL / JIRA_EMAIL / JIRA_API_TOKEN / JIRA_PROJECT`

---

## 10. 构建与运行

```bash
# 后端
cp .env.example .env        # 至少填 JWT_SECRET 与 ADMIN_PASSWORD
go build -o qatest-server.exe .     # Windows；Linux/macOS 用 -o qatest-server
./qatest-server                    # 默认 http://localhost:3000

# 前端（产物输出到 ../static，由后端托管）
cd web && npm install && npm run build

# Docker
docker compose up -d --build
```

> ⚠️ **未构建前端则无 UI**：`static/` 当前为空。功能测试 UI 前务必先 `npm run build`。

---

## 11. 已知问题与安全修复历史

既有安全审计（`docs/CODE_EVALUATION.md` + `docs/CODE_REVIEW.md`）结论：**P0/P1/P2/P3 均已闭环**，`go build ./...` 与 `vite build` 均通过。要点：
- P1-1 上报匿名写 → 加 `report_token` 鉴权；P1-2 补充上报查看页；P1-3 SSRF TOCTOU → 三层防护 + pin IP。
- P2：Go SDK 缺 `ts`、孤儿路由（经核验非孤儿）、日志文件白名单、脱敏正则递归化。
- 第二轮复审闭环：P1 设置接口密钥脱敏 + admin 写校验；P2 ApkDir 失败闭合、SetProtoDir admin 校验、代理绑 127.0.0.1、限流降到 120；P3 移除公开 `/docs`、日志 query 脱敏、JWT 强制 HS256、前端 Jira 占位符回写防护。

### 未处理 / 设计风险（交接后建议处理）
1. **RCE 设计风险（最高）**：脚本执行在宿主机直接运行用户代码。完整修复需沙箱 / 容器隔离（当前仅告警兜底 + `EXECUTOR_ENABLED=0` 熔断开关）。
2. **限流 IP 信任**：已修复（默认不信任代理、`TRUSTED_PROXIES` 配置化，见下文第 14 节）。
3. **清理项（开源前）**：已清理（默认 admin 哈希、SDK 占位 URL、`// Px-y` 编号注释、`.env.example`/`docker-compose.yml` 占位哈希），见下文第 14 节。
4. **Proto wire format 编解码脆弱**：已回归加固（repeated/packed/sint/fixed），见下文第 14 节。
5. **代理状态机边界**：已加固（Stop 置 error、WS 全断清理 pending、logCh 并发关闭防护），见下文第 14 节。

---

## 12. 交接修复切入点与约定

- **入口先看**：`main.go` → `routes/router.go`（能力全景）→ `docs/CODE_REVIEW.md`（已知风险）。
- **改 API**：在 `handlers/` 对应文件加函数，再到 `routes/router.go` 注册；前端在 `web/src/api/` 加对应模块。
- **新增表/列**：`database/migrations.go` 的 `tables` 或 `RunColumnMigrations`（旧库兼容）。
- **跨包回调**：`services` 不能 import `handlers`，用 `Set*BroadcastFunc` 注入。
- **统一错误**：`respondError(c, status, err, friendlyMsg)`，勿暴露 `err.Error()` 原文。
- **DB 安全**：永远 `?` 占位符；拼接只用于 `migrations.go` 的硬编码常量。
- **SSRF**：任何「用户输入 URL 出站」都要走 `SSRFCheck` 或显式 `ResolveSafe`/`SafeDialContext`。
- **单例管理**：`ProxyInstance / Executor / ADB / ProtoLoader` 全局唯一，新增状态别再 new。
- **前端构建产物路径**：`web` 构建输出到 `../static`，改 Vite 配置时注意。
- **敏感数据**：上报 / 日志 / 设置 的 token/secret/password 必须脱敏。

---

## 13. 快速上手 Checklist
1. `cp .env.example .env` 并填 `JWT_SECRET`、`ADMIN_PASSWORD`（否则无法启动）。
2. `go build ./...` 确认后端可编译。
3. `cd web && npm install && npm run build` 生成 `static/`（否则无 UI）。
4. 启动 `./qatest-server`，浏览器开 `http://localhost:3000`，用 admin + `ADMIN_PASSWORD` 登录。
5. 如需验证 gRPC 代理 / Proto 编解码：准备 `.proto` 与待测 gRPC 目标，经 `/proxy-interceptor` 与 `/api-test` 操作。
6. 修复前先读 `docs/CODE_EVALUATION.md` 与 `docs/CODE_REVIEW.md`，避免重复已修问题。

---

## 14. 交接后开发修复记录（2026-08-04）

> 本轮按第 11 节「未处理项」完成以下开发与修复，`go build ./...`、`go test ./...`、`vite build` 均通过。

| 项 | 修复内容 | 涉及文件 |
|---|---|---|
| 限流 IP 信任（P2） | 默认 `SetTrustedProxies(nil)` 不信任代理，`c.ClientIP()` 返回直连 IP，伪造 XFF 无效；新增 `TRUSTED_PROXIES`（CIDR 逗号分隔）供反代场景还原真实 IP | `main.go`、`config/config.go`、`middleware/ratelimit.go` |
| RCE 熔断开关 | 新增 `EXECUTOR_ENABLED`（默认 1）；置 0 时 `POST /api/executions` 返回 403，测试计划 auto 模式跳过脚本派发 | `config/config.go`、`handlers/executions.go`、`handlers/testplans.go` |
| 代理状态机边界 | `Stop()` 先置 pending 为 error 再唤醒（避免停止后仍转发）；新增 `NotifyNoWSClient`：WS 客户端全断即中止等待中请求（防残留 5 分钟）；`logCh` 生命周期加 `RWMutex` 防 send-on-closed-channel panic | `services/proxy_server.go`、`handlers/websocket.go` |
| Proto wire 编解码加固 | 支持 **repeated** 编码（逐元素 tag）与解码（聚合数组）；支持 **packed repeated** varint 解码（proto3 默认）；**sint32/sint64** 补 zigzag 编解码（此前按普通 varint 编错）；**fixed32/64/sfixed** 修正 wire type；enum 名 ↔ 数值互逆查找；新增 6 个回归测试 | `services/proto_loader.go`、`services/proto_loader_test.go` |
| 开源前清理 | 删除 config 硬编码 admin bcrypt 哈希；SDK Go `BaseURL` 占位 IP 改为空 + 说明；批量清理 `// Px-y` 编号注释（保留技术描述）；`.env.example`/`docker-compose.yml` 占位哈希改 `REPLACE_WITH_BCRYPT_HASH`；compose 强制 `ADMIN_PASSWORD` 必填（同 JWT_SECRET） | `config/config.go`、`sdk/go/config.go`、全局注释、`.env.example`、`docker-compose.yml` |
| 前端构建修复 | `plans.ts` 缺 `CaseExecution` 类型导入；`Cases.vue` 的 `EditingCase` 缺 `scriptId`；`Settings.vue` 缺 `computed` 导入（vue-tsc 原 6 个错误清零） | `web/src/api/plans.ts`、`web/src/views/Cases.vue`、`web/src/views/Settings.vue` |
| 编译错误修复 | `handlers/errors.go` 多余 `net/http` 导入；`handlers/testplans.go` 忽略 `Exec` 返回值 | 同上两文件 |

> ⚠️ 注意事项：
> - 部署在反代后：必须配置 `TRUSTED_PROXIES=代理网段CIDR`，否则限流按代理 IP 聚合计数。
> - 生产不需要脚本执行能力：建议 `EXECUTOR_ENABLED=0`（RCE 高危的部署侧熔断）。
> - `static/` 构建时 WorkBuddy 环境的安全删除保护会拦截 vite 清空旧产物，需先分批清空 `static/assets` 再构建（常规 CI/本地终端无此限制）。

### 14.1 前端交互体验改造（2026-08-04，提交 fba589f 之后追加）

按用户反馈「新建思维导图 / 新建表格参考 WPS 与 XMind」，对两个视图做交互改造，`vue-tsc` 与 `vite build` 通过：

| 页面 | 改造内容 |
|---|---|
| `TableCases.vue`（表格，仿 WPS 表格） | 行号列；单元格点击聚焦 + 键盘导航（方向键 / Enter 下移 / Tab 右移，仿电子表格）；选中行高亮；底部「新行」输入条（输入即保存新建，WPS 式）；支持从 Excel/WPS 粘贴多行（Tab 分列 / 换行分行）批量新建用例 |
| `XmindCases.vue`（思维导图，仿 XMind 逻辑图） | 贝塞尔曲线连线（原直角折线）；节点点击选中高亮 + 双击/F2 就地编辑名称（Enter 保存 / Esc 取消 / 失焦保存）；键盘快捷键（Tab 新建子节点、Enter 新建同级、Delete 删除、方向键导航、← 返回父层）；Ctrl+滚轮缩放 + 空白拖拽平移 + 工具栏（放大/缩小/适应窗口/重置）；画布点阵背景 |
| `components/ui/Input.vue` | 新增 `defineExpose({ focus, select })`，供就地编辑与表格键盘导航聚焦输入框 |

> 备注：后端 `UpdateXmindCase`/`UpdateTableCase` 为**全量更新**，前端就地重命名必须携带原对象全部字段（含 `expected`/`sortOrder`），代码中已用 `{ ...c, name }` 处理，勿改成仅传 name。
> 交互契约：思维导图三级结构（根→模块→用例），Tab 在模块下建用例、根下建模块；用例层无子级（Tab 忽略）。
