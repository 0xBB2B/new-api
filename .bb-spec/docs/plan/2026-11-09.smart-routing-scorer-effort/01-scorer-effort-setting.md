---
name: 01-scorer-effort-setting
description: 打分配置新增思考力度字段，保存校验只接受留空或七档之一
---
# 打分思考力度配置与校验

## 目标
虚拟模型的打分配置能存一个思考力度；保存接口拒绝留空和七档以外的值。

## 业务规则（来源：PRD）
- 来源 `01-scorer-effort`：
  - C7：保存时打分思考力度只能是留空或 none、minimal、low、medium、high、xhigh、max 之一，其他值 → 保存被拦下，错误字段为 `scorer.effort`，提示键 `Select a valid reasoning effort`（前端已有翻译「请选择有效的思考力度」）。
  - C10：已有配置里没有这一项时按留空处理（JSON 里缺字段 → 空字符串）。
- 校验不看模型能力：模型不支持选中的档位时，由请求发出前换档处理（另一份 plan），保存不拦。

## 涉及文件
- 修改 `setting/operation_setting/smart_routing_setting.go`
- 修改 `service/smartroute/validate.go`
- 修改 `service/smartroute/smartroute_test.go`（扩展已有的校验测试表）

## 成品定义
`SmartRouteScorer` 结构体最终形态：

```go
type SmartRouteScorer struct {
	Model     string   `json:"model"`
	Groups    []string `json:"groups"`
	TimeoutMs int      `json:"timeout_ms"`
	Effort    string   `json:"effort"` // 空串表示不发思考参数，由模型自己决定
}
```

配置 JSON 中打分部分示例（前后端契约）：

```json
{ "scorer": { "model": "deepseek-v4.1-flash", "groups": ["api"], "timeout_ms": 5000, "effort": "none" } }
```

## 函数清单
### service/smartroute/validate.go
| 函数名 | 职责 |
|---|---|
| `ValidateRoute`（修改） | 在超时等数值校验之后加一段：`route.Scorer.Effort` 非空且 `operation_setting.SmartEffortRank` 返回负数时，向 `scorer.effort` 报 `Select a valid reasoning effort` |

## 协作关系
- 复用 `operation_setting.SmartEffortRank` 判断是否七档之一，不新增档位列表。
- 前端保存时调用的校验接口（`validateSmartRoute`）把返回的字段错误挂到表单对应字段，`scorer.` 开头的字段自动定位到「打分」页签（`error-location.ts` 已支持，不改）。

## 验证方式
- 测试入口：`smartroute.ValidateRoute`（`smartroute_test.go` 里已有调用它的表驱动测试）。
- 测试输入：一份其他字段都合法的路由，`Scorer.Effort` 分别为 `""`、`"none"`、`"max"`、`"ultra"`、`"LOW"`。
- 预期结果：前三个没有错误；`"ultra"` 和 `"LOW"` 各返回一条错误，字段 `scorer.effort`，消息键 `Select a valid reasoning effort`。
- [ ] 旧配置 JSON（没有 `effort`）用 `common.UnmarshalJsonStr` 解析成 `SmartRoute` 后，`Scorer.Effort` 为空字符串，校验通过。
- [ ] `go test ./service/smartroute/ ./setting/operation_setting/` 通过。
