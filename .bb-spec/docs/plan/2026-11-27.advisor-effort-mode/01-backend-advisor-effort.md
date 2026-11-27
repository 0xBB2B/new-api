---
name: 01-backend-advisor-effort
description: 顾问配置加力度方式字段，保存时校验，顾问这一轮按力度方式定力度、续跑沿用本轮力度
---
# 后端：顾问按力度方式定力度

## 目标
网关给顾问这一轮定思考力度时，按管理员给顾问设的力度方式（不覆盖、固定、跟随打分）计算；保存路由时校验顾问的力度设置。

## 业务规则（来源：PRD 01-advisor-effort-decision）
- 顾问的力度方式三选一，规则和三个档位完全一样：
  - 不覆盖（`effort_mode` 为空）：用客户端的力度；客户端没带力度时不加思考参数。
  - 固定（`fixed`）：用 `effort`。
  - 跟随打分（`follow`）：在 `effort_min`～`effort_max` 之间、顾问模型支持的档位里，取第 `min(int(思考需求 × 档位数), 档位数-1)` 档；打分失败或没有思考需求时取其中最高一档。
- 算出的力度模型不支持时，换成最接近的支持档位，一样近取更高；模型不支持思考时去掉思考参数。三种方式都这样。
- 升级前保存的路由，顾问没有这几个字段，按「不覆盖」处理，结果和升级前一样。
- 顾问这一轮里的工具结果续跑，用这一轮用户新消息定下的力度，不重新计算。这和三个档位的规则一样。
- 保存时，顾问的力度设置按和档位一样的规则校验，错误字段前缀是 `advisor.`：
  - 不覆盖时带了任何力度字段 → 报错；
  - 固定时力度无效 → 报错，带了最低或最高 → 报错；
  - 跟随打分时最低、最高无效 → 报错，最低高于最高 → 报在 `advisor.effort_max` 上，文案 `The minimum effort cannot be higher than the maximum`；带了固定力度 → 报错；
  - 力度方式不是这三种 → 报在 `advisor.effort` 上。
  - 其余错误的文案都是 `Select a valid reasoning effort`。
- 消费日志里顾问这一轮的力度显示沿用档位已有的写法，例如「max（跟随打分）」。不改日志代码。

## 涉及文件
- 修改 `setting/operation_setting/smart_routing_setting.go`
- 修改 `service/smartroute/validate.go`
- 修改 `middleware/smart_route.go`
- 修改 `service/smartroute/smartroute_test.go`（扩展已有的 `TestValidateRoute_*` 表）
- 修改 `middleware/distributor_test.go`（扩展已有的顾问用例）

## 成品定义
配置项 `smart_routing_setting.routes` 中每个虚拟模型的 `advisor` 对象改为下面的结构。四个新字段的含义、取值与 `tiers.<档位>` 下的同名字段完全相同；缺省时都是空串，即「不覆盖」。

```go
type SmartRouteAdvisor struct {
	Model             string   `json:"model"`
	Groups            []string `json:"groups"`
	AfterDissatisfied int      `json:"after_dissatisfied"`
	EffortMode        string   `json:"effort_mode"` // 与 SmartRouteTier.EffortMode 相同
	Effort            string   `json:"effort"`
	EffortMin         string   `json:"effort_min"`
	EffortMax         string   `json:"effort_max"`
}
```

`advisor` 对象 JSON 示例：

```json
{
  "model": "gpt-6.2-astra",
  "groups": ["subscription", "api"],
  "after_dissatisfied": 2,
  "effort_mode": "follow",
  "effort": "",
  "effort_min": "medium",
  "effort_max": "max"
}
```

## 函数清单
### setting/operation_setting/smart_routing_setting.go
| 函数名 | 职责 |
|---|---|
| `SmartRouteAdvisor`（类型） | 按成品定义增加四个力度字段 |

### service/smartroute/validate.go
| 函数名 | 职责 |
|---|---|
| `ValidateRoute`（修改） | 在档位力度校验之后，用顾问的四个力度字段调用 `validateTierEffort`，前缀 `advisor` |
| `validateTierEffort`（不改） | 已有的力度设置校验，顾问复用 |

### middleware/smart_route.go
| 函数名 | 职责 |
|---|---|
| 智能路由分发函数中定 `tierConfig` 与会话力度的分支（修改，约 179–185 行） | 顾问时用顾问的四个力度字段构造 `SmartRouteTier` 作为 `tierConfig`；会话力度的沿用条件 `state.CurrentRoute == finalTier` 对顾问同样生效，去掉顾问的例外 |

## 协作关系
`ValidateRoute` → `validateTierEffort`。分发函数 → `smartroute.ResolveTierEffort`（不改），会把 `tierConfig`、会话力度、思考需求和顾问模型能力传进去 → `rewriteSmartRouteBody`（不改）。`SaveSessionAfterSuccess` 本来就会在顾问这一轮保存力度，不改。

## 验证方式
- 测试入口：`smartroute.ValidateRoute`（包级）；`middleware` 包里已有的智能路由分发测试夹具（`middleware/distributor_test.go`：构造路由 JSON、发请求、读改写后的上游请求体和日志里的智能路由结果）。
- 测试输入：顾问 `t-advisor` 的能力是 low、medium、high、xhigh、max；顾问的力度设置；客户端请求里的力度（`thinking.type=disabled`、`reasoning_effort=xhigh`、不带力度）；打分结果的思考需求（0.9、0.1、打分失败）；用户新消息和工具结果续跑。
- 预期结果：
- [ ] 顾问「跟随打分」medium～max：思考需求 0.9 → 上游力度 max；0.1 → medium；打分失败 → max；日志力度来源是 follow。
- [ ] 同上，max 那一轮之后的工具结果续跑（同一会话）→ 上游力度 max。
- [ ] 顾问「固定」high，客户端 `thinking.type=disabled` → 上游力度 high；「固定」none → 上游力度 low，日志记「由 none 升档」。
- [ ] 顾问「不覆盖」：客户端 `reasoning_effort=xhigh` → xhigh；客户端不带力度 → 上游请求没有思考参数。
- [ ] 顾问 JSON 里没有四个力度字段，客户端 `thinking.type=disabled` → 上游力度 low，日志记「low（不覆盖，由 none 升档）」。这和 PR #58 已有的用例结果一致。
- [ ] 顾问模型不支持思考，且设为「固定」high → 上游请求没有思考参数。
- [ ] `ValidateRoute`：顾问「跟随打分」最低 max、最高 medium → `advisor.effort_max` 报「最低不能高于最高」；「固定」力度为空 → `advisor.effort` 报错；「不覆盖」带 `effort_min` → `advisor.effort_min` 报错；`effort_mode` 为 `auto` → `advisor.effort` 报错；四个字段都为空 → 不报力度错误。
- [ ] `go test ./middleware/ ./service/smartroute/ ./controller/ -count=1` 通过；`gofmt -l` 无输出。
