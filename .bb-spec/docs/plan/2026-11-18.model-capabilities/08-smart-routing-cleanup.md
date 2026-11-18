---
name: 08-smart-routing-cleanup
description: 删除智能路由设置页「模型思考能力」卡片，编辑弹窗只按后端有效能力判断，改「能力未确认」提示文案
---
# 智能路由前端清理

## 目标
智能路由设置页只剩「虚拟模型」卡片；虚拟模型编辑弹窗的档位选项、超出能力警告、「能力未确认」徽标都按后端返回的有效能力判断。

## 业务规则（来源：PRD 04-single-source-and-migration C8、C9）
- 系统设置 → 请求策略 → 智能路由 页面删掉「模型思考能力」卡片，「虚拟模型」卡片本身不变（正常、空数据、加载中、出错状态不变）。
- 前端不读写选项 `smart_routing_setting.thinking_capabilities`。
- 能力判断只用 GET `/api/option/smart_routing/thinking_capabilities?models=...` 的返回（后端已按「模型记录 → Claude/Gemini 推算 → 未确认」取值，`known=false` 即能力未确认）。
- 所选模型能力未确认时仍显示黄色徽标「能力未确认」；提示气泡文案改为「这个模型还没有配置思考等级，可在「模型管理」里编辑这个模型，在「模型能力」里补充」；能力未确认按 low、medium、high 处理（不变）。
- 删掉卡片专用的文案 key（7 个 locale 同步删），新提示文案 7 个 locale 都补齐。

## 涉及文件
- 删除 `web/src/features/system-settings/request-policies/smart-routing/capabilities-card.tsx`
- 修改 `web/src/features/system-settings/request-policies/smart-routing-section.tsx`（去掉卡片引用 :27、:97 与 `saveCapabilities` :55-62，去掉 overrides 传递）
- 修改 `web/src/features/system-settings/request-policies/smart-routing/capability.ts`（`effectiveCapability` 去掉 overrides 参数与 `'override'` 来源，来源只剩 已确认 / 未确认）
- 修改 `web/src/features/system-settings/request-policies/smart-routing/lib.ts`（删 `parseThinkingCapabilities` 与该选项键）
- 修改 `web/src/features/system-settings/request-policies/smart-routing/routes-card.tsx`（去掉 overrides 入参 :64-65、:171、:209、:278、:361）
- 修改 `web/src/features/system-settings/request-policies/smart-routing/route-editor-dialog.tsx`（:106-107、:153-187、:207-216 去掉 overrides）
- 修改 `web/src/features/system-settings/request-policies/smart-routing/tier-effort-field.tsx`（:95-97 提示文案 key 换成新文案）
- 修改 `web/src/i18n/locales/*.json`（删卡片文案、删旧提示 key、加新提示 key）
- 修改测试 `smart-routing/__tests__/route-model.test.ts`（:216 选项解析、:358 effectiveCapability 用例）、`request-policies/__tests__/smart-routing.test.tsx`（删卡片相关 :2012-2440 与卡片文案 locale 测试 :2460-2490；徽标提示断言 :1087-1126、:2130-2160、:2896-2924、:2944、:3165 改为新文案）

## 函数清单
### smart-routing/capability.ts
| 函数名 | 职责 |
|---|---|
| `effectiveCapability`（改） | 只按接口返回的 `{known, no_thinking, efforts}` 生成有效能力，`known=false` 时为未确认（low/medium/high） |
| `clampEffort` / `effortOverflow` / `capabilityModelsOfRoutes`（不改行为） | 档位收敛、超出能力判断、收集要查询的模型 |

### smart-routing/tier-effort-field.tsx
| 函数名 | 职责 |
|---|---|
| `EffortCapabilityStatus`（改文案） | 徽标「能力未确认」与新提示气泡 |

## 协作关系
- 依赖 03：后端接口返回有效能力；选项键已从后端删除。
- 编辑弹窗与设置页的 query key `['smart-routing','builtin-capabilities', models]` 保留，数据含义变为有效能力（可改名为 `capabilities`，同时改两处引用）。
- 不影响虚拟模型卡片的保存流程（只写路由选项）。

## 验证方式
- 测试入口：渲染智能路由设置页与虚拟模型编辑弹窗（mock 能力接口），`route-model.test.ts` 纯函数用例。
- 测试输入：能力接口返回 `gpt-6.2-sol` known 且 none~max、`gpt-4o` known 且 no_thinking、`foo-model` known=false。
- 预期结果：
- [ ] 设置页只显示「虚拟模型」卡片，无「模型思考能力」卡片
- [ ] 页面不请求、不保存 `smart_routing_setting.thinking_capabilities`
- [ ] 选 `foo-model` 时显示徽标「能力未确认」，聚焦提示为新文案；档位选项为 low/medium/high
- [ ] 选 `gpt-6.2-sol` 不显示徽标，档位选项 none~max；选 `gpt-4o` 按不支持思考处理
- [ ] 7 个 locale 无卡片旧 key、有新提示 key
- [ ] `bun run typecheck`、`bun run lint`、`bun run test` 通过
