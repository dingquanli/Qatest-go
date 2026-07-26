# Qatest-go

基于 Go + Vue3 的轻量级 QA 自动化测试平台，支持用例管理、测试计划、多端 SDK 上报、Jira 同步以及协议 / gRPC 代理录制。前端 UI 由后端同源托管，无需单独部署。

## 功能特性

- 测试用例管理：用例增删改查、标签、分类
- 测试计划：计划编排与「登记结果」
- SDK 多端上报：内置 Go / Node.js / Python / Java / Unity / Cocos / Unreal 共 7 种语言 SDK
- 上报数据查看：事件 / 结果列表、详情抽屉、敏感字段脱敏
- Jira 同步：用例 / 缺陷与 Jira 双向同步
- 协议录制：HTTP/WebSocket 与 gRPC 代理录制
- 安全：上报接口令牌鉴权、出网 SSRF 防护、日志敏感字段脱敏

## 技术栈

- 后端：Go 1.24、gin、SQLite（modernc.org/sqlite，WAL 模式）、JWT、gorilla/websocket、protobuf
- 前端：Vue 3、Vite 5、Pinia、Vue Router、Element Plus、Tailwind CSS、ECharts
- 数据：SQLite 单文件数据库，零外部依赖

## 目录结构

```
Qatest-go/
├── main.go              # 服务入口
├── config/              # 配置加载（环境变量 + .env）
├── database/            # SQLite 初始化与数据迁移
├── handlers/            # HTTP 处理器
├── middleware/          # 鉴权、SSRF 防护等
├── models/              # 数据模型
├── routes/              # 路由注册
├── services/            # 代理服务、上报处理等
├── sdk/                 # 多语言上报 SDK 源码
├── web/                 # 前端源码（Vue3 + Vite）
├── docs/                # 项目文档
└── .env.example         # 环境配置模板
```

## 快速开始

### 1. 后端

```bash
# 复制并填写环境配置
cp .env.example .env
# 至少要设置 JWT_SECRET 与 ADMIN_PASSWORD（见下方「配置」）

# 构建（Windows 目标名匹配 start.bat / Dockerfile）
go build -o qatest-server.exe .
# Linux / macOS：
# go build -o qatest-server .

# 启动
./qatest-server
```

服务默认监听 `http://localhost:3000`。

### 2. 前端

前端构建产物输出到 `../static`，由后端同源托管。

```bash
cd web
npm install
npm run build      # 构建到 ../static
# 本地开发预览可用：npm run dev
```

> 注意：修改前端后必须重新 `npm run build`，后端才会提供更新后的 UI。

### 3. Docker

```bash
docker compose up -d --build
```

## 配置

编辑 `.env`（不要提交，已被 `.gitignore` 排除）。关键项：

| 变量 | 说明 |
|---|---|
| `PORT` | 服务端口，默认 `3000` |
| `DB_PATH` | SQLite 数据库路径，默认 `qatest.db` |
| `JWT_SECRET` | JWT 签名密钥，**必填**，留空或用默认值服务拒绝启动 |
| `ADMIN_PASSWORD` | 管理员 `admin` 登录口令（明文，启动时自动 bcrypt 哈希），**必填** |
| `JS_AUTH_TOKEN` | JS 脚本中 `adb()` 调用鉴权令牌，留空则相关调用返回 401 |
| `ALLOWED_ORIGINS` | CORS 允许源，多个用逗号分隔 |
| `LOG_LEVEL` | 日志级别：DEBUG / INFO / WARN / ERROR |

生成随机密钥示例：

```bash
openssl rand -hex 32
```

## 默认账号

管理员账号为 `admin`，口令由 `.env` 的 `ADMIN_PASSWORD` 决定。首次部署请使用强口令。

## SDK 上报

各语言 SDK 位于 `sdk/` 目录，上报协议与示例见 `docs/sdk.md`。上报需在请求头携带 `Authorization: Bearer <reportToken>`，令牌可在前端「下载 SDK / 协议录制」页查看。

## 许可证

本项目目前**未包含 LICENSE 文件，保留所有权利**。如需以开源协议发布，请在仓库中添加合适的许可证（如 MIT）。
