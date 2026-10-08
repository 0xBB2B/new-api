# claude-limit-reset 实施计划

## 阶段 1：后端
- [01-claude-usage-reset-query](01-claude-usage-reset-query.md) — Claude 用量请求带上限额重置参数和 claude-cli UA；版本号从 npm 取并缓存 1 小时，取不到时退回原请求并标出
- [02-claude-limit-reset-redeem](02-claude-limit-reset-redeem.md) — POST /api/channel/:id/claude/usage/reset 执行全部重置或 5 小时重置 [依赖: 01-claude-usage-reset-query]

## 阶段 2：前端
- [03-usage-bar-click-open](03-usage-bar-click-open.md) — 去掉「账户信息」徽章，点进度条 / - / 打码徽章打开各自的用量弹窗（与 01、02 无依赖，可并行）
- [04-claude-limit-reset-card](04-claude-limit-reset-card.md) — Claude 用量弹窗新增限额重置卡片与执行重置 [依赖: 01-claude-usage-reset-query, 02-claude-limit-reset-redeem]
