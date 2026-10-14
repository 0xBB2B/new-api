# 术语表

> 全项目术语唯一权威源，仅 /prd 维护。跨语言 / 跨文档以英文锚点对齐；别名按需增补，只登记实际使用过的语言写法。

| 锚点 | 定义 | 别名 |
|---|---|---|
| quota | 用户账户以内部计费单位存储的可用余额，API 调用按用量扣减 | 简体「额度」 |
| quota-reset-rule | 定义周期与重置值的配置，决定用户余额何时被重置为多少 | 简体「额度重置规则」 |
| reset-opt-out | 用户级标记，使该用户不参与任何自动/手动规则重置 | 简体「退出自动重置标记」 |
| reset-value | 规则触发时 quota 被覆盖写入的目标值，≥0 整数 | 简体「重置值」 |
| Claude Subscription channel | 用 Claude Max/Pro 订阅的 OAuth 凭据（而非 API key）作上游鉴权的渠道类型，上游为 api.anthropic.com | 简体「Claude 订阅渠道」 |
| claudeAiOauth credential | Claude Code 存储于 ~/.claude/.credentials.json 的 OAuth 凭据 JSON，含 accessToken/refreshToken/expiresAt（毫秒时间戳） | 简体「OAuth 凭据」 |
| Claude Code system prompt | OAuth 鉴权硬要求置于首条 system 的身份串 `You are Claude Code, Anthropic's official CLI for Claude.` | 简体「Claude Code 身份 system」 |
| username | 用户登录名，全站唯一，日志与审计记录中持久保存的用户标识文本，对应 users.username | 简体「用户名」 |
| display name | 用户可自行修改的展示用名称，允许为空与重名，对应 users.display_name | 简体「显示名」 |
| limit reset | Claude 订阅账号把用量窗口立即清零的权益，由上游按账号授予，分全部重置与 5 小时重置 | 简体「限额重置」 |
| full reset | 上游程序 `cedar_ember` 授予的重置，按授予次数使用，清空授予记录里 `clears` 列出的窗口 | 简体「全部重置」 |
| 5-hour reset | 上游程序 `juniper_tide` 的重置，每周可用次数有限，只在 5 小时额度用完时可用，只清空 5 小时窗口 | 简体「5 小时重置」 |
| enabled common user | 角色为普通用户（role=1）、状态为启用、未被删除的用户；不含管理员与 Root | 简体「开通用户」；利用率卡片上的人数称「开通人数」 |
| utilization | 数据看板中按人数统计开通用户 Token 使用程度的分区 | 简体「利用率」 |
| deep user | 近 7 天（滚动 7×24 小时）Token 用量大于 1 亿的开通用户 | 简体「深度使用」 |
| frequent user | 近 7 天（滚动 7×24 小时）Token 用量大于 100 万的开通用户，包含 deep user | 简体「经常使用」 |
| inactive user | 近 30 天（滚动 30×24 小时）Token 用量不超过 100 万的开通用户，含从未使用者 | 简体「非活跃」 |
