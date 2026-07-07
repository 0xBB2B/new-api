---
name: credential-auto-refresh
description: refresh_token 换新 token 并轮换回写 Key;后台任务仅 master、10min tick、剩余有效期 <24h 才刷新、跳过 multi-key;另有手动刷新入口。
---

# 凭据自动刷新

## 目的
在 accessToken 过期前自动续期,避免订阅渠道因 token 过期而中断;同时提供管理员手动刷新入口。

## 逻辑
- 刷新请求:`POST https://console.anthropic.com/v1/oauth/token`,JSON body `{"grant_type":"refresh_token","refresh_token":<当前 refreshToken>,"client_id":"9d1c250a-e61b-44d9-88ed-5944d1962f5e"}`。
- 响应含新 `access_token`、新 `refresh_token`、`expires_in`(秒)。刷新时**轮换 refresh token**(用返回的新值覆盖旧值)。
- 回写:把新 accessToken/refreshToken 写回渠道 `Key` 的 `claudeAiOauth`,并把 `expiresAt` 更新为 `now + expires_in`(毫秒时间戳);经 GORM 更新 Key 字段后重建渠道缓存。
- 后台任务:仅 master 节点运行;每 10 分钟 tick;分页扫描本类型且状态为启用/自动禁用的渠道;对「剩余有效期 < 24 小时(或 expiresAt 解析失败)」的渠道执行刷新;**跳过 multi-key 渠道**;单次刷新失败不影响其它渠道。
- 手动入口:管理员可对单个渠道触发一次立即刷新,并即时重建缓存。

## 约束
- 刷新用 refresh_token grant,client_id 为上述固定值,端点为上述 OAuth token URL。
- 刷新成功后 refreshToken 被新值替换、expiresAt 更新为 now+expires_in(毫秒)。
- 后台任务仅在 master 节点、每 10 分钟触发,只刷新剩余有效期 <24h 的渠道,跳过 multi-key。
- 渠道无 refreshToken → 跳过刷新(不报致命错、不禁渠道)。

## 例子
- 某渠道 expiresAt 距今 20 小时(<24h)→ 后台 tick 命中,POST 刷新端点拿到新 token,Key 内 accessToken/refreshToken 更新、expiresAt 变为 now+expires_in。
- 某渠道 expiresAt 距今 40 小时 → 后台 tick 跳过,不刷新。
- 管理员点「刷新凭据」→ 立即执行一次刷新并重建缓存,无视 24h 阈值。

## 验收
- [ ] 刷新请求体为 refresh_token grant + 固定 client_id + 正确端点。
- [ ] 刷新后 Key 内 accessToken/refreshToken 更新、expiresAt=now+expires_in(毫秒)。
- [ ] 后台任务仅 master、10min tick、只处理剩余 <24h 的渠道、跳过 multi-key。
- [ ] 手动刷新对单渠道立即生效并重建缓存。
