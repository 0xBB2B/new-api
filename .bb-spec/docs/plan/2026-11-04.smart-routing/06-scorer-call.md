---
name: 06-scorer-call
description: 内置打分提示词；服务端内部调用打分模型，走现有预扣费、结算和消费日志，拿不到用量不扣费
---
# 打分调用与计费

## 目标
对每条用户新消息，new-api 在服务端调用一次打分模型，拿到标签；这次调用按打分模型价格单独扣用户额度、单独记一条消费日志，请求 ID 与主请求相同。

## 业务规则（来源：PRD 02-scoring C2、C5~C10）
- 打分调用的分组：密钥分组是 auto → 按打分模型配置的分组顺序尝试，跳过用户无权使用的分组（`service.IsUserSelectableGroup`）；不是 auto → 只用密钥自己的分组。找不到可用渠道 → 打分失败（`scorer_error`）。
- 打分输出长度固定上限 300 token（`max_tokens=300`），temperature 0，非流式，不受用户请求里任何参数影响。
- 超时取配置的 `timeout_ms`，超时 → `scorer_timeout`；上游报错 → `scorer_error`；返回内容解析失败 → `scorer_invalid`（解析用 04 的 `ParseScorerOutput`）。
- 计费（必须遵守 `.agents/rules/billing.md`）：
  - 先预扣费。用户余额不够 → 返回"余额不足"，07 据此拒绝整个请求，不调用打分模型，也不调用主请求；
  - 拿到上游用量 → 结算：按"打分模型价格 × 实际分组倍率"扣费，写一条消费日志；
  - 没拿到用量（超时、报错）→ 退回预扣，不写消费日志；
  - 金额换算只用 `common/quota_math.go` 里的函数，不新增本地换算。
- 打分消费日志的 `other` 里写 09 成品中的 scorer 标记，其中包括主请求选中的档位和模型。打分调用返回时还没选档，所以执行函数先完成预扣费和上游调用，返回标签和一个"结算"函数。07 选完档后，调用结算函数并传入档位和模型，结算函数再扣费、写日志。拿不到用量时，执行函数直接退款，不返回结算函数。

## 涉及文件
- service/smartroute/scorer.go（新建：提示词、构造打分请求、执行函数变量）
- relay/smart_route_scorer.go（新建：真正执行打分调用和计费；在包 init 里注册到 `smartroute.ScorerRunner`）

## 成品定义
打分请求按 Chat Completions 格式构造（各渠道的适配器负责转成自己的格式）：`model` 为打分模型；`messages` 依次为下面这段 system 提示词和一条 user 消息；`max_tokens: 300`；`temperature: 0`；`stream: false`。

system 提示词（原样内置，不对外开放修改）：

```text
You are a router that labels a user's latest request so a gateway can pick a model tier. You never answer the request itself.

Read the user messages (oldest first; earlier messages show only their beginning) and the local features, then return exactly one JSON object and nothing else:

{"task_type":"","reasoning":"","workload":"","risk":"","domain":"","confidence":0,"feedback":[]}

Field rules:
- task_type, judged on the LATEST message only, one of:
  chat_qa  = small talk or a simple factual question
  lookup   = find information, files, code locations or facts on the web
  rewrite  = translate, summarize, reformat or polish given text
  generate = write or change content such as code, reports, documents or copy
  analyze  = analyze data, investigate a failure, research and draw a conclusion
  plan     = design a solution, architecture, strategy or multi-part plan
- reasoning: low | medium | high. How much multi-step thinking, calculation or trade-off weighing the latest request needs.
- workload: one_step | few_steps | many_steps. How many actions (reading files, searching, running tools, fetching data) it needs.
- risk: low | high. high when the result will be published externally, used for decisions or bookkeeping, or touches production data, billing or security, and a mistake would be hard to notice.
- domain: code | finance | operations | data_analysis | other.
- confidence: a number from 0 to 1 for how sure you are about these labels.
- feedback: one value per user message shown, in the same order, each satisfied | dissatisfied | neutral. Use dissatisfied only when that message says the previous answer was wrong, did not work or missed the point.

Return only the JSON object.
```

user 消息模板（`{...}` 由 03 的 `BuildScorerInput` 结果填入；每条用户消息一行，最后一条标 `[latest]`）：

```text
Local features: latest_message_chars={chars}, user_turns={turns}, has_attachment={true|false}, has_code_or_table={true|false}
User messages (oldest first):
[1] {earlier message, first 200 chars}
[2] {earlier message, first 200 chars}
[latest] {latest message, up to 4000 chars}
```

## 函数清单
### service/smartroute/scorer.go
| 函数名 | 职责 |
|---|---|
| `ScorerRunner` | 包级函数变量，由 relay 包注册；07 通过 `RunScorer` 间接调用 |
| `RunScorer` | 检查 `ScorerRunner` 已注册后调用；未注册时按 `scorer_error` 返回 |
| `BuildScorerMessages` | 生成上面成品里的 system 和 user 两条消息 |
| `ScorerGroups` | 按密钥分组是否为 auto，算出打分要尝试的分组列表（已做权限过滤） |

### relay/smart_route_scorer.go
| 函数名 | 职责 |
|---|---|
| `init` | 把 `runSmartRouteScorer` 赋给 `smartroute.ScorerRunner` |
| `runSmartRouteScorer` | 在复制出的 gin context 上完成一次打分：选渠道 → 预扣费 → 调上游（带超时）→ 取用量。拿到用量就返回标签（或格式不对的原因）和结算函数；拿不到就退款并返回失败原因；余额不足时返回专门的错误 |
| `settleSmartRouteScorer` | 结算函数的实现：接收主请求的档位和模型，写进打分日志的 scorer 标记，调用 `service.PostTextConsumeQuota` 扣费并写日志 |

`runSmartRouteScorer` 的流程照 `controller/channel-test.go:73` 的 `testChannel` 构造请求和适配器调用；预扣费、退款、结算复用 `relay/request_billing.go` 的 `PrepareRequestBilling` / `RefundFailedRequestBilling` 和 `service.PostTextConsumeQuota`。复制 context 时保留父 context 的用户、令牌、分组、`common.RequestIdKey`，换掉 Request 和请求体缓存（`common.KeyBodyStorage`），保证父请求的请求体不受影响。

## 协作关系
- 07 → `smartroute.RunScorer` → `ScorerRunner`（relay）→ 适配器 `DoRequest` / `DoResponse` → `PostTextConsumeQuota` → `model.RecordConsumeLog`。
- 打分日志里的主请求档位和模型：07 选完档后，通过调用结算函数传入。
- 返回内容"格式不对"时也已经产生了用量，照样返回结算函数，由 07 调用扣费。

## 验证方式
- 测试入口：`smartroute.BuildScorerMessages`、`smartroute.ScorerGroups`（service/smartroute/smartroute_test.go）；`relay` 包里对 `runSmartRouteScorer` 的测试，用 httptest 假上游返回固定 JSON，写在 relay 包现有的最相近测试文件里，没有就新建 `relay/smart_route_scorer_test.go`。
- 测试输入：用户余额充足 / 不足；假上游返回合法标签 JSON、非 JSON、500、超过超时才响应。
- 预期结果：
- [ ] system 消息与成品一字不差；user 消息含本地特征行，最后一行以 `[latest]` 开头
- [ ] 打分请求的 max_tokens 为 300、temperature 为 0、stream 为 false
- [ ] 合法 JSON → 返回标签；有一条消费日志，model_name 为打分模型，request_id 与父 context 相同，扣费 = 价格 × 分组倍率
- [ ] 非 JSON → `scorer_invalid`；500 → `scorer_error`；超时 → `scorer_timeout`；后两种情况没有消费日志，用户额度不变
- [ ] 余额不足 → 返回余额不足错误，假上游没有收到请求
- [ ] auto 密钥：打分分组顺序为 vip → api，用户无权使用 vip → 用 api；非 auto 密钥只用自己的分组
