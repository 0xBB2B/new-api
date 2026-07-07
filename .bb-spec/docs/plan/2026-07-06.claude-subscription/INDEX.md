# Claude 订阅渠道 实施计划

## 阶段 1：注册
- [01-channel-registration](01-channel-registration.md) — 渠道类型 59 与 APITypeClaudeSubscription 注册、base URL/显示名/分派/流式白名单

## 阶段 2：转发 adaptor
- [02-oauth-adaptor](02-oauth-adaptor.md) — claude_oauth adaptor（凭据解析/Bearer 鉴权/身份 system 注入/复用 claude 转发与计费）[依赖: 01-channel-registration]

## 阶段 3：凭据刷新
- [03-credential-refresh](03-credential-refresh.md) — OAuth 刷新 service 三件套 + 后台任务 + 手动刷新 handler/路由 + 单 key 校验 [依赖: 01-channel-registration, 02-oauth-adaptor]

## 阶段 4：前端配置 UI
- [04-frontend-config](04-frontend-config.md) — type 59 下拉/凭据提示/图标/刷新按钮/免责声明/隐藏 batch/6 语言文案 [依赖: 01-channel-registration, 03-credential-refresh]
