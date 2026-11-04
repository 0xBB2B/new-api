---
name: 04-tier-rules
description: 解析打分结果、按内置规则选档、数连续不满意次数、判断降档保护，全部为纯函数
---
# 选档规则

## 目标
给定打分结果、配置和会话记录，算出这一轮用哪一档（或顾问），以及原因代码。纯函数，便于逐条测试。

## 业务规则（来源：PRD 02-scoring C3~C5，03-session-tier-switch C5~C7，04-advisor C1~C5）
- 打分结果必须是 06 成品里规定的 JSON：字段齐全，每个取值都在规定选项内，confidence 在 0~1，feedback 条数等于发给打分模型的用户消息条数。任何一项不符 → 格式不对。
- 选档规则按顺序匹配，命中第一条就停：
  1. confidence < 可信度门槛 → 默认档，原因 `low_confidence`；
  2. reasoning = high，或 risk = high，或 task_type = plan → hard，原因 `rule_hard`，同时记下命中的是哪几项；
  3. task_type ∈ {chat_qa, lookup, rewrite}，且 reasoning = low，且 workload = one_step，且最新消息字数 < 简单题长度上限 → simple，原因 `rule_simple`；
  4. 其余 → normal，原因 `rule_normal`。
- 顾问：从最新一条往前数，feedback 连续为 dissatisfied 的条数 ≥ 配置的连续不满意次数 → 这一轮交给顾问，原因 `advisor`（参数：连续条数）。打分失败时不交给顾问。
- 打分失败（超时 / 上游报错 / 格式不对）→ 默认档，原因 `scorer_timeout` / `scorer_error` / `scorer_invalid`。
- 降档保护：档位从低到高为 simple < normal < hard。拿"会话里最近一次不是顾问的档位"来比较：
  - 新档位更高 → 用新档位；
  - 新档位更低，且本次输入 token 估算值 < 降档上限 → 用新档位；
  - 新档位更低，且 token 估算值 ≥ 上限 → 保留原档位，原因 `downgrade_blocked`（参数：token 数、上限、打分得出的档位、保留的档位）；
  - 会话没有记录 → 直接用新档位。

  默认档同样受降档保护约束。顾问那一轮不参与比较，也不改变"最近一次不是顾问的档位"。
- 工具结果续跑：沿用会话里"本轮选定的模型"（可能是顾问），原因 `tool_continuation`；会话没有记录 → 默认档，原因 `tool_continuation`。

## 涉及文件
- service/smartroute/rules.go（新建）

## 函数清单
### service/smartroute/rules.go
| 函数名 | 职责 |
|---|---|
| `ParseScorerOutput` | 把打分模型返回的文字解析成标签结构；允许外面包了 ```json 代码块，除此以外的格式问题一律返回"格式不对" |
| `SelectTier` | 按四条规则算出档位和原因代码（不含顾问和降档保护） |
| `CountTrailingDissatisfied` | 从最新一条往前数连续 dissatisfied 的条数 |
| `ApplyDowngradeGuard` | 按会话记录和 token 估算值决定最终档位，并给出是否被拦 |
| `DecideRoute` | 组合以上函数：输入为打分结果或失败原因、配置、会话记录、token 估算值、是否用户新消息，输出最终选择（档位或顾问）、原因代码和参数、打分得出的档位，以及经过降档保护的备用档位（交给顾问但顾问没有可用渠道时，07 改用它） |

## 协作关系
- 07 在打分（06）完成后调用 `DecideRoute`。
- 标签枚举值、原因代码会写进日志（09），前端（13）按原因代码显示文字，所以原因代码的拼写必须和本文件一致：`low_confidence`、`rule_hard`、`rule_simple`、`rule_normal`、`advisor`、`scorer_timeout`、`scorer_error`、`scorer_invalid`、`downgrade_blocked`、`tool_continuation`。

## 验证方式
- 测试入口：`smartroute.ParseScorerOutput`、`smartroute.SelectTier`、`smartroute.DecideRoute`（表格驱动，写在 service/smartroute/smartroute_test.go）。
- 测试输入与预期：
- [ ] confidence 0.5、门槛 0.6 → 默认档 / low_confidence
- [ ] risk=high → hard；task_type=plan → hard；reasoning=high → hard
- [ ] rewrite + low + one_step + 1500 字（上限 2000）→ simple；同上但 2500 字 → normal；lookup + low + many_steps → normal
- [ ] task_type="poem" → 格式不对；confidence=1.2 → 格式不对；feedback 条数不对 → 格式不对；外面包了 ```json 代码块 → 正常解析
- [ ] N=2，feedback 为 [neutral, dissatisfied, dissatisfied] → 顾问；为 [dissatisfied, neutral, dissatisfied] → 不交给顾问
- [ ] 上一档 hard、打分 simple、token 15000 / 上限 20000 → simple；token 86000 → hard / downgrade_blocked，参数完整
- [ ] 上一档 normal、上一轮是顾问、本次打分 simple、token 86000 → normal（与顾问之前的档位比较）
- [ ] 打分超时、默认档 normal、上一档 hard、token 86000 → hard / downgrade_blocked
- [ ] 工具结果续跑、本轮是顾问 → 顾问 / tool_continuation；没有会话记录 → 默认档
