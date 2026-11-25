# channel-failover 实施计划
## 阶段 1：选取基础
- [01-channel-pick-order](01-channel-pick-order.md) — 先订阅后其他的分档排序、排除式选渠道与选密钥
- [02-failover-ledger](02-failover-ledger.md) — 尝试记录与换渠道选取 [依赖: 01-channel-pick-order]
## 阶段 2：请求流程
- [04-stream-opening-hold](04-stream-opening-hold.md) — 扣住开场事件，正文前失败返回可重试错误
- [03-sync-retry-loop](03-sync-retry-loop.md) — 同步重试循环试完为止、清空残留状态 [依赖: 02-failover-ledger, 04-stream-opening-hold]
## 阶段 3：界面
- [05-retry-setting-copy](05-retry-setting-copy.md) — 重试设置文案与七种语言翻译
