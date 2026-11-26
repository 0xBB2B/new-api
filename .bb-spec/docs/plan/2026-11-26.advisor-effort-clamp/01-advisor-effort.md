---
name: advisor-effort
description: 顾问接手的一轮按顾问模型的思考能力给客户端力度换档，模型不支持思考时去掉思考参数
---
# 顾问这一轮的力度换档
## 目标
智能路由把一轮交给顾问时，发给上游的思考力度一定是顾问模型支持的档位，模型不支持思考时不发思考参数。
## 业务规则（来源：PRD 2026-11-26.advisor-effort-clamp / 01-advisor-effort）
- 顾问这一轮以客户端带的思考力度为准：模型支持 → 照原值发；模型支持思考但不支持这一档 → 换成模型支持档位里最接近的一档，上下一样近取更高的一档；模型不支持思考 → 去掉思考力度参数。
- 客户端没带力度 → 不发，日志来源为 client_unset，详情写「未发送（客户端未指定）」。
- 客户端力度不在 none、minimal、low、medium、high、xhigh、max 七档里 → 照原值发，不换档。
- 模型能力未确认（模型记录没登记、推算也认不出）→ 按只支持 low、medium、high 换档。
- 顾问那一轮里的用户新消息和工具结果续跑都按上述规则。
- 日志：顾问这一轮的来源记为 client（与「不覆盖」档位相同）；换档时记原力度（from），详情显示「low（不覆盖，由 none 升档）」；去掉思考参数时来源 no_thinking，详情「未发送（模型不支持思考）」。
- 三个普通档位、打分模型、顾问不可用退回档位的处理不变。
- 这条规则取代旧 PRD 2026-11-05.smart-routing-effort 01 C9「顾问模型不支持也不改写」。
## 涉及文件
- 修改 `middleware/smart_route.go`
- 修改 `middleware/distributor_test.go`（测试）
## 函数清单
### middleware/smart_route.go
| 函数名 | 职责 |
|---|---|
| 智能路由分发里决定思考力度的那一段（约 177-199 行 `finalTier == smartroute.RouteAdvisor` 分支） | 顾问分支不再手写 EffortDecision，改为走与「不覆盖」档位相同的换档：客户端力度 + 顾问模型的 `smartroute.EffectiveThinkingCapability(realModel)`，复用 `service/smartroute/effort.go` 末尾已有的换档逻辑（NoThinking 丢弃、`ClampEffortToCapability`）。用最少改动：优先调用 `smartroute.ResolveTierEffort`，传一个效果等同「不覆盖」的档位输入（若档位结构里"不覆盖"就是空的力度方式，直接构造即可）；不新增导出函数，除非现有入口无法表达 |
## 协作关系
`middleware/smart_route.go` → `service/smartroute.ResolveTierEffort` / `EffectiveThinkingCapability` → `ClampEffortToCapability`；改写请求体的 `rewriteSmartRouteBody` 已按 EffortDecision（Value / Drop / Source）工作，不需改动。日志由 `service/smartroute/log.go` 按 EffortDecision 写入，不需改动。
## 验证方式
- 测试入口：经智能路由分发中间件的请求（`middleware/distributor_test.go` 现有夹具 `setupSmartEffortRoute`、`smartEffortResponsesBody` 等，以及交给顾问的会话夹具），观察上游收到的请求体里的思考力度和路由日志里的 effort 记录。
- 测试输入 → 预期结果：
  - [ ] 顾问模型能力 low..max，客户端 none（Anthropic `thinking.type=disabled`，或 Responses `reasoning.effort=none`）→ 上游力度 low，日志 from=none、source=client
  - [ ] 同上，客户端 high → 上游 high，日志无 from
  - [ ] 同上，客户端 minimal → 上游 low
  - [ ] 顾问模型只支持 low、medium、high，客户端 max → 上游 high
  - [ ] 顾问模型不支持思考，客户端 medium → 上游请求无思考力度参数，source=no_thinking
  - [ ] 客户端不带力度 → 上游无思考力度参数，source=client_unset
  - [ ] 客户端带 turbo → 上游 turbo
  - [ ] 顾问模型能力未确认，客户端 xhigh → 上游 high
  - [ ] 顾问那一轮的工具结果续跑，客户端 none → 上游换档后的力度
  - [ ] 现有测试 `TestSmartRouteDistributeEffortAdvisorKeepsClientValue` 改为断言新规则（它原来断言顾问不支持也照发）；其余普通档位与顾问退回档位的测试照常通过
