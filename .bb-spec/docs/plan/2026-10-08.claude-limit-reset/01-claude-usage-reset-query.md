---
name: 01-claude-usage-reset-query
description: Claude 订阅用量请求带上限额重置参数和 claude-cli UA；版本号从 npm 取并缓存 1 小时，取不到时退回原请求并在接口响应里标出。
---
# Claude 用量请求顺带查询限额重置

## 目标
Claude 订阅渠道的每次用量请求都顺带拿回全部重置（`cedar_ember`）与 5 小时重置（`juniper_tide`）信息，接口 `GET /api/channel/:id/claude/usage` 的 `data` 里原样带出；取不到客户端版本号时用量照常，响应里标出原因。

## 业务规则（来源：spec claude-limit-reset-query）
- 适用于所有发往 `<base_url>/api/oauth/usage` 的 Claude 订阅用量请求：打开弹窗、弹窗内刷新、后台轮询、凭据刷新后的重试。
- 客户端版本号：GET `https://registry.npmjs.org/@anthropic-ai/claude-code/latest`，读 JSON 的 `version` 字段并去掉首尾空白；空串或非 2xx 视为失败。进程内缓存 1 小时；过期后查询失败时沿用旧值，并再缓存 1 小时；从没查到过且本次失败 = 取不到版本号。查询使用与用量请求同一个 HTTP 客户端（含渠道代理），超时 ≤ 10 秒。
- 拿到版本号：请求地址加 `cedar_ember=1&at_wall=1`，请求头加 `User-Agent: claude-cli/<版本号> (external, cli)`；`Authorization`、`anthropic-beta: oauth-2025-04-20`、`Accept: application/json` 不变。
- 取不到版本号：请求不加这两个参数，也不设置 `User-Agent`（与改动前相同）；用量照常返回，接口 JSON 额外带 `"limit_reset_unavailable": "client_version"`。
- 版本号查询失败不能让用量查询失败。
- 快照 `other_info.subscription_usage` 只存 5 小时与每周窗口，不存任何重置信息（`parseClaudeUsageSnapshot` 不改）。

## 涉及文件
- 新建 `service/claude_cli_version.go`
- 修改 `service/claude_usage.go`
- 修改 `controller/claude_usage.go`
- 修改 `service/subscription_usage_poll_task.go`（只跟着改调用方式）
- 修改 `service/claude_usage_test.go`

## 函数清单
### service/claude_cli_version.go（新建）
| 函数名 | 职责 |
|---|---|
| `claudeCLIVersionCache`（类型） | 带锁的版本号缓存：版本号 + 过期时刻；写法参照 `service/codex_models.go` 的 `codexClientVersionCache` |
| `claudeCLIVersionCache.get` | 按上面的缓存规则返回版本号；npm 地址与当前时间由参数传入，便于测试 |
| `fetchLatestClaudeCLIVersion` | 请求 npm 地址并解析 `version`，非 2xx、解析失败、空串都返回错误 |
| `GetLatestClaudeCLIVersion` | 导出入口：用包级缓存实例和固定的 npm 地址调用 `get`；供用量查询与 02 的执行重置共用 |
| `claudeCLIUserAgent` | 由版本号拼出 `claude-cli/<版本号> (external, cli)` |

### service/claude_usage.go（修改）
| 函数名 | 职责 |
|---|---|
| `fetchClaudeOAuthUsage` | 增加版本号输入：非空时加查询参数 `cedar_ember=1&at_wall=1` 并设 UA，空时保持原请求 |
| `SyncClaudeChannelUsage` | 开头用 ≤ 10 秒的 context 调 `GetLatestClaudeCLIVersion`，失败时记 `common.SysLog` 并以空版本号继续；首次请求和刷新后的重试都传同一版本号；返回值新增「是否取到版本号」 |

### controller/claude_usage.go（修改）
| 函数名 | 职责 |
|---|---|
| `GetClaudeChannelUsage` | 取不到版本号时，响应 JSON 加 `limit_reset_unavailable: "client_version"`；取到时不出现该字段 |

### service/subscription_usage_poll_task.go（修改）
| 函数名 | 职责 |
|---|---|
| Claude 分支调用处 | 适配 `SyncClaudeChannelUsage` 新增的返回值并忽略它，轮询行为不变 |

## 协作关系
`GetClaudeChannelUsage` / 轮询任务 → `SyncClaudeChannelUsage` → `GetLatestClaudeCLIVersion` → `claudeCLIVersionCache.get` → `fetchLatestClaudeCLIVersion`（外部：registry.npmjs.org）；随后 `fetchClaudeOAuthUsage`（外部：api.anthropic.com `/api/oauth/usage`）。JSON 解析用 `common.DecodeJson` / `common.Unmarshal`。

## 验证方式
- 测试入口：`go test ./service/ -run 'ClaudeCLIVersion|ClaudeOAuthUsage'`（测试写在 `service/claude_usage_test.go`，testify require/assert，表驱动）；另跑 `go test ./service/ ./controller/` 回归。
- 测试输入与预期：
  - 用 httptest 假冒 npm，返回 `{"version":"2.1.293"}` → 缓存 `get` 返回 `2.1.293`；同一缓存在 30 分钟后再取，npm 收到的请求次数仍为 1。
  - 缓存 61 分钟后过期、假 npm 改为返回 503 → 返回旧值 `2.1.293`；再过 30 分钟取一次，npm 没有收到新请求。
  - 空缓存、假 npm 返回 `{"version":"  "}` 或 500 → 返回错误。
  - 用 httptest 假冒 Anthropic，版本号为 `2.1.293` 调用 `fetchClaudeOAuthUsage` → 上游收到的查询参数包含 `cedar_ember=1` 与 `at_wall=1`，`User-Agent` 为 `claude-cli/2.1.293 (external, cli)`，`Authorization` 与 `anthropic-beta` 不变。
  - 版本号为空串 → 上游请求不含这两个查询参数，`User-Agent` 不是 `claude-cli/` 开头。
- [ ] 上面五种情况的测试全部通过
- [ ] `go build ./...` 与 `go vet ./service/ ./controller/` 通过
- [ ] 新改的 Go 文件已 gofmt，JSON 全部走 `common.*`
