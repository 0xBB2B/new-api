# claude-usage-dialog 实施计划

## 阶段 1：后端实时端点

- [claude-usage-endpoint](01-claude-usage-endpoint.md) — GET /api/channel/:id/claude/usage：service 回源 + 401 刷新重试 + 信封（含 subscription_type）

## 阶段 2：前端弹窗与列表样式

- [claude-dialog-list-style](02-claude-dialog-list-style.md) — Claude 弹窗 + 列表条前数后与阈值配色 + 按类型点击分流 [依赖: claude-usage-endpoint]
