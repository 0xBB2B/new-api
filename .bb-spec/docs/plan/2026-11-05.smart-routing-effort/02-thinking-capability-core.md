---
name: 02-thinking-capability-core
description: 内置模型思考能力表、覆盖合并、就近降档规则，以及给前端查内置能力的接口
---
# 模型思考能力：内置表与收敛规则

## 目标
new-api 能回答「某个模型支持哪几档思考力度」（内置 / 已覆盖 / 未确认），能把任意力度收敛到模型支持的最近一档，前端能查到内置值。

## 业务规则（来源：PRD）
- 02-thinking-capability C1：内置表覆盖 OpenAI（gpt-5 系列、o 系列）、Claude、Gemini、DeepSeek，按模型名匹配；每个模型记「不支持思考」或七档（none / minimal / low / medium / high / xhigh / max）里的哪几档；取值以各厂商官方文档为准，代码里写明来源链接。gpt-4.1 这类非推理模型记「不支持思考」。
- C2：内置表没有、管理员也没覆盖 →「未确认」，按 low / medium / high 处理。
- C3：管理员覆盖优先于内置；可覆盖任何模型。
- 01-tier-effort C7：算出的力度模型不支持时，换成模型支持的档位里最接近的一档（按七档的顺序算距离），上下一样近取更高的那档；模型不支持思考时结果为「不发送」。
- Claude、Gemini 的能力已经在 relaykit 的思考力度换算里有一份（`relaykit/relayconvert/reasoning/claude.go`、`gemini.go`），内置表必须从那里推出，不另写一份，避免两边对不上。
- 前端在「保存更改」之前就要显示草稿覆盖的效果，所以接口只返回内置值，覆盖由前端按草稿合并。

## 涉及文件
- 修改 `relaykit/relayconvert/reasoning/claude.go`、`relaykit/relayconvert/reasoning/gemini.go`
- 新建 `service/smartroute/capability.go`
- 修改 `controller/smart_routing.go`、`router/api-router.go`、`middleware/access_token_routes.go`
- 测试：扩展 `relaykit/relayconvert/reasoning` 下现有的 claude / gemini 测试文件；扩展 `service/smartroute/smartroute_test.go`、`controller/smart_routing_test.go`

## 成品定义
接口契约：

```
GET /api/option/smart_routing/thinking_capabilities?models=gpt-5.6-sol,gpt-4.1,glm-5.1
权限：挂在 optionRoute 组里（RootAuth，和 /api/option/smart_routing/validate 相同）；在 access_token_routes.go 登记为 accessTokenScopeRule("option:read")

200
{
  "success": true,
  "data": {
    "gpt-5.6-sol": {"known": true,  "no_thinking": false, "efforts": ["low","medium","high","xhigh"]},
    "gpt-4.1":     {"known": true,  "no_thinking": true,  "efforts": []},
    "glm-5.1":     {"known": false, "no_thinking": false, "efforts": ["low","medium","high"]}
  }
}
```

`known=false` 表示内置表里没有（未确认），`efforts` 给出按未确认处理的 low / medium / high。不含管理员覆盖。

## 函数清单
### relaykit/relayconvert/reasoning/claude.go、gemini.go
| 函数名 | 职责 |
|---|---|
| `ClaudeSupportedEfforts` | 按现有 Claude 能力表推出某个 Claude 模型支持的七档子集；不是已知 Claude 模型时返回未知 |
| `GeminiSupportedEfforts` | 按现有 Gemini 能力表推出支持的子集或「不支持思考」；不是已知 Gemini 模型时返回未知 |
### service/smartroute/capability.go
| 函数名 | 职责 |
|---|---|
| `ThinkingCapability`（结构体） | 结果：来源（builtin / override / unconfirmed）、是否不支持思考、支持的档位 |
| `BuiltinThinkingCapability` | 查内置：先问 relaykit 的 Claude、Gemini 函数，再查本文件里 OpenAI、DeepSeek 的表（每条带官方文档链接注释），都没有返回未确认 |
| `EffectiveThinkingCapability` | 覆盖表有就用覆盖，否则用内置 |
| `ClampEffortToCapability` | 把一个力度收敛到能力里最接近的一档，一样近取更高；不支持思考返回空；同时返回是否发生了降档 |
### controller/smart_routing.go、router/api-router.go、middleware/access_token_routes.go
| 函数名 | 职责 |
|---|---|
| `GetSmartRouteThinkingCapabilities` | 解析 `models` 参数，逐个返回内置能力 |
| 路由登记 | 在智能路由校验接口旁边挂新路由；访问令牌路由表登记同一路径 |

## 协作关系
`EffectiveThinkingCapability` 读 01 新增的 `GetThinkingCapabilityOverrides`。`ClampEffortToCapability` 给 03 的力度计算用；接口给前端 07、10 用。relaykit 改动后必须在 `relaykit/` 里 `GOWORK=off go build ./...`。

## 验证方式
- 测试入口：`reasoning.ClaudeSupportedEfforts`、`reasoning.GeminiSupportedEfforts`；`smartroute.BuiltinThinkingCapability`、`EffectiveThinkingCapability`、`ClampEffortToCapability`；`GET /api/option/smart_routing/thinking_capabilities`
- 测试输入：Claude 模型（如 claude-opus-5-5、claude-fable-5-1、claude-haiku-4-5）、Gemini 模型（gemini-2.5-pro、gemini-3-pro）、gpt-5.6-sol、o3、gpt-4.1、deepseek 模型、glm-5.1；覆盖表；力度和能力组合
- 预期结果：
  - [ ] 每个内置家族至少一个模型返回的档位和代码注释里链接的官方文档一致；gpt-4.1 → 不支持思考；glm-5.1 → 未确认、low / medium / high
  - [ ] 覆盖表里 glm-5.1 为 low / high → `EffectiveThinkingCapability` 来源 override、档位 low / high
  - [ ] 能力 low / high，输入 medium → high、发生降档；能力 low～high，输入 xhigh → high、发生降档；输入 low → low、没有降档；不支持思考 → 空
  - [ ] 接口：请求三个模型 → 返回结构和成品定义一致；不带 `models` → 返回空对象；普通用户请求 → 403；访问令牌请求 → 200
  - [ ] `cd relaykit && GOWORK=off go build ./...` 通过
