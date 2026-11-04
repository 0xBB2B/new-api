---
name: 01-route-config
description: 智能路由配置的数据结构、注册、读取函数，以及把配置键加进请求策略白名单
---
# 智能路由配置

## 目标
new-api 能保存、热加载、读取"虚拟模型 → 三档 / 打分 / 顾问"配置，并能经请求策略接口（`PATCH /api/option/request_policy`）读写。

## 业务规则（来源：PRD 01-virtual-model-routing、05-config-page）
- 每个虚拟模型的配置都写全：名称、三档（simple / normal / hard，每档有模型、分组顺序、思考力度）、默认档、打分模型（模型、分组顺序、超时毫秒）、顾问（模型、分组顺序、连续不满意几次后交给顾问）、降档时历史 token 上限、可信度门槛、简单题消息长度上限。不设全局默认层。
- 默认值：思考力度为空，表示"不覆盖"；默认档 normal；超时 2000；连续不满意次数 2；降档上限 20000；可信度门槛 0.6；简单题消息长度上限 2000 字。
- 管理员保存后，新请求立即按新配置路由，不用重启（PRD 01 C11）。
- 接口类型不存进配置，运行时按模型推导（由 02、07 负责）。

## 涉及文件
- setting/operation_setting/smart_routing_setting.go（新建）
- model/request_policy.go（修改：`IsRequestPolicyOption` 只放行 `smart_routing_setting.routes` 这一个键；请求策略默认值里补上本配置的当前值）
- model/option.go（修改：`handleConfigUpdate` 遇到这个键时调用 `LoadSmartRoutesFromJSONString`）

## 成品定义
配置在 option 表里存成一个键：`smart_routing_setting.routes`（常量 `SmartRoutingRoutesOptionKey`），值是下面这种 JSON 数组的字符串。内存里用 `atomic.Pointer[[]SmartRoute]` 保存，每次更新都解码出一份新的数组再整体替换，读的一方拿到的数组永远不会被改写；JSON 解析失败时保留旧值并用 `common.SysError` 记录。

```json
[
  {
    "name": "auto-gpt",
    "tiers": {
      "simple": { "model": "gpt-5.4-mini", "groups": ["api"], "effort": "low" },
      "normal": { "model": "gpt-5.5", "groups": ["subscription", "api"], "effort": "" },
      "hard":   { "model": "gpt-5.6-sol", "groups": ["subscription", "api"], "effort": "high" }
    },
    "default_tier": "normal",
    "scorer":  { "model": "gpt-5.6-luna", "groups": ["subscription", "api"], "timeout_ms": 2000 },
    "advisor": { "model": "gpt-5.6-sol", "groups": ["subscription", "api"], "after_dissatisfied": 2 },
    "downgrade_max_history_tokens": 20000,
    "confidence_threshold": 0.6,
    "simple_max_message_chars": 2000
  }
]
```

Go 结构（字段名与 json 标签固定，exec 原样使用）：

```go
type SmartRouteTier struct {
	Model  string   `json:"model"`
	Groups []string `json:"groups"`
	Effort string   `json:"effort"` // "" 表示不覆盖客户端的值；否则为 low / medium / high
}

type SmartRouteScorer struct {
	Model     string   `json:"model"`
	Groups    []string `json:"groups"`
	TimeoutMs int      `json:"timeout_ms"`
}

type SmartRouteAdvisor struct {
	Model             string   `json:"model"`
	Groups            []string `json:"groups"`
	AfterDissatisfied int      `json:"after_dissatisfied"`
}

type SmartRoute struct {
	Name                      string                    `json:"name"`
	Tiers                     map[string]SmartRouteTier `json:"tiers"` // 键只允许 simple / normal / hard
	DefaultTier               string                    `json:"default_tier"`
	Scorer                    SmartRouteScorer          `json:"scorer"`
	Advisor                   SmartRouteAdvisor         `json:"advisor"`
	DowngradeMaxHistoryTokens int                       `json:"downgrade_max_history_tokens"`
	ConfidenceThreshold       float64                   `json:"confidence_threshold"`
	SimpleMaxMessageChars     int                       `json:"simple_max_message_chars"`
}

type SmartRoutingSetting struct {
	Routes []SmartRoute `json:"routes"`
}
```

档位常量：`SmartTierSimple = "simple"`、`SmartTierNormal = "normal"`、`SmartTierHard = "hard"`，排序从低到高依次是 simple、normal、hard。

## 函数清单
### setting/operation_setting/smart_routing_setting.go
| 函数名 | 职责 |
|---|---|
| `init` | 把空数组存进原子指针 |
| `LoadSmartRoutesFromJSONString` | 解码一份新数组并整体替换；失败时保留旧值并记录错误 |
| `GetSmartRoutingSetting` | 返回当前配置（只读用） |
| `FindSmartRoute` | 按虚拟模型名查找配置，找不到返回 false；名称比较区分大小写 |
| `SmartRouteNames` | 返回所有虚拟模型名，供模型列表和重名检查使用 |
| `SmartTierRank` | 返回档位的高低顺序，供升降档比较 |
| `SmartRoutingRoutesToString` | 把当前配置序列化成存进 option 的字符串，供请求策略默认值使用 |

### model/request_policy.go
| 函数名 | 职责 |
|---|---|
| `IsRequestPolicyOption` | 新增：`smart_routing_setting.routes` 返回 true，同前缀的其他键返回 false |
| （默认值构造函数，即写 `defaults[...]` 的那个函数） | 补 `defaults["smart_routing_setting.routes"] = operation_setting.SmartRoutingRoutesToString()` |

## 协作关系
- 保存：前端 `PATCH /api/option/request_policy` → `IsRequestPolicyOption` 放行 → 现有 option 保存流程 → `model/option.go` 的 `handleConfigUpdate` → `LoadSmartRoutesFromJSONString` 整体替换。启动加载和每 60 秒的定时同步走同一个入口。
- 读取方：02 校验、06 打分、07 路由、10 模型列表都通过 `FindSmartRoute` / `GetSmartRoutingSetting` 读取。
- 并发：靠原子指针整体替换保证读写安全，读取方不加锁。

## 验证方式
- 测试入口：`model.IsRequestPolicyOption`；`PATCH /api/option/request_policy`、`GET /api/option/request_policy`；`operation_setting.FindSmartRoute`。
- 测试输入：上面成品里的 JSON，作为 `smart_routing_setting.routes` 的值。
- 预期结果：PATCH 成功；随后 GET 返回的 options 里有这个键，值和提交的一致；`FindSmartRoute("auto-gpt")` 返回的配置中困难档模型为 gpt-5.6-sol；`FindSmartRoute("Auto-GPT")` 返回 false。
- [ ] `IsRequestPolicyOption("smart_routing_setting.routes")` 为 true，`IsRequestPolicyOption("smart_routing_x")` 为 false
- [ ] 保存后无需重启，`FindSmartRoute` 立即读到新值
- [ ] 测试写在现有的 model/request_policy 相关测试文件里（如有），没有就合并进 02 的测试文件
