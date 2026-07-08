# codex-usage-balancing 实施计划

## 阶段 1：基础

- [usage-cache](01-usage-cache.md) — model 包进程级 Codex 用量缓存：写入钳制、触顶迁移日志、过期与触顶判定

## 阶段 2：采集与选择（两份互不依赖）

- [usage-poll-task](02-usage-poll-task.md) — 60s 轮询启用 Codex 渠道 wham usage 写入缓存，仅 master、失败不阻断 [依赖: usage-cache]
- [balanced-selection](03-balanced-selection.md) — 选择两路径触顶过滤 + 动态权重，亲和触顶跳过不清缓存 [依赖: usage-cache]
