---
name: 03-smartroute-single-source
description: 智能路由思考能力改为先查模型记录再走 Claude/Gemini 推算，删除内置表与管理员覆盖设置及其接口校验
---
# 智能路由思考能力只留一处

## 目标
智能路由和 `/v1/models` 用同一个取值顺序拿思考能力；代码运行时只查模型记录和推算规则，不读「模型思考能力」卡片的覆盖设置。

## 业务规则（来源：PRD 04-single-source-and-migration）
- 取值顺序：① 命中该名字的模型记录里配置的思考等级（完全匹配 → 前缀 → 后缀 → 包含，整条记录生效）；② 记录没配或没命中 → 2026 年 Claude、Gemini 3.x 推算规则（现有逻辑保留）；③ 都没有 → 能力未确认，按 low、medium、high 处理。
- 档位换档（`ClampEffortToCapability`）、打分模型力度、虚拟模型编辑弹窗查询的能力，都走这个顺序。
- 在模型管理页改了思考等级后，下一个请求就用新值，不用重启。
- 智能路由设置页删掉「模型思考能力」卡片，读写覆盖数据的设置项、校验、默认值一并删掉。
- 已确认不支持思考时，档位决策为丢弃思考参数（现有 `NoThinking` 行为不变）。

## 涉及文件
- 修改 `service/smartroute/capability.go`（删 `builtinThinkingCapabilities` 表与相关注释出处；`ThinkingSource*` 改为 记录 / 推算 / 未确认 三种；`EffectiveThinkingCapability` 改为按上面顺序取值）
- 删除 `setting/operation_setting/smart_routing_capability_setting.go`（覆盖设置、`GetThinkingCapabilityOverrides`、`SmartThinkingCapabilitiesOptionKey`、`ThinkingCapabilityOverride`）——搬迁需要的选项键名由 04 自带常量
- 修改 `model/option.go`（约 :728 删除该选项的加载）
- 修改 `model/request_policy.go`（约 :52、:64 删除该选项的默认值）
- 修改 `controller/smart_routing.go`（删 `validateThinkingCapabilitiesOption`；`GetSmartRouteThinkingCapabilities` 改为返回有效能力，`known` 表示来源不是未确认）
- 修改 `controller/option.go`（约 :233 删除该选项的校验分支）
- 修改 `controller/request_policy.go`（约 :30 删除该选项的校验）
- 修改测试：`service/smartroute/smartroute_test.go`（:1864-1990 内置表用例改为模型记录驱动）、`controller/smart_routing_test.go`（:255-320）、`middleware/distributor_test.go`（:1148）、`router/access_token_scope_test.go`（:212-230 如涉及该接口）

## 函数清单
### service/smartroute/capability.go
| 函数名 | 职责 |
|---|---|
| `EffectiveThinkingCapability`（现有，改） | 先调 `model.GetModelCapabilityByName` 取记录上的思考配置；未配置时调推算函数；都没有返回能力未确认 |
| `inferredThinkingCapability`（由现有 `BuiltinThinkingCapability` 改名、去掉查表段） | 只保留 2026 Claude 前缀与 Gemini 3.x 推算，认不出返回未确认 |
| `ClampEffortToCapability`（现有，不改） | 按能力收敛档位 |

### controller/smart_routing.go
| 函数名 | 职责 |
|---|---|
| `GetSmartRouteThinkingCapabilities`（现有，改） | 对 `?models=` 里每个名字返回 `{known, no_thinking, efforts}`，数据来自 `EffectiveThinkingCapability`；`known` = 来源不是未确认 |

## 协作关系
- `service/smartroute` 已经 import `model`（`validate.go:48,72`），`model` 不 import smartroute，无循环。
- 调用方 `middleware/smart_route.go:198`、`relay/smart_route_scorer.go:64` 不改调用方式。
- 02 的 `buildOpenAIModel` 调 `EffectiveThinkingCapability`，按来源决定是否输出 `reasoning`。
- 接口路径 `/api/option/smart_routing/thinking_capabilities` 保留（前端编辑弹窗仍用它判断徽标），返回含义改为有效能力。
- 删除选项后，前端写入该选项的入口由 08 删除；后端收到旧键的写入按现有"未知选项"处理。

## 验证方式
- 测试入口：`smartroute.EffectiveThinkingCapability`、`smartroute.ClampEffortToCapability`、`GET /api/option/smart_routing/thinking_capabilities?models=...`、智能路由中间件请求（沿用 `middleware/distributor_test.go` 夹具）。
- 测试输入：模型记录 `gpt-6.2-sol`（none~max）、前缀记录 `qwen`（low/high）、`gpt-4o` 记录不支持思考、无记录的 `claude-opus-5`、`gemini-3.1-pro`、`foo-model`。
- 预期结果：
- [ ] `gpt-6.2-sol` 来源为记录、档位 none~max
- [ ] `qwen4-max` 命中前缀记录，档位 low/high
- [ ] `gpt-4o` 不支持思考，档位决策丢弃
- [ ] `claude-opus-5`、`gemini-3.1-pro` 来源为推算，档位与改动前一致
- [ ] `foo-model` 能力未确认，`xhigh` 收敛为 `high`
- [ ] 修改记录思考等级后不重启，下一次 `EffectiveThinkingCapability` 返回新值
- [ ] 接口对上述名字返回的 `known` / `no_thinking` / `efforts` 与上面一致
- [ ] 代码中搜不到 `builtinThinkingCapabilities`、`GetThinkingCapabilityOverrides`
- [ ] `go test ./service/smartroute/ ./controller/ ./middleware/ ./router/` 通过
