# 执行进度
| 序号 | Plan | 状态 | 完成时间 |
|---|---|---|---|
| 01 | 01-channel-pick-order | done | 2026-11-25 |
| 02 | 02-failover-ledger | pending | — |
| 04 | 04-stream-opening-hold | pending | — |
| 03 | 03-sync-retry-loop | pending | — |
| 05 | 05-retry-setting-copy | pending | — |
## 当前
并行组 1：04-stream-opening-hold（Red）。之后：组 2 = 02 + 05，组 3 = 03。
## 阻塞
（无）
## 重试与自修（每次记一条）
- 01-channel-pick-order 自修第 1 轮：补测试（排除式选取的归一模型名回退与 filters、缓存关闭时轮询位置写回数据库）+ 改实现（内联 channelTierKey、修正 3 处注释、恢复 totalWeight）；结果：通过，复查合规（另收紧过滤用例断言）
## 无关发现（发现一条记一条）
- model/channel_cache.go:57-70 InitChannelCache：渠道的 Group 里有分组在 abilities 表没有任何记录时，newGroup2model2channels[group] 是 nil map，赋值会 panic；后果：数据不一致时刷新渠道缓存崩溃；估计 15 分钟（已读代码确认，未实际复现）
- model/ability.go:62 getPriority、model/ability.go:92 getChannelQuery 全仓库无调用方（改动前已如此）；后果：死代码；估计 5 分钟
