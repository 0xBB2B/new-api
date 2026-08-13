---
name: subscription-usage-poll-task
description: 轮询任务泛化：60s 基础 tick 按类型间隔调度（Codex 每轮 / Claude 每 3 轮），type switch 分派数据源，Redis 快照 key 更名。
---

# 订阅渠道用量轮询任务泛化

## 目标

后台轮询任务覆盖 Codex 与 Claude 订阅两类渠道，按各自数据源间隔（60s / 180s）刷新瓶颈使用率缓存并经 Redis 快照同步非 master 节点。

## 业务规则（来源：spec polling / codex-usage-source / claude-usage-source）

- 仅 master 节点轮询；非 master 不发起任何上游用量请求。
- Codex 渠道每 60 秒被查询一轮；Claude 订阅渠道每 180 秒被查询一轮；任务启动的首轮两类都执行。
- 只轮询启用状态的订阅渠道；单渠道失败（网络错误、非 2xx、响应不可解析、无有效窗口）仅记日志，保留旧缓存，本轮其余渠道不受影响。
- Codex 瓶颈 = `rate_limit.primary_window/secondary_window` 的 `used_percent` 最大值（窗口出现值缺失按 0，两窗口均缺失为失败）；Claude 瓶颈由数据源解析函数给出。
- 单渠道请求 15 秒超时；渠道配置代理时经代理发起；读渠道 Setting 必须零写副作用（沿用只读解析，不用会回写 DB 的 `ch.GetSetting()`）。
- master 每处理完一批渠道就把全部缓存条目以 JSON 全量快照写入 Redis（key `subscription_channel_usage_snapshot`，TTL 10 分钟）；写失败仅记日志。
- 非 master 且 Redis 可用时每 30 秒读快照整体替换本地缓存，读失败保留旧值；Redis 不可用时启动记一条「用量均衡与触顶保护在该节点不生效」警示日志。

## 涉及文件

- 修改（更名重写）`service/codex_usage_poll_task.go` → `service/subscription_usage_poll_task.go`
- 修改（更名重写）`service/codex_usage_poll_task_test.go` → `service/subscription_usage_poll_task_test.go`
- 修改 `main.go`（两处 Start 调用更名）

## 函数清单

### service/subscription_usage_poll_task.go

| 函数名 | 职责 |
|---|---|
| `StartSubscriptionUsagePollTask` | master 启动 60s 基础 ticker，维护轮次计数，计算当轮到期渠道类型集合后触发单轮执行 |
| `runSubscriptionUsagePollOnce` | 按到期类型集合分批查询启用渠道（`type IN (?)`），批内并发轮询，每批后写 Redis 快照 |
| `pollSubscriptionChannelUsage` | 单渠道入口：构建代理 client 与超时上下文，按渠道类型 switch 分派到对应数据源分支 |
| `pollCodexChannelUsage` | Codex 分支：解析 OAuth key（复用 `parseCodexOAuthKey`）→ `FetchCodexWhamUsage` → 瓶颈解析 → 写缓存 |
| `pollClaudeChannelUsage` | Claude 分支：解析凭据（复用 `parseClaudeCredentialEnvelope`）取 accessToken → `FetchClaudeOAuthUsage` → `parseClaudeOAuthUsageBottleneck` → 写缓存 |
| `parseCodexWhamUsageBottleneck` | 改造原 `parseCodexWhamUsageWindows`：两窗口取最大值返回单值，均缺失报错 |
| `StartSubscriptionUsageSyncTask` | 非 master 启动 30s 同步 ticker；Redis 不可用时记警示日志后不启动 |
| `syncSubscriptionUsageSnapshotOnce` | 读 Redis 快照并调缓存加载整体替换，失败仅记日志 |

调度实现：轮次计数从 0 起，Codex 每轮到期，Claude 在 `轮次 % 3 == 0` 的轮到期（首轮 0 两者都含）。写缓存统一走 `model.CacheSetSubscriptionChannelUsage`。

### main.go

| 位置 | 改动 |
|---|---|
| 现 `service.StartCodexUsagePollTask()` / `StartCodexUsageSyncTask()` 两行 | 更名为 `StartSubscriptionUsagePollTask` / `StartSubscriptionUsageSyncTask` |

## 协作关系

- 依赖 plan 01 的缓存 API 与 `constant.IsSubscriptionChannel`（查询条件的类型集合来源）、plan 02 的 Claude 数据源两函数。
- 外部依赖：主库渠道表（只读）、上游 wham usage 与 oauth usage 接口、Redis（可选）。
- 复用：`NewProxyHttpClient`、`common.RedisSet/RedisGet`、`gopool.Go`。

## 验证方式

- 测试入口：`go test ./service -run SubscriptionUsagePoll`（包内测试注入 `httptest` client 与内存 DB fixture）。
- 测试输入：显式建渠道 fixture（启用/禁用 × type 57/61/普通类型）、可编程假上游（成功/500/坏 JSON/无窗口）、轮次序列。
- 预期结果：
  - 单轮执行只查询到期类型的启用渠道：Codex 轮不含 Claude 渠道，第 3 轮两类都含；禁用与非订阅类型渠道从不被请求。
  - Codex 响应 (primary=42.5, secondary=80.1) 写缓存瓶颈 80.1；仅 primary=30 写 30；两窗口缺失不写缓存。
  - Claude 渠道成功响应后缓存瓶颈值与解析结果一致；401 不写缓存且保留旧值。
  - 某渠道 500 时同轮其余渠道正常刷新。
  - master 每批后 Redis 中出现 key `subscription_channel_usage_snapshot` 且内容可被加载函数往返；写失败轮次正常结束。
  - 非 master 同步：快照存在则本地缓存被整体替换，读失败保留旧值。
- [ ] 上述用例全绿（testify，表驱动 + t.Run）
- [ ] `go build ./...` 通过，仓库无 `StartCodexUsagePollTask`/`codex_usage` 旧符号残留
