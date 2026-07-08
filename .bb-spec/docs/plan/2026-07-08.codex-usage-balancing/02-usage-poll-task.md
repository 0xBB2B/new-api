---
name: usage-poll-task
description: service 后台任务每 60s 轮询启用 Codex 渠道的 wham usage，解析两窗口 used_percent 写入 model 缓存；仅 master、单渠道失败不阻断。
---

# Codex 用量轮询任务

## 目标

master 节点每 60 秒抓取一轮所有启用 Codex 渠道（type 57）的官方使用率，写入 model 用量缓存。

## 业务规则（来源：spec usage-polling）

- 轮询周期固定 60 秒；仅 master 节点执行，非 master 不发起任何上游用量请求
- 只轮询启用状态（`common.ChannelStatusEnabled`）的 Codex 渠道；禁用渠道不轮询、其缓存条目不再更新（注意：与凭据刷新任务不同，**不含** AutoDisabled）
- 单渠道失败（网络错误、非 2xx、响应不可解析、凭据缺失）只记日志，本轮继续其余渠道；失败渠道不写缓存、保留旧值
- 响应窗口挂在 `rate_limit`（单数）字段下：`rate_limit.primary_window.used_percent`（5h）、`rate_limit.secondary_window.used_percent`（7d）；某窗口或其 used_percent 缺失按 0；两窗口均缺失视为本轮无效、等同失败
- 401 按失败处理，不就地刷新凭据（凭据由既有 10 分钟凭据刷新任务负责）

## 涉及文件

- `service/codex_usage_poll_task.go` — 新建
- `service/codex_usage_poll_task_test.go` — 新建
- `main.go` — 修改：`service.StartCodexCredentialAutoRefreshTask()` 之后追加一行注册

## 函数清单

### service/codex_usage_poll_task.go

整体照抄 `service/codex_credential_refresh_task.go` 的任务范式：`sync.Once` 单次启动、`common.IsMasterNode` 守卫、`gopool.Go` 起协程、`time.NewTicker` + 先跑一次再循环、`atomic.Bool` CAS 防重入、分页裸查（`model.DB.Select(...).Where("type = ? AND status = ?", ...)` 批 200，Select 需含 id/name/key/status/setting/base_url——代理取自 setting 列、上游地址取自 base_url 列）；日志用 `logger.LogXxx(ctx, ...)`。

| 函数名 | 职责 |
|---|---|
| `StartCodexUsagePollTask` | 任务入口：Once + master 守卫 + 60s ticker 循环，每 tick 调 `runCodexUsagePollOnce` |
| `runCodexUsagePollOnce` | 单轮执行：CAS 防重入，分页遍历启用 Codex 渠道，逐个调 `pollCodexChannelUsage`，失败记 warn 日志并继续 |
| `pollCodexChannelUsage` | 单渠道抓取：`parseCodexOAuthKey` 解析 AccessToken/AccountID → `NewProxyHttpClient(ch.GetSetting().Proxy)` → `FetchCodexWhamUsage(ctx, client, ch.GetBaseURL(), ...)` → 非 2xx 返回错误；成功交给解析函数后 `model.CacheSetCodexChannelUsage` |
| `parseCodexWhamUsageWindows` | 从响应 body 提取 (5h, 7d) used_percent：单窗缺失补 0，双窗均缺失返回错误；反序列化走 `common.Unmarshal` |

响应解析用本文件内私有 DTO（仅 `rate_limit.primary_window/secondary_window.used_percent` 三层所需字段，used_percent 用 `*float64` 区分缺失与显式 0）。

## 协作关系

- 复用（不新建）：`parseCodexOAuthKey`（service/codex_credential_refresh.go）、`NewProxyHttpClient`（service/http_client.go）、`FetchCodexWhamUsage`（service/codex_wham_usage.go）、`ch.GetBaseURL()` / `ch.GetSetting().Proxy`（model/channel.go）
- 写入：`model.CacheSetCodexChannelUsage`（01-usage-cache 产物）
- 注册：main.go 后台任务段，紧跟 `StartCodexCredentialAutoRefreshTask()`
- 外部依赖：上游 `GET {baseURL}/backend-api/wham/usage`（经渠道代理设置）

## 验证方式

- [ ] `parseCodexWhamUsageWindows` 表格测试：双窗齐全精确值 / 仅 primary（secondary 补 0）/ 仅 secondary / 双窗缺失返回错误 / used_percent 显式 0 保留
- [ ] `pollCodexChannelUsage` httptest 测试（仿 `claude_credential_refresh_test.go`）：mock 返回 200 + 合法 body → 缓存被写入且值正确；mock 返回 500 → 返回错误、缓存不变；Key 非法 → 返回错误不发请求
- [ ] `go build ./...` 通过，main.go 注册行存在
- [ ] 本地起服务（master）观察日志：60s 一轮、禁用渠道不出现在抓取日志中
