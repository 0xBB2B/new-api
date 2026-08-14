---
name: claude-realtime-usage-endpoint
description: 管理端实时端点 GET /api/channel/:id/claude/usage：用渠道凭据回源查询用量，401/403 自动刷新凭据重试一次，透传上游数据。
---

# Claude 订阅实时用量端点

## 目的

让管理员按需实时查看单个 Claude 订阅渠道（type 61）的官方用量详情，数据直接来自上游、不经轮询缓存。

## 逻辑

`GET /api/channel/:id/claude/usage`（管理员 + 渠道读权限）。按 `:id` 加载渠道并校验后，从渠道 Key 的 claudeAiOauth JSON 解析 accessToken，调 `GET {baseURL}/api/oauth/usage`（请求头 `Authorization: Bearer`、`anthropic-beta: oauth-2025-04-20`、claude-code User-Agent；渠道配置代理时经代理；单次请求 15 秒超时）。上游返回 401/403 且凭据含 refreshToken 时：自动执行凭据刷新，成功则把新凭据回写渠道 Key 并刷新渠道缓存，用新 accessToken 重试一次；刷新失败或重试后仍非 2xx，按上游失败返回。

响应恒为 HTTP 200，信封：

- `success`：上游最终状态码是否 2xx。
- `message`：成功为空串；上游非 2xx 时为 `upstream status: <code>`；本地校验失败时为对应错误描述。
- `upstream_status`：上游最终 HTTP 状态码（本地校验失败时无此字段或为 0）。
- `data`：上游响应体的 JSON 反序列化结果；反序列化失败时为原始字符串。

## 约束

- 渠道不存在、类型非 61、多 key 渠道三种情况直接返回 `success:false` 与对应 message，不发起上游请求。
- 渠道 Key 解析失败或 accessToken 为空返回 `success:false`，不发起上游请求。
- 401/403 自动刷新仅在凭据含 refreshToken 时触发，且整个请求周期内至多重试一次；刷新成功必须回写渠道 Key 并刷新渠道缓存。
- 上游返回的用量数据原样透传（不筛选、不改字段名）；响应中不出现渠道凭据。
- 端点不读写轮询用量缓存。

## 例子

渠道 12（type 61，凭据含有效 accessToken）请求本端点，上游返回 200 与 `{"five_hour":{"utilization":41},"seven_day":{"utilization":12}}`：响应为 `{"success":true,"message":"","upstream_status":200,"data":{"five_hour":{"utilization":41},"seven_day":{"utilization":12}}}`。

渠道 13 的 accessToken 已过期且凭据含 refreshToken：首次上游 401 → 自动刷新成功并回写 Key → 重试返回 200 → 响应 `success:true`，管理员无感知。渠道 14 的凭据无 refreshToken：上游 401 → 响应 `{"success":false,"message":"upstream status: 401","upstream_status":401,...}`。

## 验收

- [ ] 正常凭据请求返回 success:true 且 data 与上游 body 逐字段一致。
- [ ] 渠道不存在 / 类型非 61 / 多 key 渠道各返回 success:false 与对应 message，且无上游请求发出。
- [ ] 上游 401 且凭据含 refreshToken：刷新后重试成功，响应 success:true，渠道 Key 已更新为新凭据。
- [ ] 上游 401 且凭据无 refreshToken：响应 success:false、upstream_status=401，不触发刷新。
- [ ] 刷新成功但重试仍 401：响应 success:false，不再二次刷新。
- [ ] 上游返回非 JSON body 时 data 为原始字符串，success 按状态码判定。
