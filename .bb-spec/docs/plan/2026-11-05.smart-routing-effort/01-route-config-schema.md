---
name: 01-route-config-schema
description: 虚拟模型配置新增接口列表与三种思考力度方式，新增思考能力覆盖选项，启动时迁移一期配置，保存校验同步
---
# 虚拟模型配置结构与迁移

## 目标
虚拟模型配置能存「管理员勾选的接口」和「不覆盖 / 固定 / 跟随打分」三种力度方式，能存模型思考能力覆盖；一期已存的配置在启动时自动迁移，保存时按新规则校验。

## 业务规则（来源：PRD）
- 03-endpoint-formats C1/C2/C7：接口三选多（`openai-response`、`openai`、`anthropic`），至少一项，新建默认只选 Responses。一期配置没有接口字段，按一期的自动判断结果补上：三档和顾问模型的 `GetModelSupportEndpointTypes` 里 Responses、Chat 两种的交集；交集为空时补 `openai-response`。迁移后不需要管理员重新保存。一期「三档和顾问的模型接口不一致」这项保存检查去掉。
- 01-tier-effort C1/C10：力度方式三选一，「不覆盖」是默认。可选值是 none / minimal / low / medium / high / xhigh / max。「固定」存一个值；「跟随打分」存最低、最高两个值，最低不能高于最高。一期存的 `effort` 为 low / medium / high 的档位迁移成「固定」+ 同样的值；为空的仍是不覆盖。
- 02-thinking-capability C3/C8：管理员覆盖按模型名存，值是「不支持思考」或七档里至少一档；和虚拟模型一起通过「保存更改」保存、即时生效。
- 迁移要幂等：已有接口字段、已经是新力度方式的配置不改；多次启动结果相同。

## 涉及文件
- 修改 `setting/operation_setting/smart_routing_setting.go`
- 新建 `setting/operation_setting/smart_routing_capability_setting.go`
- 修改 `model/request_policy.go`、`model/option.go`、`controller/option.go`、`controller/request_policy.go`、`controller/smart_routing.go`
- 新建 `model/smart_route_migration.go`
- 修改 `main.go`
- 修改 `service/smartroute/validate.go`
- 修改 `middleware/smart_route.go`、`controller/model.go`（只改读取接口的地方）
- 测试：扩展 `service/smartroute/smartroute_test.go`、`controller/smart_routing_test.go`、`controller/model_list_test.go`；新建 `model/smart_route_migration_test.go`

## 成品定义
配置 JSON 的最终形状（`smart_routing_setting.routes` 一项）：

```json
{
  "name": "auto-gpt",
  "endpoints": ["openai-response", "anthropic"],
  "tiers": {
    "simple": {"model": "gpt-5.4-mini", "groups": ["api"], "effort_mode": "", "effort": "", "effort_min": "", "effort_max": ""},
    "normal": {"model": "gpt-5.5", "groups": ["subscription", "api"], "effort_mode": "fixed", "effort": "medium", "effort_min": "", "effort_max": ""},
    "hard":   {"model": "gpt-5.6-sol", "groups": ["subscription", "api"], "effort_mode": "follow", "effort": "", "effort_min": "medium", "effort_max": "xhigh"}
  },
  "default_tier": "normal",
  "scorer": {"model": "gpt-5.6-luna", "groups": ["subscription", "api"], "timeout_ms": 2000},
  "advisor": {"model": "gpt-5.6-sol", "groups": ["subscription", "api"], "after_dissatisfied": 2},
  "downgrade_max_history_tokens": 20000,
  "confidence_threshold": 0.6,
  "simple_max_message_chars": 2000
}
```

`effort_mode`：`""` 不覆盖（此时 `effort`、`effort_min`、`effort_max` 必须为空）；`"fixed"` 只用 `effort`；`"follow"` 只用 `effort_min`、`effort_max`。

思考能力覆盖选项 `smart_routing_setting.thinking_capabilities`：

```json
{
  "glm-5.1": {"no_thinking": false, "efforts": ["low", "high"]},
  "gpt-4.1-custom": {"no_thinking": true, "efforts": []}
}
```

## 函数清单
### setting/operation_setting/smart_routing_setting.go
| 函数名 | 职责 |
|---|---|
| `SmartRouteTier`（结构体） | 增加 `EffortMode`、`EffortMin`、`EffortMax` 字段，`Effort` 只表示固定值 |
| `SmartRoute`（结构体） | 增加 `Endpoints` 字段 |
| `SmartEffortLevels` | 七档力度从低到高的有序列表，供校验、收敛、前端说明共用 |
| `SmartEffortRank` | 返回某档在七档里的位置，不认识返回 -1 |
### setting/operation_setting/smart_routing_capability_setting.go
| 函数名 | 职责 |
|---|---|
| `ThinkingCapabilityOverride`（结构体） | 一个模型的覆盖值：`NoThinking`、`Efforts` |
| `SmartThinkingCapabilitiesOptionKey` | 选项键常量 |
| `LoadThinkingCapabilitiesFromJSONString` | 解析并原子替换覆盖表，解析失败保留旧值并记系统日志（照 `LoadSmartRoutesFromJSONString`） |
| `GetThinkingCapabilityOverrides` | 读当前覆盖表 |
| `ThinkingCapabilitiesToString` | 覆盖表序列化，供默认选项使用 |
### model/smart_route_migration.go
| 函数名 | 职责 |
|---|---|
| `MigrateSmartRouteSettings` | 启动时一次性迁移：为没有 `endpoints` 的虚拟模型按一期规则补接口，为一期 `effort` 有值、`effort_mode` 为空的档位改成 fixed；有改动才写回选项表并重新加载 |
### model/request_policy.go、model/option.go、controller/option.go、controller/request_policy.go、controller/smart_routing.go
| 函数名 | 职责 |
|---|---|
| `requestPolicyDefaultOptions` / `IsRequestPolicyOption` | 登记新选项键 |
| `handleConfigUpdate` | 新选项键走 `LoadThinkingCapabilitiesFromJSONString` |
| `validateThinkingCapabilitiesOption` | 校验覆盖表：模型名非空，`no_thinking` 为 false 时 `efforts` 至少一档且都在七档里 |
| `UpdateOption` / `UpdateRequestPolicy` | 保存前对新选项键调用上面的校验 |
### service/smartroute/validate.go
| 函数名 | 职责 |
|---|---|
| `ValidateRoute` | 接口为空报「至少选择一种接口」、出现三项以外的值报错；按力度方式校验各字段；跟随打分最低高于最高报错；去掉一期接口一致检查和 `modelEndpoints` / `sharedEndpoints` / `RouteEndpoints` / `endpointDisplayNames` |
| `ValidateSmartRoute`（controller） | 返回值去掉 `endpoints` |
### middleware/smart_route.go、controller/model.go（调用方跟随）
| 函数名 | 职责 |
|---|---|
| `applySmartRoute` | 支持的接口改读 `route.Endpoints` |
| `smartRouteEndpointNames` | 增加 `anthropic` → `Anthropic` |
| `buildSmartRouteModels` | `SupportedEndpointTypes` 改读 `route.Endpoints` |

## 协作关系
`main.go` 在 `model.GetPricing()` 之后调用 `model.MigrateSmartRouteSettings()`（要用定价里的接口信息），出错只记系统日志。迁移通过现有的选项更新函数写回，走 `handleConfigUpdate` 重新加载。

## 验证方式
- 测试入口：`smartroute.ValidateRoute`；`POST /api/option/smart_routing/validate`；`PUT /api/option/`（键 `smart_routing_setting.thinking_capabilities`）；`model.MigrateSmartRouteSettings`
- 测试输入：各种 `endpoints` / 力度方式组合的路由；一期格式的路由 JSON（无 `endpoints`，`effort: "high"`）存进选项表；覆盖表 JSON
- 预期结果：
  - [ ] `endpoints` 为空 → 字段 `endpoints` 报「Select at least one endpoint」；含 `gemini` → 报错
  - [ ] `effort_mode=""` 且 `effort="low"` → 报错；`fixed` 且 `effort` 不在七档 → 报错；`follow` 且 `effort_min="high"`、`effort_max="low"` → 报错；七档里任意合法值 → 不报错
  - [ ] 三档模型一个只支持 Responses、一个只支持 Chat → 不再报接口不一致
  - [ ] 迁移：一期配置（三档和顾问都支持 Responses 和 Chat，困难档 effort=high）启动后 → `endpoints` 为 `["openai-response","openai"]`，困难档 `effort_mode="fixed"`、`effort="high"`；再跑一次迁移 → 选项表内容不变
  - [ ] 覆盖表 `no_thinking=false` 且 `efforts=[]` → 保存返回错误；`efforts` 含 `ultra` → 错误；合法值保存后 `GetThinkingCapabilityOverrides` 立即返回新值
  - [ ] auto-gpt `endpoints=["openai-response","anthropic"]` → `/v1/models` 里它的 `supported_endpoint_types` 是这两项；用 Chat Completions 请求它 → 400，信息「Virtual model auto-gpt only supports these endpoints: Responses, Anthropic」
