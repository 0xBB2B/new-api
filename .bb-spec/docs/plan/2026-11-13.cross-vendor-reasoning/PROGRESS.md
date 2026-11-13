# 执行进度
| 序号 | Plan | 状态 | 完成时间 |
|---|---|---|---|
| 01 | 01-reasoning-tag-codec | done | 2026-11-13 |
| 02 | 02-responses-output-tag | done | 2026-11-13 |
| 03 | 03-claude-output-tag | pending | — |
| 04 | 04-request-filter-by-tag | in-progress | — |
| 05 | 05-chat-convert-reasoning | done | 2026-11-13 |
## 当前
01、05、02 已完成。正在执行 04-request-filter-by-tag（Red），之后 03。之后依次 02 → 04 → 03。
## 阻塞
（无）
## 重试与自修（每次记一条）
- 05-chat-convert-reasoning 自修第 1 轮：reasoning→assistant→reasoning→function_call 并入同一条 assistant 时第二段文字被丢（应按 C3 用 \n\n 接上）；补测试含 custom_tool_call、待输出图片附件两条缺口。结果：已修，复查通过
- 02-responses-output-tag 自修第 1 轮：codex 旧用例回归；渠道号为 0 仍打标记；WebSocket 读 lockedChannelID 数据竞争（:364 无锁写）；response.done/response.cancelled 终止事件未改写（同根因）；补 WebSocket、缓冲流式、Claude/Gemini 非流式、OaiChatToResponsesHandler 测试。结果：已修，复查通过；另补 response.canceled（单 l 拼法），DecodeReasoningTag 改严格 base64 解码
## 无关发现（发现一条记一条）
- logger/logger.go:114 与 relay/helper/stream_scanner.go:364：`go test -race ./relay/helper/` 的 StreamScannerHandler 用例报数据竞争，两文件本次未改（审查子代理报告，未复核：跑 `go test -race ./relay/helper/ -run StreamScanner` 验证）；不处理的后果：-race 检测失败，可能是真实并发读写；估计 30 分钟
