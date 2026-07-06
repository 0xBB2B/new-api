---
name: oauth-request-headers
description: 上游鉴权头用 Authorization Bearer + anthropic-beta oauth-2025-04-20 + anthropic-version;禁带 x-api-key。
---

# OAuth 上游请求头

## 目的
定义 Claude 订阅渠道向 `api.anthropic.com` 发请求时的鉴权与协议头,区别于标准 claude 渠道的 x-api-key 方式。

## 逻辑
- 从渠道凭据解析出的 `accessToken` 作 OAuth Bearer token。
- 必设头:
  - `Authorization: Bearer <accessToken>`
  - `anthropic-beta: oauth-2025-04-20`
  - `anthropic-version: 2023-06-01`(客户端已带则沿用客户端值)
- 禁设 `x-api-key`:OAuth 鉴权与 x-api-key 互斥,若同时出现上游会拒绝。
- 客户端已带 `anthropic-beta` 时,`oauth-2025-04-20` 需并入而非丢弃(去重合并)。

## 约束
- 上游请求含 `Authorization: Bearer <accessToken>`。
- 上游请求含 `anthropic-beta` 且其值包含 `oauth-2025-04-20`。
- 上游请求含 `anthropic-version`(默认 `2023-06-01`)。
- 上游请求不含 `x-api-key` 头。

## 例子
- 渠道 accessToken=`sk-ant-oat01-xxx`,客户端未带 beta → 上游头:`Authorization: Bearer sk-ant-oat01-xxx`、`anthropic-beta: oauth-2025-04-20`、`anthropic-version: 2023-06-01`,无 `x-api-key`。
- 客户端带 `anthropic-beta: prompt-caching-2024-07-31` → 上游 `anthropic-beta` 合并为 `prompt-caching-2024-07-31,oauth-2025-04-20`。

## 验收
- [ ] 上游请求头含 `Authorization: Bearer <accessToken>`,不含 `x-api-key`。
- [ ] `anthropic-beta` 值包含 `oauth-2025-04-20`;客户端原有 beta 值保留并去重。
- [ ] `anthropic-version` 缺省为 `2023-06-01`,客户端已带时用客户端值。
