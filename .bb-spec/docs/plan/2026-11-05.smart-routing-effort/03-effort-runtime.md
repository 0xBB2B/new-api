---
name: 03-effort-runtime
description: 请求时按力度方式算出思考力度、按模型能力收敛、改写三种格式的请求体，并记进会话和日志字段
---
# 请求时的思考力度

## 目标
每次虚拟模型请求按档位的力度方式算出要发的思考力度，收敛到模型能力后写进上游请求；续跑沿用本轮的力度；日志字段记下实际值、来源、是否降档。

## 业务规则（来源：PRD）
- 01-tier-effort C2：不覆盖 → 用客户端带的值；没带就不加。C3：固定 → 用固定值。
- C4：跟随打分 → 用户新消息按「要想多深」低→low、中→medium、高→high，再限制在最低～最高之间。
- C5：跟随打分时打分失败（超时、报错、格式不对）→ 用最高值；打分把握不足而用默认档时「要想多深」照样有值，按 C4 算。
- C6：工具结果续跑、Responses 压缩请求不打分 → 沿用会话里本轮用户新消息最终发出的力度；会话没有记录时，跟随打分用最高值，固定用固定值，不覆盖用客户端的值。续跑落到的档位和会话里记的档位不同（如档位没渠道换成默认档）时，按新档位的方式重新算，没有打分结果按「会话没有记录」处理。
- C7：算出的力度按「有效思考能力」收敛到最接近的一档（一样近取更高）。「不覆盖」时客户端的值也收敛；客户端带的值不是七档之一（如大写、拼错）时先转小写再判断，仍不认识就原样保留、不改写，source 记 client。
- C8：档位模型不支持思考 → 不论哪种方式都去掉请求里的思考参数：Responses 的 `reasoning.effort`、Chat Completions 的 `reasoning_effort` 和 `reasoning`、Anthropic 的 `thinking` 和 `output_config.effort`。
- C9：顾问那一轮用客户端带的值，不收敛不改写。
- C11：三种格式都按上面处理。Anthropic 格式写力度的方式：none → `thinking` 设成 `{"type":"disabled"}` 并去掉 `output_config.effort`；其他值 → `output_config.effort` 设成这个值，并去掉 `thinking.budget_tokens`（否则客户端的预算会盖过档位力度），`thinking.type` 为 disabled 时去掉整个 `thinking`。Responses 写 `reasoning.effort`；Chat 写 `reasoning_effort` 并删 `reasoning`（一期做法）。
- 06-log-effort C1：日志要能写出「<力度>（跟随打分 / 固定 / 不覆盖，客户端传入）」「，由 <原力度> 降档」「未发送（模型不支持思考）」「未发送（客户端未指定）」，所以要记实际值、来源、降档前的值。

## 涉及文件
- 新建 `service/smartroute/effort.go`
- 修改 `service/smartroute/session.go`、`service/smartroute/result.go`、`service/smartroute/log.go`
- 修改 `middleware/smart_route.go`
- 测试：扩展 `service/smartroute/smartroute_test.go`、`middleware/distributor_test.go`

## 成品定义
主请求日志 `smart_routing` 字段新增 `effort` 对象（打分记录不写）：

```json
"effort": {
  "value": "high",
  "source": "follow",
  "from": "xhigh"
}
```

- `value`：实际发给上游的力度；不发送时为空字符串。
- `source`：`follow`（跟随打分）、`fixed`（固定）、`client`（不覆盖或顾问，客户端带了值）、`client_unset`（不覆盖，客户端没带）、`no_thinking`（模型不支持思考，已去掉参数）。
- `from`：发出前按模型能力换了档位（升或降）时为换档前的值，否则省略。

## 函数清单
### service/smartroute/effort.go
| 函数名 | 职责 |
|---|---|
| `EffortDecision`（结构体） | 结果：要发的值、来源、降档前的值、是否去掉思考参数 |
| `ScoredEffort` | 「要想多深」标签 → low / medium / high |
| `ResolveTierEffort` | 输入档位配置、本次是否用户新消息、打分标签（可能为空）、打分是否失败、客户端带的值、会话里记的力度和档位、模型有效能力，按 C2~C8 得出 `EffortDecision` |
### service/smartroute/session.go
| 函数名 | 职责 |
|---|---|
| `SessionState`（结构体） | 增加本轮用户新消息最终发出的力度及其来源 |
| `SaveSessionAfterSuccess` | 用户新消息成功后一并保存本轮力度；续跑不改它 |
### service/smartroute/result.go、log.go
| 函数名 | 职责 |
|---|---|
| `RouteResult`（结构体） | 增加 `Effort EffortDecision` |
| `MainLogFields` | 按成品定义写 `effort` 对象 |
### middleware/smart_route.go
| 函数名 | 职责 |
|---|---|
| `applySmartRoute` | 选定最终档位后调用 `ResolveTierEffort`（顾问时直接记客户端的值），结果放进 `RouteResult` |
| `rewriteSmartRouteBody` | 按 `EffortDecision` 和请求格式改写或去掉思考参数，三种格式分别处理 |
| `clientEffort` | 从三种格式的请求体里读出客户端带的力度（Responses `reasoning.effort`、Chat `reasoning_effort`、Anthropic `output_config.effort`，Anthropic `thinking.type=disabled` 视为 none） |
| `saveSmartRouteSession` | 把本次力度交给 `SaveSessionAfterSuccess` |

## 协作关系
`ResolveTierEffort` 调 02 的 `EffectiveThinkingCapability` 和 `ClampEffortToCapability`，读 01 的档位字段与 `SmartEffortRank`。Anthropic 格式请求在 04 接进 `applySmartRoute`；本 plan 先让 `rewriteSmartRouteBody` 支持 `anthropic` 格式，用直接构造的请求体测试。

## 验证方式
- 测试入口：`smartroute.ResolveTierEffort`、`smartroute.MainLogFields`；`middleware.Distribute` 处理虚拟模型请求后读改写过的请求体（`distributor_test.go` 现有的智能路由用例写法）
- 测试输入：三种力度方式 × 打分成功 / 失败 / 续跑 / 会话有无记录 / 顾问；能力表（含覆盖、不支持思考）；Responses、Chat、Anthropic 三种请求体
- 预期结果：
  - [ ] 跟随打分 medium～xhigh、能力 low～xhigh：要想多深 高 / 中 / 低 → high / medium / medium，source=follow
  - [ ] 跟随打分 medium～xhigh，打分超时 → xhigh；能力被覆盖成只到 high → value=high、from=xhigh
  - [ ] 把握不足、要想多深=低 → medium
  - [ ] 固定 medium，客户端带 high → medium，source=fixed
  - [ ] 不覆盖，客户端带 xhigh，能力最高 high → value=high、source=client、from=xhigh；客户端没带 → 请求里没有力度，source=client_unset
  - [ ] 能力只有 low、high，算出 medium → high
  - [ ] 不支持思考的模型：三种格式请求体里的思考参数全部去掉，source=no_thinking
  - [ ] 一轮发了 high 后同一会话的工具结果续跑、压缩请求 → high；新会话第一条就是续跑、跟随打分 medium～xhigh → xhigh
  - [ ] 顾问一轮，客户端带 low → 上游 low，即使顾问模型能力不含 low；source=client
  - [ ] Anthropic 请求体：力度 high → `output_config.effort=high`、没有 `thinking.budget_tokens`；力度 none → `thinking.type=disabled`、没有 `output_config.effort`
  - [ ] 日志字段 `effort` 和成品定义一致；没有降档时不写 `from`
