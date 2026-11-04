---
name: 02-route-validation
description: 虚拟模型配置的逐项检查（字段级错误）、单条检查接口、保存请求策略时的整体检查
---
# 虚拟模型配置检查

## 目标
弹窗点「保存」时，后台按字段返回每一处问题；保存请求策略时，再整体检查一遍，不合格就拒绝保存。

## 业务规则（来源：PRD 05-config-page C7、C9，01-virtual-model-routing）
- 检查项和提示文字：
  - 名称为空 → "请填写名称"；
  - 和真实模型重名 → "<名称> 是真实模型名，虚拟模型不能和真实模型重名"；
  - 和其他虚拟模型重名 → "已经有叫 <名称> 的虚拟模型"；
  - 三档、打分模型、顾问的模型没选 → "请选择模型"；
  - 分组顺序为空 → "至少选择一个分组"；
  - 模型在所配的某个分组里没有可用渠道 → "<模型> 在分组 <分组> 里没有可用渠道"；
  - 模型没定价 → "<模型> 还没有设置价格，请先到「模型定价」里设置"；
  - 三档和顾问的模型接口不一致 → "<模型> 只支持 <接口>，这个虚拟模型的其他模型走 <接口>"（打分模型不要求接口一致）；
  - 数字超出范围 → 写明允许的范围。范围是：超时 100~30000 毫秒；连续不满意次数 1~10；降档上限为 ≥ 0 的整数；可信度门槛 0~1；简单题消息长度上限 1~100000。
- "真实模型"指在任意分组里有渠道的模型名（abilities 表），以及配置了价格的模型名。
- 接口推导：取三档模型和顾问模型各自支持的接口类型（`model.GetModelSupportEndpointTypes`），每个都只保留 `openai`（Chat Completions）和 `openai-response`（Responses）两种，再取交集。交集为空就是"接口不一致"，错误挂在第一个和其他模型没有交集的模型字段上。交集就是这个虚拟模型支持的接口集合。
- 档位分组里出现 `auto` 视为无效分组，按"没有可用渠道"处理。
- 弹窗里的「JSON」视图保存时同样检查（PRD 05 C9）。
- 管理后台的文字由前端翻译：后端返回英文原文作为键（`common.NewMessage`），前端 7 个语言文件都要有对应翻译。

## 涉及文件
- service/smartroute/validate.go（新建）
- controller/smart_routing.go（新建）
- controller/option.go（修改：所有会写入 `smart_routing_setting.routes` 的入口在保存前都调用整体检查：`UpdateRequestPolicy`、通用的 `UpdateOption`（在它的 `switch option.Key` 里加一个 case），以及其他调用 `model.UpdateOptionsBulk` 写入这个键的入口（如有）。检查不通过就返回错误、不保存。这样负数等非法值无论走哪个接口都存不进去，运行时代码可以直接信任配置）
- router/api-router.go（修改：在 optionRoute 下新增一条路由）
- middleware/access_token_routes.go（修改：登记新路由）
- service/smartroute/smartroute_test.go（新建，本功能后端唯一新增的测试文件，03~06 的用例也写在这里）

## 成品定义
路由（放在 `router/api-router.go` 的 optionRoute 组里，沿用该组的 Root 鉴权）：

```go
optionRoute.POST("/smart_routing/validate", controller.ValidateSmartRoute)
```

访问令牌登记（`middleware/access_token_routes.go`，紧挨 `/api/option/request_policy` 两行）：

```go
"POST /api/option/smart_routing/validate": accessTokenScopeRule("option:write"),
```

请求体：`{ "route": <SmartRoute JSON>, "original_name": "<编辑前的名称，添加时为空>" }`。

响应（检查通过）：`{ "success": true, "data": { "errors": [], "endpoints": ["openai-response"] } }`

响应（检查不通过，HTTP 仍为 200）：

```json
{
  "success": true,
  "data": {
    "errors": [
      { "field": "tiers.hard.model", "message": { "key": "{{model}} has no available channel in group {{group}}", "params": { "model": "gpt-5.6-sol", "group": "api" } } },
      { "field": "scorer.model", "message": { "key": "{{model}} has no price yet. Set it in Model Pricing first", "params": { "model": "gpt-5.6-luna" } } }
    ],
    "endpoints": []
  }
}
```

`field` 的取值：`name`、`tiers.simple.model`、`tiers.simple.groups`、`tiers.normal.*`、`tiers.hard.*`、`default_tier`、`scorer.model`、`scorer.groups`、`scorer.timeout_ms`、`advisor.model`、`advisor.groups`、`advisor.after_dissatisfied`、`downgrade_max_history_tokens`、`confidence_threshold`、`simple_max_message_chars`。

消息键（英文原文，前端 7 个语言文件都要补）：
- `Enter a name`
- `{{name}} is a real model name. A virtual model cannot use the name of a real model`
- `A virtual model named {{name}} already exists`
- `Select a model`
- `Select at least one group`
- `{{model}} has no available channel in group {{group}}`
- `{{model}} has no price yet. Set it in Model Pricing first`
- `{{model}} only supports {{endpoint}}. The other models of this virtual model use {{other}}`
- `Enter a value between {{min}} and {{max}}`
- `Enter an integer of 0 or more`

保存请求策略时整体检查失败 → 按现有 `common.ApiErrorT` 写法返回第一条错误，前缀为虚拟模型名，消息键为 `Virtual model {{name}}: {{detail}}`。

## 函数清单
### service/smartroute/validate.go
| 函数名 | 职责 |
|---|---|
| `ValidateRoute` | 对单个虚拟模型逐项检查，返回字段错误列表和推导出的接口集合 |
| `ValidateRoutes` | 对整份配置检查（逐个调用 `ValidateRoute`，再加名称两两不重复），返回第一条错误 |
| `RouteEndpoints` | 推导一个虚拟模型支持的接口集合（运行时 07 也调用它） |

### controller/smart_routing.go
| 函数名 | 职责 |
|---|---|
| `ValidateSmartRoute` | 解析请求体，调用 `ValidateRoute`，按成品格式返回 |

### controller/option.go
| 函数名 | 职责 |
|---|---|
| `UpdateRequestPolicy` | 修改：提交的 options 里含 `smart_routing_setting.routes` 时，先解析并调用 `ValidateRoutes`，失败就返回错误、不保存 |
| `UpdateOption` | 修改：`option.Key` 为 `smart_routing_setting.routes` 时，先调用 `ValidateRoutes`，失败就返回错误、不保存 |

## 协作关系
- `ValidateRoute` 依赖：`operation_setting.SmartRouteNames`（重名检查）；`model.GetRandomSatisfiedChannel`（渠道检查，`retry=0`，不带过滤条件）；`helper.HasModelBillingConfig`（定价）；`model.GetModelSupportEndpointTypes`（接口）；`model.GetGroupEnabledModels` 或 abilities 查询（判断是否为真实模型名）。
- 前端 12 调用校验接口，11 调用请求策略 PATCH。

## 验证方式
- 测试入口：`smartroute.ValidateRoute`、`smartroute.ValidateRoutes`；`POST /api/option/smart_routing/validate`；`PATCH /api/option/request_policy`。
- 测试输入：用 sqlite 内存库建渠道和 abilities，写法照 `service/channel_select_auto_groups_test.go` 的 `setupChannelSelectAutoGroupsTest`；用 ratio_setting 设置价格；逐个构造每一种错误。
- 预期结果：每种错误各返回一条，`field` 和消息键与成品一致；全部合格时 `errors` 为空，`endpoints` 为三档和顾问接口的交集；PATCH 不合格的配置时返回错误，option 不变。
- [ ] 名称为空、和真实模型重名、和其他虚拟模型重名（编辑时名称等于 `original_name` 不算重名）
- [ ] 模型没选、分组为空、分组里有 `auto`
- [ ] 某分组里没有可用渠道、没定价、接口没有交集（打分模型只支持 Chat Completions 时不报错）
- [ ] 五个数字各取边界值和越界值
- [ ] 通过通用的 `PUT /api/option/` 提交 `after_dissatisfied` 为 -1 的配置 → 返回错误，option 不变
- [ ] 未登记 access token 路由时 `go test ./router/ ./middleware/` 会失败；登记后通过
