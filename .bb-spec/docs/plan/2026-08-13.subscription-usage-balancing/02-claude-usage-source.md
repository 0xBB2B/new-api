---
name: claude-usage-source
description: 新建 service/claude_oauth_usage.go：三头强制的 oauth usage 查询与两形态兼容的瓶颈解析。
---

# Claude 订阅用量数据源

## 目标

service 包提供 Claude 订阅渠道的官方用量查询（`GET {baseURL}/api/oauth/usage`）与瓶颈使用率解析能力，供轮询任务分派调用。

## 业务规则（来源：spec claude-usage-source）

- 请求头必须同时携带三者：`Authorization: Bearer <accessToken>`、`anthropic-beta: oauth-2025-04-20`、`User-Agent: claude-code/<固定版本常量>`；缺 claude-code UA 会落入上游激进限流桶持续 429。
- 响应存在两种形态，解析时同时兼容：顶层窗口对象（键为 `five_hour` 或 `seven_day` 前缀族，如 `seven_day`、`seven_day_opus`、`seven_day_sonnet`，各含 `utilization` 0–100；`extra_usage` 等其他顶层对象不是窗口，不参与）；`limits` 数组（各项含 `percent` 0–100）。
- 瓶颈使用率 = 响应中所有出现窗口使用率的最大值；窗口对象出现但 `utilization`（或数组项 `percent`）缺失时该窗口按 0 参与。
- 两种形态均未出现任何用量窗口时，本次查询判定无效（返回错误）。
- accessToken 过期导致的 401 由调用方按查询失败处理，本数据源不做凭据刷新。
- baseURL 为空或 accessToken 为空直接报错；请求经调用方传入的 http client 发出（代理与超时由调用方控制）。

## 涉及文件

- 新建 `service/claude_oauth_usage.go`
- 新建 `service/claude_oauth_usage_test.go`

## 函数清单

### service/claude_oauth_usage.go

| 函数名 | 职责 |
|---|---|
| `FetchClaudeOAuthUsage` | 构造并发出带三个强制头的 GET 请求，返回状态码与响应体字节 |
| `parseClaudeOAuthUsageBottleneck` | 从响应 JSON 提取所有窗口使用率取最大值（顶层 five_hour 与 seven_day* 族 + limits[]，忽略 extra_usage 等非窗口键）；无任何窗口报错 |

常量 `claudeUsageUserAgent`：值为 `claude-code/<版本>`，版本号在 exec 落盘前经 npm registry 查询 `@anthropic-ai/claude-code` 最新稳定版填入，禁凭记忆书写。JSON 解析走 `common.Unmarshal`；请求构造风格对齐 `FetchCodexWhamUsage`（service/codex_wham_usage.go）：TrimSpace/TrimRight 清洗、空参数前置报错、defer 关闭响应体。

## 协作关系

- 被轮询任务（plan 03 的 Claude 分派分支）调用：传入代理 client、渠道 baseURL（type 61 默认值已由 `constant.ChannelBaseURLs` 承担，经 `channel.GetBaseURL()` 取得）与凭据 accessToken。
- 外部依赖：上游 `GET {baseURL}/api/oauth/usage`。无 DB、无 Redis。

## 验证方式

- 测试入口：`go test ./service -run ClaudeOAuthUsage`（包内测试直接调用两个函数）。
- 测试输入：
  - `httptest.Server` 捕获请求：断言路径为 `/api/oauth/usage`、方法 GET、三个请求头逐一精确匹配（Bearer 值、`oauth-2025-04-20`、`claude-code/` 前缀的 UA 常量值）。
  - 解析表测：顶层形态 `{"five_hour":{"utilization":23},"seven_day":{"utilization":12},"seven_day_opus":{"utilization":68}}`；数组形态 `{"limits":[{"kind":"session","percent":40},{"kind":"weekly_all","percent":85}]}`；混合形态（顶层 + limits 并存取全局 max）；窗口出现但值缺失 `{"five_hour":{}}`；空对象 `{}`；非 JSON 字节。
- 预期结果：
  - 三头断言全部通过；空 baseURL / 空 accessToken 不发请求直接报错。
  - 解析：顶层形态得 68；数组形态得 85；混合取全局最大；`{"five_hour":{}}` 得 0（窗口出现值缺失按 0）；`{}` 报错；非 JSON 报错。
- [ ] 上述用例全绿（testify，表驱动 + t.Run）
- [ ] `cd relaykit && GOWORK=off go build ./...` 不受影响（本 plan 不触碰 relaykit）
