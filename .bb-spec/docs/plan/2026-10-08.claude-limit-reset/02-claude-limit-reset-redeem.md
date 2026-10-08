---
name: 02-claude-limit-reset-redeem
description: 新接口 POST /api/channel/:id/claude/usage/reset（ChannelOperate）执行全部重置或 5 小时重置；校验 → 取版本号 → 取组织 UUID → 调上游，request_id 固定，仅 result=reset 成功。
---
# 执行 Claude 限额重置接口

## 目标
有 `ChannelOperate` 权限的管理员可以通过接口对一个 Claude 订阅渠道执行一次全部重置或 5 小时重置；同一次重试不会重复扣次数，失败原因用中文返回。

## 业务规则（来源：spec claude-limit-reset-redeem）
- 请求体：`{"program": "cedar_ember" | "juniper_tide", "grant_id": string, "resets_left": int}`；`grant_id`、`resets_left` 仅 `cedar_ember` 必填。
- 处理顺序：
  1. 校验渠道：存在、类型为 Claude 订阅、不是多 key 渠道、凭据可解析；
  2. 校验请求：`program` 只能取两个值；`cedar_ember` 时 `grant_id` 匹配 `^[a-z0-9_-]{1,40}$`、`resets_left` 为 1～100 的整数；
  3. 取客户端版本号（与 01 共用缓存）；
  4. `GET <base_url>/api/oauth/profile` 取 `organization.uuid`，必须是标准 UUID 格式；
  5. `POST <base_url>/api/organizations/<uuid>/reset_rate_limits`。
- 第 2～4 步任一失败都不向 `reset_rate_limits` 发请求。
- 上游请求体：`cedar_ember` 为 `{"program":"cedar_ember","grant_id":"<grant_id>","request_id":"<grant_id>-u<resets_left>"}`，`juniper_tide` 为 `{"program":"juniper_tide"}`。
- 两次上游请求都带 `Authorization: Bearer <accessToken>`、`anthropic-beta: oauth-2025-04-20`、`User-Agent: claude-cli/<版本号> (external, cli)`、`Accept: application/json`。
- 任一步拿到 401/403 时，调 `RefreshClaudeChannelCredential(ctx, ch.Id, ClaudeCredentialRefreshOptions{ResetCaches: true})` 刷新凭据，再从第 4 步起整体重试一次。
- 结果：上游 2xx 且 `result == "reset"` 才 `success=true`。其余按下表给中文 `message`，并带回 `upstream_status` 和原始 JSON（`data`）；上游错误文本不进 `message`。
  - `already_used` → 这次重置已经用过了
  - `not_limited` → 当前没有触顶，不需要重置
  - `cooldown` → 冷却中，请稍后再试
  - `ineligible` → 账号不符合使用条件
  - `unavailable` → 上游暂时不可用
  - 429 → 请求太频繁，请稍后再试
  - 401/403（重试后仍是）→ 凭据无效或权限不足
  - 其他 → 上游返回 HTTP <状态码>
- 本地校验失败的 message：program 不对 → 不支持的重置类型；grant_id 或 resets_left 不对 → 重置参数无效；取不到版本号 → 取不到 Claude Code 最新版本，无法执行重置；组织 ID 缺失或格式不对 → 获取账号组织信息失败。
- 不写审计日志。接口 HTTP 状态恒为 200，成败看 `success`，与 Codex 重置接口一致。

## 涉及文件
- 新建 `service/claude_limit_reset.go`
- 修改 `controller/claude_usage.go`
- 修改 `router/channel-router.go`
- 修改 `service/claude_usage_test.go`

## 成品定义
`router/channel-router.go` 权限路由表，紧跟 `/:id/claude/usage` 那一行之后新增：

```go
	{method: http.MethodPost, path: "/:id/claude/usage/reset", permission: authz.ChannelOperate, handler: controller.ResetClaudeChannelLimit},
```

## 函数清单
### service/claude_limit_reset.go（新建）
| 函数名 | 职责 |
|---|---|
| `ClaudeLimitResetRequest`（类型） | 请求体：program / grant_id / resets_left |
| `ClaudeLimitResetResult`（类型） | 结果：成功与否、中文 message、上游状态码、上游原始 body |
| `validateClaudeLimitResetRequest` | 按规则校验请求，失败返回对应中文 message |
| `fetchClaudeOAuthOrganizationUUID` | 请求 `/api/oauth/profile`，读 `organization.uuid` 并校验 UUID 格式（可用 `github.com/google/uuid` 解析，仓库已有该依赖） |
| `postClaudeResetRateLimits` | 构造上游请求体（含固定 request_id）并 POST 到 `reset_rate_limits` |
| `claudeLimitResetMessage` | 把上游状态码和 `result` 映射成中文 message |
| `ResetClaudeChannelLimit` | 导出入口：按处理顺序编排；遇 401/403 时刷新凭据，从取组织 ID 起整体重试一次；返回 `ClaudeLimitResetResult` |

### controller/claude_usage.go（修改）
| 函数名 | 职责 |
|---|---|
| `ResetClaudeChannelLimit` | 解析 `:id` 和请求体；渠道检查与 `GetClaudeChannelUsage` 相同（不存在 / 类型不对 / 多 key / 凭据解析失败分别返回原有文案）；调 service，按结果输出 `{success, message, upstream_status, data}` |

## 协作关系
路由（`ChannelOperate`，访问令牌权限范围由 `handlePermissionRoute` 自动登记）→ controller `ResetClaudeChannelLimit` → service `ResetClaudeChannelLimit` → `validateClaudeLimitResetRequest` → `GetLatestClaudeCLIVersion`（01）→ `fetchClaudeOAuthOrganizationUUID` → `postClaudeResetRateLimits` → `claudeLimitResetMessage`；401/403 时走 `RefreshClaudeChannelCredential`。HTTP 客户端用 `GetHttpClientWithProxy(ch.GetSetting().Proxy)`，每次上游请求超时 15 秒；JSON 走 `common.*`。

## 验证方式
- 测试入口：`go test ./service/ -run ClaudeLimitReset`、`go test ./router/ ./middleware/`；测试写在 `service/claude_usage_test.go`，用 httptest 假冒 Anthropic 的 profile 与 reset 两个端点，版本号通过预先填好的缓存或可注入的 npm 地址提供。
- 测试输入与预期：
  - `{"program":"cedar_ember","grant_id":"opus55-launch-promax-20260921","resets_left":1}`，profile 返回合法 UUID `11111111-2222-3333-4444-555555555555`，reset 返回 `{"result":"reset"}` → 成功；上游收到 `POST /api/organizations/11111111-2222-3333-4444-555555555555/reset_rate_limits`，请求体为 `{"program":"cedar_ember","grant_id":"opus55-launch-promax-20260921","request_id":"opus55-launch-promax-20260921-u1"}`，UA 为 `claude-cli/<版本> (external, cli)`。
  - 同样的请求提交两次 → 两次上游收到的 `request_id` 完全相同。
  - `{"program":"juniper_tide"}`，reset 返回 `{"result":"not_limited"}` → 失败，message「当前没有触顶，不需要重置」；上游请求体只有 `program`。
  - 表驱动：`already_used` / `cooldown` / `ineligible` / `unavailable` / HTTP 429 / HTTP 500 → 各自对应的 message，`success=false`。
  - 非法输入：program `foo`、grant_id `../x`、grant_id 41 个字符、resets_left 0 和 101 → 失败，reset 端点收到的请求数为 0。
  - profile 返回 `{"organization":{"uuid":"not-a-uuid"}}` → 失败，message「获取账号组织信息失败」，reset 端点收到的请求数为 0。
- [ ] 以上用例全部通过
- [ ] `go test ./router/ ./middleware/` 通过（访问令牌权限范围覆盖检查包含新路由）
- [ ] `go build ./...` 通过；新改的 Go 文件已 gofmt
