# 执行进度
| 序号 | Plan | 状态 | 完成时间 |
|---|---|---|---|
| 01 | 01-channel-pick-order | done | 2026-11-25 |
| 02 | 02-failover-ledger | done | 2026-11-25 |
| 04 | 04-stream-opening-hold | done | 2026-11-25 |
| 03 | 03-sync-retry-loop | done | 2026-11-25 |
| 05 | 05-retry-setting-copy | done | 2026-11-25 |
## 当前
全部 5 个 plan 完成，进入收尾：全量测试、三库验证、界面对比、doc-update。
## 阻塞
（无）
## 重试与自修（每次记一条）
- 01-channel-pick-order 自修第 1 轮：补测试（排除式选取的归一模型名回退与 filters、缓存关闭时轮询位置写回数据库）+ 改实现（内联 channelTierKey、修正 3 处注释、恢复 totalWeight）；结果：通过，复查合规（另收紧过滤用例断言）
- 04-stream-opening-hold 自修第 1 轮：用户选定"最终失败与以前一样"后 PRD 加 C17（经用户同意的 PRD 小改），plan 02/03/04 同步；加"会换渠道"探针，并修审查问题（Claude/Gemini 格式最后一块被判空流、扣留状态残留吞掉后续写出、Claude/Gemini→Responses 错误详情丢失、2 处注释），补 5 条测试缺口；结果：6 条问题已修，复查新发现 2 条 C17 违规
- 04-stream-opening-hold 自修第 2 轮：Chat←Responses 正文前 response.failed 未走 RecordStreamFailure；探针判断的错误与 handler 实际返回的错误不一致（responses_via_chat.go:160、gemini/relay_responses.go:161）；补 2 条测试；结果：通过，复查 11/11 合规
- 02-failover-ledger 自修第 1 轮：锁内回调重入读锁可能死锁、无锁读多密钥状态 map 可能崩溃（同根因修到 model 层 GetUntriedSatisfiedChannel：锁内取快照、锁外判断）；auto 分组起点改按 ContextKeyAutoGroup；合并两层重复测试；补真实首选流程 auto 用例与请求中途禁用用例；删 1 处复述注释；结果：通过，复查 8/8 合规（另把死锁用例改为确定性断言、中途禁用改走 UpdateChannelStatus）
- 03-sync-retry-loop 自修第 1 轮：探针判定前套用状态码映射与违规费归一化；每次重试前检查客户端是否已断开（Relay 与 WS 建连循环）；claude_signature_buffer 并入 ResetStreamHold；补状态码映射、客户端断开、非流式 Chat、向量、图片换渠道用例；结果：通过，复查无 ❌（顺带并入 plan 04 同根因：Claude 格式接 Chat 上游的错误块识别）。Realtime 与 WS 建连取消无法确定性构造测试，未写
## 无关发现（发现一条记一条）
- model/channel_cache.go:57-70 InitChannelCache：渠道的 Group 里有分组在 abilities 表没有任何记录时，newGroup2model2channels[group] 是 nil map，赋值会 panic；后果：数据不一致时刷新渠道缓存崩溃；估计 15 分钟（已读代码确认，未实际复现）
- relay/channel/claude/relay_responses.go:93/102/128/137/148/157 sr.Stop(streamErr) 在 streamErr 为 typed nil 时数据 goroutine panic（改动前就有，审查子代理实测日志可见）；后果：该路径偶发 panic 日志、流异常结束；估计 20 分钟
- web/src/i18n/locales/*.json：`bun run i18n:sync` 会补入约 230 行代码已用但缺失的键、清掉多余键（与本需求无关的漂移，本次已还原只留两个键）；后果：部分界面文字缺翻译；估计 20 分钟（跑同步后人工核对译文）
- service/channel_select.go:180-186 首选渠道时 crossGroupRetry && RetryTimes=0 会提前把 AutoGroupIndex 设成下一组（异步任务路径仍用，同步路径本次已改为不依赖它）；未验证对异步任务的影响
- relay/smart_route_scorer_test.go:1070 与 logger/logger.go:113：go test -race ./relay/ 报 DATA RACE，main 上同样复现（已验证，改动前就有）；后果：-race 跑 relay 包失败；估计 30 分钟
- model/ability.go:62 getPriority、model/ability.go:92 getChannelQuery 全仓库无调用方（改动前已如此）；后果：死代码；估计 5 分钟
