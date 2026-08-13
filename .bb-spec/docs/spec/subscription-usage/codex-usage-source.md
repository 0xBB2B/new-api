---
name: codex-usage-source
description: Codex 渠道用量数据源：GET /backend-api/wham/usage，取 primary/secondary 窗口 used_percent 的最大值为瓶颈；最小轮询间隔 60 秒。
---

# Codex 用量数据源

## 目的

定义 Codex 订阅渠道（type 57）的官方用量查询方式与瓶颈使用率提取规则。

## 逻辑

用渠道 OAuth 凭据中的 access_token 与 account_id 调上游 `GET {baseURL}/backend-api/wham/usage`。响应的 `rate_limit.primary_window`（5 小时窗口）与 `rate_limit.secondary_window`（7 天窗口）各含 `used_percent`；瓶颈使用率取出现窗口的 `used_percent` 最大值。

## 约束

- 请求路径为 `{baseURL}/backend-api/wham/usage`，鉴权用渠道 OAuth 凭据的 access_token 与 account_id；渠道配置了代理时经代理发起。
- 最小轮询间隔 60 秒。
- 窗口出现但其 `used_percent` 缺失时，该窗口按 0 参与取最大值。
- `rate_limit.primary_window` 与 `rate_limit.secondary_window` 均缺失时，本次查询判定无效，等同失败。
- 单次查询请求必须设置超时，超时视为失败。

## 例子

某 Codex 渠道查询返回 `rate_limit.primary_window.used_percent=42.5`、`rate_limit.secondary_window.used_percent=80.1`，瓶颈使用率为 80.1。另一渠道返回的 JSON 中两窗口均缺失，本次查询判定无效。

## 验收

- [ ] 响应含 (primary=42.5, secondary=80.1) 时瓶颈使用率为 80.1。
- [ ] 响应仅含 primary_window（used_percent=30）时瓶颈使用率为 30。
- [ ] 响应两窗口均缺失时判定为失败。
- [ ] 同一渠道相邻两次查询间隔不小于 60 秒。
