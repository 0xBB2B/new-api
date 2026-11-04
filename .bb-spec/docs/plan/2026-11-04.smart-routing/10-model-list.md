---
name: 10-model-list
description: /v1/models 返回的列表里加上已配置的虚拟模型名，令牌白名单照常过滤
---
# 模型列表里显示虚拟模型

## 目标
客户端读 `/v1/models` 时能看到 auto-gpt 这类虚拟模型名，直接选用。

## 业务规则（来源：PRD 01-virtual-model-routing C9，OVERVIEW 决策记录）
- 列表里包含所有已配置的虚拟模型名。
- 令牌设了模型白名单时，虚拟模型和普通模型一样按白名单过滤：白名单里没有虚拟模型名，就不显示它。
- 虚拟模型本身没有价格，跳过"未定价模型不显示"的那条过滤（`billing_setting.HasModelBillingConfig`）。
- 和真实模型重名的情况在保存配置时已经被拒绝（02），这里不再去重。
- 价格页不显示虚拟模型（不改价格页相关接口）。
- 每个虚拟模型条目的 `supported_endpoint_types` 填 02 推导出的接口集合（`smartroute.RouteEndpoints`）。

## 涉及文件
- controller/model.go（修改：`ListModels`，`:218` 起，在 `:246-257` 的过滤循环之后追加虚拟模型）

## 函数清单
### controller/model.go
| 函数名 | 职责 |
|---|---|
| `ListModels` | 修改：生成普通模型列表后，按白名单追加虚拟模型条目 |
| `buildSmartRouteModels` | 新增：把虚拟模型名转成与 `buildOpenAIModel` 同结构的条目，接口类型取自 `smartroute.RouteEndpoints` |

## 协作关系
- 依赖 01 的 `operation_setting.SmartRouteNames` / `FindSmartRoute`，以及 02 的 `smartroute.RouteEndpoints`。

## 验证方式
- 测试入口：`GET /v1/models`（扩展 controller 包现有的模型列表测试；没有就合并进 middleware/distributor_test.go 旁边已有的 controller 测试文件，不新建文件）。
- 测试输入：配置 auto-gpt；令牌 A 不设白名单，令牌 B 白名单只有 gpt-5.5，令牌 C 白名单含 auto-gpt。
- 预期结果：
- [ ] 令牌 A、C 的结果里有 auto-gpt，`supported_endpoint_types` 为 `["openai-response"]`（按测试数据推导）
- [ ] 令牌 B 的结果里没有 auto-gpt
- [ ] 没有配置虚拟模型时，结果与改动前一致
