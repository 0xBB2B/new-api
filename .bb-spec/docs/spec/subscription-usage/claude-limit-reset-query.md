---
name: claude-limit-reset-query
description: Claude 订阅用量请求带 cedar_ember=1&at_wall=1 和 claude-cli/<npm 最新版> (external, cli) UA，顺带拿回全部重置与 5 小时重置信息；取不到版本号时退回不带重置参数。
---

# Claude 限额重置信息查询

## 目的
上游只在用量接口带特定参数、并且客户端身份像官方命令行工具时，才返回限额重置信息。把这两个条件加到现有的 Claude 订阅用量请求上，一次请求同时拿到用量和两种重置的状态，不额外调用上游。用量接口有频率限制，多一次调用就多一次被拒的风险。

## 逻辑
- 适用：Claude 订阅渠道向 `<base_url>/api/oauth/usage` 发出的所有用量请求，包括打开弹窗、弹窗内刷新、后台轮询、凭据刷新后的重试。
- 客户端版本号：
  - 从 `https://registry.npmjs.org/@anthropic-ai/claude-code/latest` 读 `version` 字段，进程内缓存 1 小时；
  - 缓存过期后再次查询失败时，继续使用旧值并再缓存 1 小时；
  - 从未查到过（没有旧值）且本次查询失败时，视为「取不到版本号」。
- 拿到版本号时：请求地址带查询参数 `cedar_ember=1&at_wall=1`，请求头 `User-Agent: claude-cli/<版本号> (external, cli)`；其余请求头（`Authorization`、`anthropic-beta: oauth-2025-04-20`、`Accept`）不变。
- 取不到版本号时：请求不带这两个查询参数、不设置 `User-Agent`（与改动前相同），用量照常返回；接口响应额外带 `limit_reset_unavailable: "client_version"`，供弹窗说明原因。
- 接口 `GET /api/channel/:id/claude/usage` 的 `data` 仍是上游原始 JSON，重置信息就在其中的 `cedar_ember`、`juniper_tide` 两块里，后端不另行改写。
- 用量快照（`other_info.subscription_usage`）只存 5 小时和每周窗口，不存任何重置信息；后台轮询拿到的重置信息直接丢弃。

## 约束
- 版本号查询失败不能让用量查询失败。
- 版本号只取 npm 返回的 `version` 字段，去掉首尾空白后为空串视为查询失败。
- 1 小时内多次用量请求最多触发一次 npm 查询。
- 版本号查询与用量请求使用同一个 HTTP 客户端（含渠道代理设置），超时不超过 10 秒。

## 例子
- npm 返回 `{"version":"2.1.293"}` → 用量请求为 `GET /api/oauth/usage?cedar_ember=1&at_wall=1`，UA 为 `claude-cli/2.1.293 (external, cli)`；上游返回的 `cedar_ember.eligible=true`、`juniper_tide.ineligible_reason="not_at_wall"` 原样出现在接口 `data` 里。
- 10:00 查到 `2.1.293`，10:30 再拉用量 → 不查 npm，沿用 `2.1.293`。
- 11:05 缓存过期，npm 返回 503 → 继续用 `2.1.293`，到 12:05 前不再查 npm。
- 服务刚启动、npm 不可达 → 用量请求为 `GET /api/oauth/usage`（无参数、无自定义 UA），接口返回 `success=true`、用量正常，并带 `limit_reset_unavailable: "client_version"`。
- 后台轮询拿到含 `cedar_ember` 的响应 → 快照里只有 5 小时与每周窗口字段。

## 验收
- [ ] 拿到版本号时，上游请求含 `cedar_ember=1`、`at_wall=1` 与 `User-Agent: claude-cli/<版本号> (external, cli)`。
- [ ] 取不到版本号时，上游请求不含这两个参数与该 UA，接口仍成功返回用量并带 `limit_reset_unavailable: "client_version"`。
- [ ] 版本号缓存 1 小时；过期后查询失败沿用旧值。
- [ ] npm 返回空 `version` 视为失败。
- [ ] 快照不含重置信息。
