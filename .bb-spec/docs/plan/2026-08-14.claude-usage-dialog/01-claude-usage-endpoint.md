---
name: claude-usage-endpoint
description: 后端实时端点 GET /api/channel/:id/claude/usage：service 层回源 + 401 刷新重试，controller 拼信封（含 subscription_type），路由注册。
---

# Claude 实时用量端点

## 目标

管理员经 `GET /api/channel/:id/claude/usage` 实时拿到单个 Claude 订阅渠道的上游用量原始数据，accessToken 过期时自动刷新后重试。

## 业务规则（来源：spec claude-realtime-usage-endpoint）

- 端点 `GET /api/channel/:id/claude/usage`，管理员 + 渠道读权限。
- 渠道不存在、类型非 61、多 key 渠道、Key 解析失败或 accessToken 为空 → `success:false` + 对应 message，不发上游请求。
- 用凭据 accessToken 调 `GET {baseURL}/api/oauth/usage`（Authorization Bearer + anthropic-beta oauth-2025-04-20 + claude-code User-Agent；渠道代理；15 秒超时）。
- 上游 401/403 且凭据含 refreshToken：自动刷新凭据（成功则回写渠道 Key 并刷新渠道缓存），用新 accessToken 重试一次；刷新失败或重试仍非 2xx 按上游失败返回；整个请求周期至多重试一次。
- 响应恒 HTTP 200，信封：`success`（上游最终状态码 2xx）、`message`（成功空串 / `upstream status: <code>` / 本地校验错误描述）、`upstream_status`（上游最终状态码）、`data`（上游 body JSON 反序列化结果，失败为原始字符串）、`subscription_type`（可选，凭据 claudeAiOauth.subscriptionType 原样标量，缺失省略）。
- 上游数据原样透传不筛选不改名；响应不含渠道凭据；不读写轮询用量缓存。

## 涉及文件

- `service/claude_oauth_usage.go` — 修改（追加渠道级回源函数）
- `service/claude_oauth_usage_test.go` — 修改（追加测试）
- `controller/claude_usage.go` — 新建
- `controller/claude_usage_test.go` — 新建
- `router/channel-router.go` — 修改（`channelPermissionRoutes` 加一行）

## 函数清单

### service/claude_oauth_usage.go

| 函数 | 职责 |
|---|---|
| `FetchClaudeChannelUsage` | 渠道级回源：解析渠道 Key 的 claudeAiOauth 凭据（复用包内既有解析函数）、经 `NewProxyHttpClient` 建带代理 client、15s 超时调 `FetchClaudeOAuthUsage`；首次 401/403 且凭据含 refreshToken 时调 `RefreshClaudeChannelCredential`（带 ResetCaches）后重新解析新 Key 重试一次；返回最终状态码、body 与凭据 subscriptionType |

### controller/claude_usage.go

| 函数 | 职责 |
|---|---|
| `GetClaudeChannelUsage` | 解析 `:id`、`model.GetChannelById` 加载、依次校验存在/类型 61/非多 key，调 `service.FetchClaudeChannelUsage`，按信封拼 `gin.H` 返回（HTTP 恒 200；data 反序列化失败退化为原始字符串；subscription_type 非空才写入） |

### router/channel-router.go

无新函数：`channelPermissionRoutes` 切片新增 `{method: http.MethodGet, path: "/:id/claude/usage", permission: authz.ChannelRead, handler: controller.GetClaudeChannelUsage}`，与既有 `POST /:id/claude/refresh` 相邻。

## 协作关系

- `GetClaudeChannelUsage` → `service.FetchClaudeChannelUsage` → 上游 `{baseURL}/api/oauth/usage`；401/403 分支 → `service.RefreshClaudeChannelCredential`（内部回写渠道 Key + 刷新缓存）→ 重试。
- 上游 baseURL 来自渠道配置（测试可用 httptest server 注入）；凭据刷新的 token URL 为包内硬编码常量，刷新重试分支不做单测（接受缺口，真实环境验证）。

## 验证方式

- 测试入口：HTTP `GET /api/channel/:id/claude/usage`（controller 直调范式）与包级 `service.FetchClaudeChannelUsage`；上游用 `httptest.NewServer` 模拟，其 URL 写入渠道 base_url；渠道数据经 sqlite in-memory `model.DB` 预置。
- 测试输入与预期：
  - 渠道类型 61、凭据 `{"claudeAiOauth":{"accessToken":"tk","subscriptionType":"max"}}`、mock 上游 200 返回 `{"five_hour":{"utilization":41}}` → 响应 `success:true`、`upstream_status:200`、`data` 逐字段一致、`subscription_type:"max"`。
  - 凭据无 subscriptionType → 响应无 `subscription_type` 字段。
  - mock 上游 401 且凭据无 refreshToken → `success:false`、`upstream_status:401`、message 为 `upstream status: 401`；上游只收到一次请求。
  - mock 上游 200 返回非 JSON 文本 → `success:true` 且 `data` 为该原始字符串。
  - 渠道不存在 / 类型非 61（如 57）/ 多 key 渠道 → `success:false` + 对应 message，mock 上游零请求。
  - 请求头断言：mock 上游收到 Authorization Bearer、anthropic-beta、claude-code User-Agent 三件套。
- [ ] 上述场景各有断言精确期望值的测试（testify，table-driven 优先）。
- [ ] 刷新重试分支在 PROGRESS 标注为接受的测试缺口。
