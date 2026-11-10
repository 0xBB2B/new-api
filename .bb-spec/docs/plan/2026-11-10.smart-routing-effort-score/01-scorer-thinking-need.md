---
name: 01-scorer-thinking-need
description: 打分提示词要求返回 effort_score，解析时单独判定有效性，日志 labels 记下它的值或 invalid
---
# 打分返回思考需求

## 目标
打分模型多返回一个 0～1 的 `effort_score`（界面叫「思考需求」）。这个值无效时只把它标成无效，不让整次打分失败。主请求日志的 `labels` 里记下它的值，无效时记 `"invalid"`。

## 业务规则（来源：PRD 01-thinking-need、03-log-thinking-need）
- 打分说明要求模型额外返回 `effort_score`，并写明含义：0～1 的数，表示最新一条请求需要多少思考；目标越模糊、问题越难，数越大；目标越明确、越简单，数越小。
- 有效的判定：值是 JSON 数字，并且在 0～1 之间（含 0 和 1）。以下都算无效：字段缺失、`null`、字符串（包括 `"0.5"` 这种字符串形式的数字）、小于 0、大于 1。
- `effort_score` 无效而其他 7 个字段都合格时，解析成功，返回的标签照常可用（选档、降档保护、转顾问都不受影响），只是思考需求标成无效。
- 其他 7 个字段里任意一个不合格时，和现在一样整次解析失败，不论 `effort_score` 是否有效。
- 选档规则（`SelectTier`、`DecideRoute`）不读 `effort_score`：7 个标签相同时，`effort_score` 取任何值，选中的档位和选档原因都相同。
- 主请求日志的 `labels` 对象里加 `effort_score` 键：有效时写数字原值，无效时写字符串 `"invalid"`。没有 `labels` 的日志（打分失败、工具续跑）不变。
- 决策理由：新字段出错只影响力度，不让走默认档的次数变多（用户 2026-11-10 选定）。

## 涉及文件
- 修改 `service/smartroute/scorer.go`（打分提示词 `scorerSystemPrompt`）
- 修改 `service/smartroute/rules.go`（`ScorerLabels` 加字段；`ParseScorerOutput`）
- 修改 `service/smartroute/log.go`（`MainLogFields`）
- 修改 `service/smartroute/smartroute_test.go`（扩展 `TestParseScorerOutput`、`TestParseScorerOutput_Fields`、`TestMainLogFields`、`TestSelectTier` 或 `TestDecideRoute` 的表格）

## 成品定义
打分提示词的 JSON 模板和字段说明，改成下面这样（只加 `effort_score` 一个字段，其余原文不动）：

```text
{"task_type":"","reasoning":"","effort_score":0,"workload":"","risk":"","domain":"","confidence":0,"feedback":[]}
```

在 `- reasoning: ...` 那一行后面加一行：

```text
- effort_score: a number from 0 to 1 for how much thinking the latest request needs. Higher when the goal is vague or the problem is hard; lower when the goal is clear and the task is simple.
```

`ScorerLabels` 新增字段：

```go
	// EffortScore 为 nil 表示打分没给出有效的 0～1 数字
	EffortScore *float64
```

主请求日志 `labels` 的形状（`other.smart_routing.labels`）：

```json
{"task_type":"analyze","reasoning":"high","effort_score":0.72,"workload":"few_steps","risk":"low","domain":"code","confidence":0.82,"feedback":"neutral"}
```

`effort_score` 无效时：

```json
{"task_type":"analyze","reasoning":"high","effort_score":"invalid","workload":"few_steps","risk":"low","domain":"code","confidence":0.82,"feedback":"neutral"}
```

## 函数清单
### service/smartroute/rules.go
| 函数名 | 职责 |
|---|---|
| `ParseScorerOutput` | 按上面的规则单独判定 `effort_score`：有效时把值放进 `EffortScore`，无效时留 nil；其他字段的校验逻辑不变 |

### service/smartroute/log.go
| 函数名 | 职责 |
|---|---|
| `MainLogFields` | 往 `labels` 里写 `effort_score`：`EffortScore` 非 nil 时写数字，nil 时写 `"invalid"` |

## 协作关系
`relay/smart_route_scorer.go` 调 `ParseScorerOutput` 得到标签，原样交给 `middleware/smart_route.go`；那边再交给 `DecideRoute`（选档）和 `ResolveTierEffort`（力度，由另一份计划改）。日志由 `SetRouteResult` 调 `MainLogFields` 写入。判断 JSON 值类型用 `common.GetJsonType`，解析用 `common.UnmarshalJsonStr`，按 AGENTS.md 的规定不直接调用 `encoding/json`。

## 验证方式
- 测试入口：`go test ./service/smartroute/`，调用 `smartroute.ParseScorerOutput`、`smartroute.MainLogFields`、`smartroute.SelectTier` / `smartroute.DecideRoute`。
- 测试输入：在现有 `validScorerJSON` 基础上加或改 `effort_score`：`0`、`0.5`、`1`、`-0.1`、`1.5`、`"0.5"`、`null`、缺失；另加"`effort_score` 是 0.4 但 workload 是 `lots`"。
- 预期结果：0 / 0.5 / 1 解析成功，`EffortScore` 等于该值；-0.1 / 1.5 / `"0.5"` / `null` / 缺失都解析成功，`EffortScore` 为 nil，其他字段照常；workload 为 `lots` 时返回错误。
- [ ] `MainLogFields`：`EffortScore` 为 0.72 时 `labels.effort_score` 是 0.72；为 nil 时是 `"invalid"`；没有标签时整个 `labels` 不出现。
- [ ] 7 个标签相同、`EffortScore` 分别为 0、1 和 nil 时，`SelectTier` 返回的档位和原因相同。
- [ ] 打分提示词里出现 `effort_score` 字段说明，含 "0 to 1"、"vague"、"hard"、"clear"、"simple"。
- [ ] `go build ./...` 通过；`gofmt` 无差异。
