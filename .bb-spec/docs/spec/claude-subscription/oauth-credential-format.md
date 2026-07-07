---
name: oauth-credential-format
description: 渠道 Key 存 Claude Code 原生 OAuth JSON;解析 accessToken/refreshToken/expiresAt,accessToken 必填,expiresAt 为毫秒时间戳。
---

# OAuth 凭据格式

## 目的
定义 Claude 订阅渠道在 `Key` 字段接受的凭据形态:管理员直接粘贴 Claude Code 的 OAuth 凭据,无需手工转换。

## 逻辑
- 渠道 `Key` 字段存的是 Claude Code `~/.claude/.credentials.json` 的原生 JSON:`{"claudeAiOauth":{"accessToken":..., "refreshToken":..., "expiresAt":..., "scopes":[...], "subscriptionType":...}}`。
- 解析时读取 `claudeAiOauth` 内层对象;`accessToken` 必填,`refreshToken` 与 `expiresAt` 用于刷新。
- `expiresAt` 语义为「毫秒 Unix 时间戳」(非秒、非 RFC3339 字符串)。
- 凭据不是 `{` 开头的 JSON 对象、缺 `claudeAiOauth`、或 `accessToken` 为空 → 判定非法,鉴权阶段直接报错,不发上游。

## 约束
- Key 必须能解析出 `claudeAiOauth.accessToken` 且非空,否则报错。
- `expiresAt` 按毫秒时间戳解释;判断过期时以毫秒为单位比较。
- 缺 `refreshToken` 时该渠道无法自动/手动刷新,但仍可用当前 accessToken 发请求直到过期。

## 例子
- 粘贴 `{"claudeAiOauth":{"accessToken":"sk-ant-oat01-xxx","refreshToken":"sk-ant-ort01-yyy","expiresAt":1751808000000,"subscriptionType":"max"}}` → 解析成功,accessToken=`sk-ant-oat01-xxx`,过期时刻为 2025-07-06 对应毫秒。
- 粘贴裸串 `sk-ant-oat01-xxx`(非 JSON)→ 鉴权阶段报错「凭据必须是 Claude Code OAuth JSON」,不发上游。

## 验收
- [ ] 合法 `{"claudeAiOauth":{...}}` 能解析出 accessToken/refreshToken/expiresAt。
- [ ] 非 JSON、缺 claudeAiOauth、或 accessToken 为空 → 报错且不发上游请求。
- [ ] expiresAt 按毫秒时间戳参与过期判断(1751808000000 视为 2025-07-06,而非 1970 年附近)。
