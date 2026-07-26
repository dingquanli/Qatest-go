# Qatest SDK 文档

> **用途**：把 Qatest 的 7 种语言 SDK 接入你的项目，将「用例结果、运行日志、API 抓包事件、错误」上报到 Qatest 平台统一查看。
> **适用范围**：Unity、Unreal Engine、Cocos Creator、Android、Python、Go、Node.js
> **下载入口**：平台「下载 SDK / 协议录制」页（或「代理录制」页）右上角「下载 SDK」→ 选择引擎 → 下载全部文件。
> **约定**：文档中 `<...>` 或 `「待填」` 需替换为你真实的值；所有示例的上报地址结尾都**不要带斜杠**。

## 1. 上报协议

所有 SDK 遵循同一 HTTP 协议：

- **端点**：`POST {BaseUrl}/api/qa/report`
- **鉴权**：请求头 `Authorization: Bearer <reportToken>`
  - `reportToken` 由平台生成，在「下载 SDK」页查看；缺失或错误返回 `401 上报令牌无效`。
- **Content-Type**：`application/json`
- **响应**：成功 `{ "success": true, "data": { "id": "..." } }`；失败 `{ "success": false, "error": "..." }`

### 上报字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `event` | string | 事件类型，见下文 |
| `name` | string | 事件名称（如用例名、接口名） |
| `result` | string | 结果：`passed` / `failed` / `skipped`（case_result 用） |
| `message` | string | 描述 / 错误信息 |
| `method` | string | HTTP 方法（request/response 用） |
| `url` | string | 请求地址 |
| `headers` | object | 请求/响应头（敏感键自动脱敏） |
| `request` | object | 请求体 |
| `response` | object | 响应体 |
| `tags` | string[] | 标签 |
| `seq` | number | 同一会话内的事件序号 |
| `ts` | string | ISO8601 时间戳 |
| `elapsedMs` | number | 耗时（毫秒） |

### 事件类型

- `case_result`：用例执行结果（`result` 必填）
- `log`：运行日志片段
- `request`：抓取的请求
- `response`：抓取/回放的响应
- `error`：异常事件

### 敏感字段脱敏

以下键（含嵌套对象/数组）的值在服务端落库前会被统一替换为 `***`：
`token` / `password` / `secret` / `apikey` / `key` / `authorization`。

## 2. 各语言接入

### Go

```go
import "your_project/sdk/go"

sdk := qatest.New(qatest.Config{
    BaseURL: "http://localhost:3000",
    Token:   "<reportToken>",
})
sdk.ReportCaseResult("登录用例", "passed", "ok")
sdk.ReportRequest("Login-API", "POST", "/api/login", headers, body, resp, 12.5)
```

### Node.js

```js
const { Qatest } = require('@qatest/sdk-node');

const sdk = new Qatest({ baseUrl: 'http://localhost:3000', token: '<reportToken>' });
sdk.reportCaseResult('登录用例', 'passed', 'ok');
```

### Python

```python
from qatest_sdk import Qatest

sdk = Qatest(base_url="http://localhost:3000", token="<reportToken>")
sdk.report_case_result("登录用例", "passed", "ok")
```

### Java

```java
import com.qatest.sdk.Qatest;

Qatest sdk = new Qatest("http://localhost:3000", "<reportToken>");
sdk.reportCaseResult("登录用例", "passed", "ok");
```

### Unity (C#)

```csharp
using Qatest.SDK;

var sdk = new QatestSdk("http://localhost:3000", "<reportToken>");
sdk.ReportCaseResult("登录用例", "passed", "ok");
```

### Cocos Creator (TypeScript)

```ts
import { Qatest } from 'qatest-sdk-cocos';

const sdk = new Qatest('http://localhost:3000', '<reportToken>');
sdk.reportCaseResult('登录用例', 'passed', 'ok');
```

### Unreal Engine (C++)

```cpp
#include "QatestSDK.h"

UQatestSDK* Sdk = UQatestSDK::Create("http://localhost:3000", TEXT("<reportToken>"));
Sdk->ReportCaseResult(TEXT("登录用例"), TEXT("passed"), TEXT("ok"));
```

## 3. 平台查看

上报后，在平台左侧「SDK 上报」页可：
- 按事件类型过滤列表
- 点击查看详情抽屉（headers / request / response 美化展示、一键复制）
- 查看错误、标签、耗时（`elapsedMs`）、序号（`seq`）与时间戳（`ts`）

## 4. 注意事项

- `reportToken` 等同写入凭证，请勿提交到客户端代码仓库的公开位置。
- 上报地址结尾不要带 `/`，SDK 会自动拼接 `/api/qa/report`。
- 本地联调时 `BaseUrl` 使用 `http://localhost:3000`；生产环境替换为实际部署地址。
