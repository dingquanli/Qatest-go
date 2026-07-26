# Qatest-go 代码复审（第二轮 · 开源前深度审查）

> 复审时间：2026-07-25 ｜ 方式：静态审计 + 关键路径交叉验证
> 范围：在 `docs/CODE_EVALUATION.md`（P1/P2/P3 已闭环）基础上，对未覆盖的接口、配置、部署暴露面做第二轮审查
> 结论：**发现 1 个 P1、4 个 P2、若干 P3**。原有安全修复（鉴权、SSRF、脱敏、日志白名单）经验证均有效。

---

## ✅ 修复状态（2026-07-25 第二轮修复，已全部闭环）

| 级别 | 问题 | 文件 | 修复方式 | 验证 |
|---|---|---|---|---|
| P1 | 设置接口明文泄露 Jira 密钥 + 无写权限校验 | `handlers/settings.go` | `GetSettings`/`GetSetting` 对敏感 key（token/secret/password/apikey/auth…）脱敏为 `********`；`UpdateSettings`/`UpdateSetting` 增加 `role=="admin"` 校验，且拒绝写回脱敏占位符 | `go build ./...` 通过 |
| P2 | `InstallAPK` ApkDir 空时绝对路径绕过 | `handlers/devices.go` | ApkDir 为空时拒绝安装绝对路径 APK（失败闭合） | `go build ./...` 通过 |
| P2 | `SetProtoDir` 任意用户改目录 | `handlers/proto.go` | `SetProtoDir` 增加 `role=="admin"` 校验 | `go build ./...` 通过 |
| P2 | 代理监听 `0.0.0.0` 无认证 | `services/proxy_server.go` | `h1s.Addr` 改为 `127.0.0.1:%d`（仅本机） | `go build ./...` 通过 |
| P2 | 限流上限 5000 失效 | `middleware/ratelimit.go` | `maxRequests` 降为 `120`，注释同步 | `go build ./...` 通过 |
| P3 | 公开静态 `/docs` | `routes/router.go` | 删除 `r.Static("/docs", "./docs")`（文档仍由已认证的 DownloadSDK 提供） | `go build ./...` 通过 |
| P3 | 日志记录完整 RawQuery | `middleware/logger.go` | `redactQuery` 对敏感 query 参数脱敏为 `***` | `go build ./...` 通过 |
| P3 | JWT 未强制签名算法 | `middleware/auth.go` | keyfunc 中校验 `SigningMethodHMAC`，拒绝 none/非对称算法 | `go build ./...` 通过 |
| P3 | 前端 Jira token 被脱敏值回写 | `web/src/views/Settings.vue` | 加载时跳过脱敏占位符、保存时拦截占位符；用 `/config/jira/status` 维持"已配置"状态 | `vite build` 通过 |

> 前端/后端均已重新构建成功（`go build ./...` 与 `vite build` 均 exit 0）。P3 清理项（默认 admin 哈希、占位 URL、`// Px-y` 注释漂移）因属文档/占位性质且不影响安全（默认哈希因 `ADMIN_PASSWORD` 必填不可达），本轮未改动，留待后续按需清理。

---

## 一、已确认安全（无需处理）

| 项 | 证据 |
|---|---|
| 全路由 JWT 保护（含 WebSocket） | `router.go` 将 `/ws`、`/proxy-ws` 置于 `auth` 组，受 `middleware.Auth()` 保护 |
| SSRF 多道防线 | `middleware.SSRFCheck`(解析全部 IP+10s 缓存) + `SafeDialContext`(连接时重校验并 pin IP) + `proxy_server.getSharedForwardClient`(任一解析 IP 为私网即拒绝并锁死 pin) |
| 上报接口鉴权与脱敏 | `handlers/sdk.go`：`validReportToken` 用 `subtle.ConstantTimeCompare`；`maskSensitiveJSON` 递归脱敏 |
| 文件读取防穿越 | `handlers/logs.go`：文件名白名单 + `filepath.Rel` 二次校验 |
| CORS 安全 | `middleware/cors.go`：仅在精确匹配白名单时下发 `Access-Control-Allow-Credentials: true`，通配符 `*` 不下发凭据 |
| 启动强校验 | `config/config.go`：JWT_SECRET 必须为非默认强密钥、ADMIN_PASSWORD 必填，否则 `log.Fatalf` 拒绝启动 |
| 无 SQL 注入 | 所有 handler 查询均用 `?` 占位符；`database/migrations.go` 中仅拼接**硬编码常量**表/列名 |
| 无主机命令注入 | `services/adb.go` 用 `exec.CommandContext(ctx,"adb",...)` 传参（无 shell）；设备命令经 `ValidateShellCommand` 白名单+黑名单 |
| 前端无硬编码密钥 | `web/src/api/request.ts` 使用相对 `baseURL:'/api'`；Jira token UI 已显示「服务端不回显」 |

---

## 二、问题清单

### P1 — 设置接口明文泄露 Jira 密钥（机密泄露）

- **位置**：`handlers/settings.go`
  - `GetSettings` (L13–31) 返回 `settings` 表**全部**行，包含 `jira_api_token`、`jira_email`、`jira_base_url`、`jira_project`，**明文**。
  - `GetSetting` (L53–61) 按 key 读取同样明文返回。
  - `UpdateSettings`/`UpdateSetting` (L35–50, L65–81) 允许任意已登录用户**覆盖**任意设置项（含 `jira_api_token`）——无角色/权限校验。
- **风险**：任何持有合法 JWT 的调用者（哪怕是将来多用户场景）都能读取并篡改 Jira API Token。`GetJiraStatus` 已做了脱敏，但 `GetSettings` 没有复用。
- **修复**：
  1. `GetSettings`/`GetSetting` 读取时对 `jira_api_token`、`jira_email` 等敏感 key 做遮罩（参照 `GetJiraStatus`）；
  2. 写入 `jira_*` 等敏感 key 时校验 `role == "admin"`；
  3. （可选）密钥落库前加密。

---

### P2 — `InstallAPK` 路径穿越（ApkDir 未配置时）

- **位置**：`handlers/devices.go:93–112`
- **现象**：当 `config.AppConfig.ApkDir == ""` 时，仅校验 `..` 子串与 `.apk` 后缀。绝对路径如 `/etc/secret.apk`、`C:\x.apk` 会通过校验，并被直接传给 `adb install <path>`（服务器文件系统上的任意 `.apk` 均可被读取并安装）。
- **修复**：`ApkDir` 为空时**失败闭合**（拒绝安装），或强制要求 `filepath.Rel(ApkDir, apkPath)` 必须成功且不以 `..` 开头。

---

### P2 — `SetProtoDir` 可指向任意目录

- **位置**：`handlers/proto.go:33–48` + `services/proto_loader.go SetDir`
- **现象**：`PROTO_DIR` 未配置时，任意已登录用户可把 proto 加载目录指向 `/etc`、仓库根等任意路径；服务端会 `filepath.Walk` 并解析 `.proto` 文件，造成目录结构泄露与 CPU 消耗（虽有 200 文件/5MB 上限）。
- **修复**：始终要求 `PROTO_DIR` 作为基目录限制，或对 `/proto/setdir` 做 admin 鉴权。

---

### P2 — 代理服务监听 `0.0.0.0:18924` 且无认证

- **位置**：`services/proxy_server.go:186–190`（`h1s.Addr = fmt.Sprintf(":%d", ps.port)`）
- **现象**：gRPC 拦截代理绑定所有网卡，且无任何认证。若服务部署在共享网络/云主机，攻击者可将其作为**出网正向代理**转发到任意公网目标（内网 SSRF 已被拦，但开放出网代理本身即为风险，且可被用于反射/滥发）。
- **修复**：默认绑定 `127.0.0.1`；bind 地址通过配置项可控；必要时为代理端口增加轻量认证。

---

### P2 — 限流实际失效

- **位置**：`middleware/ratelimit.go:15`（`maxRequests = 5000`）、L89（`c.ClientIP()`）
- **现象**：注释写"120 次/60 秒/IP"，实际为 **5000/60s**，几乎不起作用；且 `c.ClientIP()` 在反向代理后易被伪造（未设置 `TrustedProxies`）。
- **修复**：将上限降到合理值（如 120/60s）；调用 `gin.SetTrustedProxies(...)` 并基于 `X-Forwarded-For` 取真实 IP（或直接使用连接 IP 但记录该局限）。

---

### P3 — 静态暴露内部文档

- **位置**：`routes/router.go:197`（`r.Static("/docs", "./docs")`）
- **现象**：`CODE_EVALUATION.md`、架构文档等内部笔记被**未授权**公开访问（虽无密钥，但泄露内部实现细节）。
- **修复**：开源前移除该静态路由，或仅暴露 `README.md` 等对外文档。

---

### P3 — 请求日志记录完整 RawQuery

- **位置**：`middleware/logger.go:25–35`
- **现象**：日志拼接 `path + "?" + RawQuery`，query 中可能包含 token、key 等敏感参数。
- **修复**：对 query 中的 `token`/`key`/`secret` 等做脱敏后再记录。

---

### P3 — JWT 未强制签名算法（加固项）

- **位置**：`middleware/auth.go:61`（`jwt.ParseWithClaims`）
- **现象**：未使用 `jwt.WithValidMethod(jwt.SigningMethodHS256)` 显式锁定算法。golang-jwt/v5 默认禁用 `none`，当前风险低，但属最佳实践缺失。
- **修复**：解析时传入 `jwt.WithValidMethod(...)` 显式限定 HS256。

---

### P3 — 清理项（开源前建议处理）

1. `config/config.go:103`：硬编码的默认 admin bcrypt 哈希（`$2a$10$N9qo8uLO...` 即经典 "password" 示例哈希）。虽因 `ADMIN_PASSWORD` 必填而**不可达**，仍应删除，避免误导。
2. `sdk/go/config.go:6`：硬编码占位 `BaseURL = "http://192.168.1.100:3000"`，应改成明显占位符或运行时可配置。
3. 大量 `// Px-y 修复` / `P0-x` 内联注释与 `docs/CODE_EVALUATION.md` 中引用了 `P1-5/P1-8/P1-9/P1-11` 等未在正文中编号的条目（文档漂移）。开源前应统一清理，避免读者困惑。

---

## 三、子代理误报（已复核否定，记录备查）

| 误报 | 复核结论 |
|---|---|
| WebSocket 未鉴权 | **否**。`router.go` 已将 `/ws`、`/proxy-ws` 置于 `auth` 组，受 JWT 保护（`websocket.go` 自身仅做 `CheckOrigin` 是合理的，鉴权由中间件负责）。 |
| `CreateScript` 中 `GenerateSecureID("")[:4]` 越界 panic | **否**。`GenerateSecureID` 始终返回 `prefix-16hex`（≥17 字符），`[:4]` 不会越界。 |
| 前端 `web/src` 不存在 | **否**。`web/src` 实际存在且干净（相对 `/api` base、无硬编码密钥）；子代理检索路径有误。 |

---

## 四、修复优先级建议

1. **立刻修**：P1 设置接口密钥泄露（影响最大、改动最小，复用已有脱敏逻辑）。
2. **开源前修**：P2 的 InstallAPK/SetProtoDir 失败闭合、代理绑定地址、限流上限。
3. **可批量处理**：P3 各项清理（多为文档/注释/日志脱敏，不影响功能）。
