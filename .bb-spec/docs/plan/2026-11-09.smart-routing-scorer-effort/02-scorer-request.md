---
name: 02-scorer-request
description: 打分请求按配置和模型能力发思考力度、去掉输出长度上限，格式不对时写服务器警告日志
---
# 打分请求的思考参数、输出上限与格式不对日志

## 目标
打分调用按管理员选的思考力度（经模型能力换档）发给上游，不再设输出长度上限；打分结果解析失败时，服务器日志能看到原始返回。

## 业务规则（来源：PRD）
- 来源 `01-scorer-effort`：
  - C8：留空 → 不发任何思考参数，Codex 渠道（走 Responses 接口）也一样，去掉现在写死的 `low`；选了档位 → 先按打分模型的思考能力换档（换成支持的档位里最接近的一档，上下一样近取更高的一档，和档位的规则相同），Chat 格式写 `reasoning_effort`，Responses 格式写 `reasoning.effort`；打分模型不支持思考 → 不发思考参数。
  - C9：Chat 格式不带 `max_tokens` / `max_completion_tokens`，Responses 格式不带 `max_output_tokens`。
  - 决策理由：打分模型默认思考时，思考内容把 300 token 上限用光，标签 JSON 被截断。耗时由打分超时（100～30000 毫秒）兜住，超时不扣费；预扣费只按输入 token 估算（`relay/helper/price.go` 的 `preConsumedTokens := promptTokens * multiplier`），不依赖 `max_tokens`；结算按实际用量，计费链路不变。
- 来源 `03-scorer-invalid-log`：
  - C1：解析失败（不是 JSON 或标签取值不在选项里）→ 写一条服务器警告日志，包含虚拟模型名、打分模型名、解析错误、原始返回；原始返回超过 500 个字符（按 Unicode 字符计，不按字节）只取前 500 个；为空时写 `(empty)`。
  - C2：只写服务器日志（`logger.LogWarn` + `common.LogText`，英文文本），不进使用日志，不返回客户端。
  - C3：打分成功、超时、上游报错时不写这条日志。
- 格式不对时现有行为保持：仍按 `scorer_invalid` 走默认档，打分照常结算扣费。

## 涉及文件
- 修改 `relay/smart_route_scorer.go`
- 修改 `relay/smart_route_scorer_test.go`（改写 `TestRunSmartRouteScorer_SendsFixedChatRequest`、`TestRunSmartRouteScorer_CodexChannelGetsResponsesRequest` 的断言，在同一文件补新用例，不新建测试文件）

## 函数清单
### relay/smart_route_scorer.go
| 函数名 | 职责 |
|---|---|
| `runSmartRouteScorer`（修改） | 构造请求前用 `smartroute.ClampEffortToCapability(route.Scorer.Effort, smartroute.EffectiveThinkingCapability(scorerModel))` 算出实际力度；Codex 分支只在力度非空时设 `Reasoning: &dto.Reasoning{Effort: …}`；Chat 分支去掉 `MaxTokens`，力度非空时设 `ReasoningEffort`；`ParseScorerOutput` 出错时在设置 `FailureScorerInvalid` 之前写警告日志 |

## 协作关系
- `smartroute.EffectiveThinkingCapability`（内置表 + 管理员覆盖）与 `smartroute.ClampEffortToCapability`（空串、不支持思考或无可用档位时返回空串）已存在，直接复用，不在 relay 里重写换档逻辑。
- 渠道 adaptor 把 `reasoning_effort` / `reasoning.effort` 转成各家协议，不改 adaptor。
- 日志用 `logger.LogWarn(parent, common.LogText("smart routing scorer for %s returned unparsable output from %s: %s; raw: %s", …))` 这一类英文格式串；不加 `common/log_text.zh-CN.json` 条目（同文件现有打分失败日志也没有中文条目）。

## 验证方式
- 测试入口：`relay` 包内 `runSmartRouteScorer`（现有测试已直接调用，夹具 `setupScorerFixture` / `scorerRoute` / `scorerInspection` 截获上游收到的请求体）。
- 测试输入与预期结果：
  - [ ] Chat 渠道、`Effort` 留空 → 请求体没有 `max_tokens`、`max_completion_tokens`、`reasoning_effort`；仍有 `temperature: 0`、`stream: false` 和两条消息。
  - [ ] Chat 渠道、打分模型在能力覆盖里支持 none/low/high/max、`Effort="none"` → `reasoning_effort` 为 `none`。
  - [ ] 打分模型能力覆盖为只支持 none、low，`Effort="high"` → `reasoning_effort` 为 `low`。
  - [ ] 打分模型能力覆盖为不支持思考、`Effort="low"` → 没有 `reasoning_effort`。
  - [ ] Codex 渠道、`Effort` 留空 → Responses 请求体没有 `reasoning` 字段、没有 `max_output_tokens`；`Effort="low"`（模型支持）→ `reasoning.effort` 为 `low`。
  - [ ] 上游返回非 JSON 文字 → 结果 `Failure` 为 `scorer_invalid`，并写出一条包含路由名、打分模型名和原始文字的警告日志；返回 2000 个中文字符 → 日志里的原始返回正好是前 500 个字符；返回空内容 → 日志写 `(empty)`。日志可通过替换 gin 默认 writer 或项目日志输出夹具捕获。
  - [ ] 上游返回合法标签、超时、上游报错 → 不写这条警告。
- [ ] `go test ./relay/ -run SmartRouteScorer` 通过。
