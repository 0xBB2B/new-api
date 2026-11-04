---
name: 07-distributor-routing
description: 在 Distribute 选渠道前接入智能路由：接口检查、打分选档、改写模型/思考力度/分组顺序、成功后更新会话
---
# 路由接入

## 目标
客户端请求虚拟模型时，在选渠道之前定好这一轮用哪个真实模型，并让后面的选渠道、计费、日志都按真实模型和档位分组进行。

## 业务规则（来源：PRD 01-virtual-model-routing、02-scoring C9、03-session-tier-switch、04-advisor）
- 令牌模型白名单照旧用请求里的模型名（虚拟模型名）检查，档位里的真实模型不再检查。现有检查在 `middleware/distributor.go:57-74`，发生在改写之前，所以不用改。
- 客户端接口（03 的 `RequestEndpointFromPath`）不在虚拟模型支持的接口集合里（02 的 `RouteEndpoints`）→ 返回 400，用面向 AI 客户端的 i18n 消息（见成品），不打分、不扣费。
- 判断是用户新消息还是工具结果续跑（03），读会话记录（05）：
  - 用户新消息 → 打分（06）→ 选档（04 `DecideRoute`）；
  - 工具结果续跑 → 不打分，直接 `DecideRoute`；
  - 打分返回余额不足 → 按现有余额不足的错误格式拒绝，不进入主请求。
- 选定档位（或顾问）之后：
  1. 把 `modelRequest.Model` 改成该档的真实模型。后面 `SetupContextForSelectedChannel` 会把它写进 `ContextKeyOriginalModel`，relay 层的 OriginModelName、发往上游的模型名、计费、日志都会自动跟着变。
  2. 改写缓存的请求体：`model` 字段改成真实模型；档位配了思考力度时，Responses 写 `reasoning.effort`，Chat Completions 写 `reasoning_effort`。顾问不覆盖思考力度。写法照 `middleware/task_plugin_model.go:15` 的 `rewriteTaskPluginJSONModel`（ojson.SetBytes → CreateBodyStorage → 替换 `common.KeyBodyStorage`、清空 `common.KeyRequestBody`、替换 `c.Request.Body`、更新 ContentLength）。透传模式发的是原始请求体，所以必须改请求体，只改 DTO 不够。
  3. 密钥分组是 auto → 用 `common.SetContextKey(c, constant.ContextKeyTokenAutoGroups, 档位分组)` 覆盖本次请求的分组顺序。后面的选渠道、重试、亲和都通过 `service.GetRequestAutoGroups` 读取，会自动再做一次权限过滤。全部被过滤掉 → 返回现有的"没有可用渠道"错误。跨分组重试开关沿用令牌的设置。
  4. 密钥分组不是 auto → 不改分组。用 `model.GetRandomSatisfiedChannel(令牌分组, 档位模型, 0, filters)` 检查：没有可用渠道就改用默认档的模型；默认档也没有，就交给现有逻辑返回"没有可用渠道"。顾问同理：没有渠道就不交给顾问，按打分结果选档。
- 选档完成后，立刻调用 06 打分结果里的结算函数，把主请求的最终档位和模型传进去；结算函数负责扣打分费用、写打分日志。后面的主请求即使失败，这次打分也照样扣费，因为打分已经实际产生了用量。
- 把本次路由结果放进 gin context，给 08（清理思考内容）、09（写日志）读取。内容包括：虚拟模型名、最终档位、打分得出的档位、原因代码和参数、标签、打分状态、会话键、会话记录。
- `c.Next()` 之后、状态码 < 400 时，调用 05 的 `SaveSessionAfterSuccess`，渠道 ID 取最终使用的渠道。位置紧挨现有的 `service.RecordChannelAffinity`（`middleware/distributor.go:137`）。
- 不是虚拟模型的请求，行为完全不变。

## 涉及文件
- middleware/smart_route.go（新建）
- middleware/distributor.go（修改：在 `:100` 选渠道分支之前调用路由函数；`:137` 旁补会话更新）
- i18n/keys.go、i18n/locales/{en,zh-CN,zh-TW}.yaml（或 json，以 `i18n/locales/` 里现有的格式为准）（修改：新增一条面向 AI 客户端的消息）
- middleware/distributor_test.go（修改：补本功能的用例）

## 成品定义
面向 AI 客户端的错误消息：

```text
key:   distributor.smart_route_endpoint_unsupported
en:    Virtual model {{.Model}} only supports these endpoints: {{.Endpoints}}
zh-CN: 虚拟模型 {{.Model}} 只支持这些接口：{{.Endpoints}}
zh-TW: 虛擬模型 {{.Model}} 只支援這些介面：{{.Endpoints}}
```

`Endpoints` 显示名：`openai-response` → `Responses`，`openai` → `Chat Completions`，多个用 `, ` 连接。

## 函数清单
### middleware/smart_route.go
| 函数名 | 职责 |
|---|---|
| `applySmartRoute` | 虚拟模型请求的总入口：接口检查 → 识别请求 → 读会话 → 打分 → 选档 → 改写模型、请求体、分组 → 写入路由结果；不是虚拟模型直接返回 |
| `rewriteSmartRouteBody` | 改写缓存请求体里的 model 和思考力度字段 |
| `smartRouteChannelAvailable` | 非 auto 密钥时，检查档位模型在令牌分组里有没有可用渠道 |
| `saveSmartRouteSession` | 主请求成功后，从 context 取路由结果和最终渠道，调用 `SaveSessionAfterSuccess` |

### service/smartroute/result.go（新建）
| 函数名 | 职责 |
|---|---|
| `SetRouteResult` | 把路由结果写进 gin context |
| `GetRouteResult` | 从 gin context 读路由结果，给 middleware（07）、relay（08）、service（09）共用；放在 service 包是因为 service 不能引用 middleware |

### middleware/distributor.go
| 函数名 | 职责 |
|---|---|
| `Distribute` | 修改：拿到 modelRequest 且白名单检查通过后，调用 `applySmartRoute`；`c.Next()` 之后调用 `saveSmartRouteSession` |

## 协作关系
- `applySmartRoute` → `operation_setting.FindSmartRoute`（01）→ `smartroute.RouteEndpoints`（02）→ `smartroute.InspectRequest` / `SessionKey`（03）→ `smartroute.LoadSession`（05）→ `smartroute.RunScorer`（06）→ `smartroute.DecideRoute`（04）。
- 路由结果的类型定义放在 service/smartroute（避免 middleware ↔ relay 循环引用），context 键名放在 constant/context_key.go。

## 验证方式
- 测试入口：`middleware.Distribute`（照 `middleware/distributor_test.go:205` 的写法挂 RequestId + 预置 context + Distribute + 断言 handler），打分用替身 `smartroute.ScorerRunner`，数据库照 `service/channel_select_auto_groups_test.go` 建 sqlite 内存库和渠道。
- 预期结果：
- [ ] auto 密钥请求 auto-gpt、替身打分为 hard → 断言 handler 里 `ContextKeyOriginalModel` 为困难档模型，`ContextKeyTokenAutoGroups` 为困难档分组；请求体 model 已改写；思考力度为 high
- [ ] 困难档思考力度为空、客户端传 low → 请求体仍为 low；交给顾问时也保持客户端的值
- [ ] Chat Completions 请求一个只支持 Responses 的虚拟模型 → 400，消息为成品里的英文；替身打分没有被调用
- [ ] 非 auto 密钥、困难档模型在令牌分组里没有渠道 → 改用默认档模型
- [ ] 替身打分返回余额不足 → 返回余额不足错误，handler 没有执行
- [ ] 工具结果续跑请求 → 替身打分没有被调用，档位取自会话记录
- [ ] handler 返回 200 后会话记录被写入；返回 500 时不写入
- [ ] 请求普通模型 → 和改动前一样，没有路由结果
