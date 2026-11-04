---
name: 09-routing-log-fields
description: 主请求和打分请求的消费日志 other 里写入 smart_routing 字段，用户和管理员都能看到
---
# 日志里的路由字段

## 目标
每条经过智能路由的主请求日志，以及每条打分日志，都带一份结构固定的 `smart_routing` 信息，供前端显示和以后统计。

## 业务规则（来源：PRD 06-usage-log-routing C1~C7，02-scoring C8）
- 主请求日志的模型名照旧记真实模型（由 07 保证），另在 `other` 里用 `SetPublic` 写 `smart_routing`。这个字段用户和管理员都能看，不放进 `admin_info`；内容里不能有任何对话文字。
- 打分日志：模型名是打分模型，请求 ID 与主请求相同，`other.smart_routing.role` 为 `scorer`，并记下主请求的档位和模型。
- 工具结果续跑的主请求：`reason` 为 `tool_continuation`，没有 `labels` 和 `scorer`。
- 打分失败（超时、报错）时没有打分日志；主请求的 `scorer.status` 写失败原因，`scorer.billed` 为 false。
- 没有经过智能路由的请求，日志内容完全不变。

## 涉及文件
- service/log_info_generate.go（修改：`GenerateTextOtherInfo` 在 `:111-117` 附近，从路由结果补写 `smart_routing`）
- service/smartroute/log.go（新建：把路由结果转成下面的日志结构）

## 成品定义
主请求日志的 `other.smart_routing`：

```json
{
  "role": "main",
  "virtual_model": "auto-gpt",
  "tier": "hard",
  "scored_tier": "simple",
  "reason": "downgrade_blocked",
  "reason_params": { "history_tokens": 86420, "limit": 20000, "kept_tier": "hard", "scored_tier": "simple" },
  "labels": {
    "task_type": "lookup",
    "reasoning": "low",
    "workload": "one_step",
    "risk": "low",
    "domain": "code",
    "confidence": 0.91,
    "feedback": "satisfied"
  },
  "scorer": { "status": "ok", "billed": true, "timeout_ms": 2000 },
  "cleaned_reasoning": 2
}
```

字段说明：
- `tier`：simple / normal / hard / advisor。
- `scored_tier`：打分得出的档位，只在降档被拦时写。
- `reason`：取值必须和 04 的原因代码一致。
- `reason_params`：
  - `rule_hard` 时写命中的标签，例如 `{ "matched": ["risk"] }`；
  - `advisor` 时写 `{ "count": 2 }`；
  - `scorer_timeout` 时写 `{ "timeout_ms": 2000 }`；
  - 其他原因写空对象。
- `labels.feedback`：最新一条用户消息的反馈。
- `scorer.status`：ok / timeout / error / invalid / skipped，工具结果续跑时为 skipped。
- `cleaned_reasoning`：只在清理过时写。
- 没有的字段省略不写。

打分日志的 `other.smart_routing`：

```json
{
  "role": "scorer",
  "virtual_model": "auto-gpt",
  "main_tier": "hard",
  "main_model": "gpt-5.6-sol"
}
```

## 函数清单
### service/smartroute/log.go
| 函数名 | 职责 |
|---|---|
| `MainLogFields` | 把路由结果转成主请求日志的 smart_routing 结构（省略空字段） |
| `ScorerLogFields` | 生成打分日志的 smart_routing 结构 |

### service/log_info_generate.go
| 函数名 | 职责 |
|---|---|
| `GenerateTextOtherInfo` | 修改：context 里有路由结果时，`other.SetPublic("smart_routing", ...)`；主请求用 `MainLogFields`，打分请求（06 设置的标记）用 `ScorerLogFields` |

## 协作关系
- 读取 07 放进 context 的路由结果和 08 回写的清理条数；06 的打分结算也走 `GenerateTextOtherInfo`，通过 context 标记区分主请求和打分请求。
- 前端 13 按本成品的字段名解析。

## 验证方式
- 测试入口：`smartroute.MainLogFields`、`smartroute.ScorerLogFields`（service/smartroute/smartroute_test.go）；`service.GenerateTextOtherInfo`（扩展 service 包现有的测试文件）。
- 预期结果：
- [ ] 降档被拦的路由结果 → 生成的 JSON 与成品第一段字段一致
- [ ] 工具结果续跑 → 没有 labels，scorer.status 为 skipped
- [ ] 打分超时 → scorer 为 `{ "status": "timeout", "billed": false, "timeout_ms": 2000 }`，reason 为 scorer_timeout
- [ ] 打分日志 → role 为 scorer，带 main_tier 和 main_model
- [ ] context 里没有路由结果 → other 里没有 smart_routing，其余字段和改动前一致
- [ ] smart_routing 写在公开字段里：非管理员查看日志时也能看到（`admin_info` 会被剥离，但它不受影响）
