---
name: 13-frontend-log-display
description: 使用日志列表的模型列加路由小字，详情弹窗新增「智能路由」块，主请求和打分记录互相能打开
---
# 日志中的路由信息（前端）

## 目标
用户和管理员在使用日志里能看出每次请求从哪个虚拟模型路由来、落到哪一档、为什么；能从主请求打开它的打分记录，也能从打分记录回到主请求。

## 业务规则（来源：PRD 06-usage-log-routing C1~C8，以及用户确认过的预览页）
- 数据来自日志 `other.smart_routing`（后端 09 的成品）：
  - 主请求：`{ role:'main', virtual_model, tier, scored_tier?, reason, reason_params, labels?, scorer?, cleaned_reasoning? }`；
  - 打分记录：`{ role:'scorer', virtual_model, main_tier, main_model }`。
- 列表模型列（`components/columns/common-logs-columns.tsx:672-693` 的 `ModelCell`），在 `ModelBadge` 下面加一行 xs 小字：
  - 普通主请求 → "<虚拟模型> · <档位>"，`text-muted-foreground`；
  - `tier=advisor` → "<虚拟模型> · 顾问"，`text-warning`；
  - `scorer.status` 为 timeout / error / invalid → "<虚拟模型> · <档位> · 打分失败"，`text-destructive`；
  - 打分记录 → "路由打分 · <虚拟模型>"，`text-muted-foreground`。
- 详情弹窗（`components/dialogs/details-dialog.tsx`）在「请求转换」块下面新增 `DetailSection label='智能路由'`，用 `DetailRow` 按顺序显示：
  - 虚拟模型（mono）；
  - 档位：降档被拦时显示"<档位>（打分结果：<打分档位>）"；打分失败用了默认档时显示"<档位>（默认档）"；
  - 选档原因（见下）；
  - 打分标签：依次是任务类型、要想多深、工作量、出错代价、工作类型、把握、用户反馈，用 " · " 连接，每项都显示中文名和取值；
  - 思考内容：只在 `cleaned_reasoning` > 0 时显示"上一轮用的渠道不同，已清理 X 段"；
  - 打分记录：`scorer.billed=true` 时显示"查看这次打分"链接；打分失败时显示"没有拿到用量，没有扣费"。工具结果续跑没有打分标签和打分记录两行。
- 选档原因的显示（按 `reason` 取文字）：
  - `rule_hard` → 命中的标签，例如"出错代价=高"；
  - `rule_simple` / `rule_normal` / `low_confidence` → 对应规则的中文说明；
  - `downgrade_blocked` → "对话历史 X token，超过降档上限 Y，保留上一轮的<档位>"；
  - `advisor` → "用户连续 X 条消息不满意"；
  - `scorer_timeout` → "打分超时（X 毫秒），使用默认档"，红字；
  - `scorer_error` / `scorer_invalid` → "打分失败，使用默认档" / "打分结果格式不对，使用默认档"，红字；
  - `tool_continuation` → "工具结果续跑，沿用上一轮的档位"。
- 打分记录的详情里，「智能路由」块显示：类型"路由打分"、虚拟模型、属于请求（"<请求 ID>（<档位> · <模型>）"，可点击）。
- 互相打开：点击链接时，用 `usage-logs/api.ts` 的 `getAllLogs({ request_id, 时间范围为本条日志前后各 1 小时 })` 找同请求 ID、role 相反的那条日志，再在当前弹窗之上打开一个新的 `DetailsDialog` 显示它；找不到就提示"没有找到对应的记录"。管理员和普通用户各自走现有的 `/api/log` 和 `/api/log/self`。
- 没有 `smart_routing` 的日志，列表和详情与改动前完全一样。
- 新增文字要有七种翻译，做法同 11。

## 字段与文案补充（后端 09 实际写入的字段）
- `reason_params` 的字段：`downgrade_blocked` → `{ history_tokens, limit, kept_tier, scored_tier }`（原因文字里的档位用 `kept_tier`）；`advisor` → `{ count }`；`scorer_timeout` → `{ timeout_ms }`；`rule_hard` → `{ matched: ["risk" | "task_type" | "reasoning", ...] }`；其他原因为 `{}`。
- `labels` 的字段：`task_type`、`reasoning`、`workload`、`risk`、`domain`、`confidence`（0~1 的数字）、`feedback`。
- `scorer.status`：`ok` / `timeout` / `error` / `invalid` / `skipped`。
- `rule_hard` 的原因文字：按 `matched` 顺序，每项写成"<标签名>=<该标签在 labels 里的取值>"，多项用"、"（英文 ", "）连接，例如"出错代价=高"、"Risk of error=High"。
- 打分标签行：按 任务类型、要想多深、工作量、出错代价、工作类型、把握、用户反馈 的顺序，每项写成"<标签名>：<取值>"（英文 "<name>: <value>"），项之间用 " · " 连接；把握显示两位小数，例如"把握：0.91"。labels 里缺的项跳过。
- 标签名英文键：Task type（任务类型）、Reasoning depth（要想多深）、Workload（工作量）、Risk of error（出错代价）、Work domain（工作类型）、Confidence（把握）、User feedback（用户反馈）；项格式键 `{{name}}: {{value}}`（中文 `{{name}}：{{value}}`）。
- 取值英文键（中文同「标签中文名与取值」表）：chat_qa Chat or simple Q&A / lookup Look up information / rewrite Rewrite or organize / generate Generate or edit content / analyze Analyze or troubleshoot / plan Plan or design；low Low / medium Medium / high High；one_step One step / few_steps A few steps / many_steps Many steps；code Code / finance Finance / operations Operations / data_analysis Data analysis / other Other；satisfied Satisfied / dissatisfied Dissatisfied / neutral Neutral。

## 涉及文件
- web/src/features/usage-logs/types.ts（修改：`LogOtherData` 增加 `smart_routing` 类型）
- web/src/features/usage-logs/components/smart-route-caption.tsx（新建：列表小字）
- web/src/features/usage-logs/components/dialogs/smart-route-section.tsx（新建：详情块和互相打开）
- web/src/features/usage-logs/components/columns/common-logs-columns.tsx（修改：`ModelCell` 引用小字组件）
- web/src/features/usage-logs/components/dialogs/details-dialog.tsx（修改：在「请求转换」块后面渲染详情块）
- web/src/i18n/locales/*.json（修改）
- web/src/features/usage-logs/components/__tests__/detail-preview.test.tsx（修改：补本功能的用例）

## 成品定义
新增翻译键（英文原文 → 简体中文）：

| key | zh |
|---|---|
| Smart routing | 智能路由（11 已加，这里复用） |
| Virtual model | 虚拟模型 |
| Tier | 档位 |
| Tier reason | 选档原因 |
| Score labels | 打分标签 |
| Reasoning content | 思考内容 |
| Score record | 打分记录 |
| View this score | 查看这次打分 |
| No usage returned, not billed | 没有拿到用量，没有扣费 |
| Routing score | 路由打分 |
| Belongs to request | 属于请求 |
| Type | 类型 |
| {{tier}} (scored: {{scored}}) | {{tier}}（打分结果：{{scored}}） |
| {{tier}} (default tier) | {{tier}}（默认档） |
| Scoring failed | 打分失败 |
| The previous turn used another channel. Cleaned {{count}} items | 上一轮用的渠道不同，已清理 {{count}} 段 |
| History is {{tokens}} tokens, over the downgrade limit {{limit}}. Kept the previous {{tier}} | 对话历史 {{tokens}} token，超过降档上限 {{limit}}，保留上一轮的{{tier}} |
| The user was dissatisfied {{count}} messages in a row | 用户连续 {{count}} 条消息不满意 |
| Scoring timed out ({{ms}} ms). Used the default tier | 打分超时（{{ms}} 毫秒），使用默认档 |
| Scoring failed. Used the default tier | 打分失败，使用默认档 |
| Invalid score result. Used the default tier | 打分结果格式不对，使用默认档 |
| Tool result continuation. Kept the tier of this turn | 工具结果续跑，沿用上一轮的档位 |
| Scorer confidence is below the threshold | 打分把握低于可信度门槛 |
| Matched the simple tier rule | 符合简单档条件 |
| No special rule matched | 未命中特殊规则 |
| Matching record not found | 没有找到对应的记录 |

标签中文名与取值：

| 标签 | 取值 |
|---|---|
| 任务类型 | chat_qa 闲聊或简单问答 / lookup 查找资料或信息 / rewrite 改写或整理 / generate 生成或修改内容 / analyze 分析或排查 / plan 规划或设计 |
| 要想多深 | low 低 / medium 中 / high 高 |
| 工作量 | one_step 一步 / few_steps 几步 / many_steps 很多步 |
| 出错代价 | low 低 / high 高 |
| 工作类型 | code 代码 / finance 财务 / operations 运营 / data_analysis 数据分析 / other 其他 |
| 用户反馈 | satisfied 满意 / dissatisfied 不满意 / neutral 中性 |

## 函数清单
### components/smart-route-caption.tsx
| 函数名 | 职责 |
|---|---|
| `SmartRouteCaption` | 按 `other.smart_routing` 渲染列表小字；没有这个字段时什么都不渲染 |

### components/dialogs/smart-route-section.tsx
| 函数名 | 职责 |
|---|---|
| `SmartRouteSection` | 渲染详情里的「智能路由」块（主请求和打分记录两种） |
| `smartRouteReasonText` | 按原因代码和参数生成选档原因文字（返回文字和是否红字） |
| `smartRouteLabelsText` | 生成打分标签那一行 |
| `useRelatedSmartRouteLog` | 按请求 ID 查找对应的另一条日志，供链接点击后打开 |

## 协作关系
- 依赖后端 09 的日志字段，以及现有的 `getAllLogs`、`DetailsDialog`、`DetailSection`、`DetailRow`。

## 验证方式
- 测试入口：`useCommonLogsColumns` 渲染列表（照 `detail-preview.test.tsx` 现有写法），以及直接渲染 `DetailsDialog`，传入带 `smart_routing` 的日志；`vi.spyOn` 模拟 `getAllLogs`。
- 预期结果：
- [ ] 普通 / 顾问 / 打分失败 / 打分记录四种日志 → 小字文字和颜色类名与业务规则一致
- [ ] 降档被拦的主请求 → 档位行为"困难档（打分结果：简单档）"，原因行文字正确
- [ ] 工具结果续跑 → 没有打分标签行和打分记录行
- [ ] 点击"查看这次打分" → 用该请求 ID 调用 getAllLogs，打开打分记录详情；找不到时提示 "Matching record not found"
- [ ] 没有 smart_routing 的日志 → 列表和详情与改动前快照一致
