# 术语表

> 全项目术语唯一权威源，仅 /spec 维护。跨语言 / 跨文档以英文锚点对齐；别名按需增补，只登记实际使用过的语言写法。

| 锚点 | 定义 | 别名 |
|---|---|---|
| quota | 用户账户以内部计费单位存储的可用余额，API 调用按用量扣减 | 简体「额度」 |
| quota-reset-rule | 定义周期与重置值的配置，决定用户余额何时被重置为多少 | 简体「额度重置规则」 |
| reset-opt-out | 用户级标记，使该用户不参与任何自动/手动规则重置 | 简体「退出自动重置标记」 |
| reset-value | 规则触发时 quota 被覆盖写入的目标值，≥0 整数 | 简体「重置值」 |
| Claude Subscription channel | 用 Claude Max/Pro 订阅的 OAuth 凭据（而非 API key）作上游鉴权的渠道类型，上游为 api.anthropic.com | 简体「Claude 订阅渠道」 |
| claudeAiOauth credential | Claude Code 存储于 ~/.claude/.credentials.json 的 OAuth 凭据 JSON，含 accessToken/refreshToken/expiresAt（毫秒时间戳） | 简体「OAuth 凭据」 |
| Claude Code system prompt | OAuth 鉴权硬要求置于首条 system 的身份串 `You are Claude Code, Anthropic's official CLI for Claude.` | 简体「Claude Code 身份 system」 |
