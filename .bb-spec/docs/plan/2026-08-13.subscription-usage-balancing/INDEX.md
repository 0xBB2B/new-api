# subscription-usage-balancing 实施计划

## 阶段 1：基础（两份互不依赖）

- [subscription-usage-cache](01-subscription-usage-cache.md) — model 缓存泛化为瓶颈单值 + 订阅语义 API；constant 新增订阅渠道谓词
- [claude-usage-source](02-claude-usage-source.md) — 新建 Claude oauth usage 查询与两形态兼容的瓶颈解析

## 阶段 2：接线（两份互不依赖）

- [subscription-usage-poll-task](03-subscription-usage-poll-task.md) — 轮询任务按类型间隔调度 + type switch 分派数据源 + Redis key 更名 [依赖: subscription-usage-cache, claude-usage-source]
- [balanced-selection](04-balanced-selection.md) — 选择侧 5 处类型判定改谓词 + 触顶/权重函数更名订阅语义 [依赖: subscription-usage-cache]
