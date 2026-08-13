---
name: claude-usage-source
description: Claude 订阅渠道用量数据源：GET /api/oauth/usage，Bearer + oauth beta + claude-code UA，取所有窗口使用率最大值为瓶颈；最小轮询间隔 180 秒。
---

# Claude 订阅用量数据源

## 目的

定义 Claude 订阅渠道（type 61）的官方用量查询方式与瓶颈使用率提取规则。

## 逻辑

用渠道 OAuth 凭据中的 accessToken 调上游 `GET {baseURL}/api/oauth/usage`（baseURL 默认 `https://api.anthropic.com`）。该接口是 Claude Code `/usage` 命令的数据源，响应存在两种形态，解析时同时兼容：

- 顶层窗口对象形态：`five_hour`、`seven_day`、`seven_day_opus` 等对象各含 `utilization`（0–100 百分比）。
- `limits` 数组形态：各项含 `percent`（0–100 百分比）。

瓶颈使用率取响应中所有出现窗口使用率的最大值。

## 约束

- 请求头必须同时携带 `Authorization: Bearer <accessToken>`、`anthropic-beta: oauth-2025-04-20`、`User-Agent: claude-code/<固定版本常量>`；缺失 claude-code User-Agent 会落入上游激进限流桶导致持续 429。
- 最小轮询间隔 180 秒；该接口按 access token 独立限流，更短间隔会持续 429。
- 窗口对象出现但其 `utilization`（或数组项的 `percent`）缺失时，该窗口按 0 参与取最大值。
- 两种形态均未出现任何用量窗口时，本次查询判定无效，等同失败。
- accessToken 过期导致的 401 按查询失败处理，本数据源不做凭据刷新。
- 渠道配置了代理时经代理发起；单次查询请求必须设置超时，超时视为失败。

## 例子

某 Claude 订阅渠道查询返回 `{"five_hour":{"utilization":23},"seven_day":{"utilization":12},"seven_day_opus":{"utilization":68}}`，瓶颈使用率为 68。另一渠道返回 `{"limits":[{"kind":"session","percent":40},{"kind":"weekly_all","percent":85}]}`，瓶颈使用率为 85。第三个渠道 accessToken 已过期，上游返回 401，本次查询判定失败。

## 验收

- [ ] 顶层窗口对象形态 (five_hour=23, seven_day=12, seven_day_opus=68) 的瓶颈使用率为 68。
- [ ] limits 数组形态 (session=40, weekly_all=85) 的瓶颈使用率为 85。
- [ ] 响应无任何用量窗口时判定为失败。
- [ ] 请求头含 Authorization Bearer、anthropic-beta oauth-2025-04-20 与 claude-code User-Agent 三者。
- [ ] 上游返回 401 时判定为失败，不触发凭据刷新。
- [ ] 同一渠道相邻两次查询间隔不小于 180 秒。
