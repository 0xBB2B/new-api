---
name: 03-capability-scorer-models
description: 「模型思考能力」卡片和内置能力查询把打分模型也算进去
---
# 能力表列出打分模型

## 目标
「模型思考能力」卡片列出档位和打分用到的模型，管理员能覆盖打分模型的能力。

## 业务规则（来源：PRD）
- 来源 `02-capability-scorer-models`：
  - C1：表格列出所有虚拟模型三个档位和打分用到的模型，同一个模型只列一行；顾问模型不因为是顾问而列出。标题旁的数字等于表格行数。
  - C2：打分模型那一行的来源、支持的档位、操作（覆盖 / 修改 / 恢复内置）与档位模型的行完全一样。
- 决策理由：不列的话，内置表里没有的打分模型（如 deepseek-v4.1-flash）只能按「未确认」处理，选不到 none，关不掉思考。顾问没有思考力度设置，所以不列。

## 涉及文件
- 修改 `web/src/features/system-settings/request-policies/smart-routing/capability.ts`
- 修改 `web/src/features/system-settings/request-policies/smart-routing/capabilities-card.tsx`
- 修改 `web/src/features/system-settings/request-policies/smart-routing-section.tsx`
- 修改 `web/src/features/system-settings/request-policies/smart-routing/__tests__/route-model.test.ts`
- 修改 `web/src/features/system-settings/request-policies/__tests__/smart-routing.test.tsx`

## 函数清单
### web/src/features/system-settings/request-policies/smart-routing/capability.ts
| 函数名 | 职责 |
|---|---|
| `capabilityModelsOfRoutes`（由 `tierModelsOfRoutes` 改名） | 收集所有路由三个档位和打分的模型名，去空、去重；不含顾问模型 |

### capabilities-card.tsx / smart-routing-section.tsx
| 函数名 | 职责 |
|---|---|
| `ThinkingCapabilitiesCard`（修改） | 行数据改用 `capabilityModelsOfRoutes` |
| `SmartRoutingSection`（修改） | 内置能力查询的模型列表改用 `capabilityModelsOfRoutes`，让打分模型的内置能力也被查回 |

## 协作关系
- 内置能力接口 `getBuiltinThinkingCapabilities(models)` 按传入的模型列表返回，后端不用改。
- 改名后删除旧名字的导出，所有调用方一起改，不留别名。
- 卡片空状态文案「No model is used by a virtual model yet」不改。

## 验证方式
- 测试入口：`capability.ts` 导出的 `capabilityModelsOfRoutes`；系统设置页「智能路由」区块的渲染测试（`smart-routing.test.tsx` 现有的 `setCapabilityOptions()` / `show()` / `capabilityTable()` / `capabilityRow()` 夹具）。
- 测试输入与预期结果：
  - [ ] 两条路由：A 的三档为 m1、m2、m3，打分 s1，顾问 adv；B 的简单档为 s1，打分为空 → 返回的模型集合为 m1、m2、m3、s1，各一次，不含 adv。
  - [ ] 现有用例「the capability card counts the tier models once each and leaves out scorer and advisor models」改为：打分模型出现在表格里且只出现一次，标题数字等于表格行数，顾问模型不出现。
  - [ ] 打分模型是不在内置表里的模型 → 该行来源「Unconfirmed」，档位 low、medium、high；点「Override」保存后来源「Overridden」，档位为覆盖值；点「Restore built-in」→ 回到「Unconfirmed」。
- [ ] 在 `web/` 下 `bun run test` 相关文件通过，`bun run typecheck` 通过。
